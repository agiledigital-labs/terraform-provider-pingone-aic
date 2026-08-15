package generate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
)

type emittedEndpoint struct {
	Name  string
	Label string
	EP    client.Endpoint
}

type emittedSchedule struct {
	Name  string
	Label string
	Sched client.Schedule
	WasOn bool
}

func (g *gen) ingestIDM(ctx context.Context) error {
	names, err := g.c.ListEndpoints(ctx)
	if err != nil {
		return fmt.Errorf("list endpoints: %w", err)
	}
	progressf(g.opt, "Found %d idm endpoint(s)", len(names))
	for _, name := range names {
		ep, err := g.c.GetEndpoint(ctx, name)
		if err != nil {
			return fmt.Errorf("endpoint %s: %w", name, err)
		}
		logical := prefix.Strip(g.opt.Prefix, ep.Name)
		g.endpoints = append(g.endpoints, emittedEndpoint{
			Name:  logical,
			Label: g.uniqueLabel("idm_endpoint", logical),
			EP:    *ep,
		})
	}

	snames, err := g.c.ListSchedules(ctx)
	if err != nil {
		return fmt.Errorf("list schedules: %w", err)
	}
	progressf(g.opt, "Found %d idm schedule(s)", len(snames))
	for _, name := range snames {
		s, err := g.c.GetSchedule(ctx, name)
		if err != nil {
			return fmt.Errorf("schedule %s: %w", name, err)
		}
		logical := prefix.Strip(g.opt.Prefix, s.Name)
		g.schedules = append(g.schedules, emittedSchedule{
			Name:  logical,
			Label: g.uniqueLabel("idm_schedule", logical),
			Sched: *s,
			WasOn: s.Enabled,
		})
	}
	return nil
}

func (g *gen) writeIDM() error {
	if err := g.writeEndpoints(); err != nil {
		return err
	}
	return g.writeSchedules()
}

func (g *gen) writeEndpoints() error {
	if len(g.endpoints) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Join(g.opt.OutDir, "endpoints"), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# Generated IDM custom endpoints. source = file(...) points at endpoints/*.js.\n")
	b.WriteString("# File-backed product endpoints keep `file` and have no extracted source.\n\n")
	for _, e := range g.endpoints {
		ep := e.EP
		b.WriteString(fmt.Sprintf("resource \"pingoneaic_idm_endpoint\" %q {\n", e.Label))
		b.WriteString(fmt.Sprintf("  name = %s\n", hclString(e.Name)))
		if ep.Type != "" && ep.Type != "text/javascript" {
			b.WriteString(fmt.Sprintf("  type = %s\n", hclString(ep.Type)))
		}
		if ep.Description != "" {
			b.WriteString(fmt.Sprintf("  description = %s\n", hclString(ep.Description)))
		}
		if ep.Context != "" {
			b.WriteString(fmt.Sprintf("  context = %s\n", hclString(ep.Context)))
		}
		if ep.File != "" {
			b.WriteString(fmt.Sprintf("  file = %s\n", hclString(ep.File)))
		}
		if ep.GlobalsObject != "" {
			b.WriteString(fmt.Sprintf("  globals_object = %s\n", hclString(ep.GlobalsObject)))
		}
		if len(ep.AllowedRoles) > 0 {
			b.WriteString(fmt.Sprintf("  allowed_roles = %s\n", hclStringList(ep.AllowedRoles)))
		}
		if ep.Source != "" {
			rel := filepath.Join("endpoints", e.Label+".js")
			if err := os.WriteFile(filepath.Join(g.opt.OutDir, rel), []byte(ep.Source), 0o644); err != nil {
				return err
			}
			g.files = append(g.files, filepath.Join(g.opt.OutDir, rel))
			b.WriteString(fmt.Sprintf("  source = %s\n", hclFile(rel)))
		}
		b.WriteString("}\n\n")
	}
	path := filepath.Join(g.opt.OutDir, "idm_endpoints.tf")
	if err := writeTerraformFile(path, []byte(b.String())); err != nil {
		return err
	}
	g.files = append(g.files, path)
	return nil
}

func (g *gen) writeSchedules() error {
	if len(g.schedules) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Join(g.opt.OutDir, "schedules"), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# Generated IDM schedules. enabled is omitted (defaults false) so copies cannot fire.\n")
	b.WriteString("# source = file(...) points at schedules/*.js.\n\n")
	for _, e := range g.schedules {
		s := e.Sched
		if e.WasOn {
			b.WriteString(fmt.Sprintf("# original %s was enabled=true\n", e.Name))
		}
		b.WriteString(fmt.Sprintf("resource \"pingoneaic_idm_schedule\" %q {\n", e.Label))
		b.WriteString(fmt.Sprintf("  name           = %s\n", hclString(e.Name)))
		b.WriteString(fmt.Sprintf("  invoke_service = %s\n", hclString(s.InvokeService)))
		if s.Type != "" && s.Type != "cron" {
			b.WriteString(fmt.Sprintf("  type = %s\n", hclString(s.Type)))
		}
		if s.Cron != "" {
			b.WriteString(fmt.Sprintf("  schedule = %s\n", hclString(s.Cron)))
		}
		if !s.Persisted {
			b.WriteString("  persisted = false\n")
		}
		if s.InvokeLogLevel != "" {
			b.WriteString(fmt.Sprintf("  invoke_log_level = %s\n", hclString(s.InvokeLogLevel)))
		}
		if s.MisfirePolicy != "" {
			b.WriteString(fmt.Sprintf("  misfire_policy = %s\n", hclString(s.MisfirePolicy)))
		}
		if s.ConcurrentExecution != nil {
			b.WriteString(fmt.Sprintf("  concurrent_execution = %v\n", *s.ConcurrentExecution))
		}
		if s.Recoverable != nil {
			b.WriteString(fmt.Sprintf("  recoverable = %v\n", *s.Recoverable))
		}
		if s.IsCron != nil {
			b.WriteString(fmt.Sprintf("  is_cron = %v\n", *s.IsCron))
		}
		if s.RepeatCount != nil {
			b.WriteString(fmt.Sprintf("  repeat_count = %d\n", *s.RepeatCount))
		}
		if s.RepeatInterval != nil {
			b.WriteString(fmt.Sprintf("  repeat_interval = %d\n", *s.RepeatInterval))
		}
		if s.StartTime != "" {
			b.WriteString(fmt.Sprintf("  start_time = %s\n", hclString(s.StartTime)))
		}
		if s.EndTime != "" {
			b.WriteString(fmt.Sprintf("  end_time = %s\n", hclString(s.EndTime)))
		}
		if s.ManagedObject != "" {
			b.WriteString(fmt.Sprintf("  managed_object = %s\n", hclString(s.ManagedObject)))
		}
		if s.ScriptProperty != "" {
			b.WriteString(fmt.Sprintf("  script_property = %s\n", hclString(s.ScriptProperty)))
		}
		if s.InvokeContextType != "" {
			b.WriteString(fmt.Sprintf("  invoke_context_type = %s\n", hclString(s.InvokeContextType)))
		}
		if s.NumberOfThreads != nil {
			b.WriteString(fmt.Sprintf("  number_of_threads = %d\n", *s.NumberOfThreads))
		}
		if s.WaitForCompletion != nil {
			b.WriteString(fmt.Sprintf("  wait_for_completion = %v\n", *s.WaitForCompletion))
		}
		if s.ScanObject != "" {
			b.WriteString(fmt.Sprintf("  scan_object = %s\n", hclString(s.ScanObject)))
		}
		if s.ScanQueryFilter != "" {
			b.WriteString(fmt.Sprintf("  scan_query_filter = %s\n", hclString(s.ScanQueryFilter)))
		}
		if s.TaskStarted != "" {
			b.WriteString(fmt.Sprintf("  task_started = %s\n", hclString(s.TaskStarted)))
		}
		if s.TaskCompleted != "" {
			b.WriteString(fmt.Sprintf("  task_completed = %s\n", hclString(s.TaskCompleted)))
		}
		if s.RecoveryTimeout != "" {
			b.WriteString(fmt.Sprintf("  recovery_timeout = %s\n", hclString(s.RecoveryTimeout)))
		}
		if s.Globals != nil {
			b.WriteString("  globals = {")
			keys := make([]string, 0, len(s.Globals))
			for key := range s.Globals {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			if len(keys) > 0 {
				b.WriteString("\n")
				for _, key := range keys {
					b.WriteString(fmt.Sprintf("    %s = %s\n", hclString(key), hclString(s.Globals[key])))
				}
				b.WriteString("  ")
			}
			b.WriteString("}\n")
		}
		if s.Source != "" {
			rel := filepath.Join("schedules", e.Label+".js")
			if err := os.WriteFile(filepath.Join(g.opt.OutDir, rel), []byte(s.Source), 0o644); err != nil {
				return err
			}
			g.files = append(g.files, filepath.Join(g.opt.OutDir, rel))
			b.WriteString(fmt.Sprintf("  source = %s\n", hclFile(rel)))
		}
		b.WriteString("}\n\n")
	}
	path := filepath.Join(g.opt.OutDir, "idm_schedules.tf")
	if err := writeTerraformFile(path, []byte(b.String())); err != nil {
		return err
	}
	g.files = append(g.files, path)
	return nil
}
