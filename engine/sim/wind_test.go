package sim

import (
	"testing"

	"github.com/coreprime/kbot-engine/engine/fixed"
)

// TestWindReRollFixedRange verifies a pinned wind range re-rolls a constant
// speed with no MINSTD speed draw, and that the strength normalizes to
// speed/5000.
func TestWindReRollFixedRange(t *testing.T) {
	w := New(Config{Seed: 1, MinWind: 2500, MaxWind: 2500})
	drawsBefore := w.RngDraws()
	w.Step(nil) // tick 1: first re-roll
	if w.WindSpeed() != 2500 {
		t.Fatalf("wind speed = %d, want 2500", w.WindSpeed())
	}
	if got := w.WindStrengthMilli(); got != 500 {
		t.Fatalf("wind strength milli = %d, want 500", got)
	}
	// A pinned range (span 0) draws nothing for speed; only the heading draw
	// consumes the MINSTD stream on the roll.
	if got := w.RngDraws() - drawsBefore; got != 1 {
		t.Fatalf("MINSTD draws on first re-roll = %d, want 1 (heading only)", got)
	}
}

// TestWindCalmNoDraw verifies a calm world (speed 0) takes no heading draw and
// produces no drift.
func TestWindCalmNoDraw(t *testing.T) {
	w := New(Config{Seed: 1}) // MinWind == MaxWind == 0
	drawsBefore := w.RngDraws()
	w.Step(nil)
	if w.WindSpeed() != 0 {
		t.Fatalf("calm wind speed = %d, want 0", w.WindSpeed())
	}
	if w.wind.driftX != 0 || w.wind.driftZ != 0 {
		t.Fatalf("calm wind drift = (%d,%d), want zero", w.wind.driftX, w.wind.driftZ)
	}
	if got := w.RngDraws() - drawsBefore; got != 0 {
		t.Fatalf("calm re-roll drew %d MINSTD, want 0 (speed 0 skips heading)", got)
	}
}

// TestWindDriftVector checks the per-tick drift components match the
// −2·sin/cos(heading)·(speed/5000) formula for a known heading.
func TestWindDriftVector(t *testing.T) {
	w := New(Config{Seed: 1, MinWind: 2500, MaxWind: 2500})
	w.wind.speed = 2500
	w.wind.heading = fixed.FullCircle / 4 // 90°: sin=1, cos=0
	w.recomputeWind()
	// scale = -2 * (2500/5000) = -1.0; driftX = sin(90°)*(-1) = -1 world unit.
	wantX := fixed.SinScaled(fixed.FullCircle/4, fixed.FromFloat(-1))
	wantZ := fixed.CosScaled(fixed.FullCircle/4, fixed.FromFloat(-1))
	if w.wind.driftX != wantX || w.wind.driftZ != wantZ {
		t.Fatalf("drift = (%d,%d), want (%d,%d)", w.wind.driftX, w.wind.driftZ, wantX, wantZ)
	}
	if w.wind.driftX >= 0 {
		t.Fatalf("expected downwind -X drift, got %d", w.wind.driftX)
	}
}

// TestWindDeterministic confirms two worlds with the same seed roll identical
// wind over many ticks (the lockstep contract).
func TestWindDeterministic(t *testing.T) {
	run := func() (int32, int32) {
		w := New(Config{Seed: 42, MinWind: 100, MaxWind: 2000})
		for i := 0; i < 500; i++ {
			w.Step(nil)
		}
		return w.WindSpeed(), w.WindHeading()
	}
	s1, h1 := run()
	s2, h2 := run()
	if s1 != s2 || h1 != h2 {
		t.Fatalf("wind diverged: (%d,%d) != (%d,%d)", s1, h1, s2, h2)
	}
}
