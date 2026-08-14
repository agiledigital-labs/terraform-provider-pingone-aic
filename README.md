# terraform-provider-pingone-aic

An **experimental** Terraform provider for [PingOne Advanced Identity
Cloud](https://docs.pingidentity.com/pingoneaic/). It manages AM scripts,
authentication journeys, and the journey nodes those trees actually use.

This is not a JSON-passthrough wrapper around the AIC REST API. Every attribute
is typed against a catalog we verified on a live tenant. When AIC adds, removes,
or reshapes a field, generate and plan fail on purpose — that is how the
provider stays honest, and how we expect fixes to arrive (issues / PRs).

## Resource prefix

The provider prepends a prefix to every **name** it creates (script names,
journey names, inner-tree references):

```hcl
provider "pingoneaic" {
  tenant_url      = "https://openam-example.forgeblocks.com"
  resource_prefix = "Terraform_" # default
}
```

Terraform config always uses the *logical* name (`GetIP`). AIC sees
`Terraform_GetIP`. Applying generated config therefore creates **copies**
instead of overwriting the journeys you pulled. Set `resource_prefix = ""` to
write names as given (for example after importing an existing tree).

Node UUIDs are generated on create. Connections in HCL use Terraform references,
not hardcoded UUIDs.

## Resources

| Resource | AIC object |
| --- | --- |
| `pingoneaic_script` | AM `/scripts/{id}` |
| `pingoneaic_scripted_decision_node` | `ScriptedDecisionNode` |
| `pingoneaic_journey` | authentication tree |
| `pingoneaic_<type>_node` | every other node type used by the generate catalog |

`success` and `failure` are valid connection targets (AM's built-in static
nodes). `x` / `y` are optional visual chrome and are omitted from generated
HCL.

## Authentication

Either a pre-minted bearer or a service-account JWT:

| Provider argument | Environment variable |
| --- | --- |
| `tenant_url` | `PINGONEAIC_TENANT_URL` |
| `access_token` | `PINGONEAIC_ACCESS_TOKEN` |
| `service_account_id` | `PINGONEAIC_SERVICE_ACCOUNT_ID` |
| `jwk` | `PINGONEAIC_JWK` |
| `resource_prefix` | `PINGONEAIC_RESOURCE_PREFIX` |

`access_token` is the easy path when an `aic` agent is already unlocked
(`aic whoami --token`). JWT minting is what you want in CI.

## Pull existing config into Terraform

```sh
export PINGONEAIC_TENANT_URL="https://openam-example.forgeblocks.com"
export PINGONEAIC_ACCESS_TOKEN="$(aic --no-prompt whoami --token)"

make generate-cli
./bin/pingoneaic-tf -realm alpha -out generated
```

That writes:

- `generated/provider.tf`
- `generated/scripts.tf` + `generated/scripts/*.js`
- `generated/journey_<name>.tf` — nodes + tree, defaults omitted, UUIDs
  replaced with Terraform references

Unknown node types or unmodelled fields abort generate. Add them to
`internal/nodetype/catalog.go` (and a test) rather than punching a JSON hole.

## Build and install locally

```sh
make test
make install
```

`make install` drops the binary into
`~/.terraform.d/plugins/registry.terraform.io/agiledigital-labs/pingone-aic/0.1.0/…`.

```hcl
terraform {
  required_providers {
    pingoneaic = {
      source  = "agiledigital-labs/pingone-aic"
      version = "0.1.0"
    }
  }
}
```

## What this experiment is *not*

- A complete AIC provider. ESVs, OAuth2 clients, managed objects, IDM
  endpoints, and secret mappings are out of scope until the journey/script
  path is proven.
- A dump of AIC's JSON. If a field is missing, that is a catalog gap, not an
  invitation to `jsonencode()`.

## License

Apache-2.0. See [LICENSE](LICENSE).
