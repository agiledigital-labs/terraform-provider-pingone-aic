package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &idmScheduleResource{}
	_ resource.ResourceWithConfigure   = &idmScheduleResource{}
	_ resource.ResourceWithImportState = &idmScheduleResource{}
)

func NewIDMScheduleResource() resource.Resource { return &idmScheduleResource{} }

type idmScheduleResource struct{ client *client.Client }

type idmScheduleModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	RemoteName          types.String `tfsdk:"remote_name"`
	Enabled             types.Bool   `tfsdk:"enabled"`
	Persisted           types.Bool   `tfsdk:"persisted"`
	Type                types.String `tfsdk:"type"`
	Cron                types.String `tfsdk:"schedule"`
	InvokeService       types.String `tfsdk:"invoke_service"`
	InvokeLogLevel      types.String `tfsdk:"invoke_log_level"`
	MisfirePolicy       types.String `tfsdk:"misfire_policy"`
	ConcurrentExecution types.Bool   `tfsdk:"concurrent_execution"`
	Recoverable         types.Bool   `tfsdk:"recoverable"`
	IsCron              types.Bool   `tfsdk:"is_cron"`
	RepeatCount         types.Int64  `tfsdk:"repeat_count"`
	RepeatInterval      types.Int64  `tfsdk:"repeat_interval"`
	StartTime           types.String `tfsdk:"start_time"`
	EndTime             types.String `tfsdk:"end_time"`
	ManagedObject       types.String `tfsdk:"managed_object"`
	ScriptProperty      types.String `tfsdk:"script_property"`
	InvokeContextType   types.String `tfsdk:"invoke_context_type"`
	Source              types.String `tfsdk:"source"`
	ScriptType          types.String `tfsdk:"script_type"`
	Globals             types.Map    `tfsdk:"globals"`
	NumberOfThreads     types.Int64  `tfsdk:"number_of_threads"`
	WaitForCompletion   types.Bool   `tfsdk:"wait_for_completion"`
	ScanObject          types.String `tfsdk:"scan_object"`
	ScanQueryFilter     types.String `tfsdk:"scan_query_filter"`
	TaskStarted         types.String `tfsdk:"task_started"`
	TaskCompleted       types.String `tfsdk:"task_completed"`
	RecoveryTimeout     types.String `tfsdk:"recovery_timeout"`
}

func (r *idmScheduleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_idm_schedule"
}

func (r *idmScheduleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An IDM scheduled job (`/openidm/config/schedule/{name}`). Tenant-global; no realm. " +
			"Scripted jobs (`invoke_service = script`) store plaintext at `source`. Taskscanner jobs put the " +
			"same `source` under the scan task and also set scan_*. **Copies should keep `enabled = false`** " +
			"so they cannot fire on cron.",
		Attributes: map[string]schema.Attribute{
			"id":                   schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":                 schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"remote_name":          schema.StringAttribute{Computed: true},
			"enabled":              schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Default false so generated copies do not fire."},
			"persisted":            schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(client.DefaultSchedulePersisted)},
			"type":                 schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("cron")},
			"schedule":             schema.StringAttribute{Optional: true, MarkdownDescription: "Quartz cron string."},
			"invoke_service":       schema.StringAttribute{Required: true, MarkdownDescription: "script | taskscanner | org.forgerock.openidm.script"},
			"invoke_log_level":     schema.StringAttribute{Optional: true},
			"misfire_policy":       schema.StringAttribute{Optional: true},
			"concurrent_execution": schema.BoolAttribute{Optional: true},
			"recoverable":          schema.BoolAttribute{Optional: true},
			"is_cron":              schema.BoolAttribute{Optional: true},
			"repeat_count":         schema.Int64Attribute{Optional: true},
			"repeat_interval":      schema.Int64Attribute{Optional: true},
			"start_time":           schema.StringAttribute{Optional: true},
			"end_time":             schema.StringAttribute{Optional: true},
			"managed_object":       schema.StringAttribute{Optional: true},
			"script_property":      schema.StringAttribute{Optional: true},
			"invoke_context_type":  schema.StringAttribute{Optional: true},
			"source":               schema.StringAttribute{Optional: true, MarkdownDescription: "Plaintext JavaScript. Inline a string or, preferably, `source = file(\"${path.module}/schedules/foo.js\")`."},
			"script_type":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("text/javascript")},
			"globals":              schema.MapAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "String-valued globals exposed to the schedule script."},
			"number_of_threads":    schema.Int64Attribute{Optional: true},
			"wait_for_completion":  schema.BoolAttribute{Optional: true},
			"scan_object":          schema.StringAttribute{Optional: true},
			"scan_query_filter":    schema.StringAttribute{Optional: true},
			"task_started":         schema.StringAttribute{Optional: true},
			"task_completed":       schema.StringAttribute{Optional: true},
			"recovery_timeout":     schema.StringAttribute{Optional: true},
		},
	}
}

func (r *idmScheduleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *idmScheduleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan idmScheduleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.write(ctx, prefix.Apply(r.client.Prefix, plan.Name.ValueString()), plan)
	if err != nil {
		resp.Diagnostics.AddError("Create idm schedule", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func (r *idmScheduleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state idmScheduleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetSchedule(ctx, configRemoteName(state.ID, state.RemoteName, r.client.Prefix, state.Name.ValueString()))
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read idm schedule", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, scheduleToModel(got, state.Name.ValueString(), r.client.Prefix))...)
}

func (r *idmScheduleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior idmScheduleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.write(ctx, configRemoteName(prior.ID, prior.RemoteName, r.client.Prefix, plan.Name.ValueString()), plan)
	if err != nil {
		resp.Diagnostics.AddError("Update idm schedule", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func (r *idmScheduleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state idmScheduleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteSchedule(ctx, configRemoteName(state.ID, state.RemoteName, r.client.Prefix, state.Name.ValueString())); err != nil {
		resp.Diagnostics.AddError("Delete idm schedule", err.Error())
	}
}

func (r *idmScheduleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	name := strings.TrimPrefix(strings.TrimSpace(req.ID), "schedule/")
	if name == "" {
		resp.Diagnostics.AddError("Invalid import id", "Use the schedule name (with or without the schedule/ prefix).")
		return
	}
	remote := name
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), remote)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("remote_name"), remote)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), prefix.Strip(r.client.Prefix, remote))...)
}

func (r *idmScheduleResource) write(ctx context.Context, remote string, plan idmScheduleModel) (idmScheduleModel, error) {
	got, err := r.client.PutSchedule(ctx, remote, modelToSchedule(plan))
	if err != nil {
		return idmScheduleModel{}, err
	}
	return scheduleToModel(got, plan.Name.ValueString(), r.client.Prefix), nil
}

func modelToSchedule(plan idmScheduleModel) client.Schedule {
	return client.Schedule{
		Name:                plan.Name.ValueString(),
		Enabled:             plan.Enabled.ValueBool(),
		Persisted:           plan.Persisted.ValueBool(),
		Type:                plan.Type.ValueString(),
		Cron:                plan.Cron.ValueString(),
		InvokeService:       plan.InvokeService.ValueString(),
		InvokeLogLevel:      plan.InvokeLogLevel.ValueString(),
		MisfirePolicy:       plan.MisfirePolicy.ValueString(),
		ConcurrentExecution: boolFromAttr(plan.ConcurrentExecution),
		Recoverable:         boolFromAttr(plan.Recoverable),
		IsCron:              boolFromAttr(plan.IsCron),
		RepeatCount:         intFromAttr(plan.RepeatCount),
		RepeatInterval:      intFromAttr(plan.RepeatInterval),
		StartTime:           plan.StartTime.ValueString(),
		EndTime:             plan.EndTime.ValueString(),
		ManagedObject:       plan.ManagedObject.ValueString(),
		ScriptProperty:      plan.ScriptProperty.ValueString(),
		InvokeContextType:   plan.InvokeContextType.ValueString(),
		Source:              plan.Source.ValueString(),
		ScriptType:          plan.ScriptType.ValueString(),
		Globals:             stringMapFromAttr(plan.Globals),
		NumberOfThreads:     intFromAttr(plan.NumberOfThreads),
		WaitForCompletion:   boolFromAttr(plan.WaitForCompletion),
		ScanObject:          plan.ScanObject.ValueString(),
		ScanQueryFilter:     plan.ScanQueryFilter.ValueString(),
		TaskStarted:         plan.TaskStarted.ValueString(),
		TaskCompleted:       plan.TaskCompleted.ValueString(),
		RecoveryTimeout:     plan.RecoveryTimeout.ValueString(),
	}
}

func scheduleToModel(s *client.Schedule, logical, pfx string) idmScheduleModel {
	name := logical
	if name == "" {
		name = prefix.Strip(pfx, s.Name)
	}
	return idmScheduleModel{
		ID:                  types.StringValue(s.Name),
		Name:                types.StringValue(name),
		RemoteName:          types.StringValue(s.Name),
		Enabled:             types.BoolValue(s.Enabled),
		Persisted:           types.BoolValue(s.Persisted),
		Type:                types.StringValue(s.Type),
		Cron:                stringOrNull(s.Cron),
		InvokeService:       types.StringValue(s.InvokeService),
		InvokeLogLevel:      stringOrNull(s.InvokeLogLevel),
		MisfirePolicy:       stringOrNull(s.MisfirePolicy),
		ConcurrentExecution: boolOrNull(s.ConcurrentExecution),
		Recoverable:         boolOrNull(s.Recoverable),
		IsCron:              boolOrNull(s.IsCron),
		RepeatCount:         intOrNull(s.RepeatCount),
		RepeatInterval:      intOrNull(s.RepeatInterval),
		StartTime:           stringOrNull(s.StartTime),
		EndTime:             stringOrNull(s.EndTime),
		ManagedObject:       stringOrNull(s.ManagedObject),
		ScriptProperty:      stringOrNull(s.ScriptProperty),
		InvokeContextType:   stringOrNull(s.InvokeContextType),
		Source:              stringOrNull(s.Source),
		ScriptType:          types.StringValue(firstNonEmpty(s.ScriptType, "text/javascript")),
		Globals:             stringMapOrNull(s.Globals),
		NumberOfThreads:     intOrNull(s.NumberOfThreads),
		WaitForCompletion:   boolOrNull(s.WaitForCompletion),
		ScanObject:          stringOrNull(s.ScanObject),
		ScanQueryFilter:     stringOrNull(s.ScanQueryFilter),
		TaskStarted:         stringOrNull(s.TaskStarted),
		TaskCompleted:       stringOrNull(s.TaskCompleted),
		RecoveryTimeout:     stringOrNull(s.RecoveryTimeout),
	}
}

func stringMapFromAttr(v types.Map) map[string]string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	out := make(map[string]string, len(v.Elements()))
	_ = v.ElementsAs(context.Background(), &out, false)
	return out
}

func stringMapOrNull(v map[string]string) types.Map {
	if v == nil {
		return types.MapNull(types.StringType)
	}
	out, _ := types.MapValueFrom(context.Background(), types.StringType, v)
	return out
}

func intFromAttr(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	n := v.ValueInt64()
	return &n
}

func intOrNull(v *int64) types.Int64 {
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*v)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
