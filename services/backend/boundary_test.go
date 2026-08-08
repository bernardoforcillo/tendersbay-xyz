package main

// boundary_test.go mechanically enforces one half of the dependency rule in
// .claude/rules/code-organization.md: `internal/core` defines the ports, the
// adapters implement them and import core — never the reverse. Convention alone
// erodes as a codebase grows, and it erodes faster with AI agents in the loop,
// because an agent reuses whatever pattern it already finds in the tree. So the
// rule gets a test rather than a review comment.
//
// This lives in the root `package main` on purpose. The check itself imports
// nothing from `internal/...`, but the packages it inspects form a genuine
// import cycle at the source level — `postgres` imports `core/{auth,bid,…}`
// while `core/agent` → `core/credits` → `postgres` closes the loop back — so a
// checker that tried to *import* the packages it audits could not be placed
// anywhere inside `internal/`. Shelling out to `go list` sidesteps that: it
// reads the import graph without linking any of it, and it is the same graph the
// compiler sees (build tags, ignored files, _test.go files and all), which a
// `grep` is not. A grep is in fact actively wrong here — `grep -rn
// "internal/adapter" internal/core` already matches the doc comments in
// bid/types.go, tender/tender.go and document/document.go that *assert*
// compliance.

import (
	"bufio"
	"errors"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// modulePath is this module's path (see go.mod). Import paths are reported
// relative to it so failures read as `internal/core/agent imports
// internal/adapter/postgres` rather than as two 90-character URLs.
const modulePath = "github.com/bernardoforcillo/tendersbay-xyz/services/backend"

// coreDir and adapterPrefix are the two sides of the boundary, module-relative.
const (
	coreDir       = "internal/core"
	adapterPrefix = "internal/adapter/"
)

// importEdge is one `core package → adapter package` edge. Kind distinguishes
// the three lists `go list` reports separately, because they are three
// different severities of the same violation: "build" leaks the adapter's types
// into core's own API and is what makes the port un-reversible; "test" and
// "xtest" only bind the core package's tests to a concrete adapter, which is
// cheaper to unwind but still blocks the type extraction, since a port whose
// signatures change breaks its fakes too.
type importEdge struct {
	Pkg     string // module-relative core package, e.g. "internal/core/agent"
	Kind    string // "build", "test" or "xtest"
	Adapter string // module-relative adapter package, e.g. "internal/adapter/postgres"
}

// knownViolations is a SHRINKING allowlist of the edges that already existed
// when this test was written. It exists so the test can be introduced without
// first paying for the fix, and so that no *new* edge can be added unnoticed
// while that fix is in flight.
//
// Two directions, both deliberate:
//
//   - Removing an entry is the expected direction of travel. Fix the edge —
//     define the type in core, have the adapter import core and implement the
//     port (internal/core/bid/ports.go is the shape to mirror) — then delete
//     the line. The test fails on a stale entry precisely so that the deletion
//     cannot be forgotten.
//   - Adding an entry is an architectural decision, not a build fix. It says
//     "we are knowingly taking on debt that the whole team has agreed to",
//     and it belongs in a PR description with a reason, not in a green CI run.
//
// The five entries below are not five independent problems. `postgres` already
// imports core/{auth,bid,clientprofile,document,tender,workbench,workspace},
// so reversing agent's or credits' port direction requires postgres to import
// them too — and `core/agent` imports `core/credits`, closing the cycle. They
// therefore have to come out together, in one atomic change, which is exactly
// why they are still here rather than already fixed.
var knownViolations = map[importEdge]string{
	{Pkg: "internal/core/agent", Kind: "build", Adapter: "internal/adapter/postgres"}:      "ChatRepository is defined in postgres.DBChatSession/DBChatMessage; unwinding it is atomic with credits below",
	{Pkg: "internal/core/agent", Kind: "test", Adapter: "internal/adapter/postgres"}:       "the agent tests build postgres.DBChatMessage fixtures; falls out with the build edge above",
	{Pkg: "internal/core/credits", Kind: "build", Adapter: "internal/adapter/postgres"}:    "CreditRepo/PricingRepo/UsageRepo are defined in postgres.DBWorkspaceCredits/DBAgentPricing; load-bearing for the agent edge",
	{Pkg: "internal/core/credits", Kind: "test", Adapter: "internal/adapter/postgres"}:     "the credits tests build postgres.DBWorkspaceCredits fixtures; falls out with the build edge above",
	{Pkg: "internal/core/tender/eval", Kind: "test", Adapter: "internal/adapter/postgres"}: "the search eval harness talks to a real database; the only entry with no build-time counterpart, so it can be unwound on its own",
}

// TestCoreDoesNotImportAdapters asserts that the observed set of
// core → adapter edges is exactly knownViolations: no new edge, and no entry
// that has quietly stopped applying.
func TestCoreDoesNotImportAdapters(t *testing.T) {
	edges := coreImportEdges(t)

	var added []importEdge
	for _, e := range edges {
		if _, ok := knownViolations[e]; !ok {
			added = append(added, e)
		}
	}
	sortEdges(added)

	for _, e := range added {
		t.Errorf(`import-boundary violation: %s imports %s (%s import).

%s must never import %s*. The domain layer owns its ports; the adapter
implements them and imports core, not the reverse — see
.claude/rules/code-organization.md ("The dependency rule") and
internal/core/bid/ports.go for the shape to copy: declare the interface and its
argument/return types in core, and let the adapter satisfy it.

Do NOT resolve this by adding %s to knownViolations in boundary_test.go.
That allowlist only shrinks; growing it is a deliberate, reviewed architectural
concession, not a way to get CI green.`,
			e.Pkg, e.Adapter, e.Kind, coreDir, adapterPrefix, e.Pkg)
	}

	observed := make(map[importEdge]bool, len(edges))
	for _, e := range edges {
		observed[e] = true
	}

	var stale []importEdge
	for e := range knownViolations {
		if !observed[e] {
			stale = append(stale, e)
		}
	}
	sortEdges(stale)

	for _, e := range stale {
		t.Errorf(`stale allowlist entry: knownViolations claims %s imports %s (%s import), but it no longer does.

Delete the entry from boundary_test.go. The allowlist is a debt register, and a
paid-off debt has to leave it — otherwise it silently re-authorizes the same
violation the next time someone reintroduces it.`,
			e.Pkg, e.Adapter, e.Kind)
	}
}

// coreImportEdges runs `go list` over every package under internal/core and
// returns the edges that point into internal/adapter. It reports build, test
// and external-test imports separately, since a package's *_test.go files can
// reach for an adapter the package itself does not.
func coreImportEdges(t *testing.T) []importEdge {
	t.Helper()

	// One line per import, so the output parses without a JSON decoder and
	// stays readable when a `go list` invocation has to be debugged by hand.
	const format = `{{$p := .ImportPath}}` +
		`{{range .Imports}}{{$p}} build {{.}}
{{end}}` +
		`{{range .TestImports}}{{$p}} test {{.}}
{{end}}` +
		`{{range .XTestImports}}{{$p}} xtest {{.}}
{{end}}`

	cmd := exec.Command(goTool(t), "list", "-f", format, "./"+coreDir+"/...")
	// The test binary runs with its package directory as the working
	// directory, which for `package main` is the module root — exactly where
	// the ./internal/core/... pattern resolves.
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list ./%s/...: %v\n%s", coreDir, err, stderr.String())
	}

	var (
		edges []importEdge
		pkgs  = map[string]bool{}
	)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 3 {
			continue
		}
		pkg, kind, imported := rel(fields[0]), fields[1], rel(fields[2])
		pkgs[pkg] = true
		if strings.HasPrefix(imported, adapterPrefix) {
			edges = append(edges, importEdge{Pkg: pkg, Kind: kind, Adapter: imported})
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading go list output: %v", err)
	}

	// Guard against the test passing vacuously. If a pattern typo, a build
	// failure swallowed by an empty result, or a module-path change ever made
	// `go list` report nothing under internal/core, an empty edge set would
	// look identical to a clean tree — and the allowlist check below would be
	// the only thing that noticed.
	if len(pkgs) == 0 {
		t.Fatalf("go list reported no packages under ./%s/...; the boundary check would pass vacuously", coreDir)
	}

	sortEdges(edges)
	return edges
}

// rel strips the module path so paths print module-relative. Anything outside
// this module (the standard library, third-party dependencies) is returned
// unchanged and simply never matches adapterPrefix.
func rel(importPath string) string {
	return strings.TrimPrefix(importPath, modulePath+"/")
}

// goTool locates the go binary. PATH is the normal answer — both `go test` from
// a shell and actions/setup-go in ci-backend.yml put it there — but a toolchain
// invoked by absolute path would not, so fall back to the GOROOT the test
// binary was itself built against.
func goTool(t *testing.T) string {
	t.Helper()

	if path, err := exec.LookPath("go"); err == nil {
		return path
	}
	fallback := filepath.Join(build.Default.GOROOT, "bin", "go")
	if _, err := os.Stat(fallback); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Fatalf("no go binary on PATH and none at %s; cannot check the core/adapter import boundary", fallback)
		}
		t.Fatalf("stat %s: %v", fallback, err)
	}
	return fallback
}

// sortEdges gives failure output a stable order, so a diff between two CI runs
// reflects a real change in the import graph rather than map iteration order.
func sortEdges(edges []importEdge) {
	sort.Slice(edges, func(i, j int) bool {
		a, b := edges[i], edges[j]
		if a.Pkg != b.Pkg {
			return a.Pkg < b.Pkg
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Adapter < b.Adapter
	})
}
