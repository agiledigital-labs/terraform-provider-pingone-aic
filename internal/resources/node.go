package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/nodetype"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &nodeResource{}
	_ resource.ResourceWithConfigure   = &nodeResource{}
	_ resource.ResourceWithImportState = &nodeResource{}
)

func NewNodeResource(spec nodetype.Spec) resource.Resource {
	return &nodeResource{spec: spec}
}

type nodeResource struct {
	spec   nodetype.Spec
	client *client.Client
}

func (r *nodeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.spec.TFResource
}

func (r *nodeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "AM node UUID. Generated on create.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"realm": schema.StringAttribute{
			Required:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		"node_type": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "AM node type (`" + r.spec.APIType + "`).",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
	}
	for _, f := range r.spec.Fields {
		attrs[f.TFName] = fieldSchema(f)
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: fmt.Sprintf("%s (`%s`). Attributes match the AM node schema; unknown API fields are an error.", r.spec.FriendlyName, r.spec.APIType),
		Attributes:          attrs,
	}
}

func fieldSchema(f nodetype.Field) schema.Attribute {
	desc := fmt.Sprintf("AM `%s`.", f.APIName)
	switch f.Kind {
	case nodetype.KindString, nodetype.KindESVString:
		attr := schema.StringAttribute{
			Optional:            !f.Required,
			Required:            f.Required && f.Default == nil,
			Computed:            f.Default != nil,
			Sensitive:           f.Sensitive,
			MarkdownDescription: desc,
		}
		if s, ok := f.Default.(string); ok {
			attr.Default = stringdefault.StaticString(s)
		}
		return attr
	case nodetype.KindBool:
		attr := schema.BoolAttribute{Optional: true, Computed: f.Default != nil, MarkdownDescription: desc}
		if b, ok := f.Default.(bool); ok {
			attr.Default = booldefault.StaticBool(b)
		}
		return attr
	case nodetype.KindInt:
		attr := schema.Int64Attribute{Optional: !f.Required, Required: f.Required && f.Default == nil, Computed: f.Default != nil, MarkdownDescription: desc}
		switch n := f.Default.(type) {
		case float64:
			attr.Default = int64default.StaticInt64(int64(n))
		case int:
			attr.Default = int64default.StaticInt64(int64(n))
		case int64:
			attr.Default = int64default.StaticInt64(n)
		}
		return attr
	case nodetype.KindStringList:
		attr := schema.ListAttribute{
			Optional:            !f.Required,
			Required:            f.Required && f.Default == nil,
			Computed:            f.Default != nil,
			ElementType:         types.StringType,
			MarkdownDescription: desc,
		}
		if f.Default != nil {
			attr.Default = listdefault.StaticValue(stringListValue(f.Default))
		}
		return attr
	case nodetype.KindStringMap:
		attr := schema.MapAttribute{
			Optional:            true,
			Computed:            true,
			ElementType:         types.StringType,
			MarkdownDescription: desc,
			Default:             mapdefault.StaticValue(types.MapValueMust(types.StringType, map[string]attr.Value{})),
		}
		return attr
	case nodetype.KindChildren:
		return schema.ListNestedAttribute{
			Required:            f.Required,
			Optional:            !f.Required,
			MarkdownDescription: desc,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"id":           schema.StringAttribute{Required: true},
					"display_name": schema.StringAttribute{Optional: true},
					"node_type":    schema.StringAttribute{Required: true},
					"node_version": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("1.0")},
				},
			},
		}
	default:
		return schema.StringAttribute{Optional: true, MarkdownDescription: desc}
	}
}

func stringListValue(def any) types.List {
	var els []attr.Value
	switch t := def.(type) {
	case []any:
		for _, item := range t {
			s, _ := item.(string)
			els = append(els, types.StringValue(s))
		}
	case []string:
		for _, s := range t {
			els = append(els, types.StringValue(s))
		}
	}
	return types.ListValueMust(types.StringType, els)
}

func (r *nodeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("%T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *nodeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tf, diags := valuesFromConfig(ctx, r.spec, req.Plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := uuid.NewString()
	r.apply(ctx, id, tf, &resp.Diagnostics, func(realm, id string, decoded map[string]any) {
		resp.Diagnostics.Append(setNodeState(ctx, &resp.State, r.spec, realm, id, decoded)...)
	})
}

func (r *nodeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var id, realm types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("realm"), &realm)...)
	if resp.Diagnostics.HasError() {
		return
	}
	raw, err := r.client.GetNode(ctx, realm.ValueString(), r.spec.APIType, id.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read "+r.spec.APIType, err.Error())
		return
	}
	decoded, err := nodetype.DecodeAPI(r.spec, raw, r.client.Prefix)
	if err != nil {
		resp.Diagnostics.AddError("Unmodelled "+r.spec.APIType+" field", err.Error())
		return
	}
	resp.Diagnostics.Append(setNodeState(ctx, &resp.State, r.spec, realm.ValueString(), id.ValueString(), decoded)...)
}

func (r *nodeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tf, diags := valuesFromConfig(ctx, r.spec, req.Plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, _ := tf["id"].(string)
	r.apply(ctx, id, tf, &resp.Diagnostics, func(realm, id string, decoded map[string]any) {
		resp.Diagnostics.Append(setNodeState(ctx, &resp.State, r.spec, realm, id, decoded)...)
	})
}

func (r *nodeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var id, realm types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("realm"), &realm)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteNode(ctx, realm.ValueString(), r.spec.APIType, id.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete "+r.spec.APIType, err.Error())
	}
}

func (r *nodeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import id", "Use realm/<node-uuid>.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("realm"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func (r *nodeResource) apply(ctx context.Context, id string, tf map[string]any, diags *diag.Diagnostics, set func(realm, id string, decoded map[string]any)) {
	realm, _ := tf["realm"].(string)
	body, err := nodetype.EncodeAPI(r.spec, tf, r.client.Prefix)
	if err != nil {
		diags.AddError("Encode "+r.spec.APIType, err.Error())
		return
	}
	raw, err := r.client.PutNode(ctx, realm, r.spec.APIType, id, body)
	if err != nil {
		diags.AddError("Write "+r.spec.APIType, err.Error())
		return
	}
	decoded, err := nodetype.DecodeAPI(r.spec, raw, r.client.Prefix)
	if err != nil {
		diags.AddError("Unmodelled "+r.spec.APIType+" field after write", err.Error())
		return
	}
	set(realm, id, decoded)
}

func valuesFromConfig(ctx context.Context, spec nodetype.Spec, cfg tfsdk.Plan) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := map[string]any{}

	var realm, id types.String
	diags.Append(cfg.GetAttribute(ctx, path.Root("realm"), &realm)...)
	diags.Append(cfg.GetAttribute(ctx, path.Root("id"), &id)...)
	out["realm"] = realm.ValueString()
	if !id.IsNull() && !id.IsUnknown() {
		out["id"] = id.ValueString()
	}

	for _, f := range spec.Fields {
		p := path.Root(f.TFName)
		switch f.Kind {
		case nodetype.KindString, nodetype.KindESVString:
			var v types.String
			diags.Append(cfg.GetAttribute(ctx, p, &v)...)
			if !v.IsNull() && !v.IsUnknown() {
				out[f.TFName] = v.ValueString()
			}
		case nodetype.KindBool:
			var v types.Bool
			diags.Append(cfg.GetAttribute(ctx, p, &v)...)
			if !v.IsNull() && !v.IsUnknown() {
				out[f.TFName] = v.ValueBool()
			}
		case nodetype.KindInt:
			var v types.Int64
			diags.Append(cfg.GetAttribute(ctx, p, &v)...)
			if !v.IsNull() && !v.IsUnknown() {
				out[f.TFName] = v.ValueInt64()
			}
		case nodetype.KindStringList:
			var v types.List
			diags.Append(cfg.GetAttribute(ctx, p, &v)...)
			if !v.IsNull() && !v.IsUnknown() {
				var items []string
				diags.Append(v.ElementsAs(ctx, &items, false)...)
				out[f.TFName] = items
			}
		case nodetype.KindStringMap:
			var v types.Map
			diags.Append(cfg.GetAttribute(ctx, p, &v)...)
			if !v.IsNull() && !v.IsUnknown() {
				var items map[string]string
				diags.Append(v.ElementsAs(ctx, &items, false)...)
				out[f.TFName] = items
			}
		case nodetype.KindChildren:
			var v types.List
			diags.Append(cfg.GetAttribute(ctx, p, &v)...)
			if !v.IsNull() && !v.IsUnknown() {
				var children []nodetype.PageChild
				for _, el := range v.Elements() {
					obj, ok := el.(types.Object)
					if !ok {
						continue
					}
					attrs := obj.Attributes()
					str := func(k string) string {
						s, _ := attrs[k].(types.String)
						return s.ValueString()
					}
					children = append(children, nodetype.PageChild{
						ID:          str("id"),
						DisplayName: str("display_name"),
						NodeType:    str("node_type"),
						NodeVersion: str("node_version"),
					})
				}
				out[f.TFName] = children
			}
		}
	}
	return out, diags
}

func setNodeState(ctx context.Context, state *tfsdk.State, spec nodetype.Spec, realm, id string, decoded map[string]any) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.Append(state.SetAttribute(ctx, path.Root("id"), id)...)
	diags.Append(state.SetAttribute(ctx, path.Root("realm"), realm)...)
	diags.Append(state.SetAttribute(ctx, path.Root("node_type"), spec.APIType)...)

	for _, f := range spec.Fields {
		p := path.Root(f.TFName)
		v, ok := decoded[f.TFName]
		if !ok {
			continue
		}
		switch f.Kind {
		case nodetype.KindString, nodetype.KindESVString:
			s, _ := v.(string)
			if s == "" && !f.Required {
				diags.Append(state.SetAttribute(ctx, p, types.StringNull())...)
			} else {
				diags.Append(state.SetAttribute(ctx, p, s)...)
			}
		case nodetype.KindBool:
			b, _ := v.(bool)
			diags.Append(state.SetAttribute(ctx, p, b)...)
		case nodetype.KindInt:
			n, _ := v.(int64)
			diags.Append(state.SetAttribute(ctx, p, n)...)
		case nodetype.KindStringList:
			items, _ := v.([]string)
			diags.Append(state.SetAttribute(ctx, p, items)...)
		case nodetype.KindStringMap:
			items, _ := v.(map[string]string)
			if items == nil {
				items = map[string]string{}
			}
			diags.Append(state.SetAttribute(ctx, p, items)...)
		case nodetype.KindChildren:
			children, _ := v.([]nodetype.PageChild)
			objType := types.ObjectType{AttrTypes: map[string]attr.Type{
				"id":           types.StringType,
				"display_name": types.StringType,
				"node_type":    types.StringType,
				"node_version": types.StringType,
			}}
			els := make([]attr.Value, 0, len(children))
			for _, ch := range children {
				obj, d := types.ObjectValue(objType.AttrTypes, map[string]attr.Value{
					"id":           types.StringValue(ch.ID),
					"display_name": types.StringValue(ch.DisplayName),
					"node_type":    types.StringValue(ch.NodeType),
					"node_version": types.StringValue(ch.NodeVersion),
				})
				diags.Append(d...)
				els = append(els, obj)
			}
			list, d := types.ListValue(objType, els)
			diags.Append(d...)
			diags.Append(state.SetAttribute(ctx, p, list)...)
		}
	}
	return diags
}
