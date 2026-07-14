package virtualization_test

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	r "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/synology-community/terraform-provider-synology/synology/acctest"
	"github.com/synology-community/terraform-provider-synology/synology/provider/virtualization"
)

type GuestResource struct{}

// TestGuestResourceValidateConfig_ModuleVariables tests the validation logic
// when storage_name is provided via module variables (unknown during validation).
func TestGuestResourceValidateConfig_ModuleVariables(t *testing.T) {
	testCases := []struct {
		name        string
		storageID   types.String
		storageName types.String
		expectError bool
		description string
	}{
		{
			name:        "both_null_should_error",
			storageID:   types.StringNull(),
			storageName: types.StringNull(),
			expectError: true,
			description: "Both storage_id and storage_name are null",
		},
		{
			name:        "storage_name_unknown_should_not_error",
			storageID:   types.StringNull(),
			storageName: types.StringUnknown(), // This simulates a module variable
			expectError: false,
			description: "storage_name is unknown (from module variable) - should pass validation",
		},
		{
			name:        "storage_id_unknown_should_not_error",
			storageID:   types.StringUnknown(), // This simulates a module variable
			storageName: types.StringNull(),
			expectError: false,
			description: "storage_id is unknown (from module variable) - should pass validation",
		},
		{
			name:        "both_unknown_should_not_error",
			storageID:   types.StringUnknown(),
			storageName: types.StringUnknown(),
			expectError: false,
			description: "Both are unknown (from module variables) - should pass validation",
		},
		{
			name:        "storage_name_set_should_not_error",
			storageID:   types.StringNull(),
			storageName: types.StringValue("default"),
			expectError: false,
			description: "storage_name has a value - should pass validation",
		},
	}

	stringVal := func(v types.String) tftypes.Value {
		switch {
		case v.IsUnknown():
			return tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
		case v.IsNull():
			return tftypes.NewValue(tftypes.String, nil)
		default:
			return tftypes.NewValue(tftypes.String, v.ValueString())
		}
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a GuestResource instance
			guestResource := &virtualization.GuestResource{}

			res := resource.SchemaResponse{}

			guestResource.Schema(context.Background(), resource.SchemaRequest{}, &res)
			if res.Diagnostics.HasError() {
				t.Fatalf("Failed to get schema: %v", res.Diagnostics)
			}

			// Build the raw config value: all attributes null except the test inputs
			schemaType := res.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
			vals := map[string]tftypes.Value{}
			for name, at := range schemaType.AttributeTypes {
				vals[name] = tftypes.NewValue(at, nil)
			}
			vals["name"] = tftypes.NewValue(tftypes.String, "test-vm")
			vals["storage_id"] = stringVal(tc.storageID)
			vals["storage_name"] = stringVal(tc.storageName)
			vals["vcpu_num"] = tftypes.NewValue(tftypes.Number, big.NewFloat(4))
			vals["vram_size"] = tftypes.NewValue(tftypes.Number, big.NewFloat(4096))

			config := tfsdk.Config{
				Schema: res.Schema,
				Raw:    tftypes.NewValue(schemaType, vals),
			}

			// Test ValidateConfig
			req := resource.ValidateConfigRequest{
				Config: config,
			}
			resp := &resource.ValidateConfigResponse{}

			guestResource.ValidateConfig(context.Background(), req, resp)

			// Check if the result matches expectations
			hasError := resp.Diagnostics.HasError()
			if tc.expectError && !hasError {
				t.Errorf("Expected validation error for %s, but got none", tc.description)
			} else if !tc.expectError && hasError {
				t.Errorf(
					"Expected no validation error for %s, but got: %v",
					tc.description,
					resp.Diagnostics.Errors(),
				)
			}
		})
	}
}

func TestAccGuestResource_basic(t *testing.T) {
	testCases := []struct {
		Name          string
		ResourceBlock string
	}{
		{
			"guest name is set",
			`
			resource "synology_virtualization_guest" "foo" {
				name         = "testvm"
				storage_name = "default"

				vcpu_num  = 4
				vram_size = 4096

				network {
					name = "default"
				}

				disk {
					image_id = "65caaef4-6622-4643-9feb-1c3b5a915eb8"
				}

				iso {
					image_id = "65caaef4-6622-4643-9feb-1c3b5a915eb8"
				}
			}`,
		},
	}
	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			r.Test(t, r.TestCase{
				ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories(t),
				Steps: []r.TestStep{
					{
						Config: tt.ResourceBlock,
						Check: r.ComposeTestCheckFunc(
							r.TestCheckResourceAttrWith(
								"synology_virtualization_guest.foo",
								"name",
								func(attr string) error {
									if attr != "testvm" {
										return fmt.Errorf(
											"expected guest name to be 'testvm', got %s",
											attr,
										)
									}
									return nil
								},
							),
						),
					},
				},
			})
		})
	}
}
