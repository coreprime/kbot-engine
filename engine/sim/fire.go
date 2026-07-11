package sim

import "github.com/coreprime/kbot-engine/engine/fixed"

// Feature fire — ignition, the spark countdown, and spread (world.md §1.5;
// specials.md §7.5). A flammable feature (one with a burn sequence) can be lit
// by a firestarter weapon or by an already-burning neighbour. Once lit it holds
// a spark countdown drawn from the shared MINSTD stream; when the countdown
// expires it spreads ONCE: it rolls every flammable neighbour in a 7×7 box (and
// five steps downwind along the live wind vector) against that neighbour's
// spread chance, lighting the ones that pass, and detonates its burnweapon as
// an ownerless splash so a burning forest damages nearby units. Spread is
// host-authoritative in the originals; the deterministic sandbox simply runs it
// on every peer identically.

// fireBoxRadius is the half-extent of the spread box in cells (±3 = the engine's
// 7×7 neighbourhood, excluding the centre).
const fireBoxRadius = 3

// windProbeSteps is how many cells downwind the spread walks, each stepping the
// position by twice the wind vector (world.md §1.5 wind probe).
const windProbeSteps = 5

// igniteFeature lights a flammable feature. It sets the spark countdown to
// sparktime/2 + rand(sparktime/2) ticks — a single MINSTD draw (skipped when the
// half-time is below 2, the stream's small-bound rule). An already-burning or
// non-flammable feature is left untouched, so the shared stream advances at
// most once per genuine new ignition.
func (w *World) igniteFeature(f *Feature) {
	if f == nil || f.Meta == nil || !f.Meta.Flammable || f.burning || f.Kind == FeatureMetal {
		return
	}
	f.burning = true
	f.spread = false
	half := f.Meta.SparkTime / 2
	if half < 1 {
		half = 1
	}
	f.sparkTicks = half + int(w.rng.Bounded(int32(half)))
}

// stepFeatureFire advances every burning feature's spark countdown by one tick
// and fires the one-shot spread the tick a countdown reaches zero. Countdowns
// are decremented from a snapshot of the features burning at tick start, so a
// feature lit by this tick's spread waits until next tick to spread itself —
// the fire crawls outward one ring per tick rather than cascading instantly.
func (w *World) stepFeatureFire() {
	var toSpread []uint32
	for _, id := range w.featureOrder {
		f := w.features[id]
		if f == nil || !f.burning || f.spread {
			continue
		}
		if f.sparkTicks > 0 {
			f.sparkTicks--
		}
		if f.sparkTicks == 0 {
			toSpread = append(toSpread, id)
		}
	}
	for _, id := range toSpread {
		if f := w.features[id]; f != nil && !f.spread {
			w.runFireSpread(f)
			f.spread = true
		}
	}
}

// runFireSpread fires a burning feature's one-shot spread: the 7×7 box, then the
// five downwind probe cells, roll each flammable neighbour against its spread
// chance and light the winners; finally the feature detonates its burnweapon as
// an ownerless splash. The box and probe draws land on the shared MINSTD stream
// in the engines' order (box then wind probe).
func (w *World) runFireSpread(f *Feature) {
	// 7×7 box, centre excluded.
	for dz := -fireBoxRadius; dz <= fireBoxRadius; dz++ {
		for dx := -fireBoxRadius; dx <= fireBoxRadius; dx++ {
			if dx == 0 && dz == 0 {
				continue
			}
			w.trySpreadTo(f.Cx+dx, f.Cz+dz)
		}
	}
	// Wind probe: five steps along twice the wind drift vector from the anchor.
	cell := w.cellPitch()
	px := f.Pos.X
	pz := f.Pos.Z
	for step := 0; step < windProbeSteps; step++ {
		px += w.wind.driftX.Mul(fixed.FromInt(2))
		pz += w.wind.driftZ.Mul(fixed.FromInt(2))
		cx := px.Div(cell).Int()
		cz := pz.Div(cell).Int()
		w.trySpreadTo(cx, cz)
	}
	// Burnweapon: an ownerless splash at the burning feature's position, so a
	// burning forest damages units around it (the only per-burn damage source).
	if f.Meta.BurnWeapon != nil {
		w.detonateBlastAt(f.Pos, *f.Meta.BurnWeapon)
	}
}

// trySpreadTo rolls the flammable, non-burning feature anchored at (cx, cz)
// against its spread chance and lights it on success. Cells with no ignitable
// feature take no draw (the engine's short-circuit), so the stream advances
// once per candidate.
func (w *World) trySpreadTo(cx, cz int) {
	f := w.flammableAnchoredAt(cx, cz)
	if f == nil {
		return
	}
	if int(w.rng.Bounded(100)) < f.Meta.SpreadChance {
		w.igniteFeature(f)
	}
}

// flammableAnchoredAt returns the flammable, non-burning feature whose anchor
// cell is exactly (cx, cz), or nil — the spread candidate at a cell.
func (w *World) flammableAnchoredAt(cx, cz int) *Feature {
	for _, id := range w.featureOrder {
		f := w.features[id]
		if f == nil || f.Meta == nil || !f.Meta.Flammable || f.burning {
			continue
		}
		if f.Cx == cx && f.Cz == cz {
			return f
		}
	}
	return nil
}

// cellPitch is the world-units-per-cell pitch (16 on the flat grid, else the
// terrain's).
func (w *World) cellPitch() fixed.Fixed {
	if w.terrain != nil {
		return w.terrain.CellWU
	}
	return fixed.FromInt(16)
}

// detonateBlastAt runs an ownerless splash detonation at a world position — the
// burnweapon a burning feature fires. It mirrors detonateBlast's quadratic
// falloff but has no source unit, so it damages every unit in range (allies
// included), exactly like the feature-fire burnweapon in the originals.
func (w *World) detonateBlastAt(anchor fixed.Vec3, b Blast) {
	if b.Damage <= 0 || b.AoE <= 0 {
		return
	}
	aoe := b.AoE.Int()
	if aoe < splashSingleTargetWU {
		return
	}
	r := aoe / 2
	edge := fixed.Clamp(b.Edge, 0, fixed.One).Float()
	base := b.Damage.Float()
	for _, id := range w.order {
		t := w.units[id]
		if t == nil || t.Dead || t.carriedBy != 0 || t.Meta == nil {
			continue
		}
		d := blastAABBDist(anchor, t)
		if d > r {
			continue
		}
		if pts := int(base * splashWeight(d, r, edge)); pts > 0 {
			w.ApplyDamage(0, id, fixed.FromInt(pts))
		}
	}
}

// igniteFeaturesInBlast lights every flammable feature whose anchor sits inside
// a firestarter weapon's blast radius (specials.md §7.5): a firestarter hit
// ignites flammable features instead of damaging them. Called from the weapon
// detonation path when the weapon carries the firestarter flag.
func (w *World) igniteFeaturesInBlast(blast fixed.Vec3, aoe fixed.Fixed) {
	r := aoe.Int() / 2
	if r < 1 {
		r = 1
	}
	cell := w.cellPitch()
	for _, id := range w.featureOrder {
		f := w.features[id]
		if f == nil || f.Meta == nil || !f.Meta.Flammable || f.burning {
			continue
		}
		dx := (f.Pos.X - blast.X).Abs().Div(cell).Int()
		dz := (f.Pos.Z - blast.Z).Abs().Div(cell).Int()
		if dx*cell.Int() <= r && dz*cell.Int() <= r {
			w.igniteFeature(f)
		}
	}
}

// IgniteFeature lights a flammable feature by id — the scenario/inspection hook
// (a firestarter effect without staging a weapon). A missing, non-flammable or
// already-burning feature is a no-op.
func (w *World) IgniteFeature(id uint32) {
	w.igniteFeature(w.features[id])
}

// BurningFeatureCount reports how many features are currently on fire — the
// parity harness's fire-spread observable.
func (w *World) BurningFeatureCount() int {
	n := 0
	for _, id := range w.featureOrder {
		if f := w.features[id]; f != nil && f.burning {
			n++
		}
	}
	return n
}
