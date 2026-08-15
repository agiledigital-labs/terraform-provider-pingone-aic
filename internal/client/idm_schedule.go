package client

import (
	"context"
	"fmt"
)

const DefaultSchedulePersisted = true

var scheduleKeys = map[string]struct{}{
	"enabled": {}, "persisted": {}, "type": {}, "schedule": {},
	"invokeService": {}, "invokeContext": {}, "invokeLogLevel": {},
	"concurrentExecution": {}, "misfirePolicy": {},
	"startTime": {}, "endTime": {}, "repeatCount": {}, "repeatInterval": {},
	"recoverable": {}, "managedObject": {}, "scriptProperty": {}, "isCron": {},
}

var invokeContextKeys = map[string]struct{}{
	"script": {}, "type": {}, "numberOfThreads": {}, "scan": {},
	"task": {}, "waitForCompletion": {},
}

var scanKeys = map[string]struct{}{
	"_queryFilter": {}, "object": {}, "taskState": {}, "recovery": {},
}

var scriptKeys = map[string]struct{}{
	"source": {}, "type": {}, "globals": {},
}

var taskKeys = map[string]struct{}{
	"script": {},
}

var taskStateKeys = map[string]struct{}{
	"started": {}, "completed": {},
}

var recoveryKeys = map[string]struct{}{
	"timeout": {},
}

type Schedule struct {
	Name                string
	Enabled             bool
	Persisted           bool
	Type                string
	Cron                string
	InvokeService       string
	InvokeLogLevel      string
	MisfirePolicy       string
	ConcurrentExecution *bool
	Recoverable         *bool
	IsCron              *bool
	RepeatCount         *int64
	RepeatInterval      *int64
	StartTime           string
	EndTime             string
	ManagedObject       string
	ScriptProperty      string
	Source              string
	ScriptType          string
	Globals             map[string]string
	InvokeContextType   string
	NumberOfThreads     *int64
	WaitForCompletion   *bool
	ScanObject          string
	ScanQueryFilter     string
	TaskStarted         string
	TaskCompleted       string
	RecoveryTimeout     string
	present             map[string]map[string]struct{}
}

func DecodeSchedule(raw map[string]any) (*Schedule, error) {
	if err := rejectUnknown("schedule", raw, scheduleKeys); err != nil {
		return nil, err
	}
	id, err := strictString(raw, "_id")
	if err != nil {
		return nil, err
	}
	s := &Schedule{
		Name:      ConfigName("schedule", id),
		Persisted: DefaultSchedulePersisted,
		present:   map[string]map[string]struct{}{"": objectKeys(raw)},
	}
	strings := []struct {
		key string
		dst *string
	}{
		{"type", &s.Type}, {"schedule", &s.Cron}, {"invokeService", &s.InvokeService},
		{"invokeLogLevel", &s.InvokeLogLevel}, {"misfirePolicy", &s.MisfirePolicy},
		{"startTime", &s.StartTime}, {"endTime", &s.EndTime},
		{"managedObject", &s.ManagedObject}, {"scriptProperty", &s.ScriptProperty},
	}
	for _, field := range strings {
		*field.dst, err = strictString(raw, field.key)
		if err != nil {
			return nil, err
		}
	}
	if v, ok, err := strictBool(raw, "enabled"); err != nil {
		return nil, err
	} else if ok {
		s.Enabled = v
	}
	if v, ok, err := strictBool(raw, "persisted"); err != nil {
		return nil, err
	} else if ok {
		s.Persisted = v
	}
	if v, ok, err := strictBool(raw, "concurrentExecution"); err != nil {
		return nil, err
	} else if ok {
		s.ConcurrentExecution = &v
	}
	if v, ok, err := strictBool(raw, "recoverable"); err != nil {
		return nil, err
	} else if ok {
		s.Recoverable = &v
	}
	if v, ok, err := strictBool(raw, "isCron"); err != nil {
		return nil, err
	} else if ok {
		s.IsCron = &v
	}
	if v, ok, err := strictInt(raw, "repeatCount"); err != nil {
		return nil, err
	} else if ok {
		s.RepeatCount = &v
	}
	if v, ok, err := strictInt(raw, "repeatInterval"); err != nil {
		return nil, err
	} else if ok {
		s.RepeatInterval = &v
	}
	ic, err := strictObject(raw["invokeContext"], "schedule.invokeContext")
	if err != nil {
		return nil, err
	}
	if ic != nil {
		s.present["invokeContext"] = objectKeys(ic)
		if err := rejectUnknown("schedule.invokeContext", ic, invokeContextKeys); err != nil {
			return nil, err
		}
		s.InvokeContextType, err = strictString(ic, "type")
		if err != nil {
			return nil, err
		}
		if v, ok, err := strictInt(ic, "numberOfThreads"); err != nil {
			return nil, err
		} else if ok {
			s.NumberOfThreads = &v
		}
		if v, ok, err := strictBool(ic, "waitForCompletion"); err != nil {
			return nil, err
		} else if ok {
			s.WaitForCompletion = &v
		}
		if script, err := strictObject(ic["script"], "schedule.invokeContext.script"); err != nil {
			return nil, err
		} else if script != nil {
			s.present["invokeContext.script"] = objectKeys(script)
			if err := decodeScheduleScript(s, script, "schedule.invokeContext.script"); err != nil {
				return nil, err
			}
		}
		if scan, err := strictObject(ic["scan"], "schedule.invokeContext.scan"); err != nil {
			return nil, err
		} else if scan != nil {
			s.present["invokeContext.scan"] = objectKeys(scan)
			if err := rejectUnknown("schedule.invokeContext.scan", scan, scanKeys); err != nil {
				return nil, err
			}
			s.ScanObject, err = strictString(scan, "object")
			if err != nil {
				return nil, err
			}
			s.ScanQueryFilter, err = strictString(scan, "_queryFilter")
			if err != nil {
				return nil, err
			}
			if st, err := strictObject(scan["taskState"], "schedule.invokeContext.scan.taskState"); err != nil {
				return nil, err
			} else if st != nil {
				s.present["invokeContext.scan.taskState"] = objectKeys(st)
				if err := rejectUnknown("schedule.invokeContext.scan.taskState", st, taskStateKeys); err != nil {
					return nil, err
				}
				s.TaskStarted, err = strictString(st, "started")
				if err != nil {
					return nil, err
				}
				s.TaskCompleted, err = strictString(st, "completed")
				if err != nil {
					return nil, err
				}
			}
			if rec, err := strictObject(scan["recovery"], "schedule.invokeContext.scan.recovery"); err != nil {
				return nil, err
			} else if rec != nil {
				s.present["invokeContext.scan.recovery"] = objectKeys(rec)
				if err := rejectUnknown("schedule.invokeContext.scan.recovery", rec, recoveryKeys); err != nil {
					return nil, err
				}
				s.RecoveryTimeout, err = strictString(rec, "timeout")
				if err != nil {
					return nil, err
				}
			}
		}
		if task, err := strictObject(ic["task"], "schedule.invokeContext.task"); err != nil {
			return nil, err
		} else if task != nil {
			s.present["invokeContext.task"] = objectKeys(task)
			if err := rejectUnknown("schedule.invokeContext.task", task, taskKeys); err != nil {
				return nil, err
			}
			if script, err := strictObject(task["script"], "schedule.invokeContext.task.script"); err != nil {
				return nil, err
			} else if script != nil {
				s.present["invokeContext.task.script"] = objectKeys(script)
				if err := decodeScheduleScript(s, script, "schedule.invokeContext.task.script"); err != nil {
					return nil, err
				}
			}
		}
	}
	if s.Type == "" {
		s.Type = "cron"
	}
	if s.ScriptType == "" && s.Source != "" {
		s.ScriptType = "text/javascript"
	}
	return s, nil
}

func objectKeys(raw map[string]any) map[string]struct{} {
	keys := make(map[string]struct{}, len(raw))
	for key := range raw {
		keys[key] = struct{}{}
	}
	return keys
}

func restorePresentKeys(body map[string]any, keys map[string]struct{}) {
	for key := range keys {
		if key == "_id" || key == "_rev" {
			continue
		}
		if _, exists := body[key]; !exists {
			body[key] = nil
		}
	}
}

func decodeScheduleScript(s *Schedule, script map[string]any, path string) error {
	if err := rejectUnknown(path, script, scriptKeys); err != nil {
		return err
	}
	var err error
	s.Source, err = strictString(script, "source")
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	s.ScriptType, err = strictString(script, "type")
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if _, exists := script["globals"]; exists {
		globals, err := strictObject(script["globals"], path+".globals")
		if err != nil {
			return err
		}
		s.Globals = make(map[string]string, len(globals))
		for key := range globals {
			value, err := strictString(globals, key)
			if err != nil {
				return fmt.Errorf("%s.globals: %w", path, err)
			}
			s.Globals[key] = value
		}
	}
	return nil
}

func EncodeSchedule(s Schedule) map[string]any {
	body := map[string]any{
		"enabled":       s.Enabled,
		"persisted":     s.Persisted,
		"type":          s.Type,
		"invokeService": s.InvokeService,
	}
	if s.Cron != "" {
		body["schedule"] = s.Cron
	}
	if s.InvokeLogLevel != "" {
		body["invokeLogLevel"] = s.InvokeLogLevel
	}
	if s.MisfirePolicy != "" {
		body["misfirePolicy"] = s.MisfirePolicy
	}
	if s.ConcurrentExecution != nil {
		body["concurrentExecution"] = *s.ConcurrentExecution
	}
	if s.Recoverable != nil {
		body["recoverable"] = *s.Recoverable
	}
	if s.IsCron != nil {
		body["isCron"] = *s.IsCron
	}
	if s.RepeatCount != nil {
		body["repeatCount"] = *s.RepeatCount
	}
	if s.RepeatInterval != nil {
		body["repeatInterval"] = *s.RepeatInterval
	}
	if s.StartTime != "" {
		body["startTime"] = s.StartTime
	}
	if s.EndTime != "" {
		body["endTime"] = s.EndTime
	}
	if s.ManagedObject != "" {
		body["managedObject"] = s.ManagedObject
	}
	if s.ScriptProperty != "" {
		body["scriptProperty"] = s.ScriptProperty
	}

	ic := map[string]any{}
	if s.InvokeContextType != "" {
		ic["type"] = s.InvokeContextType
	}
	if s.NumberOfThreads != nil {
		ic["numberOfThreads"] = *s.NumberOfThreads
	}
	if s.WaitForCompletion != nil {
		ic["waitForCompletion"] = *s.WaitForCompletion
	}
	script := map[string]any{}
	if s.Source != "" {
		script["source"] = s.Source
	}
	if s.ScriptType != "" {
		script["type"] = s.ScriptType
	}
	if s.Globals != nil {
		globals := make(map[string]any, len(s.Globals))
		for key, value := range s.Globals {
			globals[key] = value
		}
		script["globals"] = globals
	}
	switch s.InvokeService {
	case "taskscanner":
		if len(script) > 0 {
			ic["task"] = map[string]any{"script": script}
		}
		scan := map[string]any{}
		if s.ScanObject != "" {
			scan["object"] = s.ScanObject
		}
		if s.ScanQueryFilter != "" {
			scan["_queryFilter"] = s.ScanQueryFilter
		}
		if s.TaskStarted != "" || s.TaskCompleted != "" {
			scan["taskState"] = map[string]any{
				"started":   s.TaskStarted,
				"completed": s.TaskCompleted,
			}
		}
		if s.RecoveryTimeout != "" {
			scan["recovery"] = map[string]any{"timeout": s.RecoveryTimeout}
		}
		if len(scan) > 0 {
			ic["scan"] = scan
		}
	default:
		if len(script) > 0 {
			ic["script"] = script
		}
	}
	if len(ic) > 0 {
		body["invokeContext"] = ic
	}
	if s.present != nil {
		restorePresentKeys(body, s.present[""])
		restorePresentKeys(ic, s.present["invokeContext"])
		restorePresentKeys(script, s.present["invokeContext.script"])
		if task, ok := ic["task"].(map[string]any); ok {
			restorePresentKeys(task, s.present["invokeContext.task"])
			if taskScript, ok := task["script"].(map[string]any); ok {
				restorePresentKeys(taskScript, s.present["invokeContext.task.script"])
			}
		}
		if scan, ok := ic["scan"].(map[string]any); ok {
			restorePresentKeys(scan, s.present["invokeContext.scan"])
			if state, ok := scan["taskState"].(map[string]any); ok {
				restorePresentKeys(state, s.present["invokeContext.scan.taskState"])
			}
			if recovery, ok := scan["recovery"].(map[string]any); ok {
				restorePresentKeys(recovery, s.present["invokeContext.scan.recovery"])
			}
		}
	}
	return body
}

func (c *Client) ListSchedules(ctx context.Context) ([]string, error) {
	ids, err := c.ListConfigIDs(ctx, "schedule/")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		names = append(names, ConfigName("schedule", id))
	}
	return names, nil
}

func (c *Client) GetSchedule(ctx context.Context, name string) (*Schedule, error) {
	raw, err := c.GetConfig(ctx, ConfigID("schedule", name))
	if err != nil {
		return nil, err
	}
	return DecodeSchedule(raw)
}

func (c *Client) PutSchedule(ctx context.Context, name string, s Schedule) (*Schedule, error) {
	raw, err := c.PutConfig(ctx, ConfigID("schedule", name), EncodeSchedule(s))
	if err != nil {
		return nil, err
	}
	return DecodeSchedule(raw)
}

func (c *Client) DeleteSchedule(ctx context.Context, name string) error {
	return c.DeleteConfig(ctx, ConfigID("schedule", name))
}

func ScriptedSchedule(s *Schedule) bool {
	return s != nil && s.Source != ""
}
