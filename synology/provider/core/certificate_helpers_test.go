package core

import "testing"

func TestSelectCertificateForDomainPrefersDefaultThenNewest(t *testing.T) {
	t.Parallel()

	certificates := []synologyCertificate{
		{
			ID:        "old-wildcard",
			IsDefault: false,
			IsBroken:  false,
			ValidTill: "Jan  1 00:00:00 2026 GMT",
			Subject: synologyCertSubject{
				SubAltName: []string{"*.synology.example.com"},
			},
		},
		{
			ID:        "default-wildcard",
			IsDefault: true,
			IsBroken:  false,
			ValidTill: "Jan  1 00:00:00 2025 GMT",
			Subject: synologyCertSubject{
				SubAltName: []string{"*.synology.example.com"},
			},
		},
	}

	selected := selectCertificateForDomain(certificates, "vault.synology.example.com")
	if selected == nil {
		t.Fatal("expected a certificate to be selected")
		return
	}

	if selected.ID != "default-wildcard" {
		t.Fatalf("selected wrong certificate: got %q, want %q", selected.ID, "default-wildcard")
	}
}

func TestHostnameOrWildcardMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pattern string
		domain  string
		match   bool
	}{
		{pattern: "synology.example.com", domain: "synology.example.com", match: true},
		{pattern: "*.synology.example.com", domain: "vault.synology.example.com", match: true},
		{
			pattern: "*.synology.example.com",
			domain:  "deep.vault.synology.example.com",
			match:   false,
		},
		{pattern: "*.synology.example.com", domain: "synology.example.com", match: false},
	}

	for _, tt := range tests {
		if got := hostnameOrWildcardMatches(tt.pattern, tt.domain); got != tt.match {
			t.Fatalf(
				"hostnameOrWildcardMatches(%q, %q) = %t, want %t",
				tt.pattern,
				tt.domain,
				got,
				tt.match,
			)
		}
	}
}

func TestParseCertificateBindingImportID(t *testing.T) {
	t.Parallel()

	subscriber, displayName, err := parseCertificateBindingImportID(
		"ReverseProxy/vault.synology.example.com",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if subscriber != "ReverseProxy" || displayName != "vault.synology.example.com" {
		t.Fatalf("unexpected parsed import id: %q %q", subscriber, displayName)
	}

	subscriber, displayName, err = parseCertificateBindingImportID(
		"ReverseProxy:minio.synology.example.com",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if subscriber != "ReverseProxy" || displayName != "minio.synology.example.com" {
		t.Fatalf("unexpected parsed import id: %q %q", subscriber, displayName)
	}
}
