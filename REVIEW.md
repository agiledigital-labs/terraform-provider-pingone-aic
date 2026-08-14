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
  `version`; no equivalent yet for script `evaluator_version` (see open question
  below)._
- **Know what a resource is keyed by before reasoning about renames.**
  `resource_prefix` is provider-level, so changing it never triggers
  `RequiresReplace`. Name-keyed resources (journeys/trees) must resolve every
  CRUD path from the persisted id or an update orphans the original; UUID-keyed
  resources (scripts) rename in place and are fine. _Guard:
  `TestJourneyWriteTargetsPersistedTreeAfterPrefixChange`. Add the equivalent
  for any new name-keyed resource._
- **Fail-closed validators must be reachable from one entry point.** This repo's
  core rule (unknown key = error) only holds if every caller runs every
  validator. Two validators that must be called as a pair are a latent
  passthrough. _Guard: `TreeWriteBody` now calls `validateTreeInternals` itself;
  `TestTreeWriteBodyRejectsNestedUnknownFields` locks it in._
- **Destructive filesystem work needs a marker, not just a matching name.**
  Anything that deletes under a user-supplied path must prove the directory is
  ours first. _Guard: `TestCleanGeneratedFilesRefusesUnmarkedDirectory`._

## Findings log

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

## Open questions

- **`evaluator_version` may be the same defect class as node `version`.** The
  schema defaults to `"2.0"` (`internal/resources/script.go`) but `decodeScript`
  falls back to `"1.0"` when AM omits `evaluatorVersion`. Unlike node `version`,
  `"1.0"` is a _meaningful observed value_ (legacy v1 engine), so blindly
  aligning them would mislabel legacy scripts. Needs a live-tenant check of
  whether AM ever omits `evaluatorVersion` in a PUT response before deciding. Do
  not "fix" it from first principles.
- **No linter beyond gofmt/go vet.** None of the defects above are
  vet-detectable. Proposed: `golangci-lint` in the flake and the `.#ci` shell
  with `errcheck`, `gocritic`, `dupl`, `unparam`. Deferred as its own slice —
  turning it on will surface a wave of pre-existing findings that should not be
  mixed into a defect-fix change.
