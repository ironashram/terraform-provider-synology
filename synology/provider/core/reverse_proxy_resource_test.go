package core

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestReverseProxyProtocolRoundTrip(t *testing.T) {
	t.Parallel()

	for _, protocol := range []string{"http", "https"} {
		code, err := encodeReverseProxyProtocol(protocol)
		if err != nil {
			t.Fatalf("encodeReverseProxyProtocol(%q) returned error: %v", protocol, err)
		}

		decoded, err := decodeReverseProxyProtocol(code)
		if err != nil {
			t.Fatalf("decodeReverseProxyProtocol(%d) returned error: %v", code, err)
		}

		if decoded != protocol {
			t.Fatalf("protocol round-trip mismatch: got %q, want %q", decoded, protocol)
		}
	}
}

func TestReverseProxyModelRoundTrip(t *testing.T) {
	t.Parallel()

	apiEntry := reverseProxyAPIEntry{
		UUID:                 "proxy-uuid",
		Description:          "vault",
		ProxyConnectTimeout:  60,
		ProxyReadTimeout:     60,
		ProxySendTimeout:     60,
		ProxyHTTPVersion:     1,
		ProxyInterceptErrors: false,
		Frontend: reverseProxyAPIFrontend{
			FQDN:     "vault.synology.example.com",
			Port:     443,
			Protocol: reverseProxyProtocolHTTPS,
			HTTPS: reverseProxyAPIHTTPS{
				HSTS: true,
			},
		},
		Backend: reverseProxyAPIBackend{
			FQDN:     "127.0.0.1",
			Port:     8200,
			Protocol: reverseProxyProtocolHTTP,
		},
		CustomizeHeaders: []reverseProxyAPIHeader{
			{Name: "X-Forwarded-Proto", Value: "https"},
		},
	}

	model, diags := reverseProxyModelFromAPI(apiEntry)
	if diags.HasError() {
		t.Fatalf("reverseProxyModelFromAPI returned diagnostics: %v", diags)
	}

	roundTripped, diags := model.toAPIEntry(context.Background())
	if diags.HasError() {
		t.Fatalf("toAPIEntry returned diagnostics: %v", diags)
	}

	if roundTripped.Description != apiEntry.Description {
		t.Fatalf(
			"description mismatch: got %q, want %q",
			roundTripped.Description,
			apiEntry.Description,
		)
	}
	if roundTripped.Frontend.FQDN != apiEntry.Frontend.FQDN {
		t.Fatalf(
			"frontend host mismatch: got %q, want %q",
			roundTripped.Frontend.FQDN,
			apiEntry.Frontend.FQDN,
		)
	}
	if roundTripped.Backend.FQDN != apiEntry.Backend.FQDN {
		t.Fatalf(
			"backend host mismatch: got %q, want %q",
			roundTripped.Backend.FQDN,
			apiEntry.Backend.FQDN,
		)
	}
	if roundTripped.Frontend.Protocol != apiEntry.Frontend.Protocol {
		t.Fatalf(
			"frontend protocol mismatch: got %d, want %d",
			roundTripped.Frontend.Protocol,
			apiEntry.Frontend.Protocol,
		)
	}
	if roundTripped.Backend.Protocol != apiEntry.Backend.Protocol {
		t.Fatalf(
			"backend protocol mismatch: got %d, want %d",
			roundTripped.Backend.Protocol,
			apiEntry.Backend.Protocol,
		)
	}
	if len(roundTripped.CustomizeHeaders) != len(apiEntry.CustomizeHeaders) {
		t.Fatalf(
			"custom header count mismatch: got %d, want %d",
			len(roundTripped.CustomizeHeaders),
			len(apiEntry.CustomizeHeaders),
		)
	}
}

func TestReverseProxyCustomHeadersDefaultSet(t *testing.T) {
	t.Parallel()

	value := types.SetValueMust(
		types.ObjectType{AttrTypes: ReverseProxyCustomHeaderModel{}.AttrType()},
		[]attr.Value{},
	)

	if value.IsNull() || value.IsUnknown() {
		t.Fatalf("expected empty set value, got null/unknown")
	}
}
