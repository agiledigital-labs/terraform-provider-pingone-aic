# Project guide — terraform-provider-pingone-aic

Canonical instructions for all AI agents working in this repo. `CLAUDE.md` and
`AGENTS.md` are pointers to this file.

> **Two layers.** This file is committed and true for everyone. Anything true
> only of _one machine_ — where a sibling checkout lives, local tooling paths —
> belongs in **`.ai/local.md`**, which is **gitignored**. Read `.ai/local.md`
> too if it is present; `.ai/local.md.example` is the committed template. Never
> move machine-specific paths into this file, and never commit `local.md`.

An **experimental** Terraform provider (terraform-plugin-framework, Go 1.25) for
[PingOne Advanced Identity Cloud](https://docs.pingidentity.com/pingoneaic/). It
manages AM scripts, authentication journeys, the journey node types those trees
use, OAuth2 clients, ESVs, custom managed-object types, and IDM endpoints and
schedules. User-facing overview lives in [README.md](../README.md) — don't
duplicate it here.

## The one rule that shapes everything

**This is not a JSON-passthrough wrapper.** Every attribute is typed in
`internal/nodetype/catalog.go`. Unknown API keys and unknown Terraform
attributes are _errors_, not warnings — `DecodeAPI` and `EncodeAPI` both reject
them. When AIC adds or renames a field, generate/plan fails on purpose.

The correct fix is always to teach the catalog about the field (plus a test).
Never `jsonencode()`, never add a raw-JSON escape hatch, never silently drop an
unrecognised key.

## Layout

| Path                  | What lives there                                                                          |
| --------------------- | ----------------------------------------------------------------------------------------- |
| `main.go`             | provider plugin entrypoint (`registry.terraform.io/agiledigital-labs/pingone-aic`)        |
| `cmd/generate/`       | `pingoneaic-tf` CLI — pulls live tenant config into reviewable HCL                        |
| `internal/provider/`  | provider schema, config resolution, resource registration                                 |
| `internal/resources/` | `script.go`, `journey.go`, `oauth2_client.go`, `node.go`, `idm_endpoint.go`, `idm_schedule.go` (node resources are one generic type driven by each catalog `Spec`) |
| `internal/nodetype/`  | **the node catalog** — typed field specs for all 34 node types, plus encode/decode                                |
| `internal/oauth2client/` | **the OAuth2 client catalog** — 115 typed fields in six groups, plus encode/decode                             |
| `internal/managedobject/` | typed decode/encode for one custom managed-object type (relationships remapped)                              |
| `internal/client/`    | thin AIC HTTP client: auth, trees, nodes, scripts, OAuth2 clients, ESVs, IDM config                               |
| `internal/amjson/`    | coercions for AM's loosely-typed JSON, shared by resources and generate                   |
| `internal/prefix/`    | `resource_prefix` apply/strip helpers                                                     |
| `internal/testutil/`  | test-only helpers (fake HTTP transport); imported from `_test.go` only                    |
| `examples/`           | hand-written HCL showing the intended shape                                               |
| `generated/`          | local experiment output — **gitignored**, not a source artefact                           |

Node resources aren't hand-written per type: `provider.Resources()` loops
`nodetype.All()` and builds a `nodeResource` from each `Spec`. Adding a node
type is a catalog edit, not a new file.

## Commands

The toolchain is pinned by `flake.nix` + `flake.lock` (Go, Terraform, GNU Make,
golangci-lint, gopls). Work inside it so everyone gets the same versions:

```sh
nix develop                      # interactive shell
nix develop --command make test  # one-off
nix develop .#ci                 # lean shell CI uses: Go + Make, no terraform
```

Entering the default shell also points `core.hooksPath` at `.githooks/`
(repo-local and idempotent).

Everything below assumes that shell. It is not mandatory — a system Go that
satisfies `go.mod` works fine — but version-skew bugs are yours to diagnose if
you skip it.

```sh
make test          # go test ./...
make lint          # golangci-lint run ./...
make build         # -> bin/terraform-provider-pingone-aic
make generate-cli  # -> bin/pingoneaic-tf
make install       # build + copy into ~/.terraform.d/plugins/...
make fmt           # gofmt -w .
make tidy          # go mod tidy
```

## Gates

`gofmt -l .` (must be empty), `go vet ./...`, `go build ./...`, `go test ./...`,
`golangci-lint run ./...`.

Three places run them, deliberately staged by cost:

| When                   | What runs                                       | Where                        |
| ---------------------- | ----------------------------------------------- | ---------------------------- |
| every commit           | gofmt on **staged** Go files + secret scan      | `.githooks/pre-commit`       |
| every push             | gofmt, build, vet, test, lint on the whole tree | `.githooks/pre-push`         |
| every branch push + PR | the same, via `nix develop .#ci`                | `.github/workflows/test.yml` |

Hooks install themselves when you enter `nix develop` (it sets
`core.hooksPath=.githooks`); set it by hand otherwise. `--no-verify` bypasses
them — but **work lands on `main` directly** in these early stages, so
`pre-push` is often the last automated check before a change is public. Bypass
knowingly.

The pre-commit secret scan rejects staged lines containing a JWT, a PEM private
key, or a real `*.forgeblocks.com` hostname. The sandbox hostname belongs in
`.envrc` (gitignored); use `openam-example.forgeblocks.com` as the placeholder
in anything committed.

**Linting.** `make lint` runs `golangci-lint` (pinned by the flake, present in
both shells — it is free software with a cached build, unlike terraform).
`.golangci.yml` carries the enabled set and, more importantly, the reasoning for
every check that is off: golangci-lint's standard set plus `dupl`, `errorlint`,
`gocritic` and `unparam`. Those four were chosen against the defect classes this
repo has actually shipped — see the findings log in `REVIEW.md`.

The tree is clean, so **any new finding is yours**. Prefer one central decision
in `.golangci.yml` over scattered suppressions: a spread of `//nolint` means
either a miscalibrated check (relax it there, once, with the reason) or code
fighting a good check (fix the code). The one `//nolint` in the tree, on
`nodetype.req`, carries its justification inline. Do not restructure working
code to satisfy a style-only check — turn the check off and say why.

CI uses the pinned toolchain via the lean `.#ci` shell, so it cannot drift from
local. Per-step `nix develop` overhead is ~0.1–0.3s once the store is warm; the
one-off cost is the Nix install and Go download at the start of each run.

Bumping the toolchain is `nix flake update` — commit the resulting `flake.lock`
on its own, so a version bump is never buried inside a behavioural change. Note
that nixpkgs' `terraform` is unfree (BUSL-1.1), so it is absent from the public
binary cache and compiles from source on a cold machine; `flake.nix` allows that
one package by name rather than blanket-allowing unfree.

Pulling live config:

```sh
export PINGONEAIC_TENANT_URL="https://openam-example.forgeblocks.com"
export PINGONEAIC_ACCESS_TOKEN="$(aic --no-prompt whoami --token)"
make generate-cli
./bin/pingoneaic-tf -realm alpha -out generated
```

Flags: `-realm` (default `alpha`), `-out`, `-prefix` (strip from existing
names), `-journeys` (comma-separated; default all — errors if a requested name
is not in the realm).

`-out` is **owned** by the tool: each run deletes the previous run's
`provider.tf`, `scripts.tf`, `oauth2_clients.tf`, `esv_*.tf`,
`managed_objects.tf`, `idm_endpoints.tf`, `idm_schedules.tf`, `journey_*.tf`,
`scripts/*.js`, `endpoints/*.js`, `schedules/*.js` and `hooks/*.js` so deleted
objects don't linger. It writes a `.pingoneaic-generated` marker to claim the
directory and refuses to delete anything in a directory that lacks the marker
but holds matching files — that guard is what stops `-out examples` eating
hand-written config. Progress goes to `Options.Progress` (stderr from the CLI);
`SIGINT`/`SIGTERM` cancel the run through the context.

## Conventions that bite if you miss them

**Resource prefix.** Terraform config always uses the _logical_ name (`GetIP`);
the provider prepends `resource_prefix` (default `Terraform_`) on the wire, so
applying generated config creates copies rather than clobbering the originals.
`name` vs `remote_name` on `pingoneaic_script` / `pingoneaic_journey` /
`pingoneaic_oauth2_client` is exactly this split. `internal/prefix` is
idempotent — `Apply` won't double-prefix.

**Changing `resource_prefix` does not rename existing resources.** It is
provider-level config, so it never triggers `RequiresReplace`. AM keys trees and
OAuth2 clients by name and cannot rename, so those stay at the name recorded in
state — `journeyRemoteName` / `oauth2RemoteName` resolve every CRUD path from
the persisted id, and only a create falls back to `prefix.Apply`. Recomputing
the name on update would PUT a second object and orphan the first. Scripts are
keyed by UUID, so for them a prefix change is a genuine rename in place.

**Computed attributes with a `Default` must round-trip.** If AM omits the key,
the decode path has to produce the same value the schema defaults to, or apply
dies with "Provider produced inconsistent result after apply". `version` on a
journey node is the worked example: `client.DefaultNodeVersion` backs both the
schema default and the decode fallback so they cannot drift.

**Connection sentinels.** `success` and `failure` in HCL map to AM's built-in
static node UUIDs (`client.SuccessNodeID` / `FailureNodeID`). Encode and decode
both translate; never write those UUIDs into HCL.

**Node UUIDs** are generated on create. Journey connections reference Terraform
resources, not hardcoded UUIDs.

**Defaults are omitted.** Catalog fields carry `Default` + `OmitEmpty`;
`EncodeAPI` fills defaults so AM's required list is satisfied, while generate
skips default-valued fields (`EqualDefault`) so emitted HCL stays reviewable.

**ESV strings** are wrapped on the wire as `{"$string": "&{esv.…}"}` —
`KindESVString` handles wrap/unwrap. Terraform sees the bare `&{…}` string.

**Scripts.** `source` is plaintext in HCL; the client base64-encodes on the
wire. Prefer `source = file("${path.module}/scripts/foo.js")` over an inline
string — generate emits that form. The same `file()` link works for IDM
endpoint, schedule, and managed-object hook `source`. The `file` *attribute*
on those resources is an IDM product path, not a local file. Always send `evaluator_version` (`2.0` default) — omitting it creates a
legacy v1 engine script. `SCRIPTED_DECISION_NODE` is accepted on write but AM
stores `AUTHENTICATION_TREE_DECISION_NODE`; re-read after create rather than
trusting the request form (`client.CanonicalContext`).

**Auth.** Either a pre-minted `access_token` (takes precedence) or a
service-account JWT (`service_account_id` + RSA private `jwk`, minted against
`/am/oauth2/access_token` with caching). All args have `PINGONEAIC_*` env
equivalents.

**AM API headers** are mandatory:
`Accept-API-Version: protocol=2.0,resource=1.0` and
`X-Requested-With: XMLHttpRequest`. `client.NewRequest` sets them — go through
it.

**`realm`** is `RequiresReplace` on every resource.

## Adding a node type or field

1. Add or extend the `Spec` in `internal/nodetype/catalog.go`. Use the `f()` /
   `req()` helpers so `TFName` derives from `APIName` via `snakeFromCamel`.
2. Add a test in `catalog_test.go` — the existing ones cover the patterns worth
   copying (unknown-field rejection, ESV round-trip, prefix handling, default
   fill).
3. Verify field names and defaults against a live tenant or the node schema
   endpoints (`client.NodeSchema` / `NodeTemplate`) — not from memory.
4. **Write the finding back into `pingone-aic-manager/docs/api/09-journeys.md`**
   — see
   [Feed what you learn back](#feed-what-you-learn-back-into-that-doc-set). A
   catalog gap means the API surface was documented incompletely; fixing only
   the catalog leaves the next person to rediscover it.

The resource, its Terraform schema, and generate support all follow from the
`Spec`. No other file should need touching _in this repo_ — but step 4 is not
optional.

## Reference material — read this before touching the wire format

The authoritative AIC API reference is **not in this repo**. It lives in the
sibling project [`agiledigital-labs/pingone-aic-manager`][manager] under
`docs/api/` — ~7,600 lines of endpoint-by-endpoint notes, each one verified
against a live tenant rather than transcribed from vendor docs or from
`frodo-lib` (which has known-stale claims).

**Read the relevant file before writing any code that hits a tenant. Don't guess
paths, headers, or field names.** This provider's client was derived from those
notes; they are the reason it works.

Most relevant here:

| File                              | Why you'd open it                               |
| --------------------------------- | ----------------------------------------------- |
| `00-auth.md`                      | service-account JWT bearer grant, token caching |
| `01-realms-and-paths.md`          | realm path composition                          |
| `02-headers-and-versioning.md`    | `Accept-API-Version` cheat sheet                |
| `03-esvs.md`                      | ESV ids, restart, `_pageSize` max 100           |
| `04-scripts.md`                   | script contexts, base64 body, no `_rev`         |
| `05-oauth2-oidc.md`               | OAuth2 clients, inherited wrappers, `*-encrypted` strip, protocol=2.1 |
| `09-journeys.md`                  | auth trees, nodes, custom nodes                 |
| `10-managed-objects.md`           | `config/managed` whole-doc RMW, Q14 lost updates |
| `11-idm-endpoints.md`             | IDM endpoints + schedules; no Accept-API-Version |
| `13-script-contexts.md`           | authoritative per-context binding metadata      |
| `99-quirks-and-open-questions.md` | cross-cutting weirdness worth a skim            |

The rest of the set (SAML, secret mappings, sync mappings, …) covers areas that
are [out of scope](#scope) for this provider — useful background, not current
work.

[manager]: https://github.com/agiledigital-labs/pingone-aic-manager

### Feed what you learn back into that doc set

This repo is a **consumer _and_ a contributor** of those docs. The reference is
only worth trusting because everyone who learns something writes it down, so:

> **Anything you learn about the AIC API while working here must be written back
> into `pingone-aic-manager/docs/api/`.** Landing the provider change is only
> half the job.

That includes a field the catalog was missing, a header or version the API
turned out to require, a response shape that differs from what's documented, an
undocumented default, an error the API returns for a malformed body — anything
that would have saved you time had it been written down.

Follow that repo's own procedure (see its `docs/api/README.md`):

1. **Verify first** — `scripts/verify-endpoint.sh <path> [--header ...]` against
   a tenant. Capture the real response shape; don't document what you assumed.
2. **Update the relevant capability file** (`09-journeys.md` for trees and
   nodes, `04-scripts.md` for scripts, …) and refresh its `## Verified against`
   block with the tenant and today's date.
3. **If observation contradicts the doc, the observation wins.** Fix the doc and
   append a dated entry to `99-quirks-and-open-questions.md` (that file is
   append-only).
4. **Never transcribe an unverified claim** from `frodo-lib`,
   `fr-config-manager`, or `docs.pingidentity.com`. Those have been wrong before
   — Q1 in `99-…` is exactly that story.

Only document what you actually exercised. Abbreviated response shapes are fine;
invented fields are not.

Cross-repo caveat: that is a **separate git repo**. Commit the doc change there
on its own branch — never mix it into a commit here, and don't leave the sibling
checkout dirty without saying so.

### Where it is on _this_ machine

The checkout path varies per developer, so it is **not** recorded here. It lives
in `.ai/local.md`, which is gitignored.

**Read `.ai/local.md` now if it exists** — it pins the local paths to the doc
set and any other machine-specific resources. If it's missing, copy
`.ai/local.md.example` to `.ai/local.md` and fill it in; if you can't locate a
checkout of `pingone-aic-manager` at all, say so rather than guessing at API
shapes from memory.

## Scope

In scope: scripts, journeys, journey nodes, OAuth2 clients, ESVs (variables and
secrets), custom managed-object types (including lifecycle hook blocks), IDM
endpoints and schedules. Explicitly
**out** of scope until that path is proven: SAML, secret mappings, the
realm-wide OIDC provider service, and Ping-shipped managed objects
(`alpha_user`, …) themselves. Lifecycle hooks are `hook` blocks on
`pingoneaic_managed_object` (custom types only) — never written onto
`alpha_user`.

IDM config (`/openidm/config/…`) must **not** send `Accept-API-Version`.
Schedule copies default to `enabled = false` so they cannot fire.

**Managed config writes are not read-your-writes.** `ReplaceManagedConfirmed`
re-reads until the new type is visible. Never PUT a document that was not
just read — other types in `objects[]` must be preserved.

**ESV ids cannot take `Terraform_` as a prefix.** AIC requires
`^esv-[a-z0-9_-]{1,124}$`. `prefix.ApplyESV` lowercases the provider prefix,
turns underscores into hyphens, and inserts it after `esv-`
(`esv-test11` → `esv-terraform-test11`). **Never restart the tenant from apply.**
`loaded` is computed; a write leaves the ESV pending until an operator restarts.

## Secrets

Never commit JWKs, tokens, or `.env` files — `.gitignore` covers `*.jwk`,
`*.pem`, `.env*`, `.token-cache`, and `get-token.sh`. Don't echo token values
into logs, test fixtures, or commit messages. `generated/` is gitignored because
it is pulled from a real tenant.
