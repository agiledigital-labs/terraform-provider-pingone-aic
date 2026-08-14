package generate

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
)

type emittedVariable struct {
	Name  string
	Label string
	Var   client.Variable
}

type emittedSecret struct {
	Name  string
	Label string
	Sec   client.Secret
}

func (g *gen) ingestESVs(ctx context.Context) error {
	vars, err := g.c.ListVariables(ctx)
	if err != nil {
		return fmt.Errorf("list esv variables: %w", err)
	}
	sort.Slice(vars, func(i, j int) bool { return vars[i].ID < vars[j].ID })
	progressf(g.opt, "Found %d esv variable(s)", len(vars))
	for _, v := range vars {
		name := prefix.StripESV(g.opt.Prefix, v.ID)
		g.variables = append(g.variables, emittedVariable{
			Name:  name,
			Label: g.uniqueLabel("esv_variable", name),
			Var:   v,
		})
	}

	secs, err := g.c.ListSecrets(ctx)
	if err != nil {
		return fmt.Errorf("list esv secrets: %w", err)
	}
	sort.Slice(secs, func(i, j int) bool { return secs[i].ID < secs[j].ID })
	progressf(g.opt, "Found %d esv secret(s)", len(secs))
	for _, s := range secs {
		name := prefix.StripESV(g.opt.Prefix, s.ID)
		g.secrets = append(g.secrets, emittedSecret{
			Name:  name,
			Label: g.uniqueLabel("esv_secret", name),
			Sec:   s,
		})
	}
	return nil
}

func (g *gen) writeESVs() error {
	if err := g.writeVariables(); err != nil {
		return err
	}
	return g.writeSecrets()
}

func (g *gen) writeVariables() error {
	if len(g.variables) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("# Generated ESV variables. Values are plaintext here; the provider base64-encodes on the wire.\n")
	b.WriteString("# Applying these creates prefixed copies (esv-terraform_…). They stay loaded=false until a tenant restart, which this tool never triggers.\n\n")
	for _, e := range g.variables {
		v := e.Var
		b.WriteString(fmt.Sprintf("resource \"pingoneaic_esv_variable\" %q {\n", e.Label))
		b.WriteString(fmt.Sprintf("  name  = %s\n", hclString(e.Name)))
		if v.ExpressionType != "" && v.ExpressionType != "string" {
			b.WriteString(fmt.Sprintf("  expression_type = %s\n", hclString(v.ExpressionType)))
		}
		if v.Description != "" {
			b.WriteString(fmt.Sprintf("  description = %s\n", hclString(v.Description)))
		}
		b.WriteString(fmt.Sprintf("  value = %s\n", hclString(v.Value)))
		b.WriteString("}\n\n")
	}
	path := filepath.Join(g.opt.OutDir, "esv_variables.tf")
	if err := writeTerraformFile(path, []byte(b.String())); err != nil {
		return err
	}
	g.files = append(g.files, path)
	return nil
}

func (g *gen) writeSecrets() error {
	if len(g.secrets) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("# Generated ESV secret metadata. AIC never returns secret values, so `value` is omitted.\n")
	b.WriteString("# Applying a secret resource requires setting value yourself; do not apply this file as-is.\n\n")
	for _, e := range g.secrets {
		s := e.Sec
		b.WriteString(fmt.Sprintf("resource \"pingoneaic_esv_secret\" %q {\n", e.Label))
		b.WriteString(fmt.Sprintf("  name = %s\n", hclString(e.Name)))
		if s.Encoding != "" && s.Encoding != "generic" {
			b.WriteString(fmt.Sprintf("  encoding = %s\n", hclString(s.Encoding)))
		}
		if !s.UseInPlaceholders {
			b.WriteString("  use_in_placeholders = false\n")
		}
		if s.Description != "" {
			b.WriteString(fmt.Sprintf("  description = %s\n", hclString(s.Description)))
		}
		b.WriteString("  # value = \"…\"  # required on create; never exported by AIC\n")
		b.WriteString("}\n\n")
	}
	path := filepath.Join(g.opt.OutDir, "esv_secrets.tf")
	if err := writeTerraformFile(path, []byte(b.String())); err != nil {
		return err
	}
	g.files = append(g.files, path)
	return nil
}
