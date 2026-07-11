package sim

import (
	"testing"

	"github.com/coreprime/kbot-engine/engine/fixed"
)

// producerMeta builds a structure with passive EnergyMake income (no active
// gate, no demand) for the difficulty-scaling tests.
func producerMeta(energyMake float32) *UnitMeta {
	m := &UnitMeta{Name: "producer", CanMove: false, MaxHealth: fixed.FromInt(100)}
	m.Econ.EnergyMake = energyMake
	return m
}

// TestDifficultyIncomeMul pins the exact multiplier doubles.
func TestDifficultyIncomeMul(t *testing.T) {
	if got := difficultyIncomeMul(DifficultyEasy); got != 0.5 {
		t.Fatalf("easy mul = %v, want 0.5", got)
	}
	if got := difficultyIncomeMul(DifficultyMedium); got != 0.7 {
		t.Fatalf("medium mul = %v, want 0.7 (0x3FE6666666666666)", got)
	}
	if got := difficultyIncomeMul(DifficultyHard); got != 1 {
		t.Fatalf("hard mul = %v, want 1", got)
	}
	if got := difficultyIncomeMul(99); got != 1 {
		t.Fatalf("unknown difficulty mul = %v, want 1 (unscaled)", got)
	}
}

// TestAIDifficultyScalesProduction runs one settle for an AI side at Medium and
// a human side, and checks the produced accumulators against the exact float
// products — the 0.7 handicap is applied bit-for-bit.
func TestAIDifficultyScalesProduction(t *testing.T) {
	w := New(Config{Seed: 1, AIDifficulty: map[int]int{0: DifficultyMedium}})
	w.AddUnit("producer", producerMeta(1000), nil, fixed.Vec2{}, 0, 0)                      // AI side 0
	w.AddUnit("producer", producerMeta(1000), nil, fixed.Vec2{X: fixed.FromInt(400)}, 0, 1) // human side 1
	for i := 0; i < taSettleTicks; i++ {
		w.Step(nil)
	}
	// One settle: AI side credits 1000 * 0.7 (exact double), human credits 1000.
	wantAI := 1000.0 * aiIncomeMulMedium
	if got := w.econTA[0].producedE; got != wantAI {
		t.Fatalf("AI produced = %v, want %v", got, wantAI)
	}
	if got := w.econTA[1].producedE; got != 1000.0 {
		t.Fatalf("human produced = %v, want 1000", got)
	}
}
