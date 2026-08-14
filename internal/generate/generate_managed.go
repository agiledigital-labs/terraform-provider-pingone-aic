package generate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/managedobject"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
)

type emittedManaged struct {
	Name  string
	Label string
	Obj   managedobject.Object
}

func (g *gen) ingestManaged(ctx context.Context) error {
	doc, err := g.c.GetManaged(ctx)
	if err != nil {
		return fmt.Errorf("read managed config: %w", err)
	}
	objs, err := client.ManagedObjects(doc)
	if err != nil {
		return err
	}
	progressf(g.opt, "Found %d managed object type(s)", len(objs))
	for _, raw := range objs {
		if managedobject.IsPingShipped(raw) {
			continue
		}
		decoded, err := managedobject.DecodeAPI(raw, g.opt.Prefix)
		if err != nil {
			name, _ := raw["name"].(string)
			return fmt.Errorf("managed object %s: %w", name, err)
		}
		name := prefix.Strip(g.opt.Prefix, decoded.Name)
		g.managed = append(g.managed, emittedManaged{
			Name:  name,
			Label: g.uniqueLabel("managed_object", name),
			Obj:   *decoded,
		})
	}
	progressf(g.opt, "Generating %d custom managed object type(s)", len(g.managed))
	return nil
}

func (g *gen) writeManaged() error {
	if len(g.managed) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("# Generated custom managed-object types. Ping-shipped objects (alpha_user, …) are skipped.\n")
	b.WriteString("# Applying creates prefixed copies and remaps managed/ relationship paths. Writes confirm they landed.\n")
	b.WriteString("# Lifecycle hooks stay on this type — they are never written onto alpha_user / bravo_user.\n\n")
	for _, e := range g.managed {
		o := e.Obj
		b.WriteString(fmt.Sprintf("resource \"pingoneaic_managed_object\" %q {\n", e.Label))
		b.WriteString(fmt.Sprintf("  name = %s\n", hclString(e.Name)))
		if o.Title != "" {
			b.WriteString(fmt.Sprintf("  title = %s\n", hclString(o.Title)))
		}
		if o.Description != "" {
			b.WriteString(fmt.Sprintf("  description = %s\n", hclString(o.Description)))
		}
		if o.Icon != "" {
			b.WriteString(fmt.Sprintf("  icon = %s\n", hclString(o.Icon)))
		}
		if o.IconClass != "" {
			b.WriteString(fmt.Sprintf("  icon_class = %s\n", hclString(o.IconClass)))
		}
		req := map[string]bool{}
		for _, r := range o.Required {
			req[r] = true
		}
		for _, p := range o.Properties {
			b.WriteString("\n  property {\n")
			b.WriteString(fmt.Sprintf("    name = %s\n", hclString(p.Name)))
			b.WriteString(fmt.Sprintf("    type = %s\n", hclString(p.Type)))
			if p.Title != "" {
				b.WriteString(fmt.Sprintf("    title = %s\n", hclString(p.Title)))
			}
			if p.Description != "" {
				b.WriteString(fmt.Sprintf("    description = %s\n", hclString(p.Description)))
			}
			if req[p.Name] {
				b.WriteString("    required = true\n")
			}
			writeOptBool(&b, "searchable", p.Searchable)
			writeOptBool(&b, "viewable", p.Viewable)
			writeOptBool(&b, "user_editable", p.UserEditable)
			if p.ResourcePath != "" {
				b.WriteString(fmt.Sprintf("    resource_path = %s\n", hclString(p.ResourcePath)))
			}
			if p.ResourceLabel != "" {
				b.WriteString(fmt.Sprintf("    resource_label = %s\n", hclString(p.ResourceLabel)))
			}
			if p.ReversePropertyName != "" {
				b.WriteString(fmt.Sprintf("    reverse_property_name = %s\n", hclString(p.ReversePropertyName)))
			}
			writeOptBool(&b, "reverse_relationship", p.ReverseRelationship)
			writeOptBool(&b, "validate", p.Validate)
			if len(p.Enum) > 0 {
				b.WriteString(fmt.Sprintf("    enum = %s\n", hclStringList(p.Enum)))
			}
			if p.Minimum != nil {
				b.WriteString(fmt.Sprintf("    minimum = %v\n", *p.Minimum))
			}
			if p.Maximum != nil {
				b.WriteString(fmt.Sprintf("    maximum = %v\n", *p.Maximum))
			}
			if p.Items != nil {
				if p.Items.Type != "" {
					b.WriteString(fmt.Sprintf("    items_type = %s\n", hclString(p.Items.Type)))
				}
				if p.Items.ResourcePath != "" {
					b.WriteString(fmt.Sprintf("    items_resource_path = %s\n", hclString(p.Items.ResourcePath)))
				}
				if p.Items.ReversePropertyName != "" {
					b.WriteString(fmt.Sprintf("    items_reverse_property_name = %s\n", hclString(p.Items.ReversePropertyName)))
				}
				writeOptBool(&b, "items_reverse_relationship", p.Items.ReverseRelationship)
				writeOptBool(&b, "items_validate", p.Items.Validate)
			}
			b.WriteString("  }\n")
		}
		for _, h := range o.Hooks {
			b.WriteString("\n  hook {\n")
			b.WriteString(fmt.Sprintf("    event = %s\n", hclString(h.Event)))
			if h.Type != "" && h.Type != "text/javascript" {
				b.WriteString(fmt.Sprintf("    type = %s\n", hclString(h.Type)))
			}
			if h.File != "" {
				b.WriteString(fmt.Sprintf("    file = %s\n", hclString(h.File)))
			}
			if h.Source != "" {
				rel := filepath.Join("hooks", e.Label+"."+h.Event+".js")
				if err := os.MkdirAll(filepath.Join(g.opt.OutDir, "hooks"), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(g.opt.OutDir, rel), []byte(h.Source), 0o644); err != nil {
					return err
				}
				g.files = append(g.files, filepath.Join(g.opt.OutDir, rel))
				b.WriteString(fmt.Sprintf("    source = file(%s)\n", hclString(rel)))
			}
			b.WriteString("  }\n")
		}
		b.WriteString("}\n\n")
	}
	path := filepath.Join(g.opt.OutDir, "managed_objects.tf")
	if err := writeTerraformFile(path, []byte(b.String())); err != nil {
		return err
	}
	g.files = append(g.files, path)
	return nil
}

func writeOptBool(b *strings.Builder, name string, v *bool) {
	if v != nil {
		b.WriteString(fmt.Sprintf("    %s = %v\n", name, *v))
	}
}
