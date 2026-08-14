package provider

import (
	"context"
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/nodetype"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestProviderMetadata(t *testing.T) {
	p := New("test-version")()
	var resp provider.MetadataResponse
	p.Metadata(context.Background(), provider.MetadataRequest{}, &resp)
	if resp.TypeName != "pingoneaic" || resp.Version != "test-version" {
		t.Fatalf("unexpected metadata: %#v", resp)
	}
}

func TestProviderRegistersEveryCatalogResourceOnce(t *testing.T) {
	p := New("test")()
	factories := p.Resources(context.Background())
	if want := len(nodetype.All()) + 2; len(factories) != want {
		t.Fatalf("registered %d resources, want %d", len(factories), want)
	}
	seen := map[string]bool{}
	for _, factory := range factories {
		instance := factory()
		var resp resource.MetadataResponse
		instance.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "pingoneaic"}, &resp)
		if seen[resp.TypeName] {
			t.Fatalf("duplicate resource type %q", resp.TypeName)
		}
		seen[resp.TypeName] = true
	}
}

func TestFirst(t *testing.T) {
	if got := first("", "environment", "fallback"); got != "environment" {
		t.Fatalf("got %q", got)
	}
}
