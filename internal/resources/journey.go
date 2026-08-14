package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &journeyResource{}
	_ resource.ResourceWithConfigure   = &journeyResource{}
	_ resource.ResourceWithImportState = &journeyResource{}
)

func NewJourneyResource() resource.Resource { return &journeyResource{} }

type journeyResource struct {
	client *client.Client
}

type journeyModel struct {
	ID                 types.String       `tfsdk:"id"`
	Realm              types.String       `tfsdk:"realm"`
	Name               types.String       `tfsdk:"name"`
	RemoteName         types.String       `tfsdk:"remote_name"`
	Description        types.String       `tfsdk:"description"`
	Enabled            types.Bool         `tfsdk:"enabled"`
	IdentityResource   types.String       `tfsdk:"identity_resource"`
	InnerTreeOnly      types.Bool         `tfsdk:"inner_tree_only"`
	MustRun            types.Bool         `tfsdk:"must_run"`
	NoSession          types.Bool         `tfsdk:"no_session"`
	TransactionalOnly  types.Bool         `tfsdk:"transactional_only"`
	MaximumIdleTime    types.Int64        `tfsdk:"maximum_idle_time"`
	MaximumSessionTime types.Int64        `tfsdk:"maximum_session_time"`
	TreeTimeout        types.Int64        `tfsdk:"tree_timeout"`
	EntryNode          types.String       `tfsdk:"entry_node"`
	Categories         types.List         `tfsdk:"categories"`
	Nodes              []journeyNodeModel `tfsdk:"node"`
}

type journeyNodeModel struct {
	ID          types.String  `tfsdk:"id"`
	Type        types.String  `tfsdk:"type"`
	DisplayName types.String  `tfsdk:"display_name"`
	Connections types.Map     `tfsdk:"connections"`
	X           types.Float64 `tfsdk:"x"`
	Y           types.Float64 `tfsdk:"y"`
}

func (r *journeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_journey"
}

func (r *journeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An AM authentication tree (journey). Node *configuration* lives on the node " +
			"resources; this resource is the graph: entry node, connections, and tree-level flags.\n\n" +
			"Connection targets are node UUIDs, or the sentinels `success` and `failure` " +
			"(mapped to AM's built-in static nodes).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Same as `remote_name` (the AIC tree id).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"realm": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Logical journey name (without the provider prefix).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"remote_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Name stored in AIC, including `resource_prefix`.",
			},
			"description":        schema.StringAttribute{Optional: true},
			"enabled":            schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"identity_resource":  schema.StringAttribute{Optional: true, Computed: true},
			"inner_tree_only":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
			"must_run":           schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
			"no_session":         schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
			"transactional_only": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
			"maximum_idle_time": schema.Int64Attribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Maximum idle time reported by AM for the journey.",
			},
			"maximum_session_time": schema.Int64Attribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Maximum session time reported by AM for the journey.",
			},
			"tree_timeout": schema.Int64Attribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Authentication tree timeout reported by AM.",
			},
			"entry_node": schema.StringAttribute{Required: true, MarkdownDescription: "UUID of the first node."},
			"categories": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Optional journey categories (from `uiConfig.categories`).",
			},
		},
		Blocks: map[string]schema.Block{
			"node": schema.ListNestedBlock{
				MarkdownDescription: "One tree-graph entry per real node. Page-child widgets are not listed here.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id":           schema.StringAttribute{Required: true},
						"type":         schema.StringAttribute{Required: true, MarkdownDescription: "AM node type, e.g. `ScriptedDecisionNode`."},
						"display_name": schema.StringAttribute{Optional: true},
						"connections": schema.MapAttribute{
							Required:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "outcome → next node UUID, or `success` / `failure`.",
						},
						"x": schema.Float64Attribute{Optional: true},
						"y": schema.Float64Attribute{Optional: true},
					},
				},
			},
		},
	}
}

func (r *journeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *journeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan journeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.write(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Create journey", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func (r *journeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state journeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	remote := journeyRemoteName(state, r.client.Prefix)
	raw, err := r.client.GetTree(ctx, state.Realm.ValueString(), remote)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read journey", err.Error())
		return
	}
	got, err := treeToModel(raw, state, r.client.Prefix)
	if err != nil {
		resp.Diagnostics.AddError("Unmodelled journey field", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func (r *journeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan journeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.write(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Update journey", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func (r *journeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state journeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	remote := journeyRemoteName(state, r.client.Prefix)
	if err := r.client.DeleteTree(ctx, state.Realm.ValueString(), remote); err != nil {
		resp.Diagnostics.AddError("Delete journey", err.Error())
	}
}

func journeyRemoteName(state journeyModel, pfx string) string {
	if !state.ID.IsNull() && !state.ID.IsUnknown() && state.ID.ValueString() != "" {
		return state.ID.ValueString()
	}
	if !state.RemoteName.IsNull() && !state.RemoteName.IsUnknown() && state.RemoteName.ValueString() != "" {
		return state.RemoteName.ValueString()
	}
	return prefix.Apply(pfx, state.Name.ValueString())
}

func (r *journeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import id", "Use realm/<journey-name> (the AIC name, with prefix if any).")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("realm"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), prefix.Strip(r.client.Prefix, parts[1]))...)
}

func (r *journeyResource) write(ctx context.Context, plan journeyModel) (journeyModel, error) {
	remote := prefix.Apply(r.client.Prefix, plan.Name.ValueString())
	body, err := modelToTree(plan, r.client.Prefix)
	if err != nil {
		return journeyModel{}, err
	}
	raw, err := r.client.PutTree(ctx, plan.Realm.ValueString(), remote, body)
	if err != nil {
		return journeyModel{}, err
	}
	return treeToModel(raw, plan, r.client.Prefix)
}

func modelToTree(plan journeyModel, pfx string) (map[string]any, error) {
	identity := plan.IdentityResource.ValueString()
	if identity == "" {
		identity = "managed/" + plan.Realm.ValueString() + "_user"
	}

	nodes := map[string]any{}
	for i, n := range plan.Nodes {
		if n.ID.IsNull() || n.Type.IsNull() {
			return nil, fmt.Errorf("node[%d] missing id or type", i)
		}
		conns := map[string]string{}
		if !n.Connections.IsNull() && !n.Connections.IsUnknown() {
			if d := n.Connections.ElementsAs(context.Background(), &conns, false); d.HasError() {
				return nil, fmt.Errorf("node[%d] connections: %s", i, d.Errors()[0].Detail())
			}
		}
		wired := map[string]string{}
		for outcome, dest := range conns {
			wired[outcome] = resolveConn(dest)
		}
		entry := map[string]any{
			"connections": wired,
			"nodeType":    n.Type.ValueString(),
			"version":     "1.0",
		}
		if !n.DisplayName.IsNull() && n.DisplayName.ValueString() != "" {
			entry["displayName"] = n.DisplayName.ValueString()
		}
		if !n.X.IsNull() {
			entry["x"] = n.X.ValueFloat64()
		} else {
			entry["x"] = float64(80 + i*180)
		}
		if !n.Y.IsNull() {
			entry["y"] = n.Y.ValueFloat64()
		} else {
			entry["y"] = float64(200)
		}
		nodes[n.ID.ValueString()] = entry
	}

	ui := map[string]any{}
	if !plan.Categories.IsNull() && !plan.Categories.IsUnknown() {
		var cats []string
		_ = plan.Categories.ElementsAs(context.Background(), &cats, false)
		if len(cats) > 0 {
			b, _ := json.Marshal(cats)
			ui["categories"] = string(b)
		}
	}

	body := map[string]any{
		"enabled":           plan.Enabled.ValueBool(),
		"entryNodeId":       plan.EntryNode.ValueString(),
		"identityResource":  identity,
		"innerTreeOnly":     plan.InnerTreeOnly.ValueBool(),
		"mustRun":           plan.MustRun.ValueBool(),
		"noSession":         plan.NoSession.ValueBool(),
		"transactionalOnly": plan.TransactionalOnly.ValueBool(),
		"nodes":             nodes,
		"uiConfig":          ui,
		"staticNodes": map[string]any{
			client.SuccessNodeID: map[string]any{"x": 800.0, "y": 80.0},
			client.FailureNodeID: map[string]any{"x": 800.0, "y": 320.0},
			"startNode":          map[string]any{"x": 50.0, "y": 200.0},
		},
	}
	for name, value := range map[string]types.Int64{
		"maximumIdleTime":    plan.MaximumIdleTime,
		"maximumSessionTime": plan.MaximumSessionTime,
		"treeTimeout":        plan.TreeTimeout,
	} {
		if !value.IsNull() && !value.IsUnknown() {
			body[name] = value.ValueInt64()
		}
	}
	if !plan.Description.IsNull() && plan.Description.ValueString() != "" {
		body["description"] = plan.Description.ValueString()
	}
	_ = pfx
	return body, nil
}

func resolveConn(dest string) string {
	switch dest {
	case "success", "Success":
		return client.SuccessNodeID
	case "failure", "Failure":
		return client.FailureNodeID
	default:
		return dest
	}
}

func displayConn(dest string) string {
	switch dest {
	case client.SuccessNodeID:
		return "success"
	case client.FailureNodeID:
		return "failure"
	default:
		return dest
	}
}

func treeToModel(raw map[string]any, plan journeyModel, pfx string) (journeyModel, error) {
	// Refuse unmodelled top-level keys so an AIC tree-schema change is visible.
	if _, err := client.TreeWriteBody(raw); err != nil {
		return journeyModel{}, err
	}

	remote, _ := raw["_id"].(string)
	name := plan.Name.ValueString()
	if name == "" {
		name = prefix.Strip(pfx, remote)
	}

	desc := types.StringNull()
	if s, ok := raw["description"].(string); ok && s != "" {
		desc = types.StringValue(s)
	}
	identity, _ := raw["identityResource"].(string)
	entry, _ := raw["entryNodeId"].(string)

	cats := types.ListNull(types.StringType)
	if ui, ok := raw["uiConfig"].(map[string]any); ok {
		if cs, ok := ui["categories"].(string); ok && cs != "" && cs != "[]" {
			var parsed []string
			if err := json.Unmarshal([]byte(cs), &parsed); err != nil {
				return journeyModel{}, fmt.Errorf("uiConfig.categories is not a JSON string list: %w", err)
			}
			vals := make([]attr.Value, 0, len(parsed))
			for _, c := range parsed {
				vals = append(vals, types.StringValue(c))
			}
			l, d := types.ListValue(types.StringType, vals)
			if d.HasError() {
				return journeyModel{}, fmt.Errorf("categories: %s", d.Errors()[0].Detail())
			}
			cats = l
		}
	}

	var nodes []journeyNodeModel
	if rawNodes, ok := raw["nodes"].(map[string]any); ok {
		// Preserve plan order when possible so state diffs stay quiet.
		order := make([]string, 0, len(rawNodes))
		seen := map[string]struct{}{}
		for _, n := range plan.Nodes {
			if _, ok := rawNodes[n.ID.ValueString()]; ok {
				order = append(order, n.ID.ValueString())
				seen[n.ID.ValueString()] = struct{}{}
			}
		}
		for id := range rawNodes {
			if _, ok := seen[id]; !ok {
				order = append(order, id)
			}
		}
		for _, id := range order {
			meta, _ := rawNodes[id].(map[string]any)
			conns := map[string]string{}
			if c, ok := meta["connections"].(map[string]any); ok {
				for k, v := range c {
					s, _ := v.(string)
					conns[k] = displayConn(s)
				}
			}
			mv, _ := types.MapValueFrom(context.Background(), types.StringType, conns)
			node := journeyNodeModel{
				ID:          types.StringValue(id),
				Type:        types.StringValue(str(meta, "nodeType")),
				DisplayName: types.StringNull(),
				Connections: mv,
			}
			if dn := str(meta, "displayName"); dn != "" {
				node.DisplayName = types.StringValue(dn)
			}
			// Positions are visual chrome. Keep them in state only when the
			// user set them, so generated HCL can stay layout-free.
			for _, planned := range plan.Nodes {
				if planned.ID.ValueString() == id {
					node.X = planned.X
					node.Y = planned.Y
					break
				}
			}
			nodes = append(nodes, node)
		}
	}

	idRes := types.StringValue(identity)
	if identity == "managed/"+plan.Realm.ValueString()+"_user" && (plan.IdentityResource.IsNull() || plan.IdentityResource.ValueString() == "" || plan.IdentityResource.ValueString() == identity) {
		// Keep computed default visible; that's fine.
	}

	return journeyModel{
		ID:                 types.StringValue(remote),
		Realm:              plan.Realm,
		Name:               types.StringValue(name),
		RemoteName:         types.StringValue(remote),
		Description:        desc,
		Enabled:            types.BoolValue(boolish(raw["enabled"], true)),
		IdentityResource:   idRes,
		InnerTreeOnly:      types.BoolValue(boolish(raw["innerTreeOnly"], false)),
		MustRun:            types.BoolValue(boolish(raw["mustRun"], false)),
		NoSession:          types.BoolValue(boolish(raw["noSession"], false)),
		TransactionalOnly:  types.BoolValue(boolish(raw["transactionalOnly"], false)),
		MaximumIdleTime:    int64ish(raw["maximumIdleTime"]),
		MaximumSessionTime: int64ish(raw["maximumSessionTime"]),
		TreeTimeout:        int64ish(raw["treeTimeout"]),
		EntryNode:          types.StringValue(entry),
		Categories:         cats,
		Nodes:              nodes,
	}, nil
}

func int64ish(v any) types.Int64 {
	switch n := v.(type) {
	case float64:
		return types.Int64Value(int64(n))
	case int64:
		return types.Int64Value(n)
	case int:
		return types.Int64Value(int64(n))
	default:
		return types.Int64Null()
	}
}

func str(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func boolish(v any, def bool) bool {
	b, ok := v.(bool)
	if !ok {
		return def
	}
	return b
}

// silence unused in case treeToModel diagnostics helper is referenced
var _ = diag.Diagnostics{}
