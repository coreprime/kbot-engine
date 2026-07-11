package sim

import (
	"testing"

	"github.com/coreprime/kbot-engine/engine/fixed"
)

// flammableMeta builds a 1×1 flammable feature def.
func flammableMeta(sparkTime, spreadChance int) *FeatureMeta {
	return &FeatureMeta{
		Name: "tree", FootprintX: 1, FootprintZ: 1,
		Flammable: true, SparkTime: sparkTime, SpreadChance: spreadChance,
	}
}

// cellCenter is the world point at the centre of cell (cx, cz) on a 16-wu grid.
func cellCenter(cx, cz int) fixed.Vec2 {
	return fixed.Vec2{X: fixed.FromInt(cx*16 + 8), Z: fixed.FromInt(cz*16 + 8)}
}

// TestFirestarterWeaponIgnites verifies a firestarter weapon detonation lights
// flammable features in its blast instead of only damaging units.
func TestFirestarterWeaponIgnites(t *testing.T) {
	w := New(Config{Seed: 1})
	w.SetTerrain(gridTerrain(8, 8))
	fid := w.AddFeature("tree", flammableMeta(2, 100), FeatureProp, cellCenter(4, 4), 0, -1)
	wm := &WeaponMeta{Firestarter: true, AreaOfEffectWU: fixed.FromInt(48)}
	blast := fixed.Vec3{X: fixed.FromInt(72), Z: fixed.FromInt(72)}
	w.detonateWeapon(0, 0, wm, blast)
	if f := w.FeatureByID(fid); f == nil || !f.burning {
		t.Fatalf("firestarter weapon did not ignite the flammable feature")
	}
}

// TestBurnWeaponSplashDamagesUnits verifies a burning feature's burnweapon
// detonates as an ownerless splash at spread time and damages a nearby unit.
func TestBurnWeaponSplashDamagesUnits(t *testing.T) {
	w := New(Config{Seed: 1})
	w.SetTerrain(gridTerrain(8, 8))
	meta := flammableMeta(2, 100)
	meta.BurnWeapon = &Blast{Damage: fixed.FromInt(50), AoE: fixed.FromInt(64)}
	w.AddFeature("tree", meta, FeatureProp, cellCenter(4, 4), 0, -1)
	// A victim standing on the burning tree.
	vm := &UnitMeta{Name: "victim", CanMove: false, MaxHealth: fixed.FromInt(100)}
	vid := w.AddUnit("victim", vm, nil, cellCenter(4, 4), 0, 1)
	w.igniteFeature(w.features[featureIDBase]) // ignite the tree
	before := w.units[vid].Health
	for i := 0; i < 3; i++ {
		w.Step(nil) // spark countdown (1) then spread -> burnweapon splash
	}
	if w.units[vid].Health >= before {
		t.Fatalf("burnweapon splash did no damage: health %v -> %v", before, w.units[vid].Health)
	}
}

// TestFireWindProbeDownwind verifies the downwind probe ignites a flammable
// feature outside the 7×7 box that the wind vector reaches.
func TestFireWindProbeDownwind(t *testing.T) {
	w := New(Config{Seed: 1})
	w.SetTerrain(gridTerrain(16, 16))
	// Burning source at cell (0,4); a flammable tree at cell (6,4) — six cells
	// east, outside the ±3 box, but on the downwind probe line.
	src := w.AddFeature("tree", flammableMeta(2, 100), FeatureProp, cellCenter(0, 4), 0, -1)
	tgt := w.AddFeature("tree", flammableMeta(2, 100), FeatureProp, cellCenter(6, 4), 0, -1)
	// Drift one cell east per component; each probe step advances two cells, so
	// step 3 lands on cell (6,4).
	w.wind.driftX = fixed.FromInt(16)
	w.wind.driftZ = 0
	w.igniteFeature(w.features[src])
	w.Step(nil) // countdown 1 -> spread this tick
	if f := w.FeatureByID(tgt); f == nil || !f.burning {
		t.Fatalf("downwind probe did not ignite the out-of-box feature")
	}
}
