package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type hashedRuleSpec[Model, Rule any] struct {
	typeSuffix   string
	label        string
	importPrefix string
	importHelp   string
	schema       schema.Schema
	get          func(context.Context, *client.Client) (map[string]any, error)
	mutate       func(context.Context, *client.Client, func(map[string]any) (map[string]any, client.RuleConfirm, error)) error
	objects      func(map[string]any) ([]map[string]any, error)
	decode       func(map[string]any) (*Rule, error)
	append       func(map[string]any, Rule) (map[string]any, client.RuleConfirm, error)
	replace      func(map[string]any, string, Rule) (map[string]any, client.RuleConfirm, error)
	remove       func(map[string]any, string) (map[string]any, client.RuleConfirm, error)
	modelToRule  func(Model) Rule
	ruleToModel  func(Rule, string) Model
}

func modelID(ctx context.Context, state tfsdk.State) (string, error) {
	var id types.String
	diags := state.GetAttribute(ctx, path.Root("id"), &id)
	if diags.HasError() {
		return "", fmt.Errorf("get id from state: %s", diags.Errors()[0].Detail())
	}
	return id.ValueString(), nil
}

type hashedRuleResource[Model, Rule any] struct {
	spec   hashedRuleSpec[Model, Rule]
	client *client.Client
}

var (
	_ resource.Resource                = &hashedRuleResource[accessRuleModel, client.AccessRule]{}
	_ resource.ResourceWithConfigure   = &hashedRuleResource[accessRuleModel, client.AccessRule]{}
	_ resource.ResourceWithImportState = &hashedRuleResource[accessRuleModel, client.AccessRule]{}
)

func (r *hashedRuleResource[Model, Rule]) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.spec.typeSuffix
}

func (r *hashedRuleResource[Model, Rule]) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = r.spec.schema
}

func (r *hashedRuleResource[Model, Rule]) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *hashedRuleResource[Model, Rule]) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rule := r.spec.modelToRule(plan)
	var hash string
	err := r.spec.mutate(ctx, r.client, func(doc map[string]any) (map[string]any, client.RuleConfirm, error) {
		next, confirm, err := r.spec.append(doc, rule)
		hash = confirm.Hash
		return next, confirm, err
	})
	if err != nil {
		resp.Diagnostics.AddError("Create "+r.spec.label, err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.spec.ruleToModel(rule, hash))...)
}

func (r *hashedRuleResource[Model, Rule]) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := modelID(ctx, req.State)
	if err != nil {
		resp.Diagnostics.AddError("Read "+r.spec.label, err.Error())
		return
	}
	doc, err := r.spec.get(ctx, r.client)
	if err != nil {
		resp.Diagnostics.AddError("Read "+r.spec.label, err.Error())
		return
	}
	got, err := r.read(doc, id)
	if err != nil {
		resp.Diagnostics.AddError("Read "+r.spec.label, err.Error())
		return
	}
	if got == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func (r *hashedRuleResource[Model, Rule]) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	priorHash, err := modelID(ctx, req.State)
	if err != nil {
		resp.Diagnostics.AddError("Update "+r.spec.label, err.Error())
		return
	}
	rule := r.spec.modelToRule(plan)
	var hash string
	err = r.spec.mutate(ctx, r.client, func(doc map[string]any) (map[string]any, client.RuleConfirm, error) {
		next, confirm, err := r.spec.replace(doc, priorHash, rule)
		hash = confirm.Hash
		return next, confirm, err
	})
	if err != nil {
		resp.Diagnostics.AddError("Update "+r.spec.label, err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.spec.ruleToModel(rule, hash))...)
}

func (r *hashedRuleResource[Model, Rule]) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	hash, err := modelID(ctx, req.State)
	if err != nil {
		resp.Diagnostics.AddError("Delete "+r.spec.label, err.Error())
		return
	}
	err = r.spec.mutate(ctx, r.client, func(doc map[string]any) (map[string]any, client.RuleConfirm, error) {
		return r.spec.remove(doc, hash)
	})
	if err != nil {
		resp.Diagnostics.AddError("Delete "+r.spec.label, err.Error())
	}
}

func (r *hashedRuleResource[Model, Rule]) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := strings.TrimPrefix(strings.TrimSpace(req.ID), r.spec.importPrefix)
	if id == "" {
		resp.Diagnostics.AddError("Invalid import id", r.spec.importHelp)
		return
	}
	doc, err := r.spec.get(ctx, r.client)
	if err != nil {
		resp.Diagnostics.AddError("Import "+r.spec.label, err.Error())
		return
	}
	got, err := r.read(doc, id)
	if err != nil {
		resp.Diagnostics.AddError("Import "+r.spec.label, err.Error())
		return
	}
	if got == nil {
		resp.Diagnostics.AddError("Import "+r.spec.label, fmt.Sprintf("no %s has digest %q", r.spec.label, id))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func (r *hashedRuleResource[Model, Rule]) read(doc map[string]any, hash string) (*Model, error) {
	objects, err := r.spec.objects(doc)
	if err != nil {
		return nil, err
	}
	indexes, err := client.FindRuleHashes(objects, hash)
	if err != nil {
		return nil, err
	}
	if len(indexes) == 0 {
		return nil, nil
	}
	rule, err := r.spec.decode(objects[indexes[0]])
	if err != nil {
		return nil, err
	}
	full, err := client.Digest(objects[indexes[0]])
	if err != nil {
		return nil, err
	}
	model := r.spec.ruleToModel(*rule, full)
	return &model, nil
}
