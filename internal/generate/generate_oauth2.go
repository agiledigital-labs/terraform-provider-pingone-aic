package generate

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/oauth2client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
)

type emittedOAuth2Client struct {
	Name   string
	Label  string
	Values oauth2client.Values
}

func (g *gen) ingestOAuth2Clients(ctx context.Context) error {
	ids, err := g.c.ListOAuth2Clients(ctx, g.opt.Realm)
	if err != nil {
		return fmt.Errorf("list oauth2 clients: %w", err)
	}
	sort.Strings(ids)
	progressf(g.opt, "Found %d oauth2 client(s) in realm %s", len(ids), g.opt.Realm)
	for i, id := range ids {
		progressf(g.opt, "[%d/%d] Reading oauth2 client %q", i+1, len(ids), id)
		raw, err := g.c.GetOAuth2Client(ctx, g.opt.Realm, id)
		if err != nil {
			return fmt.Errorf("oauth2 client %s: %w", id, err)
		}
		vals, err := oauth2client.DecodeAPI(raw, g.opt.Prefix)
		if err != nil {
			return fmt.Errorf("oauth2 client %s: %w", id, err)
		}
		name := prefix.Strip(g.opt.Prefix, id)
		g.oauth2 = append(g.oauth2, emittedOAuth2Client{
			Name:   name,
			Label:  g.uniqueLabel("oauth2_client", name),
			Values: vals,
		})
	}
	return nil
}

func (g *gen) writeOAuth2Clients() error {
	if len(g.oauth2) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("# Generated AM OAuth2 clients. Defaults omitted. userpassword is write-only and is never emitted.\n\n")
	for _, c := range g.oauth2 {
		b.WriteString(fmt.Sprintf("resource \"pingoneaic_oauth2_client\" %q {\n", c.Label))
		b.WriteString(fmt.Sprintf("  realm = %s\n", hclString(g.opt.Realm)))
		b.WriteString(fmt.Sprintf("  name  = %s\n", hclString(c.Name)))
		for _, group := range oauth2client.AllGroups() {
			fields := c.Values[group.TFName]
			var lines []string
			for _, f := range group.Fields {
				if f.Sensitive {
					continue
				}
				v, ok := fields[f.TFName]
				if !ok || oauth2client.EqualDefault(f, v) {
					continue
				}
				line := oauth2HCLLine(f, v)
				if line != "" {
					lines = append(lines, line)
				}
			}
			if len(lines) == 0 {
				continue
			}
			b.WriteString(fmt.Sprintf("\n  %s = {\n", group.TFName))
			for _, line := range lines {
				b.WriteString("    " + line + "\n")
			}
			b.WriteString("  }\n")
		}
		b.WriteString("}\n\n")
	}
	path := filepath.Join(g.opt.OutDir, "oauth2_clients.tf")
	if err := writeTerraformFile(path, []byte(b.String())); err != nil {
		return err
	}
	g.files = append(g.files, path)
	return nil
}

func oauth2HCLLine(f oauth2client.Field, v any) string {
	switch f.Kind {
	case oauth2client.KindString:
		s, _ := v.(string)
		if s == "" {
			return ""
		}
		return fmt.Sprintf("%s = %s", f.TFName, hclString(s))
	case oauth2client.KindBool:
		return fmt.Sprintf("%s = %v", f.TFName, v)
	case oauth2client.KindInt:
		return fmt.Sprintf("%s = %v", f.TFName, v)
	case oauth2client.KindStringList:
		var items []string
		switch t := v.(type) {
		case []string:
			items = t
		case []any:
			for _, item := range t {
				s, _ := item.(string)
				items = append(items, s)
			}
		}
		return fmt.Sprintf("%s = %s", f.TFName, hclStringList(items))
	default:
		return ""
	}
}
