package managedobject

import (
	"fmt"
	"sort"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
)

var objectChrome = map[string]struct{}{}

var schemaKeys = map[string]struct{}{
	"$schema": {}, "description": {}, "icon": {}, "mat-icon": {},
	"order": {}, "properties": {}, "required": {}, "title": {}, "type": {},
}

var propertyKeys = map[string]struct{}{
	"type": {}, "title": {}, "description": {}, "searchable": {}, "viewable": {},
	"userEditable": {}, "returnByDefault": {}, "notifySelf": {}, "validate": {},
	"reversePropertyName": {}, "reverseRelationship": {}, "resourceCollection": {},
	"items": {}, "enum": {}, "format": {}, "nullable": {}, "isVirtual": {},
	"minimum": {}, "maximum": {},
}

var propertyChrome = map[string]struct{}{
	"id": {}, "properties": {},
}

var collectionKeys = map[string]struct{}{
	"path": {}, "label": {}, "notify": {}, "query": {},
}

var hookKeys = map[string]struct{}{
	"type": {}, "source": {}, "file": {}, "globals": {},
}

// Object is the Terraform-facing view of one managed type.
type Object struct {
	Name        string
	IconClass   string
	Title       string
	Description string
	Icon        string
	MatIcon     string
	SchemaType  string
	JSONSchema  string
	Required    []string
	Properties  []Property
	Hooks       []Hook
}

// Hook is one lifecycle script on a managed type (onCreate, onUpdate, …).
// Detected by value shape, not a hardcoded event list: an object whose type
// mentions javascript and that carries source or file. File-backed hooks are
// markers only — the config API cannot read the referenced file.
type Hook struct {
	Event  string
	Type   string
	Source string
	File   string
}

type Property struct {
	Name                string
	Type                string
	Title               string
	Description         string
	Searchable          *bool
	Viewable            *bool
	UserEditable        *bool
	ReturnByDefault     *bool
	NotifySelf          *bool
	Validate            *bool
	ReversePropertyName string
	ReverseRelationship *bool
	Enum                []string
	Format              string
	Nullable            *bool
	IsVirtual           *bool
	Minimum             *float64
	Maximum             *float64
	ResourcePath        string
	ResourceLabel       string
	Items               *Property
}

func DecodeAPI(raw map[string]any, resourcePrefix string) (*Object, error) {
	var unknown []string
	var hooks []Hook
	for k, v := range raw {
		if k == "name" || k == "schema" || k == "iconClass" {
			continue
		}
		if _, chrome := objectChrome[k]; chrome {
			continue
		}
		if m, ok := v.(map[string]any); ok && isHook(m) {
			h, err := decodeHook(k, m)
			if err != nil {
				return nil, err
			}
			hooks = append(hooks, h)
			continue
		}
		unknown = append(unknown, k)
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("managed object has unmodelled fields %v — add them to internal/managedobject", unknown)
	}
	sort.Slice(hooks, func(i, j int) bool { return hooks[i].Event < hooks[j].Event })
	name, _ := raw["name"].(string)
	out := &Object{
		Name:      prefix.Strip(resourcePrefix, name),
		IconClass: stringOrEmpty(raw["iconClass"]),
		Hooks:     hooks,
	}
	schema, _ := raw["schema"].(map[string]any)
	if schema != nil {
		if err := decodeSchema(schema, out, resourcePrefix); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
	}
	return out, nil
}

func isHook(m map[string]any) bool {
	t, _ := m["type"].(string)
	if !strings.Contains(t, "javascript") {
		return false
	}
	if _, ok := m["source"].(string); ok {
		return true
	}
	if file, ok := m["file"].(string); ok && file != "" {
		return true
	}
	return false
}

func decodeHook(event string, raw map[string]any) (Hook, error) {
	var unknown []string
	for k := range raw {
		if _, ok := hookKeys[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return Hook{}, fmt.Errorf("hook %s has unmodelled fields %v — add them to internal/managedobject", event, unknown)
	}
	if g, ok := raw["globals"]; ok && g != nil {
		obj, isObj := g.(map[string]any)
		if !isObj {
			return Hook{}, fmt.Errorf("hook %s globals: unexpected %T", event, g)
		}
		if len(obj) > 0 {
			keys := make([]string, 0, len(obj))
			for k := range obj {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return Hook{}, fmt.Errorf("hook %s has non-empty globals %v — add them to internal/managedobject", event, keys)
		}
	}
	src := stringOrEmpty(raw["source"])
	file := stringOrEmpty(raw["file"])
	if src != "" && file != "" {
		return Hook{}, fmt.Errorf("hook %s has both source and file", event)
	}
	if src == "" && file == "" {
		return Hook{}, fmt.Errorf("hook %s has neither source nor file", event)
	}
	h := Hook{
		Event:  event,
		Type:   stringOrEmpty(raw["type"]),
		Source: src,
		File:   file,
	}
	if h.Type == "" {
		h.Type = "text/javascript"
	}
	return h, nil
}

func decodeSchema(schema map[string]any, out *Object, resourcePrefix string) error {
	var unknown []string
	for k := range schema {
		if _, ok := schemaKeys[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("schema has unmodelled fields %v", unknown)
	}
	out.Title = stringOrEmpty(schema["title"])
	out.Description = stringOrEmpty(schema["description"])
	out.Icon = stringOrEmpty(schema["icon"])
	out.MatIcon = stringOrEmpty(schema["mat-icon"])
	out.SchemaType = stringOrEmpty(schema["type"])
	out.JSONSchema = stringOrEmpty(schema["$schema"])
	if req, ok := schema["required"].([]any); ok {
		for _, item := range req {
			s, _ := item.(string)
			if s != "" {
				out.Required = append(out.Required, s)
			}
		}
	}
	order := stringSlice(schema["order"])
	props, _ := schema["properties"].(map[string]any)
	seen := map[string]bool{}
	for _, name := range order {
		raw, ok := props[name]
		if !ok {
			continue
		}
		p, err := decodeProperty(name, raw, resourcePrefix)
		if err != nil {
			return err
		}
		out.Properties = append(out.Properties, p)
		seen[name] = true
	}
	var extra []string
	for name := range props {
		if !seen[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	for _, name := range extra {
		p, err := decodeProperty(name, props[name], resourcePrefix)
		if err != nil {
			return err
		}
		out.Properties = append(out.Properties, p)
	}
	return nil
}

func decodeProperty(name string, raw any, resourcePrefix string) (Property, error) {
	obj, ok := raw.(map[string]any)
	if !ok {
		return Property{}, fmt.Errorf("property %s: expected object, got %T", name, raw)
	}
	var unknown []string
	for k := range obj {
		if _, chrome := propertyChrome[k]; chrome {
			continue
		}
		if _, ok := propertyKeys[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return Property{}, fmt.Errorf("property %s has unmodelled fields %v", name, unknown)
	}
	p := Property{
		Name:                name,
		Type:                stringOrEmpty(obj["type"]),
		Title:               stringOrEmpty(obj["title"]),
		Description:         stringOrEmpty(obj["description"]),
		Searchable:          boolPtr(obj, "searchable"),
		Viewable:            boolPtr(obj, "viewable"),
		UserEditable:        boolPtr(obj, "userEditable"),
		ReturnByDefault:     boolPtr(obj, "returnByDefault"),
		NotifySelf:          boolPtr(obj, "notifySelf"),
		Validate:            boolPtr(obj, "validate"),
		ReversePropertyName: stringOrEmpty(obj["reversePropertyName"]),
		ReverseRelationship: boolPtr(obj, "reverseRelationship"),
		Enum:                stringSlice(obj["enum"]),
		Format:              stringOrEmpty(obj["format"]),
		Nullable:            boolPtr(obj, "nullable"),
		IsVirtual:           boolPtr(obj, "isVirtual"),
		Minimum:             floatPtr(obj, "minimum"),
		Maximum:             floatPtr(obj, "maximum"),
	}
	if cols, _ := obj["resourceCollection"].([]any); len(cols) > 0 {
		if col, ok := cols[0].(map[string]any); ok {
			if err := checkCollection(col); err != nil {
				return Property{}, fmt.Errorf("property %s: %w", name, err)
			}
			p.ResourcePath = stripManagedPath(resourcePrefix, stringOrEmpty(col["path"]))
			p.ResourceLabel = stringOrEmpty(col["label"])
		}
	}
	if items, ok := obj["items"].(map[string]any); ok {
		item, err := decodeProperty(name+".items", items, resourcePrefix)
		if err != nil {
			return Property{}, err
		}
		p.Items = &item
	}
	return p, nil
}

func checkCollection(col map[string]any) error {
	var unknown []string
	for k := range col {
		if _, ok := collectionKeys[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("resourceCollection has unmodelled fields %v", unknown)
	}
	return nil
}

func EncodeAPI(o Object, resourcePrefix string) map[string]any {
	props := map[string]any{}
	order := make([]any, 0, len(o.Properties))
	for _, p := range o.Properties {
		props[p.Name] = encodeProperty(p, resourcePrefix)
		order = append(order, p.Name)
	}
	required := make([]any, 0, len(o.Required))
	for _, r := range o.Required {
		required = append(required, r)
	}
	schema := map[string]any{
		"type":       first(o.SchemaType, "object"),
		"properties": props,
		"order":      order,
		"required":   required,
	}
	if o.Title != "" {
		schema["title"] = o.Title
	}
	if o.Description != "" {
		schema["description"] = o.Description
	}
	if o.Icon != "" {
		schema["icon"] = o.Icon
	}
	if o.MatIcon != "" {
		schema["mat-icon"] = o.MatIcon
	}
	if o.JSONSchema != "" {
		schema["$schema"] = o.JSONSchema
	}
	out := map[string]any{
		"name":   prefix.Apply(resourcePrefix, o.Name),
		"schema": schema,
	}
	if o.IconClass != "" {
		out["iconClass"] = o.IconClass
	}
	for _, h := range o.Hooks {
		out[h.Event] = encodeHook(h)
	}
	return out
}

func encodeHook(h Hook) map[string]any {
	body := map[string]any{
		"type": first(h.Type, "text/javascript"),
	}
	if h.Source != "" {
		body["source"] = h.Source
	}
	if h.File != "" {
		body["file"] = h.File
	}
	return body
}

func encodeProperty(p Property, resourcePrefix string) map[string]any {
	out := map[string]any{}
	if p.Type != "" {
		out["type"] = p.Type
	}
	if p.Title != "" {
		out["title"] = p.Title
	}
	if p.Description != "" {
		out["description"] = p.Description
	}
	setBool(out, "searchable", p.Searchable)
	setBool(out, "viewable", p.Viewable)
	setBool(out, "userEditable", p.UserEditable)
	setBool(out, "returnByDefault", p.ReturnByDefault)
	setBool(out, "notifySelf", p.NotifySelf)
	setBool(out, "validate", p.Validate)
	setBool(out, "reverseRelationship", p.ReverseRelationship)
	setBool(out, "nullable", p.Nullable)
	setBool(out, "isVirtual", p.IsVirtual)
	if p.ReversePropertyName != "" {
		out["reversePropertyName"] = p.ReversePropertyName
	}
	if len(p.Enum) > 0 {
		arr := make([]any, len(p.Enum))
		for i, s := range p.Enum {
			arr[i] = s
		}
		out["enum"] = arr
	}
	if p.Format != "" {
		out["format"] = p.Format
	}
	if p.Minimum != nil {
		out["minimum"] = *p.Minimum
	}
	if p.Maximum != nil {
		out["maximum"] = *p.Maximum
	}
	if p.ResourcePath != "" {
		col := map[string]any{
			"path": applyManagedPath(resourcePrefix, p.ResourcePath),
		}
		if p.ResourceLabel != "" {
			col["label"] = p.ResourceLabel
		}
		out["resourceCollection"] = []any{col}
	}
	if p.Items != nil {
		out["items"] = encodeProperty(*p.Items, resourcePrefix)
	}
	return out
}

func applyManagedPath(pfx, path string) string {
	const head = "managed/"
	if !strings.HasPrefix(path, head) {
		return path
	}
	return head + prefix.Apply(pfx, strings.TrimPrefix(path, head))
}

func stripManagedPath(pfx, path string) string {
	const head = "managed/"
	if !strings.HasPrefix(path, head) {
		return path
	}
	return head + prefix.Strip(pfx, strings.TrimPrefix(path, head))
}

func stringOrEmpty(v any) string {
	s, _ := v.(string)
	return s
}

func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		s, _ := item.(string)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func boolPtr(m map[string]any, k string) *bool {
	v, ok := m[k]
	if !ok {
		return nil
	}
	b, ok := v.(bool)
	if !ok {
		return nil
	}
	return &b
}

func floatPtr(m map[string]any, k string) *float64 {
	v, ok := m[k]
	if !ok {
		return nil
	}
	switch n := v.(type) {
	case float64:
		return &n
	case int:
		f := float64(n)
		return &f
	default:
		return nil
	}
}

func setBool(m map[string]any, k string, v *bool) {
	if v != nil {
		m[k] = *v
	}
}

func first(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func IsPingShipped(raw map[string]any) bool {
	t, _ := raw["type"].(string)
	return t == "Managed Object"
}
