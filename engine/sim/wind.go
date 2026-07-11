package sim

import "github.com/coreprime/kbot-engine/engine/fixed"

// Ambient wind (world.md §1.8). Both engines re-roll the map's wind on an
// irregular timer and expose it to three sim consumers: windgenerator income
// (economy.md §1.5), ballistic/dropped projectile drift, and — later — the
// fire-spread probe. The re-roll cadence is drawn from the CRT stream while the
// speed and heading are drawn from the shared MINSTD sim stream, so the draw
// order matters for lockstep and is reproduced here in the engines' sequence.
//
// The re-roll fires when the tick passes a running deadline that advances by a
// CRT-drawn 5–14 s each time (interval = (crt·10/0x8000 + 5)·30 ticks). The new
// speed is minWind + MINSTD(maxWind−minWind) — a bound below 2 draws nothing,
// so a fixed range (minWind == maxWind) re-rolls a constant speed with no speed
// draw. The heading is a full-circle MINSTD draw, taken only when the speed is
// non-zero. Wind strength (speed/5000, clamped to 1) is the windgenerator's
// normalized output scalar; the per-tick drift vector is −2·sin/cos(heading)
// scaled by that same normalized speed, expressed in world units per tick.

// windStrengthDenom normalizes a raw wind speed into the [0,1] windgenerator
// output scalar: 5000 raw speed is full strength (world.md §1.8).
const windStrengthDenom = 5000.0

// windState is the world's live wind. speed/heading are the last rolled values;
// strength is the windgenerator scalar; driftX/driftZ are the per-tick world-
// unit displacement a drifting projectile accrues. changed latches true on the
// re-roll tick only (the windmill COB SetDirection/SetSpeed edge, cosmetic
// here). nextRoll is the tick the next re-roll fires on.
type windState struct {
	minWind, maxWind int32
	speed            int32
	heading          int32
	strength         float32
	driftX, driftZ   fixed.Fixed
	changed          bool
	nextRoll         uint64
}

// stepWind advances the ambient wind one tick. It runs in the world block after
// the economy settle (the engines' phase order: wind is step 8, after the
// player/economy phase), so a settle reads the wind the previous tick rolled.
func (w *World) stepWind() {
	w.wind.changed = false
	if w.tick <= w.wind.nextRoll {
		return
	}
	// Interval: CRT draw mapped to 5–14 s (150–420 ticks). Drawn even when the
	// range is calm, so the CRT stream advances every re-roll regardless.
	interval := (int64(w.crt.Next())*10/0x8000 + 5) * 30
	w.wind.nextRoll = w.tick + uint64(interval)
	w.wind.changed = true

	span := w.wind.maxWind - w.wind.minWind
	// Bounded's own quirk drops the draw when span < 2; speed is then exactly
	// minWind and the MINSTD stream is untouched.
	speed := w.wind.minWind + w.rng.Bounded(span)
	if speed < 0 {
		speed = 0
	}
	w.wind.speed = speed
	if speed != 0 {
		w.wind.heading = w.rng.Bounded(int32(fixed.FullCircle))
	}
	w.recomputeWind()
}

// recomputeWind derives the windgenerator strength scalar and the per-tick
// drift vector from the current speed and heading. strength = min(speed/5000,
// 1); the drift components are −2·sin/cos(heading) × (speed/5000) world units
// per tick — the normalized-speed reading of the engines' −2·isin·speed
// components (world.md §1.8; the raw fixed-point component scale is [U]).
func (w *World) recomputeWind() {
	norm := float64(w.wind.speed) / windStrengthDenom
	s := norm
	if s > 1 {
		s = 1
	}
	w.wind.strength = float32(s)
	if w.wind.speed == 0 {
		w.wind.driftX, w.wind.driftZ = 0, 0
		return
	}
	scale := fixed.FromFloat(-2 * norm)
	w.wind.driftX = fixed.SinScaled(w.wind.heading, scale)
	w.wind.driftZ = fixed.CosScaled(w.wind.heading, scale)
}

// WindSpeed / WindHeading / WindStrength expose the live wind for inspection
// and the parity harness. Strength is truncated to whole units ×1000 so an
// integer observable can read the [0,1] scalar with three-decimal resolution.
func (w *World) WindSpeed() int32   { return w.wind.speed }
func (w *World) WindHeading() int32 { return w.wind.heading }
func (w *World) WindStrengthMilli() int64 {
	return int64(float64(w.wind.strength) * 1000)
}
