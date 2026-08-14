# terraform-provider-pingone-aic

An **experimental** Terraform provider for
[PingOne Advanced Identity Cloud](https://docs.pingidentity.com/pingoneaic/). It
manages AM scripts, authentication journeys, the journey nodes those trees
actually use, OAuth2 clients, ESVs, custom managed-object types, IDM
endpoints and schedules, individual IDM access / authentication rules, and
internal roles.

This is not a JSON-passthrough wrapper around the AIC REST API. Every attribute
is typed against a catalog we verified on a live tenant. When AIC adds, removes,
or reshapes a field, generate and plan fail on purpose — that is how the
provider stays honest, and how we expect fixes to arrive (issues / PRs).

## Resource prefix

The provider prepends a prefix to every **name** it creates (script names,
journey names, OAuth2 client ids, inner-tree references). ESV ids cannot start
with `Terraform_` (AIC requires `esv-[a-z0-9_-]+`), so the prefix is lowercased
and inserted after `esv-`: `esv-test11` becomes `esv-terraform-test11`.

```hcl
provider "pingoneaic" {
  tenant_url      = "https://openam-example.forgeblocks.com"
  resource_prefix = "Terraform_" # default
}
```

Terraform config always uses the _logical_ name (`GetIP`). AIC sees
`Terraform_GetIP`. Applying generated config therefore creates **copies**
instead of overwriting the journeys you pulled. Set `resource_prefix = ""` to
write names as given (for example after importing an existing tree).

Node UUIDs are generated on create. Connections in HCL use Terraform references,
not hardcoded UUIDs.

## Resources

| Resource                            | AIC object                                         |
| ----------------------------------- | -------------------------------------------------- |
| `pingoneaic_script`                 | AM `/scripts/{id}`                                 |
| `pingoneaic_scripted_decision_node` | `ScriptedDecisionNode`                             |
| `pingoneaic_journey`                | authentication tree                                |
| `pingoneaic_oauth2_client`          | AM `/realm-config/agents/OAuth2Client/{id}`        |
| `pingoneaic_esv_variable`           | `/environment/variables/{id}`                      |
| `pingoneaic_esv_secret`             | `/environment/secrets/{id}`                        |
| `pingoneaic_managed_object`         | one type in `/openidm/config/managed`              |
| `pingoneaic_idm_endpoint`             | `/openidm/config/endpoint/{name}`                          |
| `pingoneaic_idm_schedule`             | `/openidm/config/schedule/{name}`                          |
| `pingoneaic_access_rule`              | one `configs[]` grant in `/openidm/config/access`          |
| `pingoneaic_authentication_mapping`   | one `rsFilter.staticUserMapping[]` entry                   |
| `pingoneaic_internal_role`            | `/openidm/internal/role/{id}`                              |
| `pingoneaic_<type>_node`              | every other node type used by the generate catalog         |

`success` and `failure` are valid connection targets (AM's built-in static
nodes). `x` / `y` are optional visual chrome and are omitted from generated HCL.

`source` is the script body. Point it at a `.js` file with
`source = file("${path.module}/scripts/foo.js")` (generate does this) or inline
a heredoc if you prefer. `file` on an endpoint, schedule, or hook is an IDM
**product** path (`roles/onDelete-roles.js`), not a local Terraform path.

`pingoneaic_journey` also carries the tree-level session settings
`maximum_idle_time`, `maximum_session_time` and `tree_timeout` (optional; AM
supplies them when unset), and each `node` block takes an optional `version`
(defaults to `1.0`).

Changing `resource_prefix` does **not** rename existing journeys or OAuth2
clients — AM keys both by name and cannot rename one, so the object stays at
the name recorded in state. Scripts are keyed by UUID and do get renamed in
place.

`pingoneaic_access_rule` and `pingoneaic_authentication_mapping` have no
name. Terraform finds the live entry by hashing the rule (canonical JSON
SHA-256, same form as `aic access list`). Only that one row is written;
everything else in the document stays as it was. There is nothing to prefix,
so generate emits `access_rules.tf.review` / `authentication_mappings.tf.review`
rather than applyable copies — applying generated HCL would append duplicate
grants. Import an existing rule by its hash; create is refused if that hash
is already in the document. `roles` is a comma-separated string on an access
rule and an array on an authentication mapping.

`pingoneaic_internal_role` `name` is the `_id` you choose (prefixed on the
wire). That is the string access rules must use (`internal/role/Terraform_desk`),
not `display_name`. PUT replaces the whole role; updates send `If-Match`.
Console-created roles have a UUID `_id` — generate emits them under their
display name so apply creates a copy with a chosen id.

## Authentication

Either a pre-minted bearer or a service-account JWT:

| Provider argument    | Environment variable            |
| -------------------- | ------------------------------- |
| `tenant_url`         | `PINGONEAIC_TENANT_URL`         |
| `access_token`       | `PINGONEAIC_ACCESS_TOKEN`       |
| `service_account_id` | `PINGONEAIC_SERVICE_ACCOUNT_ID` |
| `jwk`                | `PINGONEAIC_JWK`                |
| `resource_prefix`    | `PINGONEAIC_RESOURCE_PREFIX`    |

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
- `generated/oauth2_clients.tf` — one resource per OAuth2 client, defaults omitted
- `generated/esv_variables.tf` / `generated/esv_secrets.tf`
- `generated/managed_objects.tf` — custom types only, plus `hooks/*.js` for inline lifecycle scripts
- `generated/idm_endpoints.tf` + `generated/endpoints/*.js`
- `generated/idm_schedules.tf` + `generated/schedules/*.js`
- `generated/access_rules.tf.review` / `authentication_mappings.tf.review` —
  existing rules, identified by hash; **not** loaded by Terraform
- `generated/internal_roles.tf` — one resource per internal role; UUID `_id`s
  are emitted under the display name so apply creates a chosen-id copy
- `generated/journey_<name>.tf` — nodes + tree, defaults omitted, UUIDs replaced
  with Terraform references

**`-out` is a regenerated directory, not just a write target.** Each run deletes
the previous run's `provider.tf`, `scripts.tf`, `oauth2_clients.tf`,
`esv_*.tf`, `managed_objects.tf`, `idm_endpoints.tf`, `idm_schedules.tf`,
`access_rules.tf.review`, `authentication_mappings.tf.review`,
`internal_roles.tf`,
`journey_*.tf`, `scripts/*.js`, `endpoints/*.js`, `schedules/*.js` and
`hooks/*.js`, so an object removed in the tenant does not linger as a stale
file. To make that safe it drops a `.pingoneaic-generated` marker and **refuses
to touch a directory that has no marker but does hold matching files** — so
pointing `-out` at hand-written config is an error, not a silent delete. Keep
your own files somewhere else.

`-journeys` fails if any name is not present in the realm, rather than silently
generating a subset.

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

## What this experiment is _not_

- A complete AIC provider. SAML, secret mappings, the realm-wide OIDC
  provider, and Ping-shipped managed objects (`alpha_user`, …) themselves are
  out of scope until that path is proven. Lifecycle hooks are `hook` blocks on
  a custom `pingoneaic_managed_object` — apply never writes them onto
  `alpha_user`. ESV writes never restart the tenant. Schedule copies default
  to `enabled = false`.
- A dump of AIC's JSON. If a field is missing, that is a catalog gap, not an
  invitation to `jsonencode()`.

## License

Apache-2.0. See [LICENSE](LICENSE).
