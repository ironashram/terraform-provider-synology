package core

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	client "github.com/synology-community/go-synology"
)

var (
	_ resource.Resource                = &CertificateServiceBindingResource{}
	_ resource.ResourceWithImportState = &CertificateServiceBindingResource{}
)

func NewCertificateServiceBindingResource() resource.Resource {
	return &CertificateServiceBindingResource{}
}

type CertificateServiceBindingResource struct {
	client client.Api
}

type CertificateServiceBindingResourceModel struct {
	ID            types.String `tfsdk:"id"`
	CertificateID types.String `tfsdk:"certificate_id"`
	Subscriber    types.String `tfsdk:"subscriber"`
	DisplayName   types.String `tfsdk:"display_name"`
}

func (r *CertificateServiceBindingResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = buildName(req.ProviderTypeName, "certificate_service_binding")
}

func (r *CertificateServiceBindingResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Binds a DSM certificate to a DSM service entry.

This resource is intended for services that DSM already knows about, such as reverse-proxy
hosts. On destroy, the service is rebound to the DSM default certificate.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Binding identifier in the form `subscriber/display_name`.",
				Computed:            true,
			},
			"certificate_id": schema.StringAttribute{
				MarkdownDescription: "Target DSM certificate ID.",
				Required:            true,
			},
			"subscriber": schema.StringAttribute{
				MarkdownDescription: "DSM service subscriber. Default: `ReverseProxy`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("ReverseProxy"),
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "DSM service display name, such as a reverse-proxy hostname.",
				Required:            true,
			},
		},
	}
}

func (r *CertificateServiceBindingResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	if req.ProviderData == nil {
		return
	}

	configured, ok := req.ProviderData.(client.Api)
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

	r.client = configured
}

func (r *CertificateServiceBindingResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var data CertificateServiceBindingResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.ensureBinding(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Failed to bind DSM certificate service", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CertificateServiceBindingResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var data CertificateServiceBindingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	certificates, err := listCertificates(ctx, r.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to list DSM certificates", err.Error())
		return
	}

	certificate, _, err := findCertificateServiceBinding(
		certificates,
		data.Subscriber.ValueString(),
		data.DisplayName.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to inspect DSM certificate service bindings", err.Error())
		return
	}
	if certificate == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.CertificateID = types.StringValue(certificate.ID)
	data.ID = types.StringValue(buildCertificateBindingID(
		data.Subscriber.ValueString(),
		data.DisplayName.ValueString(),
	))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CertificateServiceBindingResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var data CertificateServiceBindingResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.ensureBinding(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Failed to update DSM certificate service binding", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CertificateServiceBindingResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var data CertificateServiceBindingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	certificates, err := listCertificates(ctx, r.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to list DSM certificates", err.Error())
		return
	}

	currentCertificate, rawService, err := findCertificateServiceBinding(
		certificates,
		data.Subscriber.ValueString(),
		data.DisplayName.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to inspect DSM certificate service bindings", err.Error())
		return
	}
	if currentCertificate == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	defaultCertificate := findDefaultCertificate(certificates)
	if defaultCertificate == nil {
		resp.Diagnostics.AddError(
			"Default DSM certificate not found",
			"Cannot restore certificate binding on destroy because DSM returned no non-broken default certificate.",
		)
		return
	}

	if currentCertificate.ID != defaultCertificate.ID {
		if err := setCertificateServiceBinding(
			ctx,
			r.client,
			rawService,
			currentCertificate.ID,
			defaultCertificate.ID,
		); err != nil {
			resp.Diagnostics.AddError(
				"Failed to restore DSM default certificate binding",
				err.Error(),
			)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *CertificateServiceBindingResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	subscriber, displayName, err := parseCertificateBindingImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	resp.Diagnostics.Append(
		resp.State.SetAttribute(
			ctx,
			path.Root("id"),
			buildCertificateBindingID(subscriber, displayName),
		)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("subscriber"), subscriber)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("display_name"), displayName)...)
}

func (r *CertificateServiceBindingResource) ensureBinding(
	ctx context.Context,
	data *CertificateServiceBindingResourceModel,
) error {
	certificates, err := listCertificates(ctx, r.client)
	if err != nil {
		return err
	}

	targetCertificate := findCertificateByID(certificates, data.CertificateID.ValueString())
	if targetCertificate == nil {
		return fmt.Errorf(
			"no DSM certificate with id %q was found",
			data.CertificateID.ValueString(),
		)
	}
	if targetCertificate.IsBroken {
		return fmt.Errorf("DSM certificate %q is marked as broken", targetCertificate.ID)
	}

	currentCertificate, rawService, err := findCertificateServiceBinding(
		certificates,
		data.Subscriber.ValueString(),
		data.DisplayName.ValueString(),
	)
	if err != nil {
		return err
	}
	if currentCertificate == nil {
		return fmt.Errorf(
			"no DSM service binding exists for subscriber %q and display_name %q",
			data.Subscriber.ValueString(),
			data.DisplayName.ValueString(),
		)
	}

	if currentCertificate.ID != targetCertificate.ID {
		if err := setCertificateServiceBinding(
			ctx,
			r.client,
			rawService,
			currentCertificate.ID,
			targetCertificate.ID,
		); err != nil {
			return err
		}
	}

	data.ID = types.StringValue(buildCertificateBindingID(
		data.Subscriber.ValueString(),
		data.DisplayName.ValueString(),
	))
	return nil
}
