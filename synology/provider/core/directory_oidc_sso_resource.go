package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/synology-community/go-synology"
	"github.com/synology-community/go-synology/pkg/api"
)

const (
	directoryOIDCSSOAPIName     = "SYNO.Core.Directory.OIDC.SSO"
	directorySSOProfileAPIName  = "SYNO.Core.Directory.SSO.Profile"
	directorySSOSettingAPIName  = "SYNO.Core.Directory.SSO.Setting"
	directoryOIDCSSOResourceID  = "oidc"
	directoryOIDCSSOProfileName = "oidc"
)

type DirectoryOIDCSSOResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	Enabled               types.Bool   `tfsdk:"enabled"`
	DefaultLogin          types.Bool   `tfsdk:"default_login"`
	AllowLocalUser        types.Bool   `tfsdk:"allow_local_user"`
	Name                  types.String `tfsdk:"name"`
	WellKnownURL          types.String `tfsdk:"wellknown_url"`
	ClientID              types.String `tfsdk:"client_id"`
	ClientSecret          types.String `tfsdk:"client_secret"`
	RedirectURI           types.String `tfsdk:"redirect_uri"`
	Scope                 types.String `tfsdk:"scope"`
	UsernameClaim         types.String `tfsdk:"username_claim"`
	AuthorizationEndpoint types.String `tfsdk:"authorization_endpoint"`
	TokenEndpoint         types.String `tfsdk:"token_endpoint"`
}

type directoryOIDCSSOGetResponse struct {
	AllowLocalUser        bool   `json:"oidc_allow_local_user"`
	AuthorizationEndpoint string `json:"oidc_authorization_endpoint"`
	ClientID              string `json:"oidc_client_id"`
	ClientSecret          string `json:"oidc_client_secret"`
	Name                  string `json:"oidc_name"`
	RedirectURI           string `json:"oidc_redirect_uri"`
	Scope                 string `json:"oidc_scope"`
	TokenEndpoint         string `json:"oidc_token_endpoint"`
	UsernameClaim         string `json:"oidc_user_claim"`
	WellKnownURL          string `json:"oidc_wellknown"`
}

type directoryOIDCSSOSetRequest struct {
	AllowLocalUser        bool   `url:"oidc_allow_local_user"`
	AuthorizationEndpoint string `url:"oidc_authorization_endpoint"`
	ClientID              string `url:"oidc_client_id"`
	ClientSecret          string `url:"oidc_client_secret"`
	Name                  string `url:"oidc_name"`
	RedirectURI           string `url:"oidc_redirect_uri"`
	Scope                 string `url:"oidc_scope"`
	TokenEndpoint         string `url:"oidc_token_endpoint"`
	UsernameClaim         string `url:"oidc_user_claim"`
	WellKnownURL          string `url:"oidc_wellknown"`
}

type directorySSOProfileGetResponse struct {
	Enabled bool   `json:"sso_enable"`
	Profile string `json:"sso_profile"`
}

type directorySSOProfileSetRequest struct {
	Enabled bool   `url:"sso_enable"`
	Profile string `url:"sso_profile"`
}

type directorySSOSettingGetResponse struct {
	DefaultLogin bool `json:"sso_default_login"`
}

type directorySSOSettingSetRequest struct {
	DefaultLogin bool `url:"sso_default_login"`
}

type oidcDiscoveryDocument struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
}

var (
	_ resource.Resource                = &DirectoryOIDCSSOResource{}
	_ resource.ResourceWithImportState = &DirectoryOIDCSSOResource{}
)

var (
	directoryOIDCSSOGetMethod = api.Method{
		API:            directoryOIDCSSOAPIName,
		Method:         "get",
		Version:        1,
		ErrorSummaries: api.GlobalErrors,
	}
	directoryOIDCSSOSetMethod = api.Method{
		API:            directoryOIDCSSOAPIName,
		Method:         "set",
		Version:        1,
		ErrorSummaries: api.GlobalErrors,
	}
	directorySSOProfileGetMethod = api.Method{
		API:            directorySSOProfileAPIName,
		Method:         "get",
		Version:        1,
		ErrorSummaries: api.GlobalErrors,
	}
	directorySSOProfileSetMethod = api.Method{
		API:            directorySSOProfileAPIName,
		Method:         "set",
		Version:        1,
		ErrorSummaries: api.GlobalErrors,
	}
	directorySSOSettingGetMethod = api.Method{
		API:            directorySSOSettingAPIName,
		Method:         "get",
		Version:        1,
		ErrorSummaries: api.GlobalErrors,
	}
	directorySSOSettingSetMethod = api.Method{
		API:            directorySSOSettingAPIName,
		Method:         "set",
		Version:        1,
		ErrorSummaries: api.GlobalErrors,
	}
)

func NewDirectoryOIDCSSOResource() resource.Resource {
	return &DirectoryOIDCSSOResource{}
}

type DirectoryOIDCSSOResource struct {
	client     synology.Api
	httpClient *http.Client
}

func (r *DirectoryOIDCSSOResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = buildName(req.ProviderTypeName, "directory_oidc_sso")
}

func (r *DirectoryOIDCSSOResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages the DSM OpenID Connect SSO client configuration.

This resource models the singleton DSM OIDC SSO client under Control Panel -> Domain/LDAP -> SSO Client.
It manages the OIDC client settings, whether the OIDC profile is enabled, and whether DSM should select SSO by default on the login page.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Stable singleton resource ID.",
				Computed:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether DSM should enable the OIDC SSO profile. Default: `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"default_login": schema.BoolAttribute{
				MarkdownDescription: "Whether DSM should select SSO by default on the login page. Default: `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"allow_local_user": schema.BoolAttribute{
				MarkdownDescription: "Whether DSM should allow local users for the OIDC SSO client. Default: `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name shown in DSM for the OIDC SSO client.",
				Required:            true,
			},
			"wellknown_url": schema.StringAttribute{
				MarkdownDescription: "OIDC discovery document URL.",
				Required:            true,
			},
			"client_id": schema.StringAttribute{
				MarkdownDescription: "OIDC client ID.",
				Required:            true,
			},
			"client_secret": schema.StringAttribute{
				MarkdownDescription: "OIDC client secret.",
				Required:            true,
				Sensitive:           true,
			},
			"redirect_uri": schema.StringAttribute{
				MarkdownDescription: "DSM redirect URI used for the OIDC SSO client.",
				Required:            true,
			},
			"scope": schema.StringAttribute{
				MarkdownDescription: "Authorization scope string. Default: `openid profile email`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("openid profile email"),
			},
			"username_claim": schema.StringAttribute{
				MarkdownDescription: "OIDC claim DSM should use as the username.",
				Required:            true,
			},
			"authorization_endpoint": schema.StringAttribute{
				MarkdownDescription: "Resolved authorization endpoint from the well-known URL.",
				Computed:            true,
			},
			"token_endpoint": schema.StringAttribute{
				MarkdownDescription: "Resolved token endpoint from the well-known URL.",
				Computed:            true,
			},
		},
	}
}

func (r *DirectoryOIDCSSOResource) Configure(
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
			fmt.Sprintf(
				"Expected client.Client, got: %T. Please report this issue to the provider developers.",
				req.ProviderData,
			),
		)
		return
	}

	r.client = client
	r.httpClient = &http.Client{Timeout: 15 * time.Second}
}

func (r *DirectoryOIDCSSOResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var data DirectoryOIDCSSOResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	state, err := r.apply(ctx, data)
	if err != nil {
		resp.Diagnostics.AddError("Failed to configure DSM OIDC SSO", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DirectoryOIDCSSOResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state DirectoryOIDCSSOResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	refreshed, err := r.readState(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read DSM OIDC SSO", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &refreshed)...)
}

func (r *DirectoryOIDCSSOResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var data DirectoryOIDCSSOResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	state, err := r.apply(ctx, data)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update DSM OIDC SSO", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DirectoryOIDCSSOResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	disableReq := directorySSOProfileSetRequest{
		Enabled: false,
		Profile: directoryOIDCSSOProfileName,
	}
	if _, err := api.Get[struct{}](
		r.client,
		ctx,
		&disableReq,
		directorySSOProfileSetMethod,
	); err != nil {
		resp.Diagnostics.AddError("Failed to disable DSM OIDC SSO profile", err.Error())
		return
	}

	defaultLoginReq := directorySSOSettingSetRequest{DefaultLogin: false}
	if _, err := api.Get[struct{}](
		r.client,
		ctx,
		&defaultLoginReq,
		directorySSOSettingSetMethod,
	); err != nil {
		resp.Diagnostics.AddError("Failed to reset DSM SSO default login", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *DirectoryOIDCSSOResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	if strings.TrimSpace(req.ID) == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Import ID must not be empty.")
		return
	}

	resp.Diagnostics.Append(
		resp.State.SetAttribute(ctx, path.Root("id"), directoryOIDCSSOResourceID)...)
}

func (r *DirectoryOIDCSSOResource) apply(
	ctx context.Context,
	plan DirectoryOIDCSSOResourceModel,
) (DirectoryOIDCSSOResourceModel, error) {
	if !plan.Enabled.IsUnknown() && !plan.Enabled.IsNull() && !plan.Enabled.ValueBool() &&
		!plan.DefaultLogin.IsUnknown() && !plan.DefaultLogin.IsNull() && plan.DefaultLogin.ValueBool() {
		return DirectoryOIDCSSOResourceModel{}, fmt.Errorf(
			"default_login cannot be true when enabled is false",
		)
	}

	discovery, err := r.fetchDiscoveryDocument(ctx, plan.WellKnownURL.ValueString())
	if err != nil {
		return DirectoryOIDCSSOResourceModel{}, err
	}

	oidcReq := directoryOIDCSSOSetRequest{
		AllowLocalUser:        plan.AllowLocalUser.ValueBool(),
		AuthorizationEndpoint: discovery.AuthorizationEndpoint,
		ClientID:              plan.ClientID.ValueString(),
		ClientSecret:          plan.ClientSecret.ValueString(),
		Name:                  plan.Name.ValueString(),
		RedirectURI:           plan.RedirectURI.ValueString(),
		Scope:                 plan.Scope.ValueString(),
		TokenEndpoint:         discovery.TokenEndpoint,
		UsernameClaim:         plan.UsernameClaim.ValueString(),
		WellKnownURL:          plan.WellKnownURL.ValueString(),
	}
	if _, err := api.Get[struct{}](r.client, ctx, &oidcReq, directoryOIDCSSOSetMethod); err != nil {
		return DirectoryOIDCSSOResourceModel{}, err
	}

	profileReq := directorySSOProfileSetRequest{
		Enabled: plan.Enabled.ValueBool(),
		Profile: directoryOIDCSSOProfileName,
	}
	if _, err := api.Get[struct{}](
		r.client,
		ctx,
		&profileReq,
		directorySSOProfileSetMethod,
	); err != nil {
		return DirectoryOIDCSSOResourceModel{}, err
	}

	defaultLoginReq := directorySSOSettingSetRequest{DefaultLogin: plan.DefaultLogin.ValueBool()}
	if _, err := api.Get[struct{}](
		r.client,
		ctx,
		&defaultLoginReq,
		directorySSOSettingSetMethod,
	); err != nil {
		return DirectoryOIDCSSOResourceModel{}, err
	}

	return r.readState(ctx)
}

func (r *DirectoryOIDCSSOResource) readState(
	ctx context.Context,
) (DirectoryOIDCSSOResourceModel, error) {
	oidcResp, err := api.Get[directoryOIDCSSOGetResponse](
		r.client,
		ctx,
		&struct{}{},
		directoryOIDCSSOGetMethod,
	)
	if err != nil {
		return DirectoryOIDCSSOResourceModel{}, err
	}

	profileResp, err := api.Get[directorySSOProfileGetResponse](
		r.client,
		ctx,
		&struct{}{},
		directorySSOProfileGetMethod,
	)
	if err != nil {
		return DirectoryOIDCSSOResourceModel{}, err
	}

	settingResp, err := api.Get[directorySSOSettingGetResponse](
		r.client,
		ctx,
		&struct{}{},
		directorySSOSettingGetMethod,
	)
	if err != nil {
		return DirectoryOIDCSSOResourceModel{}, err
	}

	return DirectoryOIDCSSOResourceModel{
		ID: types.StringValue(directoryOIDCSSOResourceID),
		Enabled: types.BoolValue(
			profileResp.Enabled && profileResp.Profile == directoryOIDCSSOProfileName,
		),
		DefaultLogin:          types.BoolValue(settingResp.DefaultLogin),
		AllowLocalUser:        types.BoolValue(oidcResp.AllowLocalUser),
		Name:                  types.StringValue(oidcResp.Name),
		WellKnownURL:          types.StringValue(oidcResp.WellKnownURL),
		ClientID:              types.StringValue(oidcResp.ClientID),
		ClientSecret:          types.StringValue(oidcResp.ClientSecret),
		RedirectURI:           types.StringValue(oidcResp.RedirectURI),
		Scope:                 types.StringValue(oidcResp.Scope),
		UsernameClaim:         types.StringValue(oidcResp.UsernameClaim),
		AuthorizationEndpoint: types.StringValue(oidcResp.AuthorizationEndpoint),
		TokenEndpoint:         types.StringValue(oidcResp.TokenEndpoint),
	}, nil
}

func (r *DirectoryOIDCSSOResource) fetchDiscoveryDocument(
	ctx context.Context,
	wellKnownURL string,
) (oidcDiscoveryDocument, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnownURL, nil)
	if err != nil {
		return oidcDiscoveryDocument{}, err
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return oidcDiscoveryDocument{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return oidcDiscoveryDocument{}, fmt.Errorf(
			"OIDC discovery request to %q returned %s",
			wellKnownURL,
			resp.Status,
		)
	}

	var discovery oidcDiscoveryDocument
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return oidcDiscoveryDocument{}, err
	}

	if discovery.AuthorizationEndpoint == "" {
		return oidcDiscoveryDocument{}, fmt.Errorf(
			"OIDC discovery document at %q did not include authorization_endpoint",
			wellKnownURL,
		)
	}
	if discovery.TokenEndpoint == "" {
		return oidcDiscoveryDocument{}, fmt.Errorf(
			"OIDC discovery document at %q did not include token_endpoint",
			wellKnownURL,
		)
	}

	return discovery, nil
}
