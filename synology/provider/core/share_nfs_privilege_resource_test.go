package core

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/synology-community/go-synology/pkg/api"
)

func TestShareNFSPrivilegeModelRoundTrip(t *testing.T) {
	t.Parallel()

	apiModel := shareNFSPrivilegeAPIResponse{
		ShareName: "media",
		Rules: []shareNFSPrivilegeAPIRule{
			{
				Client:     "10.1.0.0/24",
				Privilege:  "rw",
				RootSquash: "root",
				Async:      true,
				Crossmnt:   true,
				Insecure:   true,
				SecurityFlavor: shareNFSPrivilegeAPISecurityType{
					Sys: true,
				},
			},
			{
				Client:     "10.1.0.42",
				Privilege:  "ro",
				RootSquash: "admin",
				Async:      false,
				Crossmnt:   false,
				Insecure:   false,
				SecurityFlavor: shareNFSPrivilegeAPISecurityType{
					KerberosIntegrity: true,
				},
			},
		},
	}

	model, diags := shareNFSPrivilegeModelFromAPI(apiModel)
	if diags.HasError() {
		t.Fatalf("shareNFSPrivilegeModelFromAPI returned diagnostics: %v", diags)
	}

	roundTripped, diags := model.toAPI(context.Background())
	if diags.HasError() {
		t.Fatalf("toAPI returned diagnostics: %v", diags)
	}

	if roundTripped.ShareName != apiModel.ShareName {
		t.Fatalf("share name mismatch: got %q, want %q", roundTripped.ShareName, apiModel.ShareName)
	}
	if len(roundTripped.Rules) != len(apiModel.Rules) {
		t.Fatalf(
			"rule count mismatch: got %d, want %d",
			len(roundTripped.Rules),
			len(apiModel.Rules),
		)
	}
	if roundTripped.Rules[0].Client != apiModel.Rules[0].Client {
		t.Fatalf(
			"client mismatch: got %q, want %q",
			roundTripped.Rules[0].Client,
			apiModel.Rules[0].Client,
		)
	}
	if !roundTripped.Rules[0].SecurityFlavor.Sys {
		t.Fatal("expected sys security flavor to remain enabled")
	}
	if !roundTripped.Rules[1].SecurityFlavor.KerberosIntegrity {
		t.Fatal("expected kerberos_integrity to remain enabled")
	}
}

func TestShareNFSPrivilegeRuleDefaultsSecurityFlavorToSys(t *testing.T) {
	t.Parallel()

	ruleObject, diags := shareNFSPrivilegeRuleModel{
		Client:         types.StringValue("10.1.0.0/24"),
		Privilege:      types.StringValue("rw"),
		RootSquash:     types.StringValue("root"),
		Async:          types.BoolValue(false),
		Crossmnt:       types.BoolValue(false),
		Insecure:       types.BoolValue(false),
		SecurityFlavor: types.ObjectNull(shareNFSPrivilegeSecurityFlavorModel{}.AttrType()),
	}.ObjectValue()
	if diags.HasError() {
		t.Fatalf("ObjectValue returned diagnostics: %v", diags)
	}

	model := ShareNFSPrivilegeResourceModel{
		ShareName: types.StringValue("media"),
		Rules: types.ListValueMust(
			types.ObjectType{AttrTypes: shareNFSPrivilegeRuleModel{}.AttrType()},
			[]attr.Value{ruleObject},
		),
	}

	apiModel, diags := model.toAPI(context.Background())
	if diags.HasError() {
		t.Fatalf("toAPI returned diagnostics: %v", diags)
	}

	if !apiModel.Rules[0].SecurityFlavor.Sys {
		t.Fatal("expected sys security flavor to default to true")
	}
}

func TestShareNFSPrivilegeRuleRejectsEmptySecurityFlavor(t *testing.T) {
	t.Parallel()

	securityFlavor, diags := shareNFSPrivilegeSecurityFlavorModel{
		Sys:               types.BoolValue(false),
		Kerberos:          types.BoolValue(false),
		KerberosIntegrity: types.BoolValue(false),
		KerberosPrivacy:   types.BoolValue(false),
	}.ObjectValue()
	if diags.HasError() {
		t.Fatalf("security flavor ObjectValue returned diagnostics: %v", diags)
	}

	ruleObject, diags := shareNFSPrivilegeRuleModel{
		Client:         types.StringValue("10.1.0.0/24"),
		Privilege:      types.StringValue("rw"),
		RootSquash:     types.StringValue("root"),
		Async:          types.BoolValue(false),
		Crossmnt:       types.BoolValue(false),
		Insecure:       types.BoolValue(false),
		SecurityFlavor: securityFlavor,
	}.ObjectValue()
	if diags.HasError() {
		t.Fatalf("rule ObjectValue returned diagnostics: %v", diags)
	}

	model := ShareNFSPrivilegeResourceModel{
		ShareName: types.StringValue("media"),
		Rules: types.ListValueMust(
			types.ObjectType{AttrTypes: shareNFSPrivilegeRuleModel{}.AttrType()},
			[]attr.Value{ruleObject},
		),
	}

	_, diags = model.toAPI(context.Background())
	if !diags.HasError() {
		t.Fatal("expected diagnostics when all security flavors are disabled")
	}
}

func TestIsMissingShareAPIError(t *testing.T) {
	t.Parallel()

	if !IsMissingShareAPIError(api.ApiError{Code: 5801}) {
		t.Fatal("expected share-not-found API code to be treated as missing share")
	}
	if IsMissingShareAPIError(api.ApiError{Code: 402}) {
		t.Fatal("did not expect generic busy/permission code to be treated as missing share")
	}
	if IsMissingShareAPIError(errors.New("boom")) {
		t.Fatal("did not expect arbitrary errors to be treated as missing share")
	}
}
