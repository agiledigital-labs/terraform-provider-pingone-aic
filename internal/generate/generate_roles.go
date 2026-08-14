package generate

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
)

type emittedRole struct {
	Name  string
	Label string
	Role  client.Role
}

func (g *gen) ingestRoles(ctx context.Context) error {
	ids, err := g.c.ListRoles(ctx)
	if err != nil {
		return fmt.Errorf("list internal roles: %w", err)
	}
	progressf(g.opt, "Found %d internal role(s)", len(ids))
	for _, id := range ids {
		role, err := g.c.GetRole(ctx, id)
		if err != nil {
			return fmt.Errorf("role %s: %w", id, err)
		}
		logical := prefix.Strip(g.opt.Prefix, role.ID)
		if client.RoleLooksLikeUUID(role.ID) && role.Name != "" {
			logical = role.Name
		}
		g.roles = append(g.roles, emittedRole{
			Name:  logical,
			Label: g.uniqueLabel("internal_role", logical),
			Role:  *role,
		})
	}
	return nil
}

func (g *gen) writeRoles() error {
	if len(g.roles) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("# Generated IDM internal roles. name is the `_id` access rules must reference.\n")
	b.WriteString("# Console-created UUID ids are emitted under their display name so apply creates a copy with a chosen id.\n")
	b.WriteString("# privilege order is not significant. path is not prefixed.\n\n")
	for _, e := range g.roles {
		r := e.Role
		b.WriteString(fmt.Sprintf("resource \"pingoneaic_internal_role\" %q {\n", e.Label))
		b.WriteString(fmt.Sprintf("  name = %s\n", hclString(e.Name)))
		if r.Name != "" && r.Name != r.ID {
			b.WriteString(fmt.Sprintf("  display_name = %s\n", hclString(r.Name)))
		}
		if r.Description != "" {
			b.WriteString(fmt.Sprintf("  description = %s\n", hclString(r.Description)))
		}
		if r.Condition != "" {
			b.WriteString(fmt.Sprintf("  condition = %s\n", hclString(r.Condition)))
		}
		for _, p := range r.Privileges {
			b.WriteString("  privilege {\n")
			b.WriteString(fmt.Sprintf("    name        = %s\n", hclString(p.Name)))
			b.WriteString(fmt.Sprintf("    path        = %s\n", hclString(p.Path)))
			b.WriteString(fmt.Sprintf("    actions     = %s\n", hclStringList(p.Actions)))
			b.WriteString(fmt.Sprintf("    permissions = %s\n", hclStringList(p.Permissions)))
			if p.Filter != "" {
				b.WriteString(fmt.Sprintf("    filter      = %s\n", hclString(p.Filter)))
			}
			for _, f := range p.AccessFlags {
				b.WriteString("    access_flag {\n")
				b.WriteString(fmt.Sprintf("      attribute = %s\n", hclString(f.Attribute)))
				b.WriteString(fmt.Sprintf("      read_only = %v\n", f.ReadOnly))
				b.WriteString("    }\n")
			}
			b.WriteString("  }\n")
		}
		b.WriteString("}\n\n")
	}
	path := filepath.Join(g.opt.OutDir, "internal_roles.tf")
	if err := writeTerraformFile(path, []byte(b.String())); err != nil {
		return err
	}
	g.files = append(g.files, path)
	return nil
}
