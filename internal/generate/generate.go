// Package generate pulls live AIC journeys/scripts and emits reviewable HCL.
//
// It refuses to dump raw JSON: every node field must be in the typed catalog,
// every tree key must be modelled, and defaults are omitted so the files stay
// small enough to review.
package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/nodetype"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

type Options struct {
	Realm    string
	OutDir   string
	Prefix   string    // stripped from existing names if present; not written into HCL
	Journeys []string  // empty = all
	Progress io.Writer // optional human-readable progress; diagnostics belong on stderr
}

type Result struct {
	Journeys int
	Scripts  int
	Nodes    int
	Files    []string
}

func Run(ctx context.Context, c *client.Client, opt Options) (*Result, error) {
	if opt.Realm == "" {
		opt.Realm = "alpha"
	}
	if opt.OutDir == "" {
		opt.OutDir = "generated"
	}
	if err := os.MkdirAll(filepath.Join(opt.OutDir, "scripts"), 0o755); err != nil {
		return nil, err
	}

	names, err := c.ListTrees(ctx, opt.Realm)
	if err != nil {
		return nil, err
	}
	if len(opt.Journeys) > 0 {
		names, err = selectJourneys(names, opt.Journeys)
		if err != nil {
			return nil, err
		}
	}
	progressf(opt, "Found %d journey(s) in realm %s", len(names), opt.Realm)

	g := &gen{
		c:            c,
		opt:          opt,
		scripts:      map[string]*client.Script{},
		scriptLabels: map[string]string{},
		nodes:        map[string]emittedNode{},
		addr:         map[string]string{},
	}

	for i, name := range names {
		progressf(opt, "[%d/%d] Reading journey %q", i+1, len(names), name)
		if err := g.ingestJourney(ctx, name); err != nil {
			return nil, fmt.Errorf("journey %s: %w", name, err)
		}
	}

	// Scripts referenced by ingested nodes.
	for _, n := range g.nodes {
		if n.Spec.APIType == "ScriptedDecisionNode" {
			if sid, _ := n.Values["script_id"].(string); sid != "" {
				progressf(opt, "Reading script %s", sid)
				if err := g.ingestScript(ctx, sid); err != nil {
					return nil, fmt.Errorf("script %s (from node %s): %w", sid, n.ID, err)
				}
			}
		}
	}

	res := &Result{Journeys: len(g.journeys), Scripts: len(g.scripts), Nodes: len(g.nodes)}
	progressf(opt, "Writing %d journey(s), %d script(s), and %d node(s) to %s", res.Journeys, res.Scripts, res.Nodes, opt.OutDir)
	if err := cleanGeneratedFiles(opt.OutDir); err != nil {
		return nil, err
	}

	if err := g.writeProvider(); err != nil {
		return nil, err
	}
	if err := g.writeScripts(); err != nil {
		return nil, err
	}
	if err := g.writeJourneys(); err != nil {
		return nil, err
	}
	res.Files = g.files
	return res, nil
}

func progressf(opt Options, format string, args ...any) {
	if opt.Progress != nil {
		fmt.Fprintf(opt.Progress, format+"\n", args...)
	}
}

func selectJourneys(available, requested []string) ([]string, error) {
	want := make(map[string]struct{}, len(requested))
	for _, name := range requested {
		want[name] = struct{}{}
	}
	found := make(map[string]struct{}, len(requested))
	selected := make([]string, 0, len(requested))
	for _, name := range available {
		if _, ok := want[name]; ok {
			selected = append(selected, name)
			found[name] = struct{}{}
		}
	}
	missing := make([]string, 0)
	for name := range want {
		if _, ok := found[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("requested journeys not found in realm: %s", strings.Join(missing, ", "))
	}
	return selected, nil
}

func cleanGeneratedFiles(outDir string) error {
	patterns := []string{
		filepath.Join(outDir, "journey_*.tf"),
		filepath.Join(outDir, "scripts", "*.js"),
	}
	paths := []string{
		filepath.Join(outDir, "provider.tf"),
		filepath.Join(outDir, "scripts.tf"),
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("match old generated files %q: %w", pattern, err)
		}
		paths = append(paths, matches...)
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove old generated file %q: %w", path, err)
		}
	}
	return nil
}

type gen struct {
	c            *client.Client
	opt          Options
	scripts      map[string]*client.Script
	scriptLabels map[string]string
	nodes        map[string]emittedNode
	journeys     []emittedJourney
	addr         map[string]string
	files        []string
	used         map[string]int
}

type emittedNode struct {
	ID     string
	Spec   nodetype.Spec
	Values map[string]any
	Label  string
}

type emittedJourney struct {
	Name               string
	Description        string
	Enabled            bool
	Identity           string
	Inner              bool
	MustRun            bool
	NoSession          bool
	Transactional      bool
	MaximumIdleTime    *int64
	MaximumSessionTime *int64
	TreeTimeout        *int64
	Entry              string
	Categories         []string
	Nodes              []emittedTreeNode
}

type emittedTreeNode struct {
	ID          string
	Type        string
	DisplayName string
	Version     string
	Connections map[string]string
}

func (g *gen) ingestJourney(ctx context.Context, name string) error {
	raw, err := g.c.GetTree(ctx, g.opt.Realm, name)
	if err != nil {
		return err
	}
	if _, err := client.TreeWriteBody(raw); err != nil {
		return err
	}
	if err := client.ValidateTreeInternals(raw); err != nil {
		return err
	}

	j := emittedJourney{
		Name:               prefix.Strip(g.opt.Prefix, name),
		Description:        str(raw["description"]),
		Enabled:            boolDef(raw["enabled"], true),
		Identity:           str(raw["identityResource"]),
		Inner:              boolDef(raw["innerTreeOnly"], false),
		MustRun:            boolDef(raw["mustRun"], false),
		NoSession:          boolDef(raw["noSession"], false),
		Transactional:      boolDef(raw["transactionalOnly"], false),
		MaximumIdleTime:    optionalInt64(raw["maximumIdleTime"]),
		MaximumSessionTime: optionalInt64(raw["maximumSessionTime"]),
		TreeTimeout:        optionalInt64(raw["treeTimeout"]),
		Entry:              str(raw["entryNodeId"]),
	}
	if ui, ok := raw["uiConfig"].(map[string]any); ok {
		if cs := str(ui["categories"]); cs != "" && cs != "[]" {
			// stored as a JSON string
			var cats []string
			if err := jsonUnmarshal(cs, &cats); err != nil {
				return fmt.Errorf("uiConfig.categories: %w", err)
			}
			j.Categories = cats
		}
	}

	rawNodes, _ := raw["nodes"].(map[string]any)
	ids := make([]string, 0, len(rawNodes))
	for id := range rawNodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		meta, _ := rawNodes[id].(map[string]any)
		nt := str(meta["nodeType"])
		if nt == "" {
			return fmt.Errorf("tree node %s has no nodeType", id)
		}
		if err := g.ingestNode(ctx, id, nt, str(meta["displayName"])); err != nil {
			return err
		}
		conns := map[string]string{}
		if c, ok := meta["connections"].(map[string]any); ok {
			for k, v := range c {
				conns[k] = displayConn(str(v))
			}
		}
		j.Nodes = append(j.Nodes, emittedTreeNode{
			ID:          id,
			Type:        nt,
			DisplayName: str(meta["displayName"]),
			Version:     str(meta["version"]),
			Connections: conns,
		})
	}
	g.journeys = append(g.journeys, j)
	return nil
}

func (g *gen) ingestNode(ctx context.Context, id, apiType, hint string) error {
	if _, ok := g.nodes[id]; ok {
		return nil
	}
	spec, ok := nodetype.Lookup(apiType)
	if !ok {
		return fmt.Errorf("node %s has unmodelled type %s — add it to internal/nodetype/catalog.go", id, apiType)
	}
	progressf(g.opt, "Reading node %s/%s", apiType, id)
	raw, err := g.c.GetNode(ctx, g.opt.Realm, apiType, id)
	if err != nil {
		return fmt.Errorf("GET %s/%s: %w", apiType, id, err)
	}
	vals, err := nodetype.DecodeAPI(spec, raw, g.opt.Prefix)
	if err != nil {
		return err
	}
	if hint == "" {
		hint = displayHint(vals, apiType, id)
	}
	label := g.uniqueLabel(spec.TFResource, hint)
	g.nodes[id] = emittedNode{ID: id, Spec: spec, Values: vals, Label: label}
	g.addr[id] = fmt.Sprintf("pingoneaic_%s.%s.id", spec.TFResource, label)

	// Page children are first-class node resources.
	if kids, ok := vals["page_nodes"].([]nodetype.PageChild); ok {
		for _, kid := range kids {
			if err := g.ingestNode(ctx, kid.ID, kid.NodeType, kid.DisplayName); err != nil {
				return fmt.Errorf("page child %s: %w", kid.ID, err)
			}
		}
	}
	return nil
}

func (g *gen) ingestScript(ctx context.Context, id string) error {
	if _, ok := g.scripts[id]; ok {
		return nil
	}
	s, err := g.c.GetScript(ctx, g.opt.Realm, id)
	if err != nil {
		return err
	}
	s.Name = prefix.Strip(g.opt.Prefix, s.Name)
	g.scripts[id] = s
	label := g.uniqueLabel("script", s.Name)
	g.scriptLabels[id] = label
	g.addr[id] = fmt.Sprintf("pingoneaic_script.%s.id", label)
	return nil
}

func (g *gen) scriptLabel(s *client.Script) string {
	for id, sc := range g.scripts {
		if sc == s {
			if l, ok := g.scriptLabels[id]; ok {
				return l
			}
		}
	}
	return g.uniqueLabel("script", s.Name)
}

func displayHint(vals map[string]any, apiType, id string) string {
	if s, ok := vals["tree"].(string); ok && s != "" {
		return s
	}
	return strings.TrimSuffix(apiType, "Node")
}

func (g *gen) uniqueLabel(kind, hint string) string {
	if g.used == nil {
		g.used = map[string]int{}
	}
	base := sanitizeIdent(hint)
	if base == "" {
		base = sanitizeIdent(kind)
	}
	// Uniqueness is per Terraform resource type, so a script and a node
	// can both be called get_ip.
	key := kind + "/" + base
	n := g.used[key]
	g.used[key] = n + 1
	if n == 0 {
		return base
	}
	return fmt.Sprintf("%s_%d", base, n+1)
}

var identRe = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func sanitizeIdent(s string) string {
	s = strings.TrimSpace(s)
	s = identRe.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	s = strings.ToLower(s)
	if s == "" {
		return "unnamed"
	}
	if s[0] >= '0' && s[0] <= '9' {
		s = "n_" + s
	}
	return s
}

func (g *gen) writeProvider() error {
	path := filepath.Join(g.opt.OutDir, "provider.tf")
	body := `terraform {
  required_providers {
    pingoneaic = {
      source  = "agiledigital-labs/pingone-aic"
      version = ">= 0.1.0"
    }
  }
}

provider "pingoneaic" {
  # tenant_url / credentials come from PINGONEAIC_* env vars.
  # resource_prefix defaults to "Terraform_" so applying this directory
  # creates copies instead of overwriting the journeys we pulled.
}
`
	if err := writeTerraformFile(path, []byte(body)); err != nil {
		return err
	}
	g.files = append(g.files, path)
	return nil
}

func (g *gen) writeScripts() error {
	if len(g.scripts) == 0 {
		return nil
	}
	ids := make([]string, 0, len(g.scripts))
	for id := range g.scripts {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return g.scripts[ids[i]].Name < g.scripts[ids[j]].Name })

	var b strings.Builder
	b.WriteString("# Generated AM scripts. Bodies live in scripts/*.js so reviews see the JS, not base64.\n\n")
	for _, id := range ids {
		s := g.scripts[id]
		label := g.scriptLabel(s)
		rel := filepath.Join("scripts", label+".js")
		abs := filepath.Join(g.opt.OutDir, rel)
		if err := os.WriteFile(abs, []byte(s.Source), 0o644); err != nil {
			return err
		}
		g.files = append(g.files, abs)

		b.WriteString(fmt.Sprintf("resource \"pingoneaic_script\" %q {\n", label))
		b.WriteString(fmt.Sprintf("  realm  = %s\n", hclString(g.opt.Realm)))
		b.WriteString(fmt.Sprintf("  name   = %s\n", hclString(s.Name)))
		b.WriteString(fmt.Sprintf("  context = %s\n", hclString(s.Context)))
		if s.Language != "" && s.Language != "JAVASCRIPT" {
			b.WriteString(fmt.Sprintf("  language = %s\n", hclString(s.Language)))
		}
		if s.EvaluatorVersion != "" && s.EvaluatorVersion != "2.0" {
			b.WriteString(fmt.Sprintf("  evaluator_version = %s\n", hclString(s.EvaluatorVersion)))
		}
		if s.Description != "" {
			b.WriteString(fmt.Sprintf("  description = %s\n", hclString(s.Description)))
		}
		b.WriteString(fmt.Sprintf("  source = file(%s)\n", hclString(rel)))
		b.WriteString("}\n\n")
	}
	path := filepath.Join(g.opt.OutDir, "scripts.tf")
	if err := writeTerraformFile(path, []byte(b.String())); err != nil {
		return err
	}
	g.files = append(g.files, path)
	return nil
}

func (g *gen) writeJourneys() error {
	for _, j := range g.journeys {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("# Journey %s\n\n", j.Name))

		// Emit the nodes this journey owns (and page children) before the tree.
		seen := map[string]struct{}{}
		var emitNode func(id string) error
		emitNode = func(id string) error {
			if _, ok := seen[id]; ok {
				return nil
			}
			n, ok := g.nodes[id]
			if !ok {
				return fmt.Errorf("missing node %s", id)
			}
			seen[id] = struct{}{}
			if kids, ok := n.Values["page_nodes"].([]nodetype.PageChild); ok {
				for _, kid := range kids {
					if err := emitNode(kid.ID); err != nil {
						return err
					}
				}
			}
			g.writeNode(&b, n)
			return nil
		}
		for _, tn := range j.Nodes {
			if err := emitNode(tn.ID); err != nil {
				return err
			}
		}

		b.WriteString(fmt.Sprintf("resource \"pingoneaic_journey\" %q {\n", sanitizeIdent(j.Name)))
		b.WriteString(fmt.Sprintf("  realm = %s\n", hclString(g.opt.Realm)))
		b.WriteString(fmt.Sprintf("  name  = %s\n", hclString(j.Name)))
		if j.Description != "" {
			b.WriteString(fmt.Sprintf("  description = %s\n", hclString(j.Description)))
		}
		if !j.Enabled {
			b.WriteString("  enabled = false\n")
		}
		defIdentity := "managed/" + g.opt.Realm + "_user"
		if j.Identity != "" && j.Identity != defIdentity {
			b.WriteString(fmt.Sprintf("  identity_resource = %s\n", hclString(j.Identity)))
		}
		if j.Inner {
			b.WriteString("  inner_tree_only = true\n")
		}
		if j.MustRun {
			b.WriteString("  must_run = true\n")
		}
		if j.NoSession {
			b.WriteString("  no_session = true\n")
		}
		if j.Transactional {
			b.WriteString("  transactional_only = true\n")
		}
		for _, setting := range []struct {
			name  string
			value *int64
		}{
			{"maximum_idle_time", j.MaximumIdleTime},
			{"maximum_session_time", j.MaximumSessionTime},
			{"tree_timeout", j.TreeTimeout},
		} {
			if setting.value != nil {
				b.WriteString(fmt.Sprintf("  %s = %d\n", setting.name, *setting.value))
			}
		}
		if len(j.Categories) > 0 {
			b.WriteString(fmt.Sprintf("  categories = %s\n", hclStringList(j.Categories)))
		}
		b.WriteString(fmt.Sprintf("  entry_node = %s\n", g.refOrLit(j.Entry)))
		b.WriteByte('\n')
		for _, tn := range j.Nodes {
			b.WriteString("  node {\n")
			b.WriteString(fmt.Sprintf("    id   = %s\n", g.refOrLit(tn.ID)))
			b.WriteString(fmt.Sprintf("    type = %s\n", hclString(tn.Type)))
			if tn.DisplayName != "" {
				b.WriteString(fmt.Sprintf("    display_name = %s\n", hclString(tn.DisplayName)))
			}
			if tn.Version != "" && tn.Version != "1.0" {
				b.WriteString(fmt.Sprintf("    version = %s\n", hclString(tn.Version)))
			}
			b.WriteString("    connections = {\n")
			keys := make([]string, 0, len(tn.Connections))
			for k := range tn.Connections {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				b.WriteString(fmt.Sprintf("      %s = %s\n", hclIdentOrQuote(k), g.connRef(tn.Connections[k])))
			}
			b.WriteString("    }\n")
			b.WriteString("  }\n")
		}
		b.WriteString("}\n")

		path := filepath.Join(g.opt.OutDir, "journey_"+sanitizeIdent(j.Name)+".tf")
		if err := writeTerraformFile(path, []byte(b.String())); err != nil {
			return err
		}
		g.files = append(g.files, path)
	}
	return nil
}

func writeTerraformFile(path string, body []byte) error {
	return os.WriteFile(path, hclwrite.Format(body), 0o644)
}

func (g *gen) writeNode(b *strings.Builder, n emittedNode) {
	b.WriteString(fmt.Sprintf("resource \"pingoneaic_%s\" %q {\n", n.Spec.TFResource, n.Label))
	b.WriteString(fmt.Sprintf("  realm = %s\n", hclString(g.opt.Realm)))
	for _, f := range n.Spec.Fields {
		v, ok := n.Values[f.TFName]
		if !ok {
			continue
		}
		if nodetype.EqualDefault(f, v) {
			continue
		}
		switch f.Kind {
		case nodetype.KindString, nodetype.KindESVString:
			s, _ := v.(string)
			if s == "" {
				continue
			}
			if f.TFName == "script_id" {
				b.WriteString(fmt.Sprintf("  %s = %s\n", f.TFName, g.refOrLit(s)))
				continue
			}
			b.WriteString(fmt.Sprintf("  %s = %s\n", f.TFName, hclString(s)))
		case nodetype.KindBool:
			b.WriteString(fmt.Sprintf("  %s = %v\n", f.TFName, v))
		case nodetype.KindInt:
			b.WriteString(fmt.Sprintf("  %s = %v\n", f.TFName, v))
		case nodetype.KindStringList:
			items, _ := v.([]string)
			if len(items) == 0 {
				continue
			}
			b.WriteString(fmt.Sprintf("  %s = %s\n", f.TFName, hclStringList(items)))
		case nodetype.KindStringMap:
			m, _ := v.(map[string]string)
			if len(m) == 0 {
				continue
			}
			b.WriteString(fmt.Sprintf("  %s = {\n", f.TFName))
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				b.WriteString(fmt.Sprintf("    %s = %s\n", hclIdentOrQuote(k), hclString(m[k])))
			}
			b.WriteString("  }\n")
		case nodetype.KindChildren:
			kids, _ := v.([]nodetype.PageChild)
			if len(kids) == 0 {
				continue
			}
			b.WriteString("  page_nodes = [\n")
			for _, kid := range kids {
				b.WriteString("    {\n")
				b.WriteString(fmt.Sprintf("      id           = %s\n", g.refOrLit(kid.ID)))
				if kid.DisplayName != "" {
					b.WriteString(fmt.Sprintf("      display_name = %s\n", hclString(kid.DisplayName)))
				}
				b.WriteString(fmt.Sprintf("      node_type    = %s\n", hclString(kid.NodeType)))
				if kid.NodeVersion != "" && kid.NodeVersion != "1.0" {
					b.WriteString(fmt.Sprintf("      node_version = %s\n", hclString(kid.NodeVersion)))
				}
				b.WriteString("    },\n")
			}
			b.WriteString("  ]\n")
		}
	}
	b.WriteString("}\n\n")
}

func (g *gen) refOrLit(id string) string {
	if a, ok := g.addr[id]; ok {
		return a
	}
	return hclString(id)
}

func (g *gen) connRef(dest string) string {
	if dest == "success" || dest == "failure" {
		return hclString(dest)
	}
	return g.refOrLit(dest)
}

func displayConn(dest string) string {
	switch dest {
	case client.SuccessNodeID:
		return "success"
	case client.FailureNodeID:
		return "failure"
	default:
		return dest
	}
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func boolDef(v any, def bool) bool {
	b, ok := v.(bool)
	if !ok {
		return def
	}
	return b
}

func optionalInt64(v any) *int64 {
	var value int64
	switch n := v.(type) {
	case float64:
		value = int64(n)
	case int64:
		value = n
	case int:
		value = int64(n)
	default:
		return nil
	}
	return &value
}

func jsonUnmarshal(s string, dest any) error {
	return json.Unmarshal([]byte(s), dest)
}

func hclString(s string) string {
	return strconv.Quote(s)
}

func hclStringList(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = hclString(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func hclIdentOrQuote(s string) string {
	if identRe.MatchString(s) || s == "" || (s[0] >= '0' && s[0] <= '9') {
		return hclString(s)
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return hclString(s)
		}
	}
	return s
}
