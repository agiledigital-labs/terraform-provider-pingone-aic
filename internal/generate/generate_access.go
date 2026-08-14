package generate

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
)

type emittedAccessRule struct {
	Label   string
	Hash    string
	Copies  int
	Indices []int
	Rule    client.AccessRule
}

type emittedAuthMapping struct {
	Label   string
	Hash    string
	Mapping client.AuthMapping
}

func (g *gen) ingestAccess(ctx context.Context) error {
	doc, err := g.c.GetAccess(ctx)
	if err != nil {
		return fmt.Errorf("get config/access: %w", err)
	}
	rules, err := client.AccessRules(doc)
	if err != nil {
		return err
	}
	progressf(g.opt, "Found %d access rule(s)", len(rules))
	seen := map[string]int{}
	for i, raw := range rules {
		decoded, err := client.DecodeAccessRule(raw)
		if err != nil {
			return fmt.Errorf("access configs[%d]: %w", i, err)
		}
		hash, err := client.Digest(raw)
		if err != nil {
			return err
		}
		if idx := seen[hash]; idx > 0 {
			g.accessRules[idx-1].Copies++
			g.accessRules[idx-1].Indices = append(g.accessRules[idx-1].Indices, i)
			continue
		}
		g.accessRules = append(g.accessRules, emittedAccessRule{
			Label:   g.uniqueLabel("access_rule", decoded.Pattern),
			Hash:    hash,
			Copies:  1,
			Indices: []int{i},
			Rule:    *decoded,
		})
		seen[hash] = len(g.accessRules)
	}

	auth, err := g.c.GetAuthentication(ctx)
	if err != nil {
		return fmt.Errorf("get config/authentication: %w", err)
	}
	maps, err := client.AuthMappings(auth)
	if err != nil {
		return err
	}
	progressf(g.opt, "Found %d authentication mapping(s)", len(maps))
	for i, raw := range maps {
		decoded, err := client.DecodeAuthMapping(raw)
		if err != nil {
			return fmt.Errorf("staticUserMapping[%d]: %w", i, err)
		}
		hash, err := client.Digest(raw)
		if err != nil {
			return err
		}
		g.authMappings = append(g.authMappings, emittedAuthMapping{
			Label:   g.uniqueLabel("authentication_mapping", decoded.Subject),
			Hash:    hash,
			Mapping: *decoded,
		})
	}
	return nil
}

func (g *gen) writeAccess() error {
	if err := g.writeAccessRules(); err != nil {
		return err
	}
	return g.writeAuthMappings()
}

func (g *gen) writeAccessRules() error {
	if len(g.accessRules) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("# Review dump of existing /openidm/config/access rules.\n")
	b.WriteString("# This file is NOT loaded by Terraform (.tf.review). Rules have no name to prefix,\n")
	b.WriteString("# so applying them as copies would append duplicate grants. To manage one rule,\n")
	b.WriteString("# copy its resource into a .tf file and:\n")
	b.WriteString("#   terraform import pingoneaic_access_rule.<label> <hash>\n")
	b.WriteString("# Create refuses if the hash already exists. Destroy removes only that one copy.\n\n")
	for _, e := range g.accessRules {
		r := e.Rule
		b.WriteString(fmt.Sprintf("# hash %s", e.Hash))
		if e.Copies > 1 {
			b.WriteString(fmt.Sprintf(" (%d identical copies at indices %v)", e.Copies, e.Indices))
		} else {
			b.WriteString(fmt.Sprintf(" (index %d)", e.Indices[0]))
		}
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("resource \"pingoneaic_access_rule\" %q {\n", e.Label))
		b.WriteString(fmt.Sprintf("  pattern = %s\n", hclString(r.Pattern)))
		b.WriteString(fmt.Sprintf("  roles   = %s\n", hclString(r.Roles)))
		b.WriteString(fmt.Sprintf("  methods = %s\n", hclString(r.Methods)))
		if r.Actions != nil {
			b.WriteString(fmt.Sprintf("  actions = %s\n", hclString(*r.Actions)))
		}
		if r.CustomAuthz != nil {
			b.WriteString(fmt.Sprintf("  custom_authz = %s\n", hclString(*r.CustomAuthz)))
		}
		if r.ExcludePatterns != nil {
			b.WriteString(fmt.Sprintf("  exclude_patterns = %s\n", hclString(*r.ExcludePatterns)))
		}
		b.WriteString("}\n\n")
	}
	path := filepath.Join(g.opt.OutDir, "access_rules.tf.review")
	if err := writeTerraformFile(path, []byte(b.String())); err != nil {
		return err
	}
	g.files = append(g.files, path)
	return nil
}

func (g *gen) writeAuthMappings() error {
	if len(g.authMappings) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("# Review dump of existing /openidm/config/authentication staticUserMapping entries.\n")
	b.WriteString("# This file is NOT loaded by Terraform (.tf.review). Subjects have no safe prefix\n")
	b.WriteString("# (they are token ids, not names). To manage one mapping, copy its resource into a\n")
	b.WriteString("# .tf file and:\n")
	b.WriteString("#   terraform import pingoneaic_authentication_mapping.<label> <hash>\n\n")
	for _, e := range g.authMappings {
		m := e.Mapping
		b.WriteString(fmt.Sprintf("# hash %s\n", e.Hash))
		b.WriteString(fmt.Sprintf("resource \"pingoneaic_authentication_mapping\" %q {\n", e.Label))
		b.WriteString(fmt.Sprintf("  subject    = %s\n", hclString(m.Subject)))
		b.WriteString(fmt.Sprintf("  local_user = %s\n", hclString(m.LocalUser)))
		if len(m.Roles) > 0 {
			b.WriteString(fmt.Sprintf("  roles      = %s\n", hclStringList(m.Roles)))
		}
		if m.UserRoles != "" {
			b.WriteString(fmt.Sprintf("  user_roles = %s\n", hclString(m.UserRoles)))
		}
		if m.ExecuteAugmentationScript != nil {
			b.WriteString(fmt.Sprintf("  execute_augmentation_script = %v\n", *m.ExecuteAugmentationScript))
		}
		b.WriteString("}\n\n")
	}
	path := filepath.Join(g.opt.OutDir, "authentication_mappings.tf.review")
	if err := writeTerraformFile(path, []byte(b.String())); err != nil {
		return err
	}
	g.files = append(g.files, path)
	return nil
}
