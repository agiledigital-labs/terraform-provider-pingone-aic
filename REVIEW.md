# REVIEW.md — terraform-provider-pingone-aic review notes

Repo-specific review guidance, accumulated by the review-craft skill. The skill
reads **Standing checks** before every review and appends to the **Findings
log** when a review uncovers a durable lesson. Keep entries terse.

## Standing checks

Mandatory extra criteria every review applies here (promoted from recurring
findings). Each should name the guard that will eventually retire it.

- **Every `Computed` attribute with a `Default` must round-trip.** If the decode
  path can produce a different concrete value than the schema default when the
  AM key is absent, Terraform aborts with "Provider produced inconsistent result
  after apply". Check the missing-key case, not just the present-key case.
  _Guard: `TestTreeToModelDefaultsNodeVersionWhenAMOmitsIt` covers node
  `version`; `TestDecodeScheduleDefaultsPersisted` covers schedule `persisted`
  via `DefaultSchedulePersisted`. Script `evaluator_version` was checked against
  the tenant and is not affected — see Verified against._
- **And the mirror image: a bare `Optional` attribute must be written back
  exactly as planned.** `Optional` without `Computed` means state must equal
  plan, so substituting a remembered value when the plan is null — or collapsing
  null and `""` into each other in a `*ToModel` helper — produces the same
  inconsistent-result abort from the other direction. Write-only values the API
  never returns (`esv_secret.value`) want
  `Optional + Computed + UseStateForUnknown`, not bare `Optional`. _Guard:
  `TestSecretToModelPreservesPlannedValueExactly`. Take `types.String`, not
  `string`, in any `*ToModel` that carries an optional attribute — the `string`
  signature is what made the collapse invisible._
- **Know what a resource is keyed by before reasoning about renames.**
  `resource_prefix` is provider-level, so changing it never triggers
  `RequiresReplace`. Name-keyed resources (journeys/trees, OAuth2 clients) must
  resolve every CRUD path from the persisted id or an update orphans the
  original; UUID-keyed resources (scripts) rename in place and are fine. _Guard:
  `TestWriteTargetsPersistedNameAfterPrefixChange` — one table over all eight
  name-keyed resources, create and update. Add a row for any new one._ **The
  test must drive the write path through a fake transport and assert the request
  path, not call the resolver helper.** The 2026-08-14 entry below is the whole
  reason: the bug was `write()` ignoring the helper, and a helper-level test
  cannot see that. The helper-level tests are gone; adding one back is a
  regression. The rule itself lives once, in `remoteName`
  (`internal/resources/remote.go`) — a second copy of that loop is the other
  half of this check.
- **Fail-closed validators must be reachable from one entry point.** This repo's
  core rule (unknown key = error) only holds if every caller runs every
  validator. Two validators that must be called as a pair are a latent
  passthrough. _Guard: `TreeWriteBody` now calls `validateTreeInternals` itself;
  `TestTreeWriteBodyRejectsNestedUnknownFields` locks it in._
- **Every nested object a decoder descends into needs its own `rejectUnknown`.**
  Guarding the top-level body and stopping one level short is the same
  passthrough hole, only harder to see: the decoder reads the two keys it knows
  and the re-encode silently deletes the rest. Count the `asObject(...)` calls
  in a decoder and check each has a matching key set. _Guard: applied.
  `TestIDMFixtureDecodeEncodePreservesRecursiveKeySet` runs `Decode → Encode`
  over every fixture under `testdata/{endpoints,schedules,roles}` and compares
  the full recursive key path set, so any key the model cannot carry fails the
  build. Adding a fixture directory is one table row. The permissive
  `stringVal`/`boolVal`/`intVal` accessors were deleted at the same time in
  favour of erroring `strictString`/`strictBool`/`strictInt`/`strictObject` — a
  wrong-typed value must fail, not decode to a confident zero._
- **Destructive filesystem work needs a marker, not just a matching name.**
  Anything that deletes under a user-supplied path must prove the directory is
  ours first. _Guard: `TestCleanGeneratedFilesRefusesUnmarkedDirectory`._
- **Anything that emits a document in another language needs an escaping
  round-trip property, not examples.** `generate` writes HCL containing tenant
  text — JS expressions, query filters, ESV values — and `${`/`%{` are
  interpolation and directive sigils there. Hand-picked cases cannot cover the
  input space. _Guard: `FuzzHclStringRoundTrip` asserts that parsing
  `x = hclString(s)` evaluates back to `s`. Note `strconv.Quote` is **not** the
  right primitive: it emits `\xHH`/`\a`/`\b`/`\v`, which HCL rejects.
  `hclwrite.TokensForValue` is. `hclFile` is the deliberate exception that emits
  a real `${path.module}`._
- **One list, one definition — especially when the second copy is prose.**
  `generatedPaths` and the `.pingoneaic-generated` marker text both enumerate
  what a run deletes; the marker copy had already gone stale on three entries
  before anyone noticed, because nothing compares prose to code. _Guard:
  `generatedOutputs` is the single table both read, plus a test asserting every
  path the writers record is matched by `generatedPaths`. The remaining copies
  in `README.md` and `.ai/core.md` are still hand-maintained._
- **A gate that nothing runs is not a gate.** `gofmt`/`vet`/`build`/`test`/
  `lint` all pass with a directly imported module still marked `// indirect`;
  `make tidy` existed but was in no hook and no workflow, so it never ran. This
  cost a follow-up commit once (`go-cty`, 2026-08-15). When reviewing, check the
  `Makefile` against `.githooks/` and `.github/workflows/` — a target with no
  caller is a gap. _Guard: applied. `go mod tidy` + `git diff --quiet go.mod
  go.sum` now run in `.githooks/pre-push` and `.github/workflows/test.yml`._
- **Fail-closed allowlists must be validated against a live tenant sweep, not
  hand-written fixtures.** Fetch every object of the type and run the real
  bodies through the validator; a shape you did not think of is exactly what the
  allowlist will reject in production. _Guard: none automated — the sweep is
  manual. A `-tags live` test reading a fixture directory would retire it._
  Managed custom types: `TestDecodeAllCustomObjectsFromTenantSweep`.
- **Whole-document RMW must serialise in-process.** Two Terraform resources
  mutating `/openidm/config/managed` in parallel each confirmed their own
  insert, then the second PUT dropped the first. _Guard: `Client.MutateManaged`
  holds `managedMu` around GET+mutate+PUT+confirm. `MutateAccess` /
  `MutateAuthentication` do the same with `accessMu` / `authMu`._

- **Error identity must survive wrapping.** A helper that classifies an error
  (`IsNotFound` and friends) must use `errors.As`/`errors.Is`, never a bare type
  assertion — the bug is invisible until someone adds a `%w` upstream. _Guard:
  `errorlint`, enabled 2026-08-14._

## Findings log

### 2026-08-15 — Schedule script `globals` decoded away, then written away

- **What:** `DecodeSchedule` descends into `invokeContext.script` and
  `invokeContext.task.script` and reads only `source` and `type`. Neither
  sub-object gets a `rejectUnknown`. Three of the six schedules in the tenant
  sweep (`Test`, `UpdateReviewList`, `test_sign_in`) carry a `globals` object
  there. `EncodeSchedule` rebuilds `script` from `{source, type}`, so importing
  or generating one of those schedules and applying it **deletes its `globals`**
  — silently, which is the one thing this repo's core rule exists to prevent.
  `scan.taskState` and `scan.recovery` have the same hole.
- **Why missed:** the top-level allowlist and `invokeContext` / `scan` were
  guarded, so the file reads as fail-closed at a glance. Standing check 3 was
  written about two validators that must be _called as a pair_; it did not
  prompt for validators that were never written for the third and fourth level
  down. `internal/client/idm_endpoint.go`, in the same commit, does guard
  `globals.endpointConfig` — the author knew the shape mattered.
  `TestDecodeAllLiveScheduleFixtures` passes because it only asserts that decode
  succeeds and `Name`/`InvokeService` are non-empty.
- **Guard:** applied. Every nested schedule object now has its own key set,
  `Globals` is carried on `client.Schedule` and surfaced as a `map(string)`
  Terraform attribute, and the three per-kind decode sweeps collapsed into
  `TestIDMFixtureDecodeEncodePreservesRecursiveKeySet` — one table comparing the
  full recursive key path set of `Decode → Encode` against the fixture, for
  endpoints, schedules and roles at once. Promoted to a Standing check.
- **Found while fixing:** the `globals` in all three fixtures is `{}`, so the
  live loss was an empty key rather than user data — the hole is real, the blast
  radius smaller than the first reading suggested. The same guard immediately
  caught two things the review had missed: endpoint nested-`source` shape loss,
  and null-key loss in roles. It also forced deleting `stringVal`/`boolVal`/
  `intVal` in favour of erroring `strict*` twins, which was a separate finding in
  the same review. **A round-trip property found more than the reviewer who
  proposed it** — the argument for reaching past examples, every time.
- **Written back:** `pingone-aic-manager` `docs/api/11-idm-endpoints.md` claimed
  taskscanner schedules carry no inline script. All three in the sweep do, at
  `invokeContext.task.script.source`. Corrected on branch
  `docs/schedule-script-nesting`, together with the undocumented
  `org.forgerock.openidm.script` alias an equality filter would skip, and a
  dated `99-…` entry.

### 2026-08-15 — Generated HCL was never escaped, and `strconv.Quote` is the wrong fix

- **What:** `hclString` was `strconv.Quote`, which escapes for Go, not HCL.
  `${` and `%{` reached Terraform as interpolation and directive sigils.
  Pre-existing since the initial commit, but the last twelve commits added ~67
  call sites and started routing genuinely hostile strings through it for the
  first time: `custom_authz` JS, role `condition` query filters,
  `scan_query_filter`, plaintext ESV values.
- **Why missed:** `generate`'s tests assert the *shape* of emitted HCL against
  known-benign fixtures. Nothing ever parsed the output back. A generator with no
  round-trip test cannot notice that it emits a different language than it
  intends.
- **Guard:** applied — and the first proposed fix was wrong, which is the part
  worth keeping. The review proposed
  `strings.NewReplacer("${","$${","%{","%%{").Replace(strconv.Quote(s))`. That is
  correct for the sigils but still fails the property, because `strconv.Quote`
  emits `\xHH`, `\a`, `\b`, `\f`, `\v` — escapes HCL rejects. The right
  primitive is `hclwrite.TokensForValue(cty.StringVal(s))`.
  `FuzzHclStringRoundTrip` is what established that; it also turned up
  HCL/cty NFC-normalising string values (U+0340 evaluates as U+0300), which is an
  evaluation property rather than a quoting bug, so the fuzz skips non-NFC input.
  **The property was worth more than the fix it was written to defend.**
  Promoted to a Standing check.

### 2026-08-15 — Six copies of the remote-name resolver, tested six times at the wrong level

- **What:** `configRemoteName` and `oauth2RemoteName` are byte-identical;
  `esvRemoteName`, `secretRemoteName` and `managedRemoteName` are the same loop
  with a different fallback; `journeyRemoteName` is a sixth variant. Each got
  its own near-identical unit test asserting the _helper_.
- **Why missed:** Standing check 2 said "add the equivalent test for any new
  name-keyed resource" without saying at what level, so five resources satisfied
  it with exactly the shape the 2026-08-14 entry had already recorded as giving
  false confidence.
- **Guard:** applied. `remoteName(id, remote types.String, fallback func() string)`
  in `internal/resources/remote.go` is the only copy of the rule; every resource
  grew a `write(ctx, plan, prior)` seam that resolves through it, and
  `TestWriteTargetsPersistedNameAfterPrefixChange` drives all eight through a
  fake transport asserting the request path (managed config asserts the object
  `name` in the PUT body, since it is a whole-document write). The six
  helper-level tests are deleted. Standing check 2 amended to name the level.
- **Found while fixing:** inlining the fallback closure at each call site pushed
  `idm_endpoint.Read` and `idm_schedule.Read` past `dupl`'s threshold. The right
  answer was two one-line wrappers (`applyRemote`, `applyESVRemote`), not a
  `//nolint` — worth remembering as the shape of a correct response to `dupl`.

### 2026-08-15 — `dupl` fired and was suppressed rather than answered

- **What:** the access-rule and authentication-mapping resources are a parallel
  CRUD implementation; `dupl` caught the `Update` pair and the change added
  `//nolint:dupl` to both. `Create` and `Delete` are equally parallel and only
  escaped because the threshold is 150.
- **Why missed:** `dupl` was enabled _because_ "a third parallel copy of the AM
  JSON decode helpers reached review once" (`.golangci.yml`), and the config
  itself says a recurring directive means the code is fighting a good check. The
  nolint comments are honest and accurate, which makes them easy to wave
  through.
- **Guard:** applied. `hashedRuleResource[Model, Rule]` in `hashed.go` owns
  CRUD, import and lookup; the two resources are now a schema, a model and a
  spec. `access_rule.go` 254 → 102 lines, `authentication_mapping.go` 235 → 86,
  **both directives deleted**. The only `//nolint` left in the tree is the
  pre-existing justified `unparam` on `nodetype.req`.
- **Also fixed here:** `Replace*` gained the duplicate-hash pre-flight that
  `Append*` already had. Without it, editing a rule into a form matching another
  live entry left two copies at the new hash, `confirm*` expected one, and the
  retry loop re-PUT an already-correct document six times (~15.5 s) before
  failing with "was accepted but not persisted" — the opposite of what happened.
  `TestReplaceRuleRefusesExistingContentHashBeforePut` asserts GET=1, PUT=0 and
  a fast, accurate error. The client now returns `RuleConfirm` from every
  mutator, so the four hardcoded `Count: 1` literals are gone.

### 2026-08-14 — Node `version` default breaks apply when AM omits the key

- **What:** `journeyResource` gained `node.version` with a `"1.0"` schema
  default, but `treeToModel` set `types.StringValue("")` when AM omitted the
  key. Plan `"1.0"` vs state `""` → inconsistent-result error. Reproduced
  directly against `treeToModel`.
- **Why missed:** the accompanying tests only exercised trees where the key is
  present (`TestTreeToModelPreservesTimeoutSettings` has zero nodes).
- **Guard:** applied. `client.DefaultNodeVersion` now backs both the schema
  default and the decode fallback, with a regression test. Promoted to Standing
  check 1.

### 2026-08-14 — ID-binding fix applied to Read/Delete but not `write()`

- **What:** `journeyRemoteName` was introduced to bind the journey lifecycle to
  the persisted remote id and wired into `Read` and `Delete`, but `write()`
  (Create **and** Update) still called `prefix.Apply(prefix, name)`. After a
  `resource_prefix` change, Update PUT to the new name — creating a second tree
  and orphaning the original — while Read/Delete still targeted the old one.
- **Why missed:** the two new tests assert the helper in isolation, one of them
  named "AcrossPrefixChanges" — precisely the scenario the untouched `write()`
  path got wrong. Helper-level tests gave false confidence.
- **Reviewer error worth keeping:** the review also claimed
  `scriptResource.write` had the same bug. It does not — scripts are keyed by
  UUID, so a prefix change renames in place with nothing orphaned. Pattern-
  matched on the identical `prefix.Apply` call without checking the key.
- **Guard:** applied. `write` takes prior state and resolves through
  `journeyRemoteID`; the new test drives the real write path through a fake
  transport and was verified to fail without the fix. Promoted to Standing
  check 2.

### 2026-08-14 — `-out` directory became destructive without a doc or guard

- **What:** `cleanGeneratedFiles` unconditionally deleted `provider.tf`,
  `scripts.tf`, `journey_*.tf` and `scripts/*.js` under `-out`. `-out examples`
  would have deleted this repo's committed `examples/provider.tf`. The one test
  guarding it only checked that `notes.md` survived — never the dangerous case.
- **Guard:** applied. A `.pingoneaic-generated` marker claims the directory;
  `checkGeneratedDir` refuses (before the first tenant round-trip) to touch an
  unmarked directory holding matching files. Promoted to Standing check 4.

### 2026-08-14 — Third parallel copy of AM JSON decode helpers

- **What:** `internal/generate` had `str`/`boolDef`/`optionalInt64`;
  `internal/resources` had `str`/`boolish`/`int64ish`, with different signatures
  for `str`. The change extended the duplication rather than collapsing it.
- **Guard:** applied. Extracted to `internal/amjson`; `internal/resources` keeps
  only the thin framework-type adapters. A `dupl` lint would stop it recurring —
  see the linter entry below.

### 2026-08-14 — Tests that overclaim their scenario

- **What:** four defects this round hid behind tests that assert _helpers_ in
  isolation while the calling path stayed wrong — `journeyRemoteName`,
  `optionalInt64`, `cleanGeneratedFiles`, and `treeToModel` given full input. A
  passing test whose _name_ asserts a scenario it never exercises is more
  dangerous than no test.
- **Guard:** no automated one exists. Review heuristic: when a test name
  describes a user-visible scenario, check that it calls the code path the user
  would hit, not the helper underneath it.

### 2026-08-14 — `IsNotFound` was blind to wrapped errors

- **What:** `client.IsNotFound` used a bare type assertion, so a 404 wrapped
  with `%w` read as "still exists". It gates the "resource is gone, drop it from
  state" path in all three resources, so `Read` would surface an error instead
  of removing the resource and `apply` would never converge on an out-of-band
  deletion.
- **Why missed:** latent, not live — every read path today returns `*APIError`
  unwrapped, so no test could fail and no review would see a symptom. The client
  wraps with `%w` in two dozen places; one future wrap in a getter would have
  armed it silently. Neither the craft review nor the original change looked for
  _error-identity_ bugs, only for value bugs.
- **Guard:** applied. `errors.As`, plus a table test over bare/once/twice
  wrapped 404s that was verified to fail before the fix. `errorlint` is now in
  the gate set and would catch the next one.

### 2026-08-14 — Allowlist rejected a shape a sixth of the tenant carries

- **What:** `treeUIAttrs` allowed only `uiConfig.categories`, so every journey
  carrying the editor's canvas layout was rejected — generate aborted and the
  provider could not Read the resource. 6 of 35 trees on the sandbox carry
  `uiConfig.annotations`.
- **Why missed:** the allowlist was written from the fields the provider models,
  not from what AM returns, and its tests only exercised hand-constructed
  bodies. Nothing in the review or the test suite ever fed a real tenant
  response through it. A fail-closed allowlist is only as good as the survey
  behind it.
- **Guard:** applied — `annotations` accepted, with both live shapes as
  regression cases. Promoted to Standing check 5.

## Verified against

Sandbox tenant, `alpha` realm, 2026-08-14 — 35 trees / 177 nodes / 127 scripts.

- `uiConfig` keys observed: `categories` (18), `annotations` (6). `annotations`
  is a JSON **string**, `{"forNodes":…,"structural":…}`.
- `staticNodes` absent entirely on 3 of 35 trees; entries only ever hold
  `x`/`y`.
- Tree node keys are exactly `connections`, `displayName`, `nodeType`,
  `version`, `x`, `y` — all present on all 177 nodes; `version` is `"1.0"`
  everywhere.
- `maximumIdleTime` / `maximumSessionTime` / `treeTimeout` appear on **no**
  tree, but a PUT carrying them returns 201 and both the PUT and a follow-up GET
  echo the values. They are real, writable, and simply omitted when unset — so
  modelling them Optional+Computed with a null decode is correct.
- `evaluatorVersion` is present on **all 127** scripts (88 × `"1.0"`, 39 ×
  `"2.0"`). `decodeScript`'s `"1.0"` fallback never fires, so the open question
  below is resolved: **not a defect**. `"1.0"` really is the legacy engine, and
  importing such a script correctly diffs against the `"2.0"` schema default.

Probes used a `Terraform_`-prefixed throwaway tree and one scratch script; both
were deleted and confirmed 404 afterwards.

## Open questions

- **`modelToTree` drops `uiConfig.annotations` on write.** It rebuilds
  `uiConfig` from `categories` alone, so adopting an annotated journey and
  updating it discards the saved canvas layout. Journeys this provider creates
  have none, so it only bites on import. Fix would be to round-trip the raw
  value through state.
- **Should more linters go on?** Free today (0 findings each): `bodyclose`,
  `nilerr`, `noctx`, `misspell`, `copyloopvar`, `wastedassign` — `bodyclose` and
  `nilerr` are cheap insurance for an HTTP-client-shaped repo. `revive` would
  add 50, 49 of them missing doc comments on exported symbols: a real but
  separate documentation slice. `gosec` (6) wants 0600/0750 on generated `.tf`
  and `.js` output, which is wrong here — those are meant to be readable and
  committed.
