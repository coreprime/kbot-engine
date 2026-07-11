package sim

import (
	"testing"

	"github.com/coreprime/kbot-engine/engine/fixed"
	"github.com/coreprime/kbot-engine/engine/order"
)

// seedShareWorld builds a TA world with a seeded pool on two sides (an inert
// producer per side) and the given AI config.
func seedShareWorld(t *testing.T, ai map[int]int) *World {
	t.Helper()
	// Start below the max(start,200) storage floor so the recipient has cap
	// headroom for the transfer to land in.
	w := New(Config{Seed: 1, StartMetal: 100, StartEnergy: 100, AIDifficulty: ai})
	w.AddUnit("a", producerMeta(0), nil, fixed.Vec2{}, 0, 0)
	w.AddUnit("b", producerMeta(0), nil, fixed.Vec2{X: fixed.FromInt(400)}, 0, 1)
	return w
}

// TestShareDebitsAndCredits checks the donor is debited at once and the
// recipient lands the transfer at the next settle.
func TestShareDebitsAndCredits(t *testing.T) {
	w := seedShareWorld(t, nil)
	w.ApplyOrder(order.Share(0, 1, 50, 0))
	if got := w.econTA[0].stockM; got != 50 {
		t.Fatalf("donor metal = %v, want 50 (debited at once)", got)
	}
	if got := w.econTA[1].stockM; got != 100 {
		t.Fatalf("recipient metal = %v, want 100 before settle", got)
	}
	for i := 0; i < taSettleTicks; i++ {
		w.Step(nil)
	}
	if got := w.econTA[1].stockM; got != 150 {
		t.Fatalf("recipient metal = %v, want 150 after settle", got)
	}
}

// TestShareClampsToHoldings verifies a side cannot give more than it holds.
func TestShareClampsToHoldings(t *testing.T) {
	w := seedShareWorld(t, nil)
	w.ApplyOrder(order.Share(0, 1, 9000, 0)) // asks for far more than 100
	if got := w.econTA[0].stockM; got != 0 {
		t.Fatalf("donor metal = %v, want 0 (clamped to holdings)", got)
	}
	for i := 0; i < taSettleTicks; i++ {
		w.Step(nil)
	}
	// The donor could give only its 100; the recipient (starting at 100, cap
	// 200) lands exactly that.
	if got := w.econTA[1].stockM; got != 200 {
		t.Fatalf("recipient metal = %v, want 200 (received the donor's 100)", got)
	}
}

// TestShareToAIScaled verifies a gift to an AI ally is difficulty-scaled — the
// transfer folds into the receiver's production, which the handicap scales.
func TestShareToAIScaled(t *testing.T) {
	w := seedShareWorld(t, map[int]int{1: DifficultyEasy}) // side 1 is Easy AI (x0.5)
	w.ApplyOrder(order.Share(0, 1, 100, 0))
	for i := 0; i < taSettleTicks; i++ {
		w.Step(nil)
	}
	// 100 transferred, scaled x0.5 -> 50 credited to the AI recipient
	// (100 stored + 50 = 150).
	if got := w.econTA[1].stockM; got != 150 {
		t.Fatalf("AI recipient metal = %v, want 150 (100 gift scaled to 50)", got)
	}
}

// TestShareTAKNoOp verifies the transfer path is inert under the single-pool
// TA:K economy.
func TestShareTAKNoOp(t *testing.T) {
	w := New(Config{Seed: 1, Economy: EconomyTAK})
	w.ApplyOrder(order.Share(0, 1, 100, 0)) // no TA pools to move; must not panic
	if w.xferProdM[1] != 0 {
		t.Fatalf("TA:K share credited %v, want 0", w.xferProdM[1])
	}
}
