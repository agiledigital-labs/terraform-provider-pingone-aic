package client

import (
	"context"
)

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
	InvokeContextType   string
	NumberOfThreads     *int64
	WaitForCompletion   *bool
	ScanObject          string
	ScanQueryFilter     string
	TaskStarted         string
	TaskCompleted       string
	RecoveryTimeout     string
}

func DecodeSchedule(raw map[string]any) (*Schedule, error) {
	if err := rejectUnknown("schedule", raw, scheduleKeys); err != nil {
		return nil, err
	}
	id, _ := raw["_id"].(string)
	s := &Schedule{
		Name:           ConfigName("schedule", id),
		Type:           stringVal(raw, "type"),
		Cron:           stringVal(raw, "schedule"),
		InvokeService:  stringVal(raw, "invokeService"),
		InvokeLogLevel: stringVal(raw, "invokeLogLevel"),
		MisfirePolicy:  stringVal(raw, "misfirePolicy"),
		StartTime:      stringVal(raw, "startTime"),
		EndTime:        stringVal(raw, "endTime"),
		ManagedObject:  stringVal(raw, "managedObject"),
		ScriptProperty: stringVal(raw, "scriptProperty"),
	}
	if v, ok := boolVal(raw, "enabled"); ok {
		s.Enabled = v
	}
	if v, ok := boolVal(raw, "persisted"); ok {
		s.Persisted = v
	}
	if v, ok := boolVal(raw, "concurrentExecution"); ok {
		s.ConcurrentExecution = &v
	}
	if v, ok := boolVal(raw, "recoverable"); ok {
		s.Recoverable = &v
	}
	if v, ok := boolVal(raw, "isCron"); ok {
		s.IsCron = &v
	}
	if v, ok := intVal(raw, "repeatCount"); ok {
		s.RepeatCount = &v
	}
	if v, ok := intVal(raw, "repeatInterval"); ok {
		s.RepeatInterval = &v
	}
	ic := asObject(raw["invokeContext"])
	if ic != nil {
		if err := rejectUnknown("schedule.invokeContext", ic, invokeContextKeys); err != nil {
			return nil, err
		}
		s.InvokeContextType = stringVal(ic, "type")
		if v, ok := intVal(ic, "numberOfThreads"); ok {
			s.NumberOfThreads = &v
		}
		if v, ok := boolVal(ic, "waitForCompletion"); ok {
			s.WaitForCompletion = &v
		}
		if script := asObject(ic["script"]); script != nil {
			s.Source = decodeSource(script["source"])
			if s.Source == "" {
				s.Source = decodeSource(script)
			}
			s.ScriptType = stringVal(script, "type")
		}
		if scan := asObject(ic["scan"]); scan != nil {
			if err := rejectUnknown("schedule.invokeContext.scan", scan, scanKeys); err != nil {
				return nil, err
			}
			s.ScanObject = stringVal(scan, "object")
			s.ScanQueryFilter = stringVal(scan, "_queryFilter")
			if st := asObject(scan["taskState"]); st != nil {
				s.TaskStarted = stringVal(st, "started")
				s.TaskCompleted = stringVal(st, "completed")
			}
			if rec := asObject(scan["recovery"]); rec != nil {
				s.RecoveryTimeout = stringVal(rec, "timeout")
			}
		}
		if task := asObject(ic["task"]); task != nil {
			if script := asObject(task["script"]); script != nil {
				s.Source = decodeSource(script["source"])
				s.ScriptType = stringVal(script, "type")
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
