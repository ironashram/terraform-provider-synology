package core

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	client "github.com/synology-community/go-synology"
	"github.com/synology-community/go-synology/pkg/api"
	"github.com/synology-community/go-synology/pkg/util/form"
)

type CertificateImportResourceModel struct {
	ID                          types.String `tfsdk:"id"`
	Description                 types.String `tfsdk:"description"`
	AsDefault                   types.Bool   `tfsdk:"as_default"`
	CertificatePath             types.String `tfsdk:"certificate_path"`
	PrivateKeyPath              types.String `tfsdk:"private_key_path"`
	IntermediateCertificatePath types.String `tfsdk:"intermediate_certificate_path"`
	IsDefault                   types.Bool   `tfsdk:"is_default"`
	IsBroken                    types.Bool   `tfsdk:"is_broken"`
	CommonName                  types.String `tfsdk:"common_name"`
	SubjectAlternativeNames     types.List   `tfsdk:"subject_alternative_names"`
	ValidTill                   types.String `tfsdk:"valid_till"`
}

type certificateImportRequest struct {
	ID        string    `form:"id"         url:"id,omitempty"`
	Desc      string    `form:"desc"       url:"desc,omitempty"`
	AsDefault bool      `form:"as_default" url:"as_default,omitempty"`
	Key       form.File `form:"key"                                   kind:"file"`
	Cert      form.File `form:"cert"                                  kind:"file"`
	InterCert form.File `form:"inter_cert"                            kind:"file"`
}

type certificateImportResponse struct {
	ID           string `json:"id,omitempty"`
	RestartHTTPD bool   `json:"restart_httpd,omitempty"`
}

type certificateDeleteRequest struct {
	IDs []string `url:"ids,json"`
}

var (
	_ resource.Resource                = &CertificateImportResource{}
	_ resource.ResourceWithImportState = &CertificateImportResource{}
)

var (
	certificateImportMethod = api.Method{
		API:            "SYNO.Core.Certificate",
		Method:         "import",
		Version:        1,
		ErrorSummaries: api.GlobalErrors,
	}
	certificateDeleteMethod = api.Method{
		API:            certificateCRTAPIName,
		Method:         "delete",
		Version:        1,
		ErrorSummaries: api.GlobalErrors,
	}
)

func NewCertificateImportResource() resource.Resource {
	return &CertificateImportResource{}
}

type CertificateImportResource struct {
	client      client.Api
	fileStation interface {
		Download(ctx context.Context, path string, mode string) (*form.File, error)
	}
}

func (r *CertificateImportResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = buildName(req.ProviderTypeName, "certificate_import")
}

func (r *CertificateImportResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Imports or replaces a DSM certificate from certificate files already stored on the NAS.

The file paths must be Synology File Station paths rooted at a shared folder, such as
` + "`/docker/certbot/letsencrypt/live/synology.example.com/cert.pem`" + `.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "DSM certificate ID.",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "DSM certificate description.",
				Required:            true,
			},
			"as_default": schema.BoolAttribute{
				MarkdownDescription: "Whether DSM should set this certificate as the default certificate. Default: `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"certificate_path": schema.StringAttribute{
				MarkdownDescription: "NAS path to the PEM certificate file.",
				Required:            true,
			},
			"private_key_path": schema.StringAttribute{
				MarkdownDescription: "NAS path to the PEM private key file.",
				Required:            true,
			},
			"intermediate_certificate_path": schema.StringAttribute{
				MarkdownDescription: "NAS path to the PEM intermediate certificate chain file.",
				Required:            true,
			},
			"is_default": schema.BoolAttribute{
				MarkdownDescription: "Whether DSM marks this certificate as the default certificate.",
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

func (r *CertificateImportResource) Configure(
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
	r.fileStation = configured.FileStationAPI()
}

func (r *CertificateImportResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var data CertificateImportResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	existing, err := r.findByDescription(ctx, data.Description.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to inspect existing DSM certificates", err.Error())
		return
	}
	if existing != nil {
		data.ID = types.StringValue(existing.ID)
	}

	if err := r.importCertificate(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Failed to import DSM certificate", err.Error())
		return
	}

	refreshed, err := r.refreshState(ctx, data)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read imported DSM certificate", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &refreshed)...)
}

func (r *CertificateImportResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var data CertificateImportResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	refreshed, err := r.refreshState(ctx, data)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to read DSM certificate", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &refreshed)...)
}

func (r *CertificateImportResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var data CertificateImportResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &data.ID)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.importCertificate(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Failed to update DSM certificate", err.Error())
		return
	}

	refreshed, err := r.refreshState(ctx, data)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read updated DSM certificate", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &refreshed)...)
}

func (r *CertificateImportResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var data CertificateImportResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := api.Void(r.client, ctx, &certificateDeleteRequest{
		IDs: []string{data.ID.ValueString()},
	}, certificateDeleteMethod); err != nil {
		if _, readErr := r.findByID(ctx, data.ID.ValueString()); readErr != nil {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to delete DSM certificate", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *CertificateImportResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	certificate, err := r.findByID(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to find DSM certificate", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), certificate.ID)...)
	resp.Diagnostics.Append(
		resp.State.SetAttribute(ctx, path.Root("description"), certificate.Desc)...)
}

func (r *CertificateImportResource) importCertificate(
	ctx context.Context,
	data *CertificateImportResourceModel,
) error {
	request, err := r.buildImportRequest(ctx, data)
	if err != nil {
		return err
	}

	if _, err := api.PostFile[certificateImportResponse](
		r.client,
		ctx,
		&request,
		certificateImportMethod,
	); err != nil {
		return err
	}

	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		certificate, err := r.findByDescription(ctx, data.Description.ValueString())
		if err != nil {
			return err
		}
		if certificate == nil {
			return fmt.Errorf(
				"DSM did not return a certificate with description %q after import",
				data.Description.ValueString(),
			)
		}
		data.ID = types.StringValue(certificate.ID)
	}

	return nil
}

func (r *CertificateImportResource) buildImportRequest(
	ctx context.Context,
	data *CertificateImportResourceModel,
) (certificateImportRequest, error) {
	certFile, err := r.fileStation.Download(ctx, data.CertificatePath.ValueString(), "download")
	if err != nil {
		return certificateImportRequest{}, fmt.Errorf(
			"failed to read NAS certificate file %q: %w",
			data.CertificatePath.ValueString(),
			err,
		)
	}
	keyFile, err := r.fileStation.Download(ctx, data.PrivateKeyPath.ValueString(), "download")
	if err != nil {
		return certificateImportRequest{}, fmt.Errorf(
			"failed to read NAS private key file %q: %w",
			data.PrivateKeyPath.ValueString(),
			err,
		)
	}
	intermediateFile, err := r.fileStation.Download(
		ctx,
		data.IntermediateCertificatePath.ValueString(),
		"download",
	)
	if err != nil {
		return certificateImportRequest{}, fmt.Errorf(
			"failed to read NAS intermediate certificate file %q: %w",
			data.IntermediateCertificatePath.ValueString(),
			err,
		)
	}

	if err := validateCertificatePEM(certFile.Content, "certificate"); err != nil {
		return certificateImportRequest{}, err
	}
	if err := validateCertificatePEM(
		intermediateFile.Content,
		"intermediate certificate",
	); err != nil {
		return certificateImportRequest{}, err
	}
	if err := validatePrivateKeyPEM(keyFile.Content); err != nil {
		return certificateImportRequest{}, err
	}

	request := certificateImportRequest{
		Desc:      data.Description.ValueString(),
		AsDefault: data.AsDefault.ValueBool(),
		Key: form.File{
			Name:    fileNameFromPath(data.PrivateKeyPath.ValueString()),
			Content: keyFile.Content,
		},
		Cert: form.File{
			Name:    fileNameFromPath(data.CertificatePath.ValueString()),
			Content: certFile.Content,
		},
		InterCert: form.File{
			Name:    fileNameFromPath(data.IntermediateCertificatePath.ValueString()),
			Content: intermediateFile.Content,
		},
	}

	if !data.ID.IsNull() && !data.ID.IsUnknown() && data.ID.ValueString() != "" {
		request.ID = data.ID.ValueString()
	}

	return request, nil
}

func (r *CertificateImportResource) refreshState(
	ctx context.Context,
	data CertificateImportResourceModel,
) (CertificateImportResourceModel, error) {
	var selected *synologyCertificate
	var err error

	if !data.ID.IsNull() && !data.ID.IsUnknown() && data.ID.ValueString() != "" {
		selected, err = r.findByID(ctx, data.ID.ValueString())
	} else {
		selected, err = r.findByDescription(ctx, data.Description.ValueString())
	}
	if err != nil {
		return CertificateImportResourceModel{}, err
	}
	if selected == nil {
		return CertificateImportResourceModel{}, fmt.Errorf("DSM certificate not found")
	}

	sans, diags := types.ListValueFrom(ctx, types.StringType, selected.Subject.SubAltName)
	if diags.HasError() {
		return CertificateImportResourceModel{}, fmt.Errorf(
			"failed to build certificate SAN list: %s",
			diags.Errors()[0].Detail(),
		)
	}

	data.ID = types.StringValue(selected.ID)
	data.Description = types.StringValue(selected.Desc)
	data.IsDefault = types.BoolValue(selected.IsDefault)
	data.IsBroken = types.BoolValue(selected.IsBroken)
	data.CommonName = types.StringValue(selected.Subject.CommonName)
	data.SubjectAlternativeNames = sans
	data.ValidTill = types.StringValue(selected.ValidTill)

	return data, nil
}

func (r *CertificateImportResource) findByID(
	ctx context.Context,
	id string,
) (*synologyCertificate, error) {
	certificates, err := listCertificates(ctx, r.client)
	if err != nil {
		return nil, err
	}

	certificate := findCertificateByID(certificates, id)
	if certificate == nil {
		return nil, fmt.Errorf("DSM certificate %q not found", id)
	}

	return certificate, nil
}

func (r *CertificateImportResource) findByDescription(
	ctx context.Context,
	description string,
) (*synologyCertificate, error) {
	certificates, err := listCertificates(ctx, r.client)
	if err != nil {
		return nil, err
	}

	matches := make([]synologyCertificate, 0)
	for _, certificate := range certificates {
		if certificate.Desc == description {
			matches = append(matches, certificate)
		}
	}

	if len(matches) == 0 {
		return nil, nil
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple DSM certificates already use description %q", description)
	}

	selected := matches[0]
	return &selected, nil
}

func validateCertificatePEM(content string, label string) error {
	remaining := []byte(content)
	found := false

	for len(remaining) > 0 {
		block, rest := pem.Decode(remaining)
		if block == nil {
			break
		}
		remaining = rest
		if block.Type != "CERTIFICATE" {
			continue
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return fmt.Errorf("invalid %s PEM: %w", label, err)
		}
		found = true
	}

	if !found {
		return fmt.Errorf("invalid %s PEM: no certificate block found", label)
	}

	return nil
}

func validatePrivateKeyPEM(content string) error {
	remaining := []byte(content)
	for len(remaining) > 0 {
		block, rest := pem.Decode(remaining)
		if block == nil {
			break
		}
		remaining = rest
		if !strings.Contains(block.Type, "PRIVATE KEY") {
			continue
		}

		if _, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
			return nil
		}
		if _, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
			return nil
		}
		if _, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
			return nil
		}

		return fmt.Errorf("invalid private key PEM: unsupported private key format")
	}

	return fmt.Errorf("invalid private key PEM: no private key block found")
}

func fileNameFromPath(path string) string {
	parts := strings.Split(strings.TrimSpace(path), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
