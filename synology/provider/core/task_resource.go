package core

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/synology-community/go-synology"
	"github.com/synology-community/go-synology/pkg/api"
	"github.com/synology-community/go-synology/pkg/api/core"
)

type TaskResourceModel struct {
	ID       types.Int64  `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Service  types.String `tfsdk:"service"`
	Script   types.String `tfsdk:"script"`
	Schedule types.String `tfsdk:"schedule"`
	User     types.String `tfsdk:"user"`
	Enabled  types.Bool   `tfsdk:"enabled"`
	Run      types.Bool   `tfsdk:"run"`
	When     types.String `tfsdk:"when"`
	TaskType types.String `tfsdk:"task_type"`
}

type taskDetailRequest struct {
	ID        int64  `url:"id"`
	RealOwner string `url:"real_owner,omitempty"`
}

type taskDetail struct {
	ID        int64             `json:"id"`
	Name      string            `json:"name"`
	Owner     string            `json:"owner"`
	RealOwner string            `json:"real_owner"`
	Enable    bool              `json:"enable"`
	Type      string            `json:"type"`
	Schedule  core.TaskSchedule `json:"schedule"`
	Extra     core.TaskExtra    `json:"extra"`
}

var (
	_ resource.Resource                = &TaskResource{}
	_ resource.ResourceWithImportState = &TaskResource{}
)

var taskDetailMethod = api.Method{
	API:            "SYNO.Core.TaskScheduler",
	Method:         "get",
	Version:        4,
	ErrorSummaries: api.GlobalErrors,
}

func NewTaskResource() resource.Resource {
	return &TaskResource{}
}

type TaskResource struct {
	client     synology.Api
	coreClient core.Api
}

func (p *TaskResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var data TaskResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	taskReq, err := getTaskRequest(data)
	if err != nil {
		resp.Diagnostics.AddError("Failed to build task request", err.Error())
		return
	}

	var taskCreate func(ctx context.Context, req core.TaskRequest) (*core.TaskResult, error)
	if taskReq.Owner == "root" {
		taskCreate = p.coreClient.RootTaskCreate
	} else {
		taskCreate = p.coreClient.TaskCreate
	}

	res, err := taskCreate(ctx, taskReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create task", err.Error())
		return
	}
	if res.ID == nil {
		resp.Diagnostics.AddError("Failed to create task", "DSM did not return a task ID.")
		return
	}

	data.ID = types.Int64Value(*res.ID)

	if data.Run.ValueBool() && data.When.ValueString() == "apply" {
		if err := p.coreClient.TaskRun(ctx, data.ID.ValueInt64()); err != nil {
			resp.Diagnostics.AddError("Failed to run task", err.Error())
			return
		}
	}

	refreshed, err := p.refreshTaskState(ctx, data, nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read created task", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &refreshed)...)
}

func (p *TaskResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan TaskResourceModel
	var state TaskResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	taskReq, err := getTaskRequest(plan)
	if err != nil {
		resp.Diagnostics.AddError("Failed to build task request", err.Error())
		return
	}

	id := state.ID.ValueInt64()
	taskReq.ID = &id

	var taskUpdate func(ctx context.Context, req core.TaskRequest) (*core.TaskResult, error)
	if taskReq.Owner == "root" {
		taskUpdate = p.coreClient.RootTaskUpdate
	} else {
		taskUpdate = p.coreClient.TaskUpdate
	}

	if _, err := taskUpdate(ctx, taskReq); err != nil {
		resp.Diagnostics.AddError("Failed to update task", err.Error())
		return
	}

	plan.ID = state.ID
	if plan.Run.ValueBool() && plan.When.ValueString() == "upgrade" {
		if err := p.coreClient.TaskRun(ctx, state.ID.ValueInt64()); err != nil {
			resp.Diagnostics.AddError("Failed to run task", err.Error())
			return
		}
	}

	refreshed, err := p.refreshTaskState(ctx, plan, &state)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read updated task", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &refreshed)...)
}

func (p *TaskResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var data TaskResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Run.ValueBool() && data.When.ValueString() == "destroy" {
		if err := p.coreClient.TaskRun(ctx, data.ID.ValueInt64()); err != nil {
			resp.Diagnostics.AddError("Failed to run task", err.Error())
			return
		}
	}

	if err := p.coreClient.TaskDelete(ctx, data.ID.ValueInt64()); err != nil {
		if _, readErr := p.coreClient.TaskGet(ctx, data.ID.ValueInt64()); readErr != nil {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to delete task", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (p *TaskResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = buildName(req.ProviderTypeName, "task")
}

func (p *TaskResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state TaskResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	refreshed, err := p.refreshTaskState(ctx, state, &state)
	if err != nil {
		if isTaskNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to read task", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &refreshed)...)
}

func (p *TaskResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages a DSM Task Scheduler entry.

This resource models script-based DSM scheduled tasks. It supports steady-state drift
detection for the task owner, enabled state, script body, task type, and schedule.
`,

		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				MarkdownDescription: "DSM task ID.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "DSM task name.",
				Required:            true,
			},
			"schedule": schema.StringAttribute{
				MarkdownDescription: "Schedule expressed as a fixed 5-field cron string such as `17 3 * * *` or `17 3 * * 0,1,2,3,4,5,6`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						cronTaskSchedulePattern(),
						"value must contain a supported fixed 5-field cron expression or a supported macro",
					),
				},
			},
			"service": schema.StringAttribute{
				MarkdownDescription: "Deprecated placeholder for future non-script task types. Leave unset.",
				Optional:            true,
			},
			"script": schema.StringAttribute{
				MarkdownDescription: "Shell script content executed by the task.",
				Optional:            true,
			},
			"user": schema.StringAttribute{
				MarkdownDescription: "DSM user that executes the task. Use `root` for root-owned tasks.",
				Required:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the task is enabled. Default: `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"run": schema.BoolAttribute{
				MarkdownDescription: "Whether to run the task as an apply/destroy hook. Default: `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"when": schema.StringAttribute{
				MarkdownDescription: "When the `run` hook should execute. Valid values are `apply`, `destroy`, and `upgrade`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("apply"),
				Validators: []validator.String{
					stringvalidator.OneOf("apply", "destroy", "upgrade"),
				},
			},
			"task_type": schema.StringAttribute{
				MarkdownDescription: "Task type reported by DSM.",
				Computed:            true,
			},
		},
	}
}

func (p *TaskResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(synology.Api)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf(
				"Expected client.Client, got: %T. Please report this issue to the provider developers.",
				req.ProviderData,
			),
		)
		return
	}

	p.client = client
	p.coreClient = client.CoreAPI()
}

func (p *TaskResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	state := TaskResourceModel{
		ID:      types.Int64Value(id),
		Run:     types.BoolValue(false),
		When:    types.StringValue("apply"),
		Enabled: types.BoolValue(true),
	}

	refreshed, err := p.refreshTaskState(ctx, state, nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read imported task", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &refreshed)...)
}

func (p *TaskResource) refreshTaskState(
	ctx context.Context,
	data TaskResourceModel,
	previous *TaskResourceModel,
) (TaskResourceModel, error) {
	detail, err := p.getTaskDetail(ctx, data.ID.ValueInt64())
	if err != nil {
		return TaskResourceModel{}, err
	}

	data.Name = types.StringValue(detail.Name)
	data.User = types.StringValue(detail.Owner)
	data.Enabled = types.BoolValue(detail.Enable)
	data.TaskType = types.StringValue(detail.Type)

	if detail.Extra.Script != "" {
		data.Script = types.StringValue(detail.Extra.Script)
	} else {
		data.Script = types.StringNull()
	}
	data.Service = types.StringNull()

	if previous != nil &&
		!previous.Schedule.IsNull() &&
		!previous.Schedule.IsUnknown() &&
		previous.Schedule.ValueString() != "" &&
		taskScheduleMatchesSpec(previous.Schedule.ValueString(), detail.Schedule) {
		data.Schedule = previous.Schedule
	} else {
		rendered, err := renderTaskSchedule(detail.Schedule)
		if err != nil {
			return TaskResourceModel{}, err
		}
		if rendered == "" {
			data.Schedule = types.StringNull()
		} else {
			data.Schedule = types.StringValue(rendered)
		}
	}

	if previous != nil {
		data.Run = previous.Run
		data.When = previous.When
	}
	if data.Run.IsNull() || data.Run.IsUnknown() {
		data.Run = types.BoolValue(false)
	}
	if data.When.IsNull() || data.When.IsUnknown() || data.When.ValueString() == "" {
		data.When = types.StringValue("apply")
	}

	return data, nil
}

func (p *TaskResource) getTaskDetail(ctx context.Context, id int64) (*taskDetail, error) {
	tasks, err := p.coreClient.TaskList(ctx, core.ListTaskRequest{})
	if err != nil {
		return nil, err
	}

	var realOwner string
	found := false
	for _, task := range tasks.Tasks {
		if task.ID != nil && *task.ID == id {
			realOwner = task.RealOwner
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("task %d not found", id)
	}

	detail, err := api.Get[taskDetail](p.client, ctx, &taskDetailRequest{
		ID:        id,
		RealOwner: realOwner,
	}, taskDetailMethod)
	if err != nil {
		return nil, err
	}

	return detail, nil
}

func newTaskSchedule() core.TaskSchedule {
	return core.TaskSchedule{
		DateType:             0,
		WeekDay:              "0,1,2,3,4,5,6",
		MonthlyWeek:          []string{},
		RepeatDate:           1001,
		RepeatMinStoreConfig: []int64{1, 5, 10, 15, 20, 30},
		RepeatHourStoreConfig: []int64{
			1,
			2,
			3,
			4,
			5,
			6,
			7,
			8,
			9,
			10,
			11,
			12,
			13,
			14,
			15,
			16,
			17,
			18,
			19,
			20,
			21,
			22,
			23,
		},
	}
}

func getTaskRequest(data TaskResourceModel) (core.TaskRequest, error) {
	if !data.Service.IsNull() && !data.Service.IsUnknown() && data.Service.ValueString() != "" {
		return core.TaskRequest{}, fmt.Errorf("service-based tasks are not supported yet")
	}

	if data.Script.IsNull() || data.Script.IsUnknown() || data.Script.ValueString() == "" {
		return core.TaskRequest{}, fmt.Errorf("script is required")
	}

	taskReq := core.TaskRequest{
		Name:      data.Name.ValueString(),
		Owner:     data.User.ValueString(),
		RealOwner: "root",
		Type:      "script",
		Enable:    data.Enabled.ValueBool(),
		Extra: core.TaskExtra{
			Script: data.Script.ValueString(),
		},
	}

	if !data.Schedule.IsNull() && !data.Schedule.IsUnknown() && data.Schedule.ValueString() != "" {
		schedule, err := parseTaskScheduleSpec(data.Schedule.ValueString())
		if err != nil {
			return core.TaskRequest{}, err
		}
		taskReq.Schedule = schedule
	} else {
		taskReq.Schedule = newTaskSchedule()
	}

	return taskReq, nil
}

func parseTaskScheduleSpec(spec string) (core.TaskSchedule, error) {
	schedule := newTaskSchedule()
	spec = strings.TrimSpace(spec)
	switch spec {
	case "":
		return schedule, nil
	case "@daily":
		return schedule, nil
	case "@weekly":
		schedule.WeekDay = "0"
		return schedule, nil
	}

	parts := strings.Fields(spec)
	if len(parts) != 5 {
		return core.TaskSchedule{}, fmt.Errorf(
			"unsupported task schedule %q: expected 5 fields",
			spec,
		)
	}

	minute, err := parseTaskScheduleNumber(parts[0], 0, 59, "minute")
	if err != nil {
		return core.TaskSchedule{}, err
	}
	hour, err := parseTaskScheduleNumber(parts[1], 0, 23, "hour")
	if err != nil {
		return core.TaskSchedule{}, err
	}
	if parts[2] != "*" {
		return core.TaskSchedule{}, fmt.Errorf(
			"unsupported task schedule %q: day-of-month must be *",
			spec,
		)
	}
	if parts[3] != "*" {
		return core.TaskSchedule{}, fmt.Errorf(
			"unsupported task schedule %q: month must be *",
			spec,
		)
	}
	weekDay, err := parseTaskScheduleWeekDay(parts[4])
	if err != nil {
		return core.TaskSchedule{}, err
	}

	schedule.Minute = minute
	schedule.Hour = hour
	schedule.WeekDay = weekDay

	return schedule, nil
}

func parseTaskScheduleNumber(
	value string,
	minValue int64,
	maxValue int64,
	label string,
) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q", label, value)
	}
	if parsed < minValue || parsed > maxValue {
		return 0, fmt.Errorf("invalid %s %q", label, value)
	}
	return parsed, nil
}

func parseTaskScheduleWeekDay(value string) (string, error) {
	if value == "*" {
		return "0,1,2,3,4,5,6", nil
	}

	values := strings.Split(value, ",")
	parsed := make([]string, 0, len(values))
	for _, part := range values {
		part = strings.TrimSpace(part)
		if part == "" {
			return "", fmt.Errorf("invalid weekday list %q", value)
		}
		day, err := strconv.ParseInt(part, 10, 64)
		if err != nil || day < 0 || day > 6 {
			return "", fmt.Errorf("invalid weekday %q", part)
		}
		parsed = append(parsed, strconv.FormatInt(day, 10))
	}

	return strings.Join(parsed, ","), nil
}

func renderTaskSchedule(schedule core.TaskSchedule) (string, error) {
	if schedule.Hour < 0 || schedule.Hour > 23 || schedule.Minute < 0 || schedule.Minute > 59 {
		return "", fmt.Errorf("unsupported DSM task schedule: invalid hour/minute")
	}
	if schedule.RepeatHour != 0 || schedule.RepeatMin != 0 {
		return "", fmt.Errorf(
			"unsupported DSM task schedule: repeating intervals are not supported",
		)
	}
	if schedule.DateType != 0 {
		return "", fmt.Errorf("unsupported DSM task schedule: date_type %d", schedule.DateType)
	}
	if schedule.RepeatDate != 0 && schedule.RepeatDate != 1001 {
		return "", fmt.Errorf("unsupported DSM task schedule: repeat_date %d", schedule.RepeatDate)
	}
	if len(schedule.MonthlyWeek) > 0 {
		return "", fmt.Errorf("unsupported DSM task schedule: monthly_week is not supported")
	}

	weekDay := strings.TrimSpace(schedule.WeekDay)
	if weekDay == "" || weekDay == "0,1,2,3,4,5,6" {
		return fmt.Sprintf("%d %d * * *", schedule.Minute, schedule.Hour), nil
	}

	return fmt.Sprintf("%d %d * * %s", schedule.Minute, schedule.Hour, weekDay), nil
}

func taskScheduleMatchesSpec(spec string, schedule core.TaskSchedule) bool {
	parsed, err := parseTaskScheduleSpec(spec)
	if err != nil {
		return false
	}

	return parsed.DateType == schedule.DateType &&
		parsed.Minute == schedule.Minute &&
		parsed.Hour == schedule.Hour &&
		parsed.WeekDay == schedule.WeekDay &&
		parsed.RepeatDate == schedule.RepeatDate &&
		parsed.RepeatHour == schedule.RepeatHour &&
		parsed.RepeatMin == schedule.RepeatMin
}

func cronTaskSchedulePattern() *regexp.Regexp {
	return regexp.MustCompile(
		`^(@daily|@weekly|([0-5]?\d)\s+([01]?\d|2[0-3])\s+\*\s+\*\s+(\*|[0-6](,[0-6])*))$`,
	)
}

func isTaskNotFound(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(strings.ToLower(err.Error()), "not found")
}
