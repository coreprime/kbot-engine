package sandboxverify

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/coreprime/kbot-io/testutil"
)

// ciSubset names the scenarios the CI parity gate runs against the committed
// fixture tree (testdata/fixtures/ta) with no game install. Every scenario in
// the list references only armflash and armmex — the two units whose real FBI,
// COB, movement class, and (for armflash) weapon are embedded as fixtures — so
// the whole subset builds its worlds from repo-local bytes. The subset spans
// four mechanics systems: locomotion (the bang-bang integrator, braking,
// slope, turning), economy (extractor income, storage cap, energy-stall gate),
// specials (the paralyze accumulator cap and decay), and combat (burst-spray
// RNG accounting).
var ciSubset = []string{
	"ta-combat-spray",
	"ta-econ-energy-stall-mex",
	"ta-econ-mex-income",
	"ta-econ-storage-cap",
	"ta-loco-accel",
	"ta-loco-brake",
	"ta-loco-brake-decision",
	"ta-loco-moveinfo-slope",
	"ta-loco-slope",
	"ta-loco-turn",
	"ta-loco-turn-thrust",
	"ta-loco-underwater",
	"ta-paralyze-cap",
}

// scenariosDir resolves the repo's scenario directory relative to this test.
func scenariosDir(t *testing.T) string {
	t.Helper()
	_, self, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(self), "..", "scenarios", "sandbox")
}

// fixtureTARoot resolves the embedded TA fixture root (a minimal flattened
// install: the handful of real unit files the CI subset loads).
func fixtureTARoot(t *testing.T) string {
	t.Helper()
	_, self, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(self), "testdata", "fixtures", "ta")
}

// goldenCheck is the deterministic projection of one graded check that the CI
// gate freezes: it carries the tick-exact simulated Actual alongside the
// spec-derived Expect, so any drift in the sim changes Actual and breaks the
// byte comparison.
type goldenCheck struct {
	Label      string  `json:"label"`
	Observable string  `json:"observable"`
	SpecTick   int     `json:"specTick"`
	SimTick    uint64  `json:"simTick"`
	SkewMs     int64   `json:"skewMs"`
	Expect     int64   `json:"expect"`
	Actual     int64   `json:"actual"`
	Delta      int64   `json:"delta"`
	Verdict    Verdict `json:"verdict"`
}

// goldenScenario is one scenario's frozen result set.
type goldenScenario struct {
	Name   string        `json:"name"`
	Game   string        `json:"game"`
	System string        `json:"system"`
	Error  string        `json:"error,omitempty"`
	Checks []goldenCheck `json:"checks"`
}

// project folds a report into the golden shape, dropping the volatile fields
// (generation time, absolute asset roots) so the artifact is reproducible on
// any machine.
func project(rep *Report) []goldenScenario {
	out := make([]goldenScenario, 0, len(rep.Scenarios))
	for _, s := range rep.Scenarios {
		gs := goldenScenario{Name: s.Name, Game: s.Game, System: s.System, Error: s.Error}
		for _, c := range s.Checks {
			gs.Checks = append(gs.Checks, goldenCheck{
				Label:      c.Label,
				Observable: c.Observable,
				SpecTick:   c.SpecTick,
				SimTick:    c.SimTick,
				SkewMs:     c.SkewMs,
				Expect:     c.Expect,
				Actual:     c.Actual,
				Delta:      c.Delta,
				Verdict:    c.Verdict,
			})
		}
		out = append(out, gs)
	}
	return out
}

// runSubset loads and grades the CI subset against the embedded fixtures.
func runSubset(t *testing.T) *Report {
	t.Helper()
	dir := scenariosDir(t)
	var scenarios []*Scenario
	for _, name := range ciSubset {
		s, err := Load(filepath.Join(dir, name+".yaml"))
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		scenarios = append(scenarios, s)
	}
	runner := &Runner{TARoot: fixtureTARoot(t)}
	return runner.Run(scenarios)
}

// TestParityCISubset is the permanent, install-free CI gate. It builds the
// covered scenarios from the embedded fixture tree, grades them tick-exact, and
// asserts the result matches the committed golden byte-for-byte. Any engine
// change that shifts a simulated value on a covered mechanic breaks this test.
// Regenerate the golden after an intended change with UPDATE_GOLDEN=1.
func TestParityCISubset(t *testing.T) {
	rep := runSubset(t)

	for _, s := range rep.Scenarios {
		if s.Error != "" {
			t.Fatalf("scenario %s errored (fixtures incomplete?): %s", s.Name, s.Error)
		}
		for _, c := range s.Checks {
			if c.Verdict != VerdictFaithful {
				t.Errorf("%s / %s graded %s (expect %d, actual %d) — sim drifted from spec",
					s.Name, c.Observable, c.Verdict, c.Expect, c.Actual)
			}
		}
	}

	got, err := json.MarshalIndent(project(rep), "", "  ")
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	got = append(got, '\n')

	goldenPath := filepath.Join(func() string {
		_, self, _, _ := runtime.Caller(0)
		return filepath.Dir(self)
	}(), "golden", "ci-subset.json")

	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("golden regenerated: %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with UPDATE_GOLDEN=1 to create): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("CI parity golden mismatch: the sim no longer reproduces the frozen\n"+
			"tick-exact outputs. If this change is intended, regenerate with\n"+
			"UPDATE_GOLDEN=1 go test ./sandboxverify/... and review the diff.\n\n"+
			"got:\n%s", got)
	}
}

// TestParityFull runs the complete gap matrix against real flattened installs.
// It needs both TA and TA:K assets; with ALLOW_SKIP_ASSETS=true (the CI
// setting) it skips cleanly, so CI relies on TestParityCISubset for the gate
// and this test carries the full 91-check picture locally.
func TestParityFull(t *testing.T) {
	taRoot := testutil.UnpackedPath(t)
	takRoot := testutil.TAKUnpackedPath(t)

	scenarios, err := LoadDir(scenariosDir(t))
	if err != nil {
		t.Fatalf("load scenarios: %v", err)
	}
	runner := &Runner{TARoot: taRoot, TAKRoot: takRoot}
	rep := runner.Run(scenarios)

	var faithful, wrong, missing, cosmetic int
	for _, s := range rep.Scenarios {
		if s.Error != "" {
			t.Errorf("scenario %s errored: %s", s.Name, s.Error)
		}
		for _, c := range s.Checks {
			switch c.Verdict {
			case VerdictFaithful:
				faithful++
			case VerdictWrong:
				wrong++
				t.Errorf("%s / %s WRONG: expect %d, actual %d", s.Name, c.Observable, c.Expect, c.Actual)
			case VerdictMissing:
				missing++
				t.Errorf("%s / %s MISSING: %s", s.Name, c.Observable, c.Note)
			case VerdictCosmeticGap:
				cosmetic++
			}
		}
	}
	t.Logf("full parity: %d faithful, %d wrong, %d missing, %d cosmetic-gap",
		faithful, wrong, missing, cosmetic)
	if wrong != 0 || missing != 0 || cosmetic != 0 {
		t.Fatalf("full parity regressed: want all faithful, got %d wrong / %d missing / %d cosmetic",
			wrong, missing, cosmetic)
	}
}
