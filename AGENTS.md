# AGENTS.md

Guidance for AI coding assistants working in `fluxcd/pkg`. Read this file before making changes.

## Contribution workflow for AI agents

These rules come from [`fluxcd/flux2/CONTRIBUTING.md`](https://github.com/fluxcd/flux2/blob/main/CONTRIBUTING.md) and apply to every Flux repository.

- **Do not add `Signed-off-by` or `Co-authored-by` trailers with your agent name.** Only a human can legally certify the DCO.
- **Disclose AI assistance** with an `Assisted-by` trailer naming your agent and model:
  ```sh
  git commit -s -m "Add support for X" --trailer "Assisted-by: <agent-name>/<model-id>"
  ```
  The `-s` flag adds the human's `Signed-off-by` from their git config — do not remove it.
- **Commit message format:** Subject in imperative mood ("Add feature X" instead of "Adding feature X"), capitalized, no trailing period, ≤50 characters. Body wrapped at 72 columns, explaining what and why. No `@mentions` or `#123` issue references in the commit — put those in the PR description.
- **Trim verbiage:** in PR descriptions, commit messages, and code comments. No marketing prose, no restating the diff, no emojis.
- **Rebase, don't merge:** Never merge `main` into the feature branch; rebase onto the latest `main` and push with `--force-with-lease`. Squash before merge when asked.
- **Pre-PR gate:** `make test-<module>` must pass for every module you touched. Run `make tidy` to tidy all affected modules.
- **Flux is GA:** Backward compatibility is mandatory. These modules are consumed by all Flux controllers — breaking changes to exported APIs, function signatures, or behavior will be rejected. Design additive changes.
- **Copyright:** All new `.go` files must begin with the Apache 2.0 boilerplate header. Update the year to the current year when copying.
- **Tests:** New features, improvements and fixes must have test coverage. Follow existing patterns in the module you're modifying. Run tests locally before pushing.

## Code quality

Before submitting code, review your changes for the following:

- **No unchecked I/O.** Close HTTP response bodies, file handles, and archive readers in `defer` statements. Check and propagate errors from I/O operations.
- **No path traversal.** The `tar` module uses `cyphar/filepath-securejoin` — always extract archives through it. Never `filepath.Join` with untrusted components without validation.
- **No command injection.** Do not shell out via `os/exec`. Use Go libraries for git, OCI, and cloud operations.
- **No hardcoded defaults for security settings.** TLS verification must remain enabled by default.
- **Error handling.** Wrap errors with `%w` for chain inspection. Do not swallow errors silently. Return errors that help callers diagnose the issue without leaking internal state.
- **Resource cleanup.** Ensure temporary files and directories are cleaned up on all code paths. Use `defer` and `t.TempDir()` in tests.
- **No panics.** Never use `panic` in library code. Return errors and let callers decide how to handle them.
- **Thread safety.** These packages are used in concurrent reconcilers. Do not introduce shared mutable state without synchronization.
- **Minimal surface.** Every exported type, function, and method is a backward-compatibility commitment consumed by multiple controllers. Minimize new exports.

## Project overview

`fluxcd/pkg` is the shared Go SDK for the Flux GitOps Toolkit. It is a **multi-module monorepo** — there is no top-level `go.mod`. Each subdirectory is its own independently versioned Go module, tagged separately (e.g. `runtime/v0.103.0`, `ssa/v0.23.0`, `apis/meta/v1.26.0`). All Flux controllers import specific modules from this repo.

The repository provides: controller runtime helpers, server-side apply engine, git operations, cloud auth/workload identity, OCI operations, kustomize building, artifact storage, and shared API types.

## Repository layout

There is **no top-level `go.mod`**. Each directory is its own module:

- `apis/meta/` — foundational API types: standard conditions (`Ready`, `Stalled`, `Reconciling`), reasons, annotations (`ReconcileRequestAnnotation`), artifact spec, dependency references.
- `apis/event/` — Flux event schema dispatched to notification-controller.
- `apis/acl/` — cross-namespace access control types.
- `apis/kustomize/` — Kustomize-related API types (e.g. `HealthCheckExpressions`).
- `runtime/` — largest module. Sub-packages: `conditions`, `patch`, `reconcile`, `events`, `metrics`, `features`, `cel`, `acl`, `controller`, `dependency`, `errors`, `jitter`, `leaderelection`, `logger`, `object`, `predicates`, `probes`, `pprof`, `secrets`, `statusreaders`, `testenv`, `transform`, `client`.
- `ssa/` — server-side apply engine (`ResourceManager`): apply, diff, wait, delete, change sets. Sub-packages: `jsondiff`, `normalize`, `errors`, `utils`.
- `git/` — git operations. `gogit/` sub-package is the concrete go-git implementation. `repository/` defines `Reader`/`Writer` interfaces.
- `auth/` — cloud workload identity: `aws/`, `azure/`, `gcp/`, `generic/`, `githubapp/`, `utils/`. Central `GetAccessToken()` with caching.
- `artifact/` — artifact storage: `config/`, `digest/`, `server/`, `storage/`.
- `oci/` — OCI registry client (push, pull, tag, list, build, diff, delete).
- `kustomize/` — kustomize generator and variable substitution. `filesys/` provides secure filesystem implementations.
- `cache/` — generic in-memory cache (`Cache[T]`, `LRU[K,V]`, token cache helpers).
- `http/fetch/` — HTTP archive fetcher with retry and digest verification.
- `http/transport/` — HTTP transport utilities.
- `tar/` — secure tarball extraction (path traversal prevention).
- `lockedfile/` — atomic file operations with OS-level locking.
- `masktoken/` — token redaction for secure logging.
- `envsubst/` — variable expansion (`${var}` syntax with bash string manipulation support).
- `chartutil/` — Helm chart values merging from ConfigMaps/Secrets.
- `sourceignore/` — gitignore-style source filtering.
- `ssh/` — SSH host key scanning.
- `version/` — semantic version parsing/sorting.
- `testserver/` — base test server utilities.
- `gittestserver/` — in-process Git HTTP/SSH server for tests.
- `helmtestserver/` — in-process Helm chart repository server for tests.
- `cmd/` — internal `flux-tools` binary for release automation (not tagged/released).
- `tests/integration/` — cloud provider integration tests (not tagged).
- `actions/` — reusable GitHub Actions (helm, kubectl, kustomize, etc.).

## Multi-module architecture

This is the most important thing to understand about this repo:

- **Every directory with a `go.mod` is an independent module.** There are 24 taggable public modules.
- **Each module gets its own git tag** in the form `<module-path>/v<semver>` (e.g. `runtime/v0.103.0`, `http/fetch/v0.15.0`).
- **Sibling modules reference each other via `replace` directives** during development. For example, `runtime/go.mod` has `replace github.com/fluxcd/pkg/apis/meta => ../apis/meta`. These replaces stay permanently — they enable local cross-module development without publishing intermediate tags.
- **External consumers** (controllers) import specific tagged versions: `go get github.com/fluxcd/pkg/runtime@v0.103.0`.
- **Changing one module may require updating dependents.** If you modify `apis/meta`, all modules that depend on it (e.g. `runtime`, `ssa`) may need their tests re-run. The `make prep` command handles version bumps for releases.

## Build, test, lint

All targets in the root `Makefile`. Module paths use `:` as separator in make targets (e.g. `http/fetch` → `http:fetch`).

- `make all` — runs `tidy`, `generate`, `fmt`, `vet` for all modules.
- `make test` — runs tests for ALL modules sequentially.
- `make test-chunk CHUNK=N/M` — runs tests for a 1/M slice of modules (CI uses 4 parallel chunks).
- `make test-<module>` — runs tidy, generate, fmt, vet, then `go test ./... -race -coverprofile cover.out` for a single module. Examples: `make test-runtime`, `make test-ssa`, `make test-http:fetch`.
- `make tidy` / `make tidy-<module>` — `go mod tidy` for all or one module.
- `make generate` / `make generate-<module>` — `controller-gen` codegen.
- `make fmt` / `make vet` — format and vet all modules.

Run a single test: `make test-runtime` (runs the full runtime module suite). For a specific test function within a module, cd into the module directory and run `go test ./... -run TestName -v` with `KUBEBUILDER_ASSETS` set if envtest is needed.

## Codegen and generated files

After changing API types or kubebuilder markers, regenerate:

```sh
make generate-<module>
```

Generated files (never hand-edit):

- `*/zz_generated.deepcopy.go` — in any module with API types.

No codegen output is committed at the top level. Each module manages its own generated files.

Load-bearing `replace` directives — do not remove:

- Sibling `replace` directives (e.g. `../apis/meta`) in every module that depends on another module in this repo. These are permanent and required for local development.
- `gopkg.in/yaml.v3 => gopkg.in/yaml.v3 v3.0.1` — CVE fix present in multiple modules.
- `opencontainers/go-digest` fork — provides BLAKE3 support in `artifact` and `http/fetch`.

## Conventions

- Standard `gofmt`. All exported names need doc comments. Match the style of the module you're editing.
- **Interface-first design.** Key abstractions are interfaces (`repository.Reader`/`Writer` in git, `Provider` in auth, `Policer` in controllers). Add implementations behind interfaces.
- **Condition helpers.** Use `runtime/conditions` (Get, Set, Merge, Patch) for status condition manipulation. Never manipulate condition slices directly.
- **Patch helper.** Use `runtime/patch.Helper` for conflict-safe status patching. Create the helper before reconciliation, call `Patch()` at the end with owned conditions.
- **Events.** Use `runtime/events.Recorder` which posts to both the k8s API and notification-controller's HTTP endpoint.
- **Metrics.** Standard names: `gotk_reconcile_condition`, `gotk_suspend_status`, duration histogram. Use `runtime/metrics.Recorder`.
- **Feature gates.** Use `runtime/features.FeatureGates` backed by `--feature-gates` CLI flag.
- **SSA.** Use `ssa.ResourceManager` for server-side apply. Do not use `client.Apply` directly.
- **Artifact storage.** Use `artifact/storage.Storage` with `lockedfile` for concurrency-safe writes.
- **No cross-module imports at test time only.** If a test in module A needs a helper from module B, use the existing test server packages (`testserver`, `gittestserver`, `helmtestserver`) which are designed for this.

## Testing

- Tests use standard `go test ./... -race`. The Makefile orchestrates per-module.
- Modules that need Kubernetes use `runtime/testenv` (wraps controller-runtime envtest). `KUBEBUILDER_ASSETS` must point at downloaded kube-apiserver/etcd binaries (installed by `make install-envtest`).
- Test frameworks: mix of `onsi/gomega`, and standard `testing`. Match the module's existing style.
- Git e2e tests live in `git/internal/e2e/` (separate module, runs against real GitLab/Bitbucket).
- Cloud integration tests live in `tests/integration/` (separate module, Terraform-based).
- Test servers (`gittestserver`, `helmtestserver`, `testserver`) provide in-process fakes for git repos, Helm chart repos, and HTTP artifact servers.

## Gotchas and non-obvious rules

- **No top-level `go.mod`.** You cannot `go test ./...` from the repo root. Always work within a specific module directory or use `make test-<module>`.
- **`replace` directives are permanent.** They are not development leftovers — they are how sibling module development works. Do not remove them. Do not convert them to published versions.
- **Module versioning is independent.** Changing `apis/meta` does not automatically bump `runtime`. The `flux-tools prep` command handles version propagation at release time.
- **Adding a new exported symbol is a cross-repo contract change.** All Flux controllers depend on these modules. Renaming, removing, or changing the signature of any exported type/function breaks downstream consumers even if this repo's tests pass.
- **Adding a new module** requires updating `cmd/internal/enumerate_taggable_modules.go` so the release automation knows to tag it. It also needs to be added to `README.md`.
- **The Makefile computes `MODULES` dynamically** by scanning for `go.mod` files. New modules are picked up automatically for `make test` and `make tidy`.
- **Colon encoding in make targets.** `http/fetch` is targeted as `make test-http:fetch`. This is deliberate — the Makefile translates `:` back to `/` internally.
- **envtest is needed by many modules.** If you see `KUBEBUILDER_ASSETS` errors, run `make install-envtest` first.
- **`runtime` has the most internal `replace` directives** (all four `apis/*` modules). Changes to any `apis/*` module should be tested with `make test-runtime` to catch breakage.
- **`ssa` has zero dependency on other `fluxcd/pkg` modules.** This is intentional — keep it that way to avoid circular dependencies.
- **Test modules (`cmd/`, `git/internal/e2e/`, `tests/integration/`) are not tagged.** They are excluded from release automation. Do not add `replace` directives pointing at them from taggable modules.
