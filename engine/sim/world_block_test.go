package sim

import (
	"testing"

	"github.com/coreprime/kbot-engine/engine/fixed"
)

// gridTerrain builds a flat w×h cell grid for the ambient-world tests.
func gridTerrain(w, h int) *Terrain {
	return &Terrain{
		W: w, H: h, CellWU: fixed.FromInt(16), HeightScale: fixed.FromFloat(0.5),
		Data: make([]uint8, w*h),
	}
}

// TestFeatureReproductionPlacesOffspring verifies the reproduction scan grows a
// reproducing feature into a cleared cell, drawing the shared MINSTD stream.
func TestFeatureReproductionPlacesOffspring(t *testing.T) {
	w := New(Config{Seed: 1})
	w.SetTerrain(gridTerrain(8, 8))
	meta := &FeatureMeta{Name: "tree", FootprintX: 1, FootprintZ: 1, Reproduce: 100, ReproduceArea: 3}
	// Cell (4,4): centre world point (72,72).
	w.AddFeature("tree", meta, FeatureProp, fixed.Vec2{X: fixed.FromInt(72), Z: fixed.FromInt(72)}, 0, -1)
	before := w.RngDraws()
	for i := 0; i < 30; i++ {
		w.Step(nil)
	}
	if got := w.FeatureCount(); got != 2 {
		t.Fatalf("feature count = %d, want 2 (offspring placed)", got)
	}
	// The scan drew rand(100) + two offsets once, at the frame the cursor hit
	// the tree's cell.
	if got := w.RngDraws() - before; got != 3 {
		t.Fatalf("reproduction drew %d MINSTD, want 3", got)
	}
}

// TestFeatureReproductionCalmMap verifies an empty (feature-less) map takes no
// reproduction draws.
func TestFeatureReproductionCalmMap(t *testing.T) {
	w := New(Config{Seed: 1})
	w.SetTerrain(gridTerrain(8, 8))
	before := w.RngDraws()
	for i := 0; i < 100; i++ {
		w.Step(nil)
	}
	if got := w.RngDraws() - before; got != 0 {
		t.Fatalf("feature-less map drew %d MINSTD, want 0", got)
	}
}

// TestMeteorCadence verifies the strike-begin dead-draw cadence: once at game
// start, then every warmup+duration ticks, even with meteors disabled.
func TestMeteorCadence(t *testing.T) {
	w := New(Config{Seed: 1})
	w.Step(nil) // frame 1
	if got := w.MeteorStrikes(); got != 1 {
		t.Fatalf("strikes at frame 1 = %d, want 1 (game-start dead draws)", got)
	}
	if got := w.MeteorImpacts(); got != 0 {
		t.Fatalf("impacts with meteors disabled = %d, want 0", got)
	}
	// The CRT stream advanced by the wind interval (1) plus the four meteor
	// dead draws at frame 1.
	if got := w.CrtDraws(); got != 5 {
		t.Fatalf("CRT draws at frame 1 = %d, want 5 (wind 1 + meteor 4)", got)
	}
	period := uint64(meteorDefaultWarmupTicks + meteorDefaultDurationTicks)
	for w.Tick() < period-1 {
		w.Step(nil)
	}
	if got := w.MeteorStrikes(); got != 1 {
		t.Fatalf("strikes before the next period = %d, want 1", got)
	}
	w.Step(nil) // the period boundary
	if got := w.MeteorStrikes(); got != 2 {
		t.Fatalf("strikes at the next period = %d, want 2", got)
	}
}

// TestMeteorImpactsWhenEnabled verifies an enabled meteor shower opens an active
// window whose impacts each draw two CRT values on the interval.
func TestMeteorImpactsWhenEnabled(t *testing.T) {
	w := New(Config{Seed: 1, MeteorWeather: true})
	w.Step(nil) // frame 1: strike-begin + first impact
	if got := w.MeteorStrikes(); got != 1 {
		t.Fatalf("strikes = %d, want 1", got)
	}
	if got := w.MeteorImpacts(); got != 1 {
		t.Fatalf("impacts at frame 1 = %d, want 1 (first impact fires on strike)", got)
	}
	// Run out the active window; impacts fire every intervalTicks.
	for w.Tick() < uint64(meteorDefaultDurationTicks)+2 {
		w.Step(nil)
	}
	// Impacts land at frames 1, 1+15, 1+30, ... while the window is open
	// (duration 150 ticks): frames 1..151 -> 11 impacts.
	if got := w.MeteorImpacts(); got != 11 {
		t.Fatalf("impacts over the active window = %d, want 11", got)
	}
}
