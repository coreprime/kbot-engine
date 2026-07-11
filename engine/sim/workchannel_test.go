package sim

import (
	"testing"

	"github.com/coreprime/kbot-engine/engine/fixed"
	"github.com/coreprime/kbot-engine/engine/order"
)

// conMeta is a versatile mobile construction unit that can build, repair,
// reclaim, capture and resurrect — the worst case for work-channel exclusivity.
func conMeta(name string) *UnitMeta {
	m := testMeta(name)
	m.IsBuilder = true
	m.CanReclaim = true
	m.CanResurrect = true
	m.CanCapture = true
	setWorkerTime(m, 200)
	m.BuildDistance = fixed.FromInt(60)
	return m
}

// activeWorkChannels counts how many of a unit's mutually-exclusive work
// channels are live. It must never exceed one after any arming order.
func activeWorkChannels(u *Unit) int {
	n := 0
	if u.repairTarget != 0 {
		n++
	}
	if u.capTarget != 0 {
		n++
	}
	if u.reclaimTarget != 0 {
		n++
	}
	if u.reclaimFeature != 0 {
		n++
	}
	if u.resurrectFeature != 0 {
		n++
	}
	if u.buildState != buildIdle {
		n++
	}
	return n
}

// damagedDepot adds a fully-built, wounded friendly repair target at pos.
func damagedDepot(w *World, pos fixed.Vec2) uint32 {
	m := testMeta("depot")
	m.CanMove = false
	m.MaxHealth = fixed.FromInt(100)
	setBuildStats(m, 100, 300, 200)
	id := w.AddUnit("depot", m, nil, pos, 0, 0)
	w.UnitByID(id).Health = fixed.FromInt(50)
	return id
}

// enemyTank adds a plain enemy unit (a valid reclaim/capture target — no
// cancapture flag of its own).
func enemyTank(w *World, pos fixed.Vec2) uint32 {
	m := testMeta("tank")
	m.MaxHealth = fixed.FromInt(100)
	return w.AddUnit("tank", m, nil, pos, 0, 1)
}

// TestRepairThenReclaimSingleChannel proves a reclaim order issued to a builder
// already repairing clears the repair channel — the unit ends with exactly one
// active work channel (the reclaim), never healing and reclaiming at once.
func TestRepairThenReclaimSingleChannel(t *testing.T) {
	w := New(Config{Seed: 100, Economy: EconomyTA})
	con := w.AddUnit("con", conMeta("con"), nil, fixed.Vec2{}, 0, 0)
	depot := damagedDepot(w, fixed.Vec2{X: fixed.FromInt(40)})
	en := enemyTank(w, fixed.Vec2{X: fixed.FromInt(40), Z: fixed.FromInt(40)})
	u := w.UnitByID(con)

	w.ApplyOrder(order.Repair(con, depot))
	if u.repairTarget != depot {
		t.Fatalf("repair did not arm (repairTarget=%d)", u.repairTarget)
	}
	w.ApplyOrder(order.Reclaim([]uint32{con}, en))
	if n := activeWorkChannels(u); n != 1 {
		t.Fatalf("expected exactly 1 work channel after reclaim, got %d", n)
	}
	if u.reclaimTarget != en {
		t.Fatalf("reclaim channel not armed (reclaimTarget=%d)", u.reclaimTarget)
	}
	if u.repairTarget != 0 {
		t.Fatalf("repair channel survived the reclaim order (repairTarget=%d)", u.repairTarget)
	}
}

// TestBuildThenReclaimSingleChannel proves a reclaim order issued to a builder
// mid-build cancels the build (its buildee stops advancing) and leaves only the
// reclaim channel live.
func TestBuildThenReclaimSingleChannel(t *testing.T) {
	spawn := func(name string) (*UnitMeta, Binding) {
		m := testMeta(name)
		setBuildStats(m, 800, 300, 30)
		return m, nil
	}
	w := New(Config{Seed: 101, Economy: EconomyTA, Spawn: spawn})
	con := w.AddUnit("con", conMeta("con"), nil, fixed.Vec2{}, 0, 0)
	en := enemyTank(w, fixed.Vec2{X: fixed.FromInt(40), Z: fixed.FromInt(40)})
	u := w.UnitByID(con)

	w.ApplyOrder(order.Build(con, "solar", fixed.Vec2{X: fixed.FromInt(60)}, 0))
	if u.buildState == buildIdle {
		t.Fatalf("build did not arm (buildState=%d)", u.buildState)
	}
	w.ApplyOrder(order.Reclaim([]uint32{con}, en))
	if n := activeWorkChannels(u); n != 1 {
		t.Fatalf("expected exactly 1 work channel after reclaim, got %d", n)
	}
	if u.reclaimTarget != en {
		t.Fatalf("reclaim channel not armed (reclaimTarget=%d)", u.reclaimTarget)
	}
	if u.buildState != buildIdle {
		t.Fatalf("build channel survived the reclaim order (buildState=%d)", u.buildState)
	}
}

// TestResurrectThenReclaimSingleChannel proves a reclaim order issued to a
// builder mid-resurrect clears the resurrect channel, leaving only the reclaim.
func TestResurrectThenReclaimSingleChannel(t *testing.T) {
	victim := testMeta("tank")
	victim.MaxHealth = fixed.FromInt(100)
	victim.Econ.BuildCostMetal = 60
	applyWreckWfrom(victim)
	w := New(Config{Seed: 102, Economy: EconomyTA})
	w.spawn = func(name string) (*UnitMeta, Binding) { return victim, nil }
	vid := w.AddUnit("tank", victim, nil, fixed.Vec2{X: fixed.FromInt(40)}, 0, 1)
	w.killUnit(w.UnitByID(vid), 100, Blast{})
	w.Step(nil) // flush the corpse decision -> wreck
	var wreck *Feature
	for _, id := range w.featureOrder {
		wreck = w.features[id]
	}
	if wreck == nil {
		t.Fatal("no wreck to resurrect")
	}
	con := w.AddUnit("con", conMeta("con"), nil, fixed.Vec2{}, 0, 0)
	en := enemyTank(w, fixed.Vec2{X: fixed.FromInt(40), Z: fixed.FromInt(40)})
	u := w.UnitByID(con)

	w.ApplyResurrect(con, wreck.ID, 100)
	if u.resurrectFeature != wreck.ID {
		t.Fatalf("resurrect did not arm (resurrectFeature=%d)", u.resurrectFeature)
	}
	w.ApplyOrder(order.Reclaim([]uint32{con}, en))
	if n := activeWorkChannels(u); n != 1 {
		t.Fatalf("expected exactly 1 work channel after reclaim, got %d", n)
	}
	if u.reclaimTarget != en {
		t.Fatalf("reclaim channel not armed (reclaimTarget=%d)", u.reclaimTarget)
	}
	if u.resurrectFeature != 0 {
		t.Fatalf("resurrect channel survived the reclaim order (resurrectFeature=%d)", u.resurrectFeature)
	}
}

// TestDeterministicReclaimRepairMexToggle is a lockstep check: two worlds fed
// the identical ordered command stream — reclaim, repair, and the RNG-drawing
// metal-maker auto-toggle — must stay world-hash-equal every tick.
func TestDeterministicReclaimRepairMexToggle(t *testing.T) {
	build := func() *World {
		w := New(Config{Seed: 200, Economy: EconomyTA, StartMetal: -1})
		con := w.AddUnit("con", conMeta("con"), nil, fixed.Vec2{}, 0, 0)
		depot := damagedDepot(w, fixed.Vec2{X: fixed.FromInt(80)})
		en := enemyTank(w, fixed.Vec2{X: fixed.FromInt(300)})
		w.AddUnit("maker", makerMeta(), nil, fixed.Vec2{X: fixed.FromInt(-80)}, 0, 0)
		con2 := w.AddUnit("con2", conMeta("con2"), nil, fixed.Vec2{Z: fixed.FromInt(40)}, 0, 0)
		// One con reclaims the enemy, the other walks in and repairs the depot.
		w.ApplyOrder(order.Reclaim([]uint32{con}, en))
		w.ApplyOrder(order.Repair(con2, depot))
		return w
	}
	w1, w2 := build(), build()
	for i := 0; i < 600; i++ {
		w1.Step(nil)
		w2.Step(nil)
		if h1, h2 := w1.Hash(), w2.Hash(); h1 != h2 {
			t.Fatalf("world hash diverged at tick %d: %d != %d", i+1, h1, h2)
		}
	}
}
