package filestation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	client "github.com/synology-community/go-synology"
	gofs "github.com/synology-community/go-synology/pkg/api/filestation"
	"github.com/synology-community/go-synology/pkg/models"
)

var _ datasource.DataSource = &PathDataSource{}

func NewPathDataSource() datasource.DataSource {
	return &PathDataSource{}
}

type PathDataSource struct {
	client gofs.Api
}

type PathDataSourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Path               types.String `tfsdk:"path"`
	WaitForExists      types.Bool   `tfsdk:"wait_for_exists"`
	WaitTimeoutSeconds types.Int64  `tfsdk:"wait_timeout_seconds"`
	Exists             types.Bool   `tfsdk:"exists"`
	Name               types.String `tfsdk:"name"`
	IsDir              types.Bool   `tfsdk:"is_dir"`
	RealPath           types.String `tfsdk:"real_path"`
}

func (d *PathDataSource) Metadata(
	_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = buildName(req.ProviderTypeName, "path")
}

func (d *PathDataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Looks up a File Station path on the NAS.

When ` + "`wait_for_exists`" + ` is true, the data source waits until the path appears or the timeout
expires. When it is false and the path does not exist, the data source returns ` + "`exists = false`" + `
instead of failing.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Stable identifier for the lookup, equal to `path`.",
				Computed:            true,
			},
			"path": schema.StringAttribute{
				MarkdownDescription: "NAS path rooted at a shared folder, such as `/docker/certbot/current/cert.pem`.",
				Required:            true,
			},
			"wait_for_exists": schema.BoolAttribute{
				MarkdownDescription: "Wait for the path to exist before returning. Default: `false`.",
				Optional:            true,
			},
			"wait_timeout_seconds": schema.Int64Attribute{
				MarkdownDescription: "Maximum time to wait when `wait_for_exists` is true. Default: `300`.",
				Optional:            true,
			},
			"exists": schema.BoolAttribute{
				MarkdownDescription: "Whether the path currently exists.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Basename reported by File Station.",
				Computed:            true,
			},
			"is_dir": schema.BoolAttribute{
				MarkdownDescription: "Whether the path is a directory.",
				Computed:            true,
			},
			"real_path": schema.StringAttribute{
				MarkdownDescription: "Resolved real path reported by File Station.",
				Computed:            true,
			},
		},
	}
}

func (d *PathDataSource) Configure(
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

	d.client = configured.FileStationAPI()
}

func (d *PathDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	var data PathDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	waitForExists := !data.WaitForExists.IsNull() && !data.WaitForExists.IsUnknown() &&
		data.WaitForExists.ValueBool()
	waitTimeout := fileStationPathWaitTimeout(waitForExists, data.WaitTimeoutSeconds)

	file, exists, err := d.readPath(ctx, data.Path.ValueString(), waitForExists, waitTimeout)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read File Station path", err.Error())
		return
	}

	data.ID = types.StringValue(data.Path.ValueString())
	data.Exists = types.BoolValue(exists)

	if exists {
		data.Name = types.StringValue(file.Name)
		data.IsDir = types.BoolValue(file.IsDir)
		if file.Additional.RealPath != "" {
			data.RealPath = types.StringValue(file.Additional.RealPath)
		} else {
			data.RealPath = types.StringNull()
		}
	} else {
		data.Name = types.StringNull()
		data.IsDir = types.BoolNull()
		data.RealPath = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *PathDataSource) readPath(
	ctx context.Context,
	path string,
	waitForExists bool,
	waitTimeout time.Duration,
) (*models.File, bool, error) {
	lookup := func() (*models.File, bool, error) {
		file, err := d.client.Get(ctx, path)
		if err == nil {
			return file, true, nil
		}
		if isFileStationNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	file, exists, err := lookup()
	if err != nil || exists || !waitForExists {
		return file, exists, err
	}

	deadline := time.Now().Add(waitTimeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-ticker.C:
			file, exists, err = lookup()
			if err != nil {
				return nil, false, err
			}
			if exists {
				return file, true, nil
			}
			if time.Now().After(deadline) {
				return nil, false, fmt.Errorf(
					"timed out waiting for File Station path %q to exist after %s",
					path,
					waitTimeout,
				)
			}
		}
	}
}

func fileStationPathWaitTimeout(waitForExists bool, configured types.Int64) time.Duration {
	if !waitForExists {
		return 0
	}

	if configured.IsNull() || configured.IsUnknown() || configured.ValueInt64() <= 0 {
		return 300 * time.Second
	}

	return time.Duration(configured.ValueInt64()) * time.Second
}

func isFileStationNotFound(err error) bool {
	if err == nil {
		return false
	}

	var notFound gofs.FileNotFoundError
	if errors.As(err, &notFound) {
		return true
	}

	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "file not found") || strings.Contains(errMsg, "result is empty")
}
