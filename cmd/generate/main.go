// pingoneaic-tf generate — pull live AIC journeys and OAuth2 clients into reviewable Terraform.
//
// Credentials come from the same env vars as the provider:
//
//	PINGONEAIC_TENANT_URL
//	PINGONEAIC_ACCESS_TOKEN          (or service-account JWT pair)
//	PINGONEAIC_SERVICE_ACCOUNT_ID
//	PINGONEAIC_JWK
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/generate"
)

func main() {
	var (
		realm    = flag.String("realm", "alpha", "realm to pull")
		out      = flag.String("out", "generated", "output directory")
		prefix   = flag.String("prefix", "", "strip this prefix from existing names (optional)")
		journeys = flag.String("journeys", "", "comma-separated journey names (default: all)")
	)
	flag.Parse()

	tenant := os.Getenv("PINGONEAIC_TENANT_URL")
	if tenant == "" {
		fmt.Fprintln(os.Stderr, "PINGONEAIC_TENANT_URL is required")
		os.Exit(2)
	}
	c, err := client.New(client.Config{
		TenantURL:        tenant,
		ServiceAccountID: os.Getenv("PINGONEAIC_SERVICE_ACCOUNT_ID"),
		JWK:              os.Getenv("PINGONEAIC_JWK"),
		AccessToken:      os.Getenv("PINGONEAIC_ACCESS_TOKEN"),
		ResourcePrefix:   *prefix,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	var list []string
	if *journeys != "" {
		for _, n := range strings.Split(*journeys, ",") {
			n = strings.TrimSpace(n)
			if n != "" {
				list = append(list, n)
			}
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	res, err := generate.Run(ctx, c, generate.Options{
		Realm:    *realm,
		OutDir:   *out,
		Prefix:   *prefix,
		Journeys: list,
		Progress: os.Stderr,
	})
	// Called rather than deferred: the error path below exits the process, and
	// os.Exit does not run deferred functions.
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %d journeys, %d scripts, %d nodes, %d oauth2 clients, %d variables, %d secrets, %d managed objects, %d endpoints, %d schedules → %s\n",
		res.Journeys, res.Scripts, res.Nodes, res.OAuth2Clients, res.Variables, res.Secrets, res.ManagedObjects, res.Endpoints, res.Schedules, *out)
}
