package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

var providerFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"synology": providerserver.NewProtocol6WithError(New()()),
}

var testAccProtoV6ProviderFactories = providerFactories

func preCheck(t *testing.T) {
	variables := []string{
		"SYNOLOGY_HOST",
		"SYNOLOGY_USER",
		"SYNOLOGY_PASSWORD",
	}

	for _, variable := range variables {
		value := os.Getenv(variable)
		if value == "" {
			t.Fatalf("`%s` must be set for acceptance tests!", variable)
		}
	}
}
