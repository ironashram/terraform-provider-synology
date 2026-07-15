package core

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/synology-community/go-synology"
	"github.com/synology-community/go-synology/pkg/api"
)

const (
	reverseProxyAPIName       = "SYNO.Core.AppPortal.ReverseProxy"
	reverseProxyProtocolHTTP  = int64(0)
	reverseProxyProtocolHTTPS = int64(1)
)

var (
	_ resource.Resource                = &ReverseProxyResource{}
	_ resource.ResourceWithImportState = &ReverseProxyResource{}
)

func NewReverseProxyResource() resource.Resource {
	return &ReverseProxyResource{}
}

type ReverseProxyResource struct {
	client synology.Api
}

type ReverseProxyResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	ProxyConnectTimeout  types.Int64  `tfsdk:"proxy_connect_timeout"`
	ProxyReadTimeout     types.Int64  `tfsdk:"proxy_read_timeout"`
	ProxySendTimeout     types.Int64  `tfsdk:"proxy_send_timeout"`
	ProxyHTTPVersion     types.Int64  `tfsdk:"proxy_http_version"`
	ProxyInterceptErrors types.Bool   `tfsdk:"proxy_intercept_errors"`
	Frontend             types.Object `tfsdk:"frontend"`
	Backend              types.Object `tfsdk:"backend"`
	CustomizeHeaders     types.Set    `tfsdk:"customize_headers"`
}

type ReverseProxyFrontendModel struct {
	Host     types.String `tfsdk:"host"`
	Port     types.Int64  `tfsdk:"port"`
	Protocol types.String `tfsdk:"protocol"`
	HSTS     types.Bool   `tfsdk:"hsts"`
}

type ReverseProxyBackendModel struct {
	Host     types.String `tfsdk:"host"`
	Port     types.Int64  `tfsdk:"port"`
	Protocol types.String `tfsdk:"protocol"`
}

type ReverseProxyCustomHeaderModel struct {
	Name  types.String `tfsdk:"name"`
	Value types.String `tfsdk:"value"`
}

func (m ReverseProxyFrontendModel) AttrType() map[string]attr.Type {
	return map[string]attr.Type{
		"host":     types.StringType,
		"port":     types.Int64Type,
		"protocol": types.StringType,
		"hsts":     types.BoolType,
	}
}

func (m ReverseProxyFrontendModel) ObjectValue() (types.Object, diag.Diagnostics) {
	return types.ObjectValue(m.AttrType(), map[string]attr.Value{
		"host":     m.Host,
		"port":     m.Port,
		"protocol": m.Protocol,
		"hsts":     m.HSTS,
	})
}

func (m ReverseProxyBackendModel) AttrType() map[string]attr.Type {
	return map[string]attr.Type{
		"host":     types.StringType,
		"port":     types.Int64Type,
		"protocol": types.StringType,
	}
}

func (m ReverseProxyBackendModel) ObjectValue() (types.Object, diag.Diagnostics) {
	return types.ObjectValue(m.AttrType(), map[string]attr.Value{
		"host":     m.Host,
		"port":     m.Port,
		"protocol": m.Protocol,
	})
}

func (m ReverseProxyCustomHeaderModel) AttrType() map[string]attr.Type {
	return map[string]attr.Type{
		"name":  types.StringType,
		"value": types.StringType,
	}
}

func (m ReverseProxyCustomHeaderModel) ObjectValue() (types.Object, diag.Diagnostics) {
	return types.ObjectValue(m.AttrType(), map[string]attr.Value{
		"name":  m.Name,
		"value": m.Value,
	})
}

type reverseProxyListResponse struct {
	Entries []reverseProxyAPIEntry `json:"entries"`
}

type reverseProxyEntryRequest struct {
	Entry string `url:"entry"`
}

type reverseProxyDeleteRequest struct {
	UUIDs string `url:"uuids"`
}

type reverseProxyAPIEntry struct {
	UUID string `json:"UUID,omitempty"`
	// DSM 7 has no separate name field: the UI "reverse proxy name" is
	// stored as description, and a name key in the payload fails with 4151.
	Description          string                  `json:"description,omitempty"`
	ProxyConnectTimeout  int64                   `json:"proxy_connect_timeout"`
	ProxyReadTimeout     int64                   `json:"proxy_read_timeout"`
	ProxySendTimeout     int64                   `json:"proxy_send_timeout"`
	ProxyHTTPVersion     int64                   `json:"proxy_http_version"`
	ProxyInterceptErrors bool                    `json:"proxy_intercept_errors"`
	Frontend             reverseProxyAPIFrontend `json:"frontend"`
	Backend              reverseProxyAPIBackend  `json:"backend"`
	CustomizeHeaders     []reverseProxyAPIHeader `json:"customize_headers,omitempty"`
}

type reverseProxyAPIFrontend struct {
	ACL      any                  `json:"acl"`
	FQDN     string               `json:"fqdn"`
	Port     int64                `json:"port"`
	Protocol int64                `json:"protocol"`
	HTTPS    reverseProxyAPIHTTPS `json:"https"`
}

type reverseProxyAPIHTTPS struct {
	HSTS bool `json:"hsts"`
}

type reverseProxyAPIBackend struct {
	FQDN     string `json:"fqdn"`
	Port     int64  `json:"port"`
	Protocol int64  `json:"protocol"`
}

type reverseProxyAPIHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

var (
	reverseProxyListMethod = api.Method{
		API:            reverseProxyAPIName,
		Method:         "list",
		Version:        1,
		ErrorSummaries: api.GlobalErrors,
	}
	reverseProxyCreateMethod = api.Method{
		API:            reverseProxyAPIName,
		Method:         "create",
		Version:        1,
		ErrorSummaries: api.GlobalErrors,
	}
	reverseProxyUpdateMethod = api.Method{
		API:            reverseProxyAPIName,
		Method:         "update",
		Version:        1,
		ErrorSummaries: api.GlobalErrors,
	}
	reverseProxyDeleteMethod = api.Method{
		API:            reverseProxyAPIName,
		Method:         "delete",
		Version:        1,
		ErrorSummaries: api.GlobalErrors,
	}
)

func (r *ReverseProxyResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = buildName(req.ProviderTypeName, "reverse_proxy")
}

func (r *ReverseProxyResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages a DSM reverse-proxy rule.

This resource models a single host-based DSM reverse-proxy entry. It is a good fit for
Vault, MinIO, and similar services that sit behind the Synology reverse proxy.

Certificate issuance and certificate-to-service binding remain separate concerns.

## Example Usage

` + "```hcl" + `
resource "synology_core_reverse_proxy" "vault" {
  name = "vault"

  frontend = {
    host     = "vault.synology.example.com"
    protocol = "https"
    port     = 443
    hsts     = true
  }

  backend = {
    host     = "127.0.0.1"
    protocol = "http"
    port     = 8200
  }
}
` + "```",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "DSM reverse-proxy rule UUID.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Rule name shown in DSM (stored as the entry description on DSM 7).",
				Required:            true,
			},
			"proxy_connect_timeout": schema.Int64Attribute{
				MarkdownDescription: "Upstream connect timeout in seconds. Default: `60`.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(60),
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
			},
			"proxy_read_timeout": schema.Int64Attribute{
				MarkdownDescription: "Upstream read timeout in seconds. Default: `60`.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(60),
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
			},
			"proxy_send_timeout": schema.Int64Attribute{
				MarkdownDescription: "Upstream send timeout in seconds. Default: `60`.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(60),
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
			},
			"proxy_http_version": schema.Int64Attribute{
				MarkdownDescription: "Upstream HTTP version. Default: `1`.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1),
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"proxy_intercept_errors": schema.BoolAttribute{
				MarkdownDescription: "Whether DSM should intercept upstream errors. Default: `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"frontend": schema.SingleNestedAttribute{
				MarkdownDescription: "Public-facing listener definition.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"host": schema.StringAttribute{
						MarkdownDescription: "Frontend hostname served by DSM.",
						Required:            true,
					},
					"port": schema.Int64Attribute{
						MarkdownDescription: "Frontend port. Default: `443`.",
						Optional:            true,
						Computed:            true,
						Default:             int64default.StaticInt64(443),
						Validators: []validator.Int64{
							int64validator.AtLeast(1),
						},
					},
					"protocol": schema.StringAttribute{
						MarkdownDescription: "Frontend protocol. Valid values are `http` and `https`. Default: `https`.",
						Optional:            true,
						Computed:            true,
						Default:             stringdefault.StaticString("https"),
						Validators: []validator.String{
							stringvalidator.OneOf("http", "https"),
						},
					},
					"hsts": schema.BoolAttribute{
						MarkdownDescription: "Whether to enable HSTS for HTTPS listeners. Default: `true`.",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(true),
					},
				},
			},
			"backend": schema.SingleNestedAttribute{
				MarkdownDescription: "Upstream target definition.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"host": schema.StringAttribute{
						MarkdownDescription: "Upstream hostname or IP.",
						Required:            true,
					},
					"port": schema.Int64Attribute{
						MarkdownDescription: "Upstream port.",
						Required:            true,
						Validators: []validator.Int64{
							int64validator.AtLeast(1),
						},
					},
					"protocol": schema.StringAttribute{
						MarkdownDescription: "Upstream protocol. Valid values are `http` and `https`. Default: `http`.",
						Optional:            true,
						Computed:            true,
						Default:             stringdefault.StaticString("http"),
						Validators: []validator.String{
							stringvalidator.OneOf("http", "https"),
						},
					},
				},
			},
			"customize_headers": schema.SetNestedAttribute{
				MarkdownDescription: "Custom request headers forwarded by DSM.",
				Optional:            true,
				Computed:            true,
				Default: setdefault.StaticValue(types.SetValueMust(
					types.ObjectType{AttrTypes: ReverseProxyCustomHeaderModel{}.AttrType()},
					[]attr.Value{},
				)),
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Header name.",
							Required:            true,
						},
						"value": schema.StringAttribute{
							MarkdownDescription: "Header value.",
							Required:            true,
						},
					},
				},
			},
		},
	}
}

func (r *ReverseProxyResource) Configure(
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
}

func (r *ReverseProxyResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var data ReverseProxyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	existing, err := r.findByName(ctx, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to inspect existing reverse-proxy rules", err.Error())
		return
	}
	if existing != nil {
		resp.Diagnostics.AddError(
			"Reverse-proxy rule already exists",
			fmt.Sprintf(
				"A DSM reverse-proxy rule named %q already exists with UUID %q.",
				existing.Description,
				existing.UUID,
			),
		)
		return
	}

	entry, diags := data.toAPIEntry(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.createEntry(ctx, entry); err != nil {
		resp.Diagnostics.AddError("Failed to create reverse-proxy rule", err.Error())
		return
	}

	created, err := r.waitForByName(ctx, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read created reverse-proxy rule", err.Error())
		return
	}
	if created == nil {
		resp.Diagnostics.AddError(
			"Failed to read created reverse-proxy rule",
			fmt.Sprintf(
				"DSM did not return a reverse-proxy rule named %q after creation.",
				data.Name.ValueString(),
			),
		)
		return
	}

	state, diags := reverseProxyModelFromAPI(*created)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ReverseProxyResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var data ReverseProxyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	entry, err := r.findByUUID(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read reverse-proxy rule", err.Error())
		return
	}
	if entry == nil {
		entry, err = r.waitForByUUID(ctx, data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to read reverse-proxy rule", err.Error())
			return
		}
	}
	if entry == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state, diags := reverseProxyModelFromAPI(*entry)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ReverseProxyResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan ReverseProxyResourceModel
	var state ReverseProxyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	existing, err := r.findByName(ctx, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to inspect existing reverse-proxy rules", err.Error())
		return
	}
	if existing != nil && existing.UUID != state.ID.ValueString() {
		resp.Diagnostics.AddError(
			"Reverse-proxy rule name already in use",
			fmt.Sprintf(
				"A different DSM reverse-proxy rule already uses the name %q (UUID %q).",
				existing.Description,
				existing.UUID,
			),
		)
		return
	}

	entry, diags := plan.toAPIEntry(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	entry.UUID = state.ID.ValueString()

	if err := r.updateEntry(ctx, entry); err != nil {
		resp.Diagnostics.AddError("Failed to update reverse-proxy rule", err.Error())
		return
	}

	updated, err := r.waitForByUUID(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read updated reverse-proxy rule", err.Error())
		return
	}
	if updated == nil {
		resp.Diagnostics.AddError(
			"Failed to read updated reverse-proxy rule",
			fmt.Sprintf(
				"DSM did not return reverse-proxy rule UUID %q after update.",
				state.ID.ValueString(),
			),
		)
		return
	}

	newState, diags := reverseProxyModelFromAPI(*updated)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *ReverseProxyResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var data ReverseProxyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	entry, err := r.findByUUID(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to inspect reverse-proxy rules", err.Error())
		return
	}
	if entry == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.deleteEntry(ctx, data.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete reverse-proxy rule", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *ReverseProxyResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	entry, err := r.waitForByUUID(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read imported reverse-proxy rule", err.Error())
		return
	}
	if entry == nil {
		resp.Diagnostics.AddError(
			"Reverse-proxy rule not found",
			fmt.Sprintf("No DSM reverse-proxy rule with UUID %q was found.", req.ID),
		)
		return
	}

	state, diags := reverseProxyModelFromAPI(*entry)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (m ReverseProxyResourceModel) toAPIEntry(
	ctx context.Context,
) (reverseProxyAPIEntry, diag.Diagnostics) {
	var diags diag.Diagnostics

	var frontend ReverseProxyFrontendModel
	diags.Append(m.Frontend.As(ctx, &frontend, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return reverseProxyAPIEntry{}, diags
	}

	var backend ReverseProxyBackendModel
	diags.Append(m.Backend.As(ctx, &backend, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return reverseProxyAPIEntry{}, diags
	}

	headers := []ReverseProxyCustomHeaderModel{}
	if !m.CustomizeHeaders.IsNull() && !m.CustomizeHeaders.IsUnknown() {
		diags.Append(m.CustomizeHeaders.ElementsAs(ctx, &headers, true)...)
		if diags.HasError() {
			return reverseProxyAPIEntry{}, diags
		}
	}

	frontendProtocol, err := encodeReverseProxyProtocol(frontend.Protocol.ValueString())
	if err != nil {
		diags.AddError("Invalid frontend protocol", err.Error())
		return reverseProxyAPIEntry{}, diags
	}

	backendProtocol, err := encodeReverseProxyProtocol(backend.Protocol.ValueString())
	if err != nil {
		diags.AddError("Invalid backend protocol", err.Error())
		return reverseProxyAPIEntry{}, diags
	}

	customHeaders := make([]reverseProxyAPIHeader, 0, len(headers))
	for _, header := range headers {
		customHeaders = append(customHeaders, reverseProxyAPIHeader{
			Name:  header.Name.ValueString(),
			Value: header.Value.ValueString(),
		})
	}

	return reverseProxyAPIEntry{
		Description:          m.Name.ValueString(),
		ProxyConnectTimeout:  m.ProxyConnectTimeout.ValueInt64(),
		ProxyReadTimeout:     m.ProxyReadTimeout.ValueInt64(),
		ProxySendTimeout:     m.ProxySendTimeout.ValueInt64(),
		ProxyHTTPVersion:     m.ProxyHTTPVersion.ValueInt64(),
		ProxyInterceptErrors: m.ProxyInterceptErrors.ValueBool(),
		Frontend: reverseProxyAPIFrontend{
			ACL:      nil,
			FQDN:     frontend.Host.ValueString(),
			Port:     frontend.Port.ValueInt64(),
			Protocol: frontendProtocol,
			HTTPS: reverseProxyAPIHTTPS{
				HSTS: frontend.HSTS.ValueBool(),
			},
		},
		Backend: reverseProxyAPIBackend{
			FQDN:     backend.Host.ValueString(),
			Port:     backend.Port.ValueInt64(),
			Protocol: backendProtocol,
		},
		CustomizeHeaders: customHeaders,
	}, diags
}

func reverseProxyModelFromAPI(
	entry reverseProxyAPIEntry,
) (ReverseProxyResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	frontendProtocol, err := decodeReverseProxyProtocol(entry.Frontend.Protocol)
	if err != nil {
		diags.AddError("Invalid frontend protocol returned by DSM", err.Error())
		return ReverseProxyResourceModel{}, diags
	}

	backendProtocol, err := decodeReverseProxyProtocol(entry.Backend.Protocol)
	if err != nil {
		diags.AddError("Invalid backend protocol returned by DSM", err.Error())
		return ReverseProxyResourceModel{}, diags
	}

	frontendValue, valueDiags := ReverseProxyFrontendModel{
		Host:     types.StringValue(entry.Frontend.FQDN),
		Port:     types.Int64Value(entry.Frontend.Port),
		Protocol: types.StringValue(frontendProtocol),
		HSTS:     types.BoolValue(entry.Frontend.HTTPS.HSTS),
	}.ObjectValue()
	diags.Append(valueDiags...)
	if diags.HasError() {
		return ReverseProxyResourceModel{}, diags
	}

	backendValue, valueDiags := ReverseProxyBackendModel{
		Host:     types.StringValue(entry.Backend.FQDN),
		Port:     types.Int64Value(entry.Backend.Port),
		Protocol: types.StringValue(backendProtocol),
	}.ObjectValue()
	diags.Append(valueDiags...)
	if diags.HasError() {
		return ReverseProxyResourceModel{}, diags
	}

	headerValues := make([]attr.Value, 0, len(entry.CustomizeHeaders))
	for _, header := range entry.CustomizeHeaders {
		headerValue, valueDiags := ReverseProxyCustomHeaderModel{
			Name:  types.StringValue(header.Name),
			Value: types.StringValue(header.Value),
		}.ObjectValue()
		diags.Append(valueDiags...)
		if diags.HasError() {
			return ReverseProxyResourceModel{}, diags
		}
		headerValues = append(headerValues, headerValue)
	}

	headersSet, setDiags := types.SetValue(
		types.ObjectType{AttrTypes: ReverseProxyCustomHeaderModel{}.AttrType()},
		headerValues,
	)
	diags.Append(setDiags...)
	if diags.HasError() {
		return ReverseProxyResourceModel{}, diags
	}

	return ReverseProxyResourceModel{
		ID:                   types.StringValue(entry.UUID),
		Name:                 types.StringValue(entry.Description),
		ProxyConnectTimeout:  types.Int64Value(entry.ProxyConnectTimeout),
		ProxyReadTimeout:     types.Int64Value(entry.ProxyReadTimeout),
		ProxySendTimeout:     types.Int64Value(entry.ProxySendTimeout),
		ProxyHTTPVersion:     types.Int64Value(entry.ProxyHTTPVersion),
		ProxyInterceptErrors: types.BoolValue(entry.ProxyInterceptErrors),
		Frontend:             frontendValue,
		Backend:              backendValue,
		CustomizeHeaders:     headersSet,
	}, diags
}

func (r *ReverseProxyResource) listEntries(ctx context.Context) ([]reverseProxyAPIEntry, error) {
	r.resetClientSessionQuery()
	req := struct{}{}
	resp, err := api.Get[reverseProxyListResponse](r.client, ctx, &req, reverseProxyListMethod)
	if err != nil {
		return nil, err
	}
	return resp.Entries, nil
}

func (r *ReverseProxyResource) findByUUID(
	ctx context.Context,
	uuid string,
) (*reverseProxyAPIEntry, error) {
	entries, err := r.listEntries(ctx)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.UUID == uuid {
			found := entry
			return &found, nil
		}
	}
	return nil, nil
}

func (r *ReverseProxyResource) findByName(
	ctx context.Context,
	name string,
) (*reverseProxyAPIEntry, error) {
	entries, err := r.listEntries(ctx)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.Description == name {
			found := entry
			return &found, nil
		}
	}
	return nil, nil
}

func (r *ReverseProxyResource) createEntry(
	ctx context.Context,
	entry reverseProxyAPIEntry,
) error {
	r.resetClientSessionQuery()
	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	req := reverseProxyEntryRequest{
		Entry: string(entryJSON),
	}

	_, err = api.Get[struct{}](r.client, ctx, &req, reverseProxyCreateMethod)
	return err
}

func (r *ReverseProxyResource) updateEntry(
	ctx context.Context,
	entry reverseProxyAPIEntry,
) error {
	r.resetClientSessionQuery()
	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	req := reverseProxyEntryRequest{
		Entry: string(entryJSON),
	}

	_, err = api.Get[struct{}](r.client, ctx, &req, reverseProxyUpdateMethod)
	return err
}

func (r *ReverseProxyResource) deleteEntry(ctx context.Context, uuid string) error {
	r.resetClientSessionQuery()
	uuidsJSON, err := json.Marshal([]string{uuid})
	if err != nil {
		return err
	}

	req := reverseProxyDeleteRequest{
		UUIDs: string(uuidsJSON),
	}

	_, err = api.Get[struct{}](r.client, ctx, &req, reverseProxyDeleteMethod)
	return err
}

func (r *ReverseProxyResource) resetClientSessionQuery() {
	c, ok := r.client.(*synology.Client)
	if !ok {
		return
	}

	session := c.ExportSession()
	if session.SessionID == "" && session.SynoToken == "" {
		return
	}

	c.ImportSession(session)
}

func (r *ReverseProxyResource) waitForByUUID(
	ctx context.Context,
	uuid string,
) (*reverseProxyAPIEntry, error) {
	return r.waitForEntry(ctx, func(ctx context.Context) (*reverseProxyAPIEntry, error) {
		return r.findByUUID(ctx, uuid)
	})
}

func (r *ReverseProxyResource) waitForByName(
	ctx context.Context,
	name string,
) (*reverseProxyAPIEntry, error) {
	return r.waitForEntry(ctx, func(ctx context.Context) (*reverseProxyAPIEntry, error) {
		return r.findByName(ctx, name)
	})
}

func (r *ReverseProxyResource) waitForEntry(
	ctx context.Context,
	find func(context.Context) (*reverseProxyAPIEntry, error),
) (*reverseProxyAPIEntry, error) {
	const (
		maxAttempts = 10
		retryDelay  = 500 * time.Millisecond
	)

	for attempt := 0; attempt < maxAttempts; attempt++ {
		entry, err := find(ctx)
		if err != nil {
			return nil, err
		}
		if entry != nil {
			return entry, nil
		}
		if attempt == maxAttempts-1 {
			break
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryDelay):
		}
	}

	return nil, nil
}

func encodeReverseProxyProtocol(protocol string) (int64, error) {
	switch protocol {
	case "http":
		return reverseProxyProtocolHTTP, nil
	case "https":
		return reverseProxyProtocolHTTPS, nil
	default:
		return 0, fmt.Errorf("unsupported protocol %q", protocol)
	}
}

func decodeReverseProxyProtocol(protocol int64) (string, error) {
	switch protocol {
	case reverseProxyProtocolHTTP:
		return "http", nil
	case reverseProxyProtocolHTTPS:
		return "https", nil
	default:
		return "", fmt.Errorf("unsupported DSM protocol code %d", protocol)
	}
}
