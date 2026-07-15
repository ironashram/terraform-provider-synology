package core

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/synology-community/go-synology"
	"github.com/synology-community/go-synology/pkg/api"
)

const (
	certificateCRTAPIName     = "SYNO.Core.Certificate.CRT"
	certificateServiceAPIName = "SYNO.Core.Certificate.Service"
)

type certificateListResponse struct {
	Certificates []synologyCertificate `json:"certificates"`
}

type synologyCertificate struct {
	ID        string              `json:"id"`
	Desc      string              `json:"desc"`
	IsDefault bool                `json:"is_default"`
	IsBroken  bool                `json:"is_broken"`
	ValidTill string              `json:"valid_till"`
	Subject   synologyCertSubject `json:"subject"`
	Services  []json.RawMessage   `json:"services"`
}

type synologyCertSubject struct {
	CommonName string   `json:"common_name"`
	SubAltName []string `json:"sub_alt_name"`
}

type synologyCertificateServiceRef struct {
	Subscriber  string `json:"subscriber"`
	DisplayName string `json:"display_name"`
}

type certificateServiceSetRequest struct {
	Settings string `url:"settings"`
}

type certificateServiceSetSetting struct {
	Service json.RawMessage `json:"service"`
	OldID   string          `json:"old_id"`
	ID      string          `json:"id"`
}

var (
	certificateListMethod = api.Method{
		API:            certificateCRTAPIName,
		Method:         "list",
		Version:        1,
		ErrorSummaries: api.GlobalErrors,
	}
	certificateServiceSetMethod = api.Method{
		API:            certificateServiceAPIName,
		Method:         "set",
		Version:        1,
		ErrorSummaries: api.GlobalErrors,
	}
)

func listCertificates(
	ctx context.Context,
	client synology.Api,
) ([]synologyCertificate, error) {
	req := struct{}{}
	resp, err := api.Get[certificateListResponse](client, ctx, &req, certificateListMethod)
	if err != nil {
		return nil, err
	}
	return resp.Certificates, nil
}

func findCertificateByID(certificates []synologyCertificate, id string) *synologyCertificate {
	for _, certificate := range certificates {
		if certificate.ID == id {
			found := certificate
			return &found
		}
	}
	return nil
}

func findDefaultCertificate(certificates []synologyCertificate) *synologyCertificate {
	for _, certificate := range certificates {
		if certificate.IsDefault && !certificate.IsBroken {
			found := certificate
			return &found
		}
	}
	return nil
}

func selectCertificateForDomain(
	certificates []synologyCertificate,
	domain string,
) *synologyCertificate {
	candidates := make([]synologyCertificate, 0)
	for _, certificate := range certificates {
		if certificate.IsBroken {
			continue
		}
		if certificateMatchesDomain(certificate, domain) {
			candidates = append(candidates, certificate)
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	slices.SortFunc(candidates, func(a, b synologyCertificate) int {
		if a.IsDefault != b.IsDefault {
			if a.IsDefault {
				return 1
			}
			return -1
		}

		aTime := parseDSMCertificateTime(a.ValidTill)
		bTime := parseDSMCertificateTime(b.ValidTill)
		switch {
		case aTime.Before(bTime):
			return -1
		case aTime.After(bTime):
			return 1
		default:
			return 0
		}
	})

	selected := candidates[len(candidates)-1]
	return &selected
}

func certificateMatchesDomain(certificate synologyCertificate, domain string) bool {
	if domain == "" {
		return false
	}

	if certificate.Subject.CommonName != "" &&
		hostnameOrWildcardMatches(certificate.Subject.CommonName, domain) {
		return true
	}

	for _, san := range certificate.Subject.SubAltName {
		if hostnameOrWildcardMatches(san, domain) {
			return true
		}
	}

	return false
}

func hostnameOrWildcardMatches(pattern string, domain string) bool {
	if pattern == domain {
		return true
	}

	if strings.HasPrefix(pattern, "*.") {
		suffix := strings.TrimPrefix(pattern, "*.")
		if suffix == "" || !strings.HasSuffix(domain, "."+suffix) {
			return false
		}

		trimmed := strings.TrimSuffix(domain, "."+suffix)
		return trimmed != "" && !strings.Contains(trimmed, ".")
	}

	return false
}

func parseDSMCertificateTime(value string) time.Time {
	if value == "" {
		return time.Unix(0, 0).UTC()
	}

	parsed, err := time.Parse("Jan _2 15:04:05 2006 MST", value)
	if err != nil {
		return time.Unix(0, 0).UTC()
	}
	return parsed
}

func findCertificateServiceBinding(
	certificates []synologyCertificate,
	subscriber string,
	displayName string,
) (*synologyCertificate, json.RawMessage, error) {
	for _, certificate := range certificates {
		for _, rawService := range certificate.Services {
			var ref synologyCertificateServiceRef
			if err := json.Unmarshal(rawService, &ref); err != nil {
				return nil, nil, err
			}
			if ref.Subscriber == subscriber && ref.DisplayName == displayName {
				found := certificate
				return &found, rawService, nil
			}
		}
	}

	return nil, nil, nil
}

func setCertificateServiceBinding(
	ctx context.Context,
	client synology.Api,
	service json.RawMessage,
	oldID string,
	newID string,
) error {
	settingsJSON, err := json.Marshal([]certificateServiceSetSetting{
		{
			Service: service,
			OldID:   oldID,
			ID:      newID,
		},
	})
	if err != nil {
		return err
	}

	_, err = api.Get[struct{}](client, ctx, &certificateServiceSetRequest{
		Settings: string(settingsJSON),
	}, certificateServiceSetMethod)
	return err
}

func parseCertificateBindingImportID(id string) (string, string, error) {
	if subscriber, displayName, ok := strings.Cut(id, "/"); ok {
		if subscriber == "" || displayName == "" {
			return "", "", fmt.Errorf("invalid import ID %q, expected subscriber/display_name", id)
		}
		return subscriber, displayName, nil
	}

	if subscriber, displayName, ok := strings.Cut(id, ":"); ok {
		if subscriber == "" || displayName == "" {
			return "", "", fmt.Errorf("invalid import ID %q, expected subscriber:display_name", id)
		}
		return subscriber, displayName, nil
	}

	return "", "", fmt.Errorf("invalid import ID %q, expected subscriber/display_name", id)
}

func buildCertificateBindingID(subscriber string, displayName string) string {
	return subscriber + "/" + displayName
}
