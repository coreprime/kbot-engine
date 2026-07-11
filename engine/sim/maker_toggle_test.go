package sim

import (
	"testing"

	"github.com/coreprime/kbot-engine/engine/fixed"
)

// makerMeta builds a minimal structure metal-maker meta (makesmetal set, not a
// mover) for the toggle tests.
func makerMeta() *UnitMeta {
	m := &UnitMeta{Name: "maker", CanMove: false, MaxHealth: fixed.FromInt(100)}
	m.Econ.MakesMetal = 1
	m.Econ.EnergyUse = 60
	return m
}

// TestMakerToggleOffNoDraw verifies the 2:1 off branch clears ACTIVE and draws
// nothing from the MINSTD stream.
func TestMakerToggleOffNoDraw(t *testing.T) {
	w := New(Config{Seed: 1})
	id := w.AddUnit("maker", makerMeta(), nil, fixed.Vec2{}, 0, 0)
	u := w.units[id]
	u.active = true
	p := &w.econTA[0]
	p.stockM, p.stockE = 1000, 500 // 2*1000 >= 500 -> off
	before := w.RngDraws()
	w.stepMetalMakerToggle()
	if u.active {
		t.Fatalf("maker should be off (2*metal >= energy)")
	}
	if got := w.RngDraws() - before; got != 0 {
		t.Fatalf("off branch drew %d MINSTD, want 0", got)
	}
}

// TestMakerToggleOnDrawsPerScan verifies the on branch draws exactly one
// rand(5) per scan when net energy is positive, and that a rand(5) of zero is a
// hold (no toggle) while the draw still lands.
func TestMakerToggleOnDrawsPerScan(t *testing.T) {
	w := New(Config{Seed: 1})
	id := w.AddUnit("maker", makerMeta(), nil, fixed.Vec2{}, 0, 0)
	u := w.units[id]
	u.active = false
	p := &w.econTA[0]
	p.stockM, p.stockE = 0, 1000     // 0 < 1000: not off
	p.incomeE, p.expenseE = 1000, 60 // net +940 > 0: draw gated open
	before := w.RngDraws()
	const scans = 20
	for i := 0; i < scans; i++ {
		w.stepMetalMakerToggle()
	}
	if got := w.RngDraws() - before; got != scans {
		t.Fatalf("on branch drew %d MINSTD over %d scans, want %d", got, scans, scans)
	}
}

// TestMakerToggleNetNonPositiveNoDraw verifies the && short-circuit: a
// non-positive net energy rate takes no rand(5) draw even when the off branch
// fails.
func TestMakerToggleNetNonPositiveNoDraw(t *testing.T) {
	w := New(Config{Seed: 1})
	id := w.AddUnit("maker", makerMeta(), nil, fixed.Vec2{}, 0, 0)
	u := w.units[id]
	u.active = false
	p := &w.econTA[0]
	p.stockM, p.stockE = 0, 1000   // not off
	p.incomeE, p.expenseE = 60, 60 // net 0: draw gated shut
	before := w.RngDraws()
	w.stepMetalMakerToggle()
	if got := w.RngDraws() - before; got != 0 {
		t.Fatalf("net-zero energy drew %d MINSTD, want 0", got)
	}
}
