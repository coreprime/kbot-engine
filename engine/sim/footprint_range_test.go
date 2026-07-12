package sim

import (
	"testing"

	"github.com/coreprime/kbot-engine/engine/fixed"
	"github.com/coreprime/kbot-engine/engine/order"
)

// bigStructWorld spawns a large-footprint structure ("bigkeep", 9×9) plus the
// small "solar" the other build tests use, so a resume approach must reckon
// with a footprint wider than the builder's BuildDistance.
func bigStructWorld() *World {
	spawn := func(name string) (*UnitMeta, Binding) {
		m := testMeta(name)
		if name == "bigkeep" {
			m.CanMove = false
			m.FootprintX, m.FootprintZ = 9, 9
			setBuildStats(m, 600, 300, 300)
			return m, nil
		}
		setBuildStats(m, 800, 300, 30)
		return m, nil
	}
	w := New(Config{Seed: 21, Spawn: spawn, Economy: EconomyTA})
	w.SetTerrain(testTerrain(80, 80, 0, func(_, _ int) uint8 { return 0 }))
	return w
}

// TestBuildResumeStopsOutsideLargeFootprint pins the build-resume approach: a
// builder ordered to RESUME an existing large under-construction structure must
// stop at the footprint EDGE plus its BuildDistance and nanolathe from there —
// never walking into (or grinding against) the building's footprint because the
// approach chased a centre buried inside it. Regression for the "resume walks
// into the structure" report.
func TestBuildResumeStopsOutsideLargeFootprint(t *testing.T) {
	w := bigStructWorld()
	center := fixed.Vec2{X: fixed.FromInt(500), Z: fixed.FromInt(400)}
	meta, _ := w.spawn("bigkeep")
	// Raise the structure directly as a 30%-built nanoframe (buildRem 0.7) and
	// freeze its abandonment decay for the test window (a far-future last-touch
	// keeps stepBuildDecay from collapsing it while the builder walks in).
	sid := w.addUnit("bigkeep", meta, nil, center, 0, 0, false)
	b := w.units[sid]
	w.setBuildRem(b, 0.7)
	b.lastNanoTick = 1 << 62

	// The builder starts far to the west; footprint reach is 9*8 = 72 wu, wider
	// than the builder's 60 wu BuildDistance, so a plain centre gate would never
	// trip until the builder was 60 wu from the centre — 12 wu INSIDE the plot.
	reach := footprintReach(9, 9)
	bld := w.AddUnit("con", conMeta("con"), nil, fixed.Vec2{X: fixed.FromInt(120), Z: fixed.FromInt(400)}, 0, 0)
	u := w.UnitByID(bld)
	w.ApplyOrder(order.Repair(bld, sid))

	minDist := fixed.FromInt(1 << 20)
	for i := 0; i < 6000 && b.underConstruction(); i++ {
		w.Step(nil)
		if d := u.loco.Pos.DistTo(center); d < minDist {
			minDist = d
		}
	}
	if b.underConstruction() || b.Dead {
		t.Fatalf("resume never completed the structure: dead=%v pct=%v", b.Dead, b.BuildPercent.Float())
	}
	if u.buildeeID != 0 && u.buildeeID != sid {
		t.Fatalf("resume spawned a different buildee (%d, want %d)", u.buildeeID, sid)
	}
	// The builder must have stayed outside the footprint at all times — its
	// closest approach never crossed the footprint edge.
	if minDist < reach {
		t.Fatalf("builder walked into the footprint: closest %v wu < reach %v wu",
			minDist.Float(), reach.Float())
	}
}

// TestFeatureReclaimApproachesLargeFeature pins the feature-reclaim approach: a
// reclaimer ordered at a large (footprint-wider-than-BuildDistance) blocking
// wreck walks in, stops at the footprint edge, and consumes it — crediting its
// yield. Regression for the #174 build-range gate turning a feature reclaim into
// a permanent walk order that never salvages.
func TestFeatureReclaimApproachesLargeFeature(t *testing.T) {
	w := New(Config{Seed: 72, Economy: EconomyTA, StartMetal: -1, StartEnergy: -1})
	w.SetTerrain(testTerrain(80, 80, 0, func(_, _ int) uint8 { return 0 }))
	center := fixed.Vec2{X: fixed.FromInt(500), Z: fixed.FromInt(400)}
	wreck := &FeatureMeta{Name: "hulk", FootprintX: 9, FootprintZ: 9, Metal: 40, Energy: 20, Reclaimable: true, Blocking: true, MaxHP: 100}
	fid := w.AddFeature("hulk", wreck, FeatureWreck, center, 0, -1)
	reach := footprintReach(9, 9)

	bld := w.AddUnit("con", conMeta("con"), nil, fixed.Vec2{X: fixed.FromInt(120), Z: fixed.FromInt(400)}, 0, 0)
	u := w.UnitByID(bld)
	before := w.econView(0).stock

	w.ApplyOrder(order.Reclaim([]uint32{bld}, fid))
	minDist := fixed.FromInt(1 << 20)
	var reclaimed bool
	for i := 0; i < 6000; i++ {
		w.Step(nil)
		if d := u.loco.Pos.DistTo(center); d < minDist {
			minDist = d
		}
		if w.FeatureByID(fid) == nil {
			reclaimed = true
			break
		}
	}
	if !reclaimed {
		t.Fatalf("large feature never reclaimed; builder stalled at %v wu (reach %v)",
			u.loco.Pos.DistTo(center).Float(), reach.Float())
	}
	after := w.econView(0).stock
	if got := after.Metal - before.Metal; got < fixed.FromInt(39) {
		t.Fatalf("reclaim credited metal=%v, want ~40", got.Float())
	}
	if got := after.Energy - before.Energy; got < fixed.FromInt(19) {
		t.Fatalf("reclaim credited energy=%v, want ~20", got.Float())
	}
	if minDist < reach {
		t.Fatalf("reclaimer walked into the footprint: closest %v wu < reach %v wu",
			minDist.Float(), reach.Float())
	}
}

// TestDecayedNanoframeRemoved pins the abandonment endgame: a frame left to
// decay all the way to 0% is removed from the sim AND drops out of the render
// snapshot within the decay window, raising a positioned clean-removal event
// (no wreck, no blast) rather than stranding a 0% ghost.
func TestDecayedNanoframeRemoved(t *testing.T) {
	w := buildWorld()
	bld := w.AddUnit("builder", builderMeta(), nil, fixed.Vec2{}, 0, 0)
	w.ApplyOrder(order.Build(bld, "solar", fixed.Vec2{X: fixed.FromInt(60)}, 0))
	u := w.UnitByID(bld)
	for i := 0; i < 600 && u.buildeeID == 0; i++ {
		w.Step(nil)
	}
	beeID := u.buildeeID
	b := w.UnitByID(beeID)
	if b == nil {
		t.Fatal("build never started")
	}
	for i := 0; i < 300 && b.BuildPercent < fixed.FromInt(20); i++ {
		w.Step(nil)
	}
	w.ApplyOrder(order.Stop([]uint32{bld}))

	var removedTick = -1
	var sawRemovalEvent bool
	for i := 0; i < 20000 && removedTick < 0; i++ {
		w.Step(nil)
		snap := w.Snapshot()
		for k := range snap.Events {
			if snap.Events[k].UnitID == beeID {
				sawRemovalEvent = true
			}
		}
		if w.UnitByID(beeID) == nil {
			removedTick = i
			// The decayed frame must be absent from the render snapshot too.
			for j := range snap.Units {
				if snap.Units[j].ID == beeID {
					t.Fatalf("removed frame still present in the snapshot at tick %d", i)
				}
			}
		}
	}
	if removedTick < 0 {
		t.Fatalf("abandoned frame never removed: pct=%v dead=%v", b.BuildPercent.Float(), b.Dead)
	}
	if !sawRemovalEvent {
		t.Fatalf("collapse emitted no removal event for the decayed frame")
	}
}
