package sim

import (
	"testing"

	"github.com/coreprime/kbot-engine/engine/fixed"
	"github.com/coreprime/kbot-engine/engine/order"
)

// TestWorkingTrueDuringResurrect proves a builder that is only resurrecting
// reports working()==true, so a shift-queued order waits behind it instead of
// arming immediately (world.go working() gate).
func TestWorkingTrueDuringResurrect(t *testing.T) {
	victim := testMeta("tank")
	victim.MaxHealth = fixed.FromInt(100)
	victim.Econ.BuildCostMetal = 60
	applyWreckWfrom(victim)
	w := New(Config{Seed: 103, Economy: EconomyTA})
	w.spawn = func(name string) (*UnitMeta, Binding) { return victim, nil }
	vid := w.AddUnit("tank", victim, nil, fixed.Vec2{X: fixed.FromInt(40)}, 0, 1)
	w.killUnit(w.UnitByID(vid), 100, Blast{})
	w.Step(nil)
	var wreck *Feature
	for _, id := range w.featureOrder {
		wreck = w.features[id]
	}
	con := w.AddUnit("con", conMeta("con"), nil, fixed.Vec2{}, 0, 0)
	u := w.UnitByID(con)

	w.ApplyResurrect(con, wreck.ID, 5000) // long channel so it stays busy
	if u.resurrectFeature == 0 {
		t.Fatal("resurrect did not arm")
	}
	if !u.working() {
		t.Fatalf("a resurrecting builder must report working()==true")
	}
	// A shift-queued reclaim must therefore wait behind the resurrect.
	en := enemyTank(w, fixed.Vec2{X: fixed.FromInt(40), Z: fixed.FromInt(40)})
	w.ApplyOrder(order.ReclaimQueued([]uint32{con}, en))
	if len(u.queue) != 1 {
		t.Fatalf("queued reclaim was not deferred behind the resurrect (queue=%d)", len(u.queue))
	}
	if u.reclaimTarget != 0 {
		t.Fatalf("queued reclaim armed immediately during a resurrect")
	}
}
