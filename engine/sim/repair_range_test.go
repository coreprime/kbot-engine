package sim

import (
	"testing"

	"github.com/coreprime/kbot-engine/engine/fixed"
	"github.com/coreprime/kbot-engine/engine/order"
)

// TestRepairRequiresBuildRange proves a repair heals nothing until the builder
// closes to within BuildDistance of the target — it drives the approach rather
// than nanolathing across the map.
func TestRepairRequiresBuildRange(t *testing.T) {
	w := New(Config{Seed: 104, Economy: EconomyTA})
	con := w.AddUnit("con", conMeta("con"), nil, fixed.Vec2{}, 0, 0)
	tgt := damagedDepot(w, fixed.Vec2{X: fixed.FromInt(500)}) // far out of range
	b := w.UnitByID(tgt)
	u := w.UnitByID(con)

	w.ApplyOrder(order.Repair(con, tgt))
	// Early ticks: the builder is still far away, so the hull must not climb.
	for i := 0; i < 10; i++ {
		w.Step(nil)
		if u.loco.Pos.DistTo(b.loco.Pos) > u.Meta.BuildDistance && b.Health > fixed.FromInt(50) {
			t.Fatalf("repair healed at %v wu, beyond BuildDistance %v",
				u.loco.Pos.DistTo(b.loco.Pos).Float(), u.Meta.BuildDistance.Float())
		}
	}
	if b.Health != fixed.FromInt(50) {
		t.Fatalf("target healed before the builder arrived (at %v%%)", b.Health.Float())
	}
	// Given time to walk in, the repair completes — and only from in range.
	for i := 0; i < 4000 && b.Health < fixed.FromInt(100); i++ {
		w.Step(nil)
	}
	if b.Health < fixed.FromInt(100) {
		t.Fatalf("repair never completed after the approach (at %v%%)", b.Health.Float())
	}
	if u.loco.Pos.DistTo(b.loco.Pos) > u.Meta.BuildDistance {
		t.Fatalf("builder finished the repair from out of range (%v wu)",
			u.loco.Pos.DistTo(b.loco.Pos).Float())
	}
}
