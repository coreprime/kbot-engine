package sim

import (
	"testing"

	"github.com/coreprime/kbot-engine/engine/fixed"
	"github.com/coreprime/kbot-engine/engine/order"
)

// TestReclaimRequiresBuildRange proves a reclaim salvages nothing until the
// builder closes to within BuildDistance of the target — it drives the approach
// rather than consuming the target across the map (the same in-range gate the
// repair channel runs). The target is same-side so only the reclaim channel,
// never combat, accounts for its removal.
func TestReclaimRequiresBuildRange(t *testing.T) {
	w := New(Config{Seed: 105, Economy: EconomyTA})
	con := w.AddUnit("con", conMeta("con"), nil, fixed.Vec2{}, 0, 0)
	scrap := testMeta("scrap")
	scrap.MaxHealth = fixed.FromInt(100)
	tgt := w.AddUnit("scrap", scrap, nil, fixed.Vec2{X: fixed.FromInt(500)}, 0, 0) // far out of range
	b := w.UnitByID(tgt)
	u := w.UnitByID(con)

	w.ApplyOrder(order.Reclaim([]uint32{con}, tgt))
	// The target must never take a salvage pulse while the reclaimer is out of
	// build range — the drain only bites once it has walked in. Watch every tick.
	for i := 0; i < 4000 && !b.Dead; i++ {
		w.Step(nil)
		if b.Health < fixed.FromInt(100) && u.loco.Pos.DistTo(b.loco.Pos) > u.Meta.BuildDistance {
			t.Fatalf("reclaim drained the target from %v wu, beyond BuildDistance %v",
				u.loco.Pos.DistTo(b.loco.Pos).Float(), u.Meta.BuildDistance.Float())
		}
	}
	if !b.Dead {
		t.Fatalf("reclaim never consumed the target after the approach (at %v%%)", b.Health.Float())
	}
	if u.loco.Pos.DistTo(b.loco.Pos) > u.Meta.BuildDistance {
		t.Fatalf("reclaimer finished the salvage from out of range (%v wu)",
			u.loco.Pos.DistTo(b.loco.Pos).Float())
	}
}
