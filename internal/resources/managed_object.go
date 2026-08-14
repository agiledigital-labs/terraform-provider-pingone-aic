package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/managedobject"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &managedObjectResource{}
	_ resource.ResourceWithConfigure   = &managedObjectResource{}
	_ resource.ResourceWithImportState = &managedObjectResource{}
)

func NewManagedObjectResource() resource.Resource { return &managedObjectResource{} }

type managedObjectResource struct {
	client *client.Client
}

type managedObjectModel struct {
	ID          types.String           `tfsdk:"id"`
	Name        types.String           `tfsdk:"name"`
	RemoteName  types.String           `tfsdk:"remote_name"`
	Title       types.String           `tfsdk:"title"`
	Description types.String           `tfsdk:"description"`
	Icon        types.String           `tfsdk:"icon"`
	IconClass   types.String           `tfsdk:"icon_class"`
	Properties  []managedPropertyModel `tfsdk:"property"`
}

type managedPropertyModel struct {
	Name                types.String  `tfsdk:"name"`
	Type                types.String  `tfsdk:"type"`
	Title               types.String  `tfsdk:"title"`
	Description         types.String  `tfsdk:"description"`
	Required            types.Bool    `tfsdk:"required"`
	Searchable          types.Bool    `tfsdk:"searchable"`
	Viewable            types.Bool    `tfsdk:"viewable"`
	UserEditable        types.Bool    `tfsdk:"user_editable"`
	ResourcePath        types.String  `tfsdk:"resource_path"`
	ResourceLabel       types.String  `tfsdk:"resource_label"`
	ReversePropertyName types.String  `tfsdk:"reverse_property_name"`
	ReverseRelationship types.Bool    `tfsdk:"reverse_relationship"`
	Validate            types.Bool    `tfsdk:"validate"`
	Enum                types.List    `tfsdk:"enum"`
	Minimum             types.Float64 `tfsdk:"minimum"`
	Maximum             types.Float64 `tfsdk:"maximum"`
	ItemsType           types.String  `tfsdk:"items_type"`
	ItemsResourcePath   types.String  `tfsdk:"items_resource_path"`
	ItemsReverseName    types.String  `tfsdk:"items_reverse_property_name"`
	ItemsReverseRel     types.Bool    `tfsdk:"items_reverse_relationship"`
	ItemsValidate       types.Bool    `tfsdk:"items_validate"`
}

func (r *managedObjectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_object"
}

func (r *managedObjectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	propAttrs := map[string]schema.Attribute{
		"name":                        schema.StringAttribute{Required: true},
		"type":                        schema.StringAttribute{Required: true, MarkdownDescription: "string | number | boolean | array | relationship"},
		"title":                       schema.StringAttribute{Optional: true},
		"description":                 schema.StringAttribute{Optional: true},
		"required":                    schema.BoolAttribute{Optional: true},
		"searchable":                  schema.BoolAttribute{Optional: true},
		"viewable":                    schema.BoolAttribute{Optional: true},
		"user_editable":               schema.BoolAttribute{Optional: true},
		"resource_path":               schema.StringAttribute{Optional: true, MarkdownDescription: "Relationship target, e.g. `managed/test_to`. Prefixed on the wire."},
		"resource_label":              schema.StringAttribute{Optional: true},
		"reverse_property_name":       schema.StringAttribute{Optional: true},
		"reverse_relationship":        schema.BoolAttribute{Optional: true},
		"validate":                    schema.BoolAttribute{Optional: true},
		"enum":                        schema.ListAttribute{Optional: true, ElementType: types.StringType},
		"minimum":                     schema.Float64Attribute{Optional: true},
		"maximum":                     schema.Float64Attribute{Optional: true},
		"items_type":                  schema.StringAttribute{Optional: true},
		"items_resource_path":         schema.StringAttribute{Optional: true},
		"items_reverse_property_name": schema.StringAttribute{Optional: true},
		"items_reverse_relationship":  schema.BoolAttribute{Optional: true},
		"items_validate":              schema.BoolAttribute{Optional: true},
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "One IDM managed-object **type** in `/openidm/config/managed`. " +
			"Writes are read-modify-write of the whole document and are re-read until the change is visible — " +
			"a 200 on PUT is not enough (Q14). Other types in the tenant are left untouched. " +
			"`name` is the logical type name; `resource_prefix` is prepended on the wire, and `managed/` " +
			"relationship paths are prefixed the same way.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":        schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"remote_name": schema.StringAttribute{Computed: true},
			"title":       schema.StringAttribute{Optional: true},
			"description": schema.StringAttribute{Optional: true},
			"icon":        schema.StringAttribute{Optional: true},
			"icon_class":  schema.StringAttribute{Optional: true},
		},
		Blocks: map[string]schema.Block{
			"property": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{Attributes: propAttrs},
			},
		},
	}
}

func (r *managedObjectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *managedObjectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan managedObjectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	remote := prefix.Apply(r.client.Prefix, plan.Name.ValueString())
	if err := r.write(ctx, remote, plan, false); err != nil {
		resp.Diagnostics.AddError("Create managed object", err.Error())
		return
	}
	got, err := r.readModel(ctx, plan.Name.ValueString(), remote)
	if err != nil {
		resp.Diagnostics.AddError("Read managed object after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func (r *managedObjectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state managedObjectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	remote := managedRemoteName(state, r.client.Prefix)
	got, err := r.readModel(ctx, state.Name.ValueString(), remote)
	if client.IsNotFound(err) || (err == nil && got.ID.IsNull()) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read managed object", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func (r *managedObjectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior managedObjectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	remote := managedRemoteName(prior, r.client.Prefix)
	if err := r.write(ctx, remote, plan, true); err != nil {
		resp.Diagnostics.AddError("Update managed object", err.Error())
		return
	}
	got, err := r.readModel(ctx, plan.Name.ValueString(), remote)
	if err != nil {
		resp.Diagnostics.AddError("Read managed object after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func (r *managedObjectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state managedObjectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	remote := managedRemoteName(state, r.client.Prefix)
	err := r.client.MutateManaged(ctx, func(doc map[string]any) (map[string]any, []client.ManagedConfirm, error) {
		next, err := client.RemoveManagedObject(doc, remote)
		if err != nil {
			return nil, nil, err
		}
		return next, []client.ManagedConfirm{{Name: remote, Absent: true}}, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Delete managed object", err.Error())
	}
}

func (r *managedObjectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := strings.TrimSpace(req.ID)
	if id == "" {
		resp.Diagnostics.AddError("Invalid import id", "Use the managed object type name.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("remote_name"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), prefix.Strip(r.client.Prefix, id))...)
}

func (r *managedObjectResource) write(ctx context.Context, remote string, plan managedObjectModel, replace bool) error {
	return r.client.MutateManaged(ctx, func(doc map[string]any) (map[string]any, []client.ManagedConfirm, error) {
		if !replace {
			if _, found, err := client.FindManagedObject(doc, remote); err != nil {
				return nil, nil, err
			} else if found {
				return nil, nil, fmt.Errorf("managed object %q already exists", remote)
			}
		}
		obj := managedobject.EncodeAPI(modelToManaged(plan), r.client.Prefix)
		obj["name"] = remote
		next, err := client.SetManagedObject(doc, obj)
		if err != nil {
			return nil, nil, err
		}
		return next, []client.ManagedConfirm{{Name: remote}}, nil
	})
}

func (r *managedObjectResource) readModel(ctx context.Context, logical, remote string) (managedObjectModel, error) {
	doc, err := r.client.GetManaged(ctx)
	if err != nil {
		return managedObjectModel{}, err
	}
	raw, found, err := client.FindManagedObject(doc, remote)
	if err != nil {
		return managedObjectModel{}, err
	}
	if !found {
		return managedObjectModel{}, nil
	}
	decoded, err := managedobject.DecodeAPI(raw, r.client.Prefix)
	if err != nil {
		return managedObjectModel{}, err
	}
	return managedToModel(decoded, logical, remote), nil
}

func managedRemoteName(state managedObjectModel, pfx string) string {
	for _, v := range []types.String{state.ID, state.RemoteName} {
		if !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
			return v.ValueString()
		}
	}
	return prefix.Apply(pfx, state.Name.ValueString())
}

func modelToManaged(plan managedObjectModel) managedobject.Object {
	var required []string
	props := make([]managedobject.Property, 0, len(plan.Properties))
	for _, p := range plan.Properties {
		if p.Required.ValueBool() {
			required = append(required, p.Name.ValueString())
		}
		prop := managedobject.Property{
			Name:                p.Name.ValueString(),
			Type:                p.Type.ValueString(),
			Title:               p.Title.ValueString(),
			Description:         p.Description.ValueString(),
			Searchable:          boolFromAttr(p.Searchable),
			Viewable:            boolFromAttr(p.Viewable),
			UserEditable:        boolFromAttr(p.UserEditable),
			Validate:            boolFromAttr(p.Validate),
			ReversePropertyName: p.ReversePropertyName.ValueString(),
			ReverseRelationship: boolFromAttr(p.ReverseRelationship),
			ResourcePath:        p.ResourcePath.ValueString(),
			ResourceLabel:       p.ResourceLabel.ValueString(),
			Enum:                listStrings(p.Enum),
		}
		if !p.Minimum.IsNull() && !p.Minimum.IsUnknown() {
			v := p.Minimum.ValueFloat64()
			prop.Minimum = &v
		}
		if !p.Maximum.IsNull() && !p.Maximum.IsUnknown() {
			v := p.Maximum.ValueFloat64()
			prop.Maximum = &v
		}
		if p.ItemsType.ValueString() != "" {
			item := managedobject.Property{
				Type:                p.ItemsType.ValueString(),
				ResourcePath:        p.ItemsResourcePath.ValueString(),
				ReversePropertyName: p.ItemsReverseName.ValueString(),
				ReverseRelationship: boolFromAttr(p.ItemsReverseRel),
				Validate:            boolFromAttr(p.ItemsValidate),
			}
			prop.Items = &item
		}
		props = append(props, prop)
	}
	return managedobject.Object{
		Name:        plan.Name.ValueString(),
		Title:       plan.Title.ValueString(),
		Description: plan.Description.ValueString(),
		Icon:        plan.Icon.ValueString(),
		IconClass:   plan.IconClass.ValueString(),
		Required:    required,
		Properties:  props,
	}
}

func managedToModel(o *managedobject.Object, logical, remote string) managedObjectModel {
	name := logical
	if name == "" {
		name = o.Name
	}
	req := map[string]bool{}
	for _, r := range o.Required {
		req[r] = true
	}
	props := make([]managedPropertyModel, 0, len(o.Properties))
	for _, p := range o.Properties {
		pm := managedPropertyModel{
			Name:                types.StringValue(p.Name),
			Type:                types.StringValue(p.Type),
			Title:               stringOrNull(p.Title),
			Description:         stringOrNull(p.Description),
			Required:            boolOrNull(boolIf(req[p.Name])),
			Searchable:          boolOrNull(p.Searchable),
			Viewable:            boolOrNull(p.Viewable),
			UserEditable:        boolOrNull(p.UserEditable),
			ResourcePath:        stringOrNull(p.ResourcePath),
			ResourceLabel:       stringOrNull(p.ResourceLabel),
			ReversePropertyName: stringOrNull(p.ReversePropertyName),
			ReverseRelationship: boolOrNull(p.ReverseRelationship),
			Validate:            boolOrNull(p.Validate),
			Enum:                stringListOrNull(p.Enum),
			Minimum:             floatOrNull(p.Minimum),
			Maximum:             floatOrNull(p.Maximum),
		}
		if p.Items != nil {
			pm.ItemsType = stringOrNull(p.Items.Type)
			pm.ItemsResourcePath = stringOrNull(p.Items.ResourcePath)
			pm.ItemsReverseName = stringOrNull(p.Items.ReversePropertyName)
			pm.ItemsReverseRel = boolOrNull(p.Items.ReverseRelationship)
			pm.ItemsValidate = boolOrNull(p.Items.Validate)
		}
		props = append(props, pm)
	}
	return managedObjectModel{
		ID:          types.StringValue(remote),
		Name:        types.StringValue(name),
		RemoteName:  types.StringValue(remote),
		Title:       stringOrNull(o.Title),
		Description: stringOrNull(o.Description),
		Icon:        stringOrNull(o.Icon),
		IconClass:   stringOrNull(o.IconClass),
		Properties:  props,
	}
}

func boolIf(v bool) *bool {
	if !v {
		return nil
	}
	b := true
	return &b
}

func boolFromAttr(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

func boolOrNull(v *bool) types.Bool {
	if v == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*v)
}

func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func floatOrNull(v *float64) types.Float64 {
	if v == nil {
		return types.Float64Null()
	}
	return types.Float64Value(*v)
}

func listStrings(v types.List) []string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	var out []string
	_ = v.ElementsAs(context.Background(), &out, false)
	return out
}

func stringListOrNull(items []string) types.List {
	if len(items) == 0 {
		return types.ListNull(types.StringType)
	}
	return stringListValue(items)
}
