package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/oauth2client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &oauth2ClientResource{}
	_ resource.ResourceWithConfigure   = &oauth2ClientResource{}
	_ resource.ResourceWithImportState = &oauth2ClientResource{}
)

func NewOAuth2ClientResource() resource.Resource { return &oauth2ClientResource{} }

type oauth2ClientResource struct {
	client *client.Client
}

func (r *oauth2ClientResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_oauth2_client"
}

func (r *oauth2ClientResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Same as `remote_name` (the AIC client_id).",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"realm": schema.StringAttribute{
			Required:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		"name": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "Logical client_id (without the provider prefix). AM keys clients by this id and cannot rename one.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		"remote_name": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "client_id stored in AIC, including `resource_prefix`.",
		},
	}
	for _, g := range oauth2client.AllGroups() {
		inner := make(map[string]schema.Attribute, len(g.Fields))
		for _, f := range g.Fields {
			inner[f.TFName] = oauth2FieldSchema(f)
		}
		attrs[g.TFName] = schema.SingleNestedAttribute{
			Optional:            true,
			Computed:            true,
			MarkdownDescription: g.Doc,
			Attributes:          inner,
			Default:             objectdefault.StaticValue(oauth2GroupDefaultObject(g)),
		}
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "An AM OAuth2 client (`agents/OAuth2Client`). `name` is the logical client_id; " +
			"the provider prepends `resource_prefix` when talking to AIC so apply creates a copy. " +
			"Attributes are typed against the live tenant catalog; unknown API fields are an error. " +
			"`userpassword` is write-only — AM never returns it.",
		Attributes: attrs,
	}
}

func oauth2FieldSchema(f oauth2client.Field) schema.Attribute {
	desc := fmt.Sprintf("AM `%s`.", f.APIName)
	switch f.Kind {
	case oauth2client.KindString:
		attr := schema.StringAttribute{
			Optional:            true,
			Computed:            !f.Sensitive,
			Sensitive:           f.Sensitive,
			MarkdownDescription: desc,
		}
		if s, ok := f.Default.(string); ok && !f.Sensitive {
			attr.Default = stringdefault.StaticString(s)
		}
		return attr
	case oauth2client.KindBool:
		attr := schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: desc}
		if b, ok := f.Default.(bool); ok {
			attr.Default = booldefault.StaticBool(b)
		}
		return attr
	case oauth2client.KindInt:
		attr := schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: desc}
		if n, ok := asInt64Default(f.Default); ok {
			attr.Default = int64default.StaticInt64(n)
		}
		return attr
	case oauth2client.KindStringList:
		attr := schema.ListAttribute{
			Optional:            true,
			Computed:            true,
			ElementType:         types.StringType,
			MarkdownDescription: desc,
		}
		if f.Default != nil {
			attr.Default = listdefault.StaticValue(stringListValue(f.Default))
		}
		return attr
	default:
		return schema.StringAttribute{Optional: true, MarkdownDescription: desc}
	}
}

func asInt64Default(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	default:
		return 0, false
	}
}

func oauth2GroupAttrTypes(g oauth2client.Group) map[string]attr.Type {
	out := make(map[string]attr.Type, len(g.Fields))
	for _, f := range g.Fields {
		switch f.Kind {
		case oauth2client.KindBool:
			out[f.TFName] = types.BoolType
		case oauth2client.KindInt:
			out[f.TFName] = types.Int64Type
		case oauth2client.KindStringList:
			out[f.TFName] = types.ListType{ElemType: types.StringType}
		default:
			out[f.TFName] = types.StringType
		}
	}
	return out
}

func oauth2GroupDefaultObject(g oauth2client.Group) types.Object {
	vals := make(map[string]attr.Value, len(g.Fields))
	for _, f := range g.Fields {
		vals[f.TFName] = oauth2DefaultAttr(f)
	}
	return types.ObjectValueMust(oauth2GroupAttrTypes(g), vals)
}

func oauth2DefaultAttr(f oauth2client.Field) attr.Value {
	switch f.Kind {
	case oauth2client.KindBool:
		b, _ := f.Default.(bool)
		return types.BoolValue(b)
	case oauth2client.KindInt:
		n, _ := asInt64Default(f.Default)
		return types.Int64Value(n)
	case oauth2client.KindStringList:
		return stringListValue(f.Default)
	default:
		if s, ok := f.Default.(string); ok {
			return types.StringValue(s)
		}
		return types.StringNull()
	}
}

func (r *oauth2ClientResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *oauth2ClientResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tf, password, diags := oauth2ValuesFromConfig(ctx, req.Plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	realm, name, id, decoded, err := r.write(ctx, types.StringNull(), types.StringNull(), tf, password)
	if err != nil {
		resp.Diagnostics.AddError("Write oauth2 client", err.Error())
		return
	}
	resp.Diagnostics.Append(setOAuth2State(ctx, &resp.State, realm, name, id, decoded, password)...)
}

func (r *oauth2ClientResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var id, realm, name, password types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("realm"), &realm)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("name"), &name)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("core").AtName("userpassword"), &password)...)
	if resp.Diagnostics.HasError() {
		return
	}
	raw, err := r.client.GetOAuth2Client(ctx, realm.ValueString(), id.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read oauth2 client", err.Error())
		return
	}
	vals, err := oauth2client.DecodeAPI(raw, r.client.Prefix)
	if err != nil {
		resp.Diagnostics.AddError("Unmodelled oauth2 client field", err.Error())
		return
	}
	resp.Diagnostics.Append(setOAuth2State(ctx, &resp.State, realm.ValueString(), name.ValueString(), id.ValueString(), vals, password.ValueString())...)
}

func (r *oauth2ClientResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tf, password, diags := oauth2ValuesFromConfig(ctx, req.Plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var priorID, priorRemote types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &priorID)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("remote_name"), &priorRemote)...)
	if resp.Diagnostics.HasError() {
		return
	}
	realm, name, id, decoded, err := r.write(ctx, priorID, priorRemote, tf, password)
	if err != nil {
		resp.Diagnostics.AddError("Write oauth2 client", err.Error())
		return
	}
	resp.Diagnostics.Append(setOAuth2State(ctx, &resp.State, realm, name, id, decoded, password)...)
}

func (r *oauth2ClientResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var id, realm types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("realm"), &realm)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteOAuth2Client(ctx, realm.ValueString(), id.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete oauth2 client", err.Error())
	}
}

func (r *oauth2ClientResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import id", "Use realm/<client-id> (the AIC client_id, with prefix if any).")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("realm"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), prefix.Strip(r.client.Prefix, parts[1]))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("remote_name"), parts[1])...)
}

func (r *oauth2ClientResource) write(ctx context.Context, priorID, priorRemote types.String, tf map[string]any, password string) (realm, logicalName, id string, decoded oauth2client.Values, err error) {
	realm, _ = tf["_realm"].(string)
	logicalName, _ = tf["_name"].(string)
	remote := applyRemote(priorID, priorRemote, r.client.Prefix, logicalName)
	vals := oauth2client.Values{}
	for k, v := range tf {
		if strings.HasPrefix(k, "_") {
			continue
		}
		group, ok := v.(map[string]any)
		if !ok {
			continue
		}
		vals[k] = group
	}
	if password != "" {
		if vals["core"] == nil {
			vals["core"] = map[string]any{}
		}
		vals["core"]["userpassword"] = password
	}
	body, err := oauth2client.EncodeAPI(vals, r.client.Prefix)
	if err != nil {
		return "", "", "", nil, err
	}
	raw, err := r.client.PutOAuth2Client(ctx, realm, remote, body)
	if err != nil {
		return "", "", "", nil, err
	}
	decoded, err = oauth2client.DecodeAPI(raw, r.client.Prefix)
	if err != nil {
		return "", "", "", nil, err
	}
	id, _ = raw["_id"].(string)
	if id == "" {
		id = remote
	}
	return realm, logicalName, id, decoded, nil
}

func oauth2ValuesFromConfig(ctx context.Context, cfg tfsdk.Plan) (map[string]any, string, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := map[string]any{}

	var realm, name types.String
	diags.Append(cfg.GetAttribute(ctx, path.Root("realm"), &realm)...)
	diags.Append(cfg.GetAttribute(ctx, path.Root("name"), &name)...)
	out["_realm"] = realm.ValueString()
	out["_name"] = name.ValueString()

	password := ""
	for _, g := range oauth2client.AllGroups() {
		var obj types.Object
		diags.Append(cfg.GetAttribute(ctx, path.Root(g.TFName), &obj)...)
		if obj.IsNull() || obj.IsUnknown() {
			continue
		}
		group := map[string]any{}
		attrs := obj.Attributes()
		for _, f := range g.Fields {
			v, ok := attrs[f.TFName]
			if !ok || v.IsNull() || v.IsUnknown() {
				continue
			}
			switch f.Kind {
			case oauth2client.KindString:
				s, _ := v.(types.String)
				if f.Sensitive {
					password = s.ValueString()
					continue
				}
				group[f.TFName] = s.ValueString()
			case oauth2client.KindBool:
				b, _ := v.(types.Bool)
				group[f.TFName] = b.ValueBool()
			case oauth2client.KindInt:
				n, _ := v.(types.Int64)
				group[f.TFName] = n.ValueInt64()
			case oauth2client.KindStringList:
				l, _ := v.(types.List)
				var items []string
				diags.Append(l.ElementsAs(ctx, &items, false)...)
				group[f.TFName] = items
			}
		}
		out[g.TFName] = group
	}
	return out, password, diags
}

func setOAuth2State(ctx context.Context, state *tfsdk.State, realm, logicalName, remote string, vals oauth2client.Values, password string) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.Append(state.SetAttribute(ctx, path.Root("id"), remote)...)
	diags.Append(state.SetAttribute(ctx, path.Root("realm"), realm)...)
	diags.Append(state.SetAttribute(ctx, path.Root("name"), logicalName)...)
	diags.Append(state.SetAttribute(ctx, path.Root("remote_name"), remote)...)

	for _, g := range oauth2client.AllGroups() {
		decoded := vals[g.TFName]
		attrVals := make(map[string]attr.Value, len(g.Fields))
		for _, f := range g.Fields {
			if f.Sensitive {
				if password != "" {
					attrVals[f.TFName] = types.StringValue(password)
				} else {
					attrVals[f.TFName] = types.StringNull()
				}
				continue
			}
			attrVals[f.TFName] = oauth2AttrFromDecoded(f, decoded[f.TFName])
		}
		obj, d := types.ObjectValue(oauth2GroupAttrTypes(g), attrVals)
		diags.Append(d...)
		diags.Append(state.SetAttribute(ctx, path.Root(g.TFName), obj)...)
	}
	return diags
}

func oauth2AttrFromDecoded(f oauth2client.Field, v any) attr.Value {
	switch f.Kind {
	case oauth2client.KindBool:
		b, _ := v.(bool)
		return types.BoolValue(b)
	case oauth2client.KindInt:
		n, ok := asInt64Default(v)
		if !ok {
			return types.Int64Value(0)
		}
		return types.Int64Value(n)
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
		return stringListValue(items)
	default:
		if v == nil {
			return types.StringNull()
		}
		s, ok := v.(string)
		if !ok || s == "" {
			return types.StringNull()
		}
		return types.StringValue(s)
	}
}
