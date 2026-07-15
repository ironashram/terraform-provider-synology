package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchDiscoveryDocument(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"authorization_endpoint": "https://issuer.example.com/auth",
			"token_endpoint": "https://issuer.example.com/token"
		}`))
	}))
	defer server.Close()

	resource := &DirectoryOIDCSSOResource{
		httpClient: server.Client(),
	}

	discovery, err := resource.fetchDiscoveryDocument(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("expected discovery document to resolve, got error: %v", err)
	}

	if discovery.AuthorizationEndpoint != "https://issuer.example.com/auth" {
		t.Fatalf("unexpected authorization endpoint: %s", discovery.AuthorizationEndpoint)
	}
	if discovery.TokenEndpoint != "https://issuer.example.com/token" {
		t.Fatalf("unexpected token endpoint: %s", discovery.TokenEndpoint)
	}
}

func TestFetchDiscoveryDocumentRequiresEndpoints(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	resource := &DirectoryOIDCSSOResource{
		httpClient: server.Client(),
	}

	if _, err := resource.fetchDiscoveryDocument(context.Background(), server.URL); err == nil {
		t.Fatal("expected discovery document without endpoints to fail")
	}
}
