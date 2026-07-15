package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/synology-community/go-synology"
	"github.com/synology-community/go-synology/pkg/api"
	synocore "github.com/synology-community/go-synology/pkg/api/core"
)

const shareNFSPrivilegeAPIName = "SYNO.Core.FileServ.NFS.SharePrivilege"

var (
	_ resource.Resource                = &ShareNFSPrivilegeResource{}
	_ resource.ResourceWithImportState = &ShareNFSPrivilegeResource{}
)

func NewShareNFSPrivilegeResource() resource.Resource {
	return &ShareNFSPrivilegeResource{}
}

type ShareNFSPrivilegeResource struct {
	client     synology.Api
	coreClient synocore.Api
}

type ShareNFSPrivilegeResourceModel struct {
	ID        types.String `tfsdk:"id"`
	ShareName types.String `tfsdk:"share_name"`
	Rules     types.List   `tfsdk:"rules"`
}

type shareNFSPrivilegeRuleModel struct {
	Client         types.String `tfsdk:"client"`
	Privilege      types.String `tfsdk:"privilege"`
	RootSquash     types.String `tfsdk:"root_squash"`
	Async          types.Bool   `tfsdk:"async"`
	Crossmnt       types.Bool   `tfsdk:"crossmnt"`
	Insecure       types.Bool   `tfsdk:"insecure"`
	SecurityFlavor types.Object `tfsdk:"security_flavor"`
}

type shareNFSPrivilegeSecurityFlavorModel struct {
	Sys               types.Bool `tfsdk:"sys"`
	Kerberos          types.Bool `tfsdk:"kerberos"`
	KerberosIntegrity types.Bool `tfsdk:"kerberos_integrity"`
	KerberosPrivacy   types.Bool `tfsdk:"kerberos_privacy"`
}

type shareNFSPrivilegeAPIResponse struct {
	ShareName string                     `json:"share_name"`
	Rules     []shareNFSPrivilegeAPIRule `json:"rule"`
}

type shareNFSPrivilegeAPIRule struct {
	Async          bool                             `json:"async"`
	Client         string                           `json:"client"`
	Crossmnt       bool                             `json:"crossmnt"`
	Insecure       bool                             `json:"insecure"`
	Privilege      string                           `json:"privilege"`
	RootSquash     string                           `json:"root_squash"`
	SecurityFlavor shareNFSPrivilegeAPISecurityType `json:"security_flavor"`
}

type shareNFSPrivilegeAPISecurityType struct {
	Kerberos          bool `json:"kerberos"`
	KerberosIntegrity bool `json:"kerberos_integrity"`
	KerberosPrivacy   bool `json:"kerberos_privacy"`
	Sys               bool `json:"sys"`
}

type shareNFSPrivilegeLoadRequest struct {
	ShareName string `url:"share_name"`
}

type shareNFSPrivilegeSaveRequest struct {
	ShareName string `url:"share_name"`
	Rule      string `url:"rule"`
}

var (
	shareNFSPrivilegeLoadMethod = api.Method{
		API:            shareNFSPrivilegeAPIName,
		Method:         "load",
		Version:        1,
		ErrorSummaries: api.GlobalErrors,
	}
	shareNFSPrivilegeSaveMethod = api.Method{
		API:            shareNFSPrivilegeAPIName,
		Method:         "save",
		Version:        1,
		ErrorSummaries: api.GlobalErrors,
	}
)

func (m shareNFSPrivilegeRuleModel) AttrType() map[string]attr.Type {
	return map[string]attr.Type{
		"client":      types.StringType,
		"privilege":   types.StringType,
		"root_squash": types.StringType,
		"async":       types.BoolType,
		"crossmnt":    types.BoolType,
		"insecure":    types.BoolType,
		"security_flavor": types.ObjectType{
			AttrTypes: shareNFSPrivilegeSecurityFlavorModel{}.AttrType(),
		},
	}
}

func (m shareNFSPrivilegeRuleModel) ObjectValue() (types.Object, diag.Diagnostics) {
	return types.ObjectValue(m.AttrType(), map[string]attr.Value{
		"client":          m.Client,
		"privilege":       m.Privilege,
		"root_squash":     m.RootSquash,
		"async":           m.Async,
		"crossmnt":        m.Crossmnt,
		"insecure":        m.Insecure,
		"security_flavor": m.SecurityFlavor,
	})
}

func (m shareNFSPrivilegeSecurityFlavorModel) AttrType() map[string]attr.Type {
	return map[string]attr.Type{
		"sys":                types.BoolType,
		"kerberos":           types.BoolType,
		"kerberos_integrity": types.BoolType,
		"kerberos_privacy":   types.BoolType,
	}
}

func (m shareNFSPrivilegeSecurityFlavorModel) ObjectValue() (types.Object, diag.Diagnostics) {
	return types.ObjectValue(m.AttrType(), map[string]attr.Value{
		"sys":                m.Sys,
		"kerberos":           m.Kerberos,
		"kerberos_integrity": m.KerberosIntegrity,
		"kerberos_privacy":   m.KerberosPrivacy,
	})
}

func (r *ShareNFSPrivilegeResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = buildName(req.ProviderTypeName, "share_nfs_privilege")
}

func (r *ShareNFSPrivilegeResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages the complete DSM NFS export rule set for a shared folder.

The Synology DSM API exposes NFS privileges per shared folder as a full ordered rule list.
This resource owns that full list for one share. Applying the resource replaces the share's
current DSM NFS rule set with the Terraform-managed list.`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Terraform resource identifier. Matches `share_name`.",
				Computed:            true,
			},
			"share_name": schema.StringAttribute{
				MarkdownDescription: "Name of the DSM shared folder whose NFS rules are managed.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"rules": schema.ListNestedAttribute{
				MarkdownDescription: "Ordered NFS privilege rules for the share. This resource manages the full ordered list.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"client": schema.StringAttribute{
							MarkdownDescription: "Client host, IP, CIDR, or wildcard accepted by DSM for this NFS rule.",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
						},
						"privilege": schema.StringAttribute{
							MarkdownDescription: "DSM NFS access privilege for the client. Supported values are `ro` and `rw`.",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("rw"),
							Validators: []validator.String{
								stringvalidator.OneOf("ro", "rw"),
							},
						},
						"root_squash": schema.StringAttribute{
							MarkdownDescription: "DSM API root squash value for the client rule, using the raw keyword DSM returns for the export, for example `root`.",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
						},
						"async": schema.BoolAttribute{
							MarkdownDescription: "Whether to allow asynchronous writes for the client rule.",
							Optional:            true,
							Computed:            true,
							Default:             booldefault.StaticBool(false),
						},
						"crossmnt": schema.BoolAttribute{
							MarkdownDescription: "Whether the client may access mounted subfolders beneath the share export.",
							Optional:            true,
							Computed:            true,
							Default:             booldefault.StaticBool(false),
						},
						"insecure": schema.BoolAttribute{
							MarkdownDescription: "Whether DSM should allow client connections from non-privileged ports.",
							Optional:            true,
							Computed:            true,
							Default:             booldefault.StaticBool(false),
						},
						"security_flavor": schema.SingleNestedAttribute{
							MarkdownDescription: "Security flavors accepted for this client rule. If omitted, the resource defaults to `sys = true`.",
							Optional:            true,
							Computed:            true,
							Attributes: map[string]schema.Attribute{
								"sys": schema.BoolAttribute{
									MarkdownDescription: "Allow AUTH_SYS (`sys`).",
									Optional:            true,
									Computed:            true,
									Default:             booldefault.StaticBool(true),
								},
								"kerberos": schema.BoolAttribute{
									MarkdownDescription: "Allow Kerberos (`krb5`).",
									Optional:            true,
									Computed:            true,
									Default:             booldefault.StaticBool(false),
								},
								"kerberos_integrity": schema.BoolAttribute{
									MarkdownDescription: "Allow Kerberos integrity (`krb5i`).",
									Optional:            true,
									Computed:            true,
									Default:             booldefault.StaticBool(false),
								},
								"kerberos_privacy": schema.BoolAttribute{
									MarkdownDescription: "Allow Kerberos privacy (`krb5p`).",
									Optional:            true,
									Computed:            true,
									Default:             booldefault.StaticBool(false),
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *ShareNFSPrivilegeResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan ShareNFSPrivilegeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.toAPI(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.saveRules(ctx, payload); err != nil {
		resp.Diagnostics.AddError("Failed to save NFS share privileges", err.Error())
		return
	}

	state := normalizedShareNFSPrivilegeState(plan)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ShareNFSPrivilegeResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state ShareNFSPrivilegeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	refreshed, diags, err := r.readModel(ctx, state.ShareName.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err != nil {
		if IsMissingShareAPIError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to load NFS share privileges", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &refreshed)...)
}

func (r *ShareNFSPrivilegeResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan ShareNFSPrivilegeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.toAPI(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.saveRules(ctx, payload); err != nil {
		resp.Diagnostics.AddError("Failed to save NFS share privileges", err.Error())
		return
	}

	state := normalizedShareNFSPrivilegeState(plan)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ShareNFSPrivilegeResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state ShareNFSPrivilegeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.saveRules(ctx, shareNFSPrivilegeAPIResponse{
		ShareName: state.ShareName.ValueString(),
		Rules:     []shareNFSPrivilegeAPIRule{},
	})
	if err != nil && !IsMissingShareAPIError(err) {
		resp.Diagnostics.AddError("Failed to clear NFS share privileges", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *ShareNFSPrivilegeResource) Configure(
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
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected synology.Api, got: %T.", req.ProviderData),
		)
		return
	}

	r.client = client
	r.coreClient = client.CoreAPI()
}

func (r *ShareNFSPrivilegeResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	state, diags, err := r.readModel(ctx, req.ID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err != nil {
		if IsMissingShareAPIError(err) {
			resp.Diagnostics.AddError(
				"Shared folder not found",
				fmt.Sprintf("No DSM shared folder named %q was found.", req.ID),
			)
			return
		}
		resp.Diagnostics.AddError("Failed to load NFS share privileges", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ShareNFSPrivilegeResource) loadRules(
	ctx context.Context,
	shareName string,
) (shareNFSPrivilegeAPIResponse, error) {
	if err := RequireShareExists(ctx, r.coreClient, shareName); err != nil {
		return shareNFSPrivilegeAPIResponse{}, err
	}

	payload, err := api.GetQuery[shareNFSPrivilegeAPIResponse](
		r.client,
		ctx,
		shareNFSPrivilegeLoadRequest{
			ShareName: strconv.Quote(shareName),
		},
		shareNFSPrivilegeLoadMethod,
	)
	if err != nil {
		return shareNFSPrivilegeAPIResponse{}, err
	}

	if payload.ShareName == "" {
		payload.ShareName = shareName
	}

	return *payload, nil
}

func (r *ShareNFSPrivilegeResource) saveRules(
	ctx context.Context,
	payload shareNFSPrivilegeAPIResponse,
) error {
	if err := RequireShareExists(ctx, r.coreClient, payload.ShareName); err != nil {
		return err
	}

	rawRules, err := json.Marshal(payload.Rules)
	if err != nil {
		return err
	}

	_, err = api.GetQuery[struct{}](r.client, ctx, shareNFSPrivilegeSaveRequest{
		ShareName: strconv.Quote(payload.ShareName),
		Rule:      string(rawRules),
	}, shareNFSPrivilegeSaveMethod)
	return err
}

func (r *ShareNFSPrivilegeResource) readModel(
	ctx context.Context,
	shareName string,
) (ShareNFSPrivilegeResourceModel, diag.Diagnostics, error) {
	payload, err := r.loadRules(ctx, shareName)
	if err != nil {
		return ShareNFSPrivilegeResourceModel{}, nil, err
	}

	state, modelDiags := shareNFSPrivilegeModelFromAPI(payload)
	return state, modelDiags, nil
}

func normalizedShareNFSPrivilegeState(
	plan ShareNFSPrivilegeResourceModel,
) ShareNFSPrivilegeResourceModel {
	if plan.Rules.IsNull() || plan.Rules.IsUnknown() {
		plan.Rules = types.ListValueMust(
			types.ObjectType{AttrTypes: shareNFSPrivilegeRuleModel{}.AttrType()},
			[]attr.Value{},
		)
	}

	plan.ID = types.StringValue(plan.ShareName.ValueString())
	return plan
}

func shareNFSPrivilegeModelFromAPI(
	payload shareNFSPrivilegeAPIResponse,
) (ShareNFSPrivilegeResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	rules := make([]attr.Value, 0, len(payload.Rules))
	for _, rule := range payload.Rules {
		securityFlavor, ruleDiags := shareNFSPrivilegeSecurityFlavorModel{
			Sys:               types.BoolValue(rule.SecurityFlavor.Sys),
			Kerberos:          types.BoolValue(rule.SecurityFlavor.Kerberos),
			KerberosIntegrity: types.BoolValue(rule.SecurityFlavor.KerberosIntegrity),
			KerberosPrivacy:   types.BoolValue(rule.SecurityFlavor.KerberosPrivacy),
		}.ObjectValue()
		diags.Append(ruleDiags...)
		if diags.HasError() {
			return ShareNFSPrivilegeResourceModel{}, diags
		}

		ruleValue, ruleDiags := shareNFSPrivilegeRuleModel{
			Client:         types.StringValue(rule.Client),
			Privilege:      types.StringValue(rule.Privilege),
			RootSquash:     types.StringValue(rule.RootSquash),
			Async:          types.BoolValue(rule.Async),
			Crossmnt:       types.BoolValue(rule.Crossmnt),
			Insecure:       types.BoolValue(rule.Insecure),
			SecurityFlavor: securityFlavor,
		}.ObjectValue()
		diags.Append(ruleDiags...)
		if diags.HasError() {
			return ShareNFSPrivilegeResourceModel{}, diags
		}

		rules = append(rules, ruleValue)
	}

	ruleList, ruleListDiags := types.ListValue(
		types.ObjectType{AttrTypes: shareNFSPrivilegeRuleModel{}.AttrType()},
		rules,
	)
	diags.Append(ruleListDiags...)
	if diags.HasError() {
		return ShareNFSPrivilegeResourceModel{}, diags
	}

	return ShareNFSPrivilegeResourceModel{
		ID:        types.StringValue(payload.ShareName),
		ShareName: types.StringValue(payload.ShareName),
		Rules:     ruleList,
	}, diags
}

func (m ShareNFSPrivilegeResourceModel) toAPI(
	ctx context.Context,
) (shareNFSPrivilegeAPIResponse, diag.Diagnostics) {
	var diags diag.Diagnostics

	var rulesModel []shareNFSPrivilegeRuleModel
	if !m.Rules.IsNull() && !m.Rules.IsUnknown() {
		diags.Append(m.Rules.ElementsAs(ctx, &rulesModel, false)...)
		if diags.HasError() {
			return shareNFSPrivilegeAPIResponse{}, diags
		}
	}

	rules := make([]shareNFSPrivilegeAPIRule, 0, len(rulesModel))
	for idx, rule := range rulesModel {
		apiRule, ruleDiags := rule.toAPI(ctx, idx)
		diags.Append(ruleDiags...)
		if diags.HasError() {
			return shareNFSPrivilegeAPIResponse{}, diags
		}
		rules = append(rules, apiRule)
	}

	return shareNFSPrivilegeAPIResponse{
		ShareName: m.ShareName.ValueString(),
		Rules:     rules,
	}, diags
}

func (m shareNFSPrivilegeRuleModel) toAPI(
	ctx context.Context,
	index int,
) (shareNFSPrivilegeAPIRule, diag.Diagnostics) {
	var diags diag.Diagnostics

	securityFlavor := shareNFSPrivilegeSecurityFlavorModel{
		Sys:               types.BoolValue(true),
		Kerberos:          types.BoolValue(false),
		KerberosIntegrity: types.BoolValue(false),
		KerberosPrivacy:   types.BoolValue(false),
	}

	if !m.SecurityFlavor.IsNull() && !m.SecurityFlavor.IsUnknown() {
		diags.Append(m.SecurityFlavor.As(ctx, &securityFlavor, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return shareNFSPrivilegeAPIRule{}, diags
		}
	}

	if !securityFlavor.Sys.ValueBool() &&
		!securityFlavor.Kerberos.ValueBool() &&
		!securityFlavor.KerberosIntegrity.ValueBool() &&
		!securityFlavor.KerberosPrivacy.ValueBool() {
		diags.AddAttributeError(
			path.Root("rules").AtListIndex(index).AtName("security_flavor"),
			"Invalid NFS security flavor",
			"At least one DSM NFS security flavor must be enabled.",
		)
		return shareNFSPrivilegeAPIRule{}, diags
	}

	return shareNFSPrivilegeAPIRule{
		Client:     m.Client.ValueString(),
		Privilege:  m.Privilege.ValueString(),
		RootSquash: m.RootSquash.ValueString(),
		Async:      m.Async.ValueBool(),
		Crossmnt:   m.Crossmnt.ValueBool(),
		Insecure:   m.Insecure.ValueBool(),
		SecurityFlavor: shareNFSPrivilegeAPISecurityType{
			Sys:               securityFlavor.Sys.ValueBool(),
			Kerberos:          securityFlavor.Kerberos.ValueBool(),
			KerberosIntegrity: securityFlavor.KerberosIntegrity.ValueBool(),
			KerberosPrivacy:   securityFlavor.KerberosPrivacy.ValueBool(),
		},
	}, diags
}
