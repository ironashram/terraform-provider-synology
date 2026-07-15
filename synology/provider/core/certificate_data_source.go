package core

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	client "github.com/synology-community/go-synology"
)

var _ datasource.DataSource = &CertificateDataSource{}

func NewCertificateDataSource() datasource.DataSource {
	return &CertificateDataSource{}
}

type CertificateDataSource struct {
	client client.Api
}

type CertificateDataSourceModel struct {
	CertificateID           types.String `tfsdk:"certificate_id"`
	Domain                  types.String `tfsdk:"domain"`
	ID                      types.String `tfsdk:"id"`
	Description             types.String `tfsdk:"description"`
	IsDefault               types.Bool   `tfsdk:"is_default"`
	IsBroken                types.Bool   `tfsdk:"is_broken"`
	CommonName              types.String `tfsdk:"common_name"`
	SubjectAlternativeNames types.List   `tfsdk:"subject_alternative_names"`
	ValidTill               types.String `tfsdk:"valid_till"`
}

func (d *CertificateDataSource) Metadata(
	_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = buildName(req.ProviderTypeName, "certificate")
}

func (d *CertificateDataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Looks up a DSM certificate by ID or domain.

When ` + "`domain`" + ` is used, the provider selects the best non-broken matching certificate,
preferring the DSM default certificate and then the latest expiry time. Wildcard SAN/CN matching
is supported for single-label wildcards such as ` + "`*.synology.example.com`" + `.
`,
		Attributes: map[string]schema.Attribute{
			"certificate_id": schema.StringAttribute{
				MarkdownDescription: "Exact DSM certificate ID to look up.",
				Optional:            true,
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "Domain to match against certificate CN or SAN entries.",
				Optional:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Selected DSM certificate ID.",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "DSM certificate description.",
				Computed:            true,
			},
			"is_default": schema.BoolAttribute{
				MarkdownDescription: "Whether this is the current DSM default certificate.",
				Computed:            true,
			},
			"is_broken": schema.BoolAttribute{
				MarkdownDescription: "Whether DSM marks this certificate as broken.",
				Computed:            true,
			},
			"common_name": schema.StringAttribute{
				MarkdownDescription: "Certificate subject common name.",
				Computed:            true,
			},
			"subject_alternative_names": schema.ListAttribute{
				MarkdownDescription: "Certificate subject alternative names.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"valid_till": schema.StringAttribute{
				MarkdownDescription: "DSM certificate expiry timestamp string.",
				Computed:            true,
			},
		},
	}
}

func (d *CertificateDataSource) Configure(
	_ context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
) {
	if req.ProviderData == nil {
		return
	}

	configured, ok := req.ProviderData.(client.Api)
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

	d.client = configured
}

func (d *CertificateDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	var data CertificateDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hasCertificateID := !data.CertificateID.IsNull() && !data.CertificateID.IsUnknown() &&
		data.CertificateID.ValueString() != ""
	hasDomain := !data.Domain.IsNull() && !data.Domain.IsUnknown() &&
		data.Domain.ValueString() != ""

	if hasCertificateID == hasDomain {
		resp.Diagnostics.AddError(
			"Invalid certificate lookup configuration",
			"Exactly one of `certificate_id` or `domain` must be set.",
		)
		return
	}

	certificates, err := listCertificates(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to list DSM certificates", err.Error())
		return
	}

	var selected *synologyCertificate
	if hasCertificateID {
		selected = findCertificateByID(certificates, data.CertificateID.ValueString())
		if selected == nil {
			resp.Diagnostics.AddError(
				"Certificate not found",
				fmt.Sprintf(
					"No DSM certificate with id %q was found.",
					data.CertificateID.ValueString(),
				),
			)
			return
		}
	} else {
		selected = selectCertificateForDomain(certificates, data.Domain.ValueString())
		if selected == nil {
			resp.Diagnostics.AddError(
				"Certificate not found",
				fmt.Sprintf(
					"No non-broken DSM certificate matched domain %q.",
					data.Domain.ValueString(),
				),
			)
			return
		}
	}

	sans, diags := types.ListValueFrom(ctx, types.StringType, selected.Subject.SubAltName)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(selected.ID)
	data.Description = types.StringValue(selected.Desc)
	data.IsDefault = types.BoolValue(selected.IsDefault)
	data.IsBroken = types.BoolValue(selected.IsBroken)
	data.CommonName = types.StringValue(selected.Subject.CommonName)
	data.SubjectAlternativeNames = sans
	data.ValidTill = types.StringValue(selected.ValidTill)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
