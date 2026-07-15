package core

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/synology-community/go-synology"
	"github.com/synology-community/go-synology/pkg/api/core"
)

var (
	_ resource.Resource                = &TaskRunResource{}
	_ resource.ResourceWithImportState = &TaskRunResource{}
)

func NewTaskRunResource() resource.Resource {
	return &TaskRunResource{}
}

type TaskRunResource struct {
	coreClient core.Api
}

type TaskRunResourceModel struct {
	ID        types.String `tfsdk:"id"`
	TaskName  types.String `tfsdk:"task_name"`
	TaskID    types.Int64  `tfsdk:"task_id"`
	LastRunAt types.String `tfsdk:"last_run_at"`
}

func (r *TaskRunResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = buildName(req.ProviderTypeName, "task_run")
}

func (r *TaskRunResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Runs an existing DSM Task Scheduler entry once during resource creation or replacement.

This resource is intended for one-shot bootstrap flows. Re-run it with ` + "`terraform apply -replace=...`" + `
when you need to invoke the same DSM task again.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Stable resource ID, equal to `task_name`.",
				Computed:            true,
			},
			"task_name": schema.StringAttribute{
				MarkdownDescription: "Exact DSM task name to run.",
				Required:            true,
			},
			"task_id": schema.Int64Attribute{
				MarkdownDescription: "Resolved DSM task ID for `task_name`.",
				Computed:            true,
			},
			"last_run_at": schema.StringAttribute{
				MarkdownDescription: "UTC RFC3339 timestamp recorded when the provider last invoked the task.",
				Computed:            true,
			},
		},
	}
}

func (r *TaskRunResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	if req.ProviderData == nil {
		return
	}

	configured, ok := req.ProviderData.(synology.Api)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf(
				"Expected client.Client, got: %T. Please report this issue to the provider developers.",
				req.ProviderData,
			),
		)
		return
	}

	r.coreClient = configured.CoreAPI()
}

func (r *TaskRunResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var data TaskRunResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	task, err := r.findTaskByName(ctx, data.TaskName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to resolve DSM task", err.Error())
		return
	}

	if err := r.coreClient.TaskRun(ctx, *task.ID); err != nil {
		resp.Diagnostics.AddError("Failed to run DSM task", err.Error())
		return
	}

	data.ID = types.StringValue(data.TaskName.ValueString())
	data.TaskID = types.Int64Value(*task.ID)
	data.LastRunAt = types.StringValue(time.Now().UTC().Format(time.RFC3339))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TaskRunResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state TaskRunResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	task, err := r.findTaskByName(ctx, state.TaskName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to resolve DSM task", err.Error())
		return
	}

	state.ID = types.StringValue(state.TaskName.ValueString())
	state.TaskID = types.Int64Value(*task.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *TaskRunResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var data TaskRunResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	task, err := r.findTaskByName(ctx, data.TaskName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to resolve DSM task", err.Error())
		return
	}

	if err := r.coreClient.TaskRun(ctx, *task.ID); err != nil {
		resp.Diagnostics.AddError("Failed to run DSM task", err.Error())
		return
	}

	data.ID = types.StringValue(data.TaskName.ValueString())
	data.TaskID = types.Int64Value(*task.ID)
	data.LastRunAt = types.StringValue(time.Now().UTC().Format(time.RFC3339))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TaskRunResource) Delete(
	ctx context.Context,
	_ resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	resp.State.RemoveResource(ctx)
}

func (r *TaskRunResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	task, err := r.findTaskByName(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read imported DSM task", err.Error())
		return
	}

	state := TaskRunResourceModel{
		ID:        types.StringValue(req.ID),
		TaskName:  types.StringValue(req.ID),
		TaskID:    types.Int64Value(*task.ID),
		LastRunAt: types.StringNull(),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *TaskRunResource) findTaskByName(
	ctx context.Context,
	taskName string,
) (*core.TaskResult, error) {
	tasks, err := r.coreClient.TaskList(ctx, core.ListTaskRequest{})
	if err != nil {
		return nil, err
	}

	return findTaskResultByName(tasks.Tasks, taskName)
}

func findTaskResultByName(tasks []core.TaskResult, taskName string) (*core.TaskResult, error) {
	var match *core.TaskResult
	for i := range tasks {
		task := &tasks[i]
		if task.Name != taskName {
			continue
		}
		if task.ID == nil {
			return nil, fmt.Errorf("task %q does not have an ID", taskName)
		}
		if match != nil {
			return nil, fmt.Errorf("multiple DSM tasks were found with name %q", taskName)
		}
		match = task
	}

	if match == nil {
		return nil, fmt.Errorf("DSM task %q was not found", taskName)
	}

	return match, nil
}
