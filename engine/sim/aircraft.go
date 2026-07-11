package sim

import "github.com/coreprime/kbot-engine/engine/fixed"

// atkPhase enumerates the phases an aircraft cycles through while engaging.
// A gunship alternates approach/strafe; a fixed-wing aircraft alternates
// approach/egress, flying a fly-by pass then arcing out to a turn-around point.
type atkPhase uint8

const (
	atkApproach atkPhase = iota
	atkStrafe
	atkEgress
)

// Aircraft flight (locomotion spec §6.1). Both axes now run the exact §6.1
// law. The VERTICAL slew — vy = clamp(targetY−posY, ±max(speed>>2, 1 wu/frame))
// — lives in stepAltitude. The HORIZONTAL law is stepAirHorizontal below: a
// per-frame drag toward top speed, an overspeed trim that re-points any speed
// above the brake rate along the nose (the source of banked turns), the same
// turn-rate heading clamp ground movers use, and a steering-acceleration
// vector that pulls the flight velocity onto the aim point. It integrates a
// true 3-D velocity that diverges from the heading while banking, unlike the
// scalar-speed ground integrator.
//
// stepAirHorizontal runs one frame of the horizontal flight law, steering the
// unit's flight velocity toward target and integrating position from it. The
// caller owns the aim point (a move destination, or an attack maneuver's fly-by
// geometry) AND the aim velocity; this owns only the motion.
//
// targetVel is the dead-reckon velocity the steering law matches to in step 5 —
// the velocity the aim point is itself carrying. A stationary aim (a move
// destination) carries none, so callers on that path pass a zero vector and the
// term reduces to plain velocity damping, the exact behaviour the point-order
// scenarios pin. An engaging aircraft's motion goal instead carries its own
// fly-through velocity (nose direction at cruise speed): matching to it lets the
// craft hold speed across its aim rather than braking onto it, so it strafes
// through the pass instead of parking on the target.
func (w *World) stepAirHorizontal(u *Unit, target, targetVel fixed.Vec2) {
	m := u.Meta
	if m == nil || m.MaxVelocity <= 0 {
		return
	}
	v := &u.airVel

	// Step 1 — drag. Each frame the velocity loses the fraction accel/maxvel of
	// itself, so under constant thrust (step 5 adds ~accel per frame) it settles
	// at exactly maxvelocity. Applies to all three axes; the vertical is then
	// overwritten by the altitude slew.
	dragF := fixed.One - m.Accel.Div(m.MaxVelocity)
	v.X = v.X.Mul(dragF)
	v.Y = v.Y.Mul(dragF)
	v.Z = v.Z.Mul(dragF)

	// Step 2 — overspeed trim. Any horizontal speed beyond the brake rate is
	// peeled off the current velocity direction and re-laid along the nose:
	// keep brakerate worth along the old heading, re-point the excess forward.
	// When velocity already tracks the nose this is a no-op; while turning it
	// swings the velocity toward the heading a bank at a time.
	hs := fixed.Vec2{X: v.X, Z: v.Z}.Len()
	if hs > m.BrakeRate {
		ratio := m.BrakeRate.Div(hs)
		v.X = v.X.Mul(ratio)
		v.Z = v.Z.Mul(ratio)
		excess := hs - m.BrakeRate
		sinH, cosH := headingVec(u.loco.Heading)
		v.X += sinH.Mul(excess)
		v.Z += cosH.Mul(excess)
	}

	// Step 3 (vertical slew) is stepAltitude, applied after the movement pass;
	// it writes the vertical velocity back into airVel.Y.

	// Step 4 — heading clamp. Turn toward the aim bearing at up to the unit's
	// turn rate; the heading snaps to the bearing once the error fits one step.
	d := target.Sub(u.loco.Pos)
	bearing := fixed.Atan2(d.X, d.Z)
	err := fixed.WrapAngle(bearing - int32(u.loco.Heading.Int()))
	w.setTurnRate(u, err)

	// Step 5 — steering acceleration. Pull the velocity onto the aim point with
	// an acceleration whose direction is (pos−target)·(−gain) minus the current
	// velocity, clamped to the unit's acceleration. The negative gain makes the
	// position term point from the unit toward the target, so the craft arrives
	// rather than crawling. The distance floors at 8 wu so the gain never blows
	// up on top of the aim point.
	dx := u.loco.Pos.X - target.X
	dz := u.loco.Pos.Z - target.Z
	dist := fixed.Max(fixed.Vec2{X: dx, Z: dz}.Len(), fixed.FromInt(8))
	scale := -(m.Accel + m.Accel).Div(dist).Sqrt()
	// The velocity-match term is (v − targetVel). A move destination is static,
	// so targetVel is zero and this is pure damping of the flight velocity onto
	// the steered heading. When the aim carries its own velocity (an engaging
	// craft's fly-through), matching to it cancels the damping in the direction
	// of that velocity, so the craft keeps its speed through the aim.
	ax := dx.Mul(scale) - (v.X - targetVel.X)
	az := dz.Mul(scale) - (v.Z - targetVel.Z)
	if mag := (fixed.Vec2{X: ax, Z: az}).Len(); mag > m.Accel {
		ax = ax.Mul(m.Accel).Div(mag)
		az = az.Mul(m.Accel).Div(mag)
	}
	v.X += ax
	v.Z += az

	// Scalar speed is the full 3-D velocity magnitude (the vertical component is
	// last frame's slew, refreshed by stepAltitude after this pass). Position
	// integrates straight off the horizontal velocity — no nose decomposition,
	// since the velocity already carries its own direction.
	u.loco.Speed = fixed.Wrap32(hs3(v.X, v.Y, v.Z))
	u.loco.Pos.X = fixed.Wrap32(u.loco.Pos.X + v.X)
	u.loco.Pos.Z = fixed.Wrap32(u.loco.Pos.Z + v.Z)
}

// hs3 is the length of a 3-D vector, computed as the hypotenuse of the
// horizontal length and the vertical component so it reuses the 2-D helper.
func hs3(x, y, z fixed.Fixed) fixed.Fixed {
	return fixed.Hypot(fixed.Hypot(x, z), y)
}

// airMaxSpeed and airTurnPerFrame are the raw per-frame kinematics the hover
// gunship's strafe arc still steps against — its facing is decoupled from its
// motion (nose on the target while sliding around a standoff arc), a geometry
// the single-aim-point flight law does not express — with fallback defaults
// for aircraft whose FBI omits the stat.
func airMaxSpeed(m *UnitMeta) fixed.Fixed {
	if m.MaxVelocity > 0 {
		return m.MaxVelocity
	}
	return fixed.One
}

func airTurnPerFrame(m *UnitMeta) fixed.Fixed {
	if m.TurnRate > 0 {
		return m.TurnRate
	}
	return fixed.FromInt(600)
}

// turnToward rotates the heading toward want (a TA-angle) at the unit's FBI
// turn rate, never snapping past it.
func turnToward(st *locoState, want fixed.Fixed, m *UnitMeta) {
	dh := shortestArcFx(want - st.Heading)
	step := airTurnPerFrame(m)
	if dh.Abs() > step {
		st.setHeading(st.Heading + fixed.FromInt(dh.Sign()).Mul(step))
	} else {
		st.setHeading(want)
	}
}

// flyForwardAir drives the unit forward along the want heading using the exact
// §6.1 horizontal flight law: it aims the steering law at a far point straight
// ahead along want, so the craft turns onto that heading and cruises up to
// MaxVelocity. The maneuver planner owns want (the fly-by / egress / drop-line
// direction); this owns only the motion.
//
// This is the engaging-aircraft path, and its motion goal carries a fly-through
// velocity — nose direction at cruise speed — which the steering law matches to
// so a closing craft holds its speed across the aim rather than braking onto
// it. (The engine freezes this velocity from the airframe heading when the
// engagement begins and dead-reckons it; this per-tick planner re-derives it
// from the current run direction, which coincides at cruise — the defensible
// seam for a planner that rebuilds its aim every frame.)
//
// The aim sits a fixed distance ahead, far beyond any standoff, so the steering
// vector always saturates the acceleration clamp and the clamp absorbs the
// match term: down this path the seeded velocity is faithful but observably
// inert, and the cruise coincides with the point-order cruise to the raw
// integer. The term separates the two regimes only when the aim is close (the
// craft nearly on it), which this far-aim planner never reaches; that near-aim
// fly-through is what the flight-law unit test exercises directly.
func (w *World) flyForwardAir(u *Unit, want fixed.Fixed) {
	sin, cos := headingVec(want)
	const farAhead = 4000 // wu; well beyond any standoff so the aim stays ahead
	far := fixed.FromInt(farAhead)
	aim := fixed.Vec2{
		X: fixed.Wrap32(u.loco.Pos.X + sin.Mul(far)),
		Z: fixed.Wrap32(u.loco.Pos.Z + cos.Mul(far)),
	}
	runVel := fixed.Vec2{X: sin.Mul(u.Meta.MaxVelocity), Z: cos.Mul(u.Meta.MaxVelocity)}
	w.stepAirHorizontal(u, aim, runVel)
}

// attackManeuver flies an aircraft's attack pattern around the engagement at
// (tx, tz), using its weapon range for the standoff and fly-by geometry. It
// mutates the unit's locomotion and persistent maneuver fields in place,
// mirroring locomotion.js attackManeuver. Two flavours:
//
//   - Hover gunship (IsHover): close to within range, then strafe an arc
//     left/right around the target at standoff while always facing it.
//   - Fixed-wing: fly straight at the target, overshoot, then arc out to a
//     wide turn-around point and come back (alternating sides => figure-eight).
//     A bomber (bomberMode) holds heading through the drop window so its bomb
//     string lays on a straight line, banking away only once it has cleared the
//     far edge (target + passthrough).
func (w *World) attackManeuver(u *Unit, tx, tz, rangeF fixed.Fixed, bomberMode bool, passthrough fixed.Fixed) {
	st := &u.loco
	dx := tx - st.Pos.X
	dz := tz - st.Pos.Z
	dist := fixed.Vec2{X: dx, Z: dz}.Len()
	bearing := fixed.FromInt(int(fixed.Atan2(dx, dz)))

	if u.Meta.IsHover {
		standoff := fixed.Max(fixed.FromInt(24), rangeF.Mul(fixed.FromFloat(0.6)))
		if dist > rangeF {
			// Out of range — close in head-on.
			u.atkPhase = atkApproach
			w.flyForwardAir(u, bearing)
			return
		}
		// In range — strafe an arc around the target, nose always on it.
		if u.atkPhase != atkStrafe {
			u.atkPhase = atkStrafe
			// Lock the side we arrived from.
			u.sweepCenter = fixed.FromInt(int(fixed.Atan2(st.Pos.X-tx, st.Pos.Z-tz)))
			u.sweepPhase = 0
		}
		u.sweepPhase += perTick(fixed.FromFloat(0.8 * taAnglesPerRadian))
		sinPhase := fixed.Sin(int32(u.sweepPhase.Int()))
		ang := u.sweepCenter + sinPhase.Mul(fixed.FromFloat(0.7*taAnglesPerRadian)) // +/-40 deg sweep
		sinAng, cosAng := fixed.SinCos(int32(ang.Int()))
		desX := tx + standoff.Mul(sinAng)
		desZ := tz + standoff.Mul(cosAng)
		mdx := desX - st.Pos.X
		mdz := desZ - st.Pos.Z
		md := fixed.Vec2{X: mdx, Z: mdz}.Len()
		eps := fixed.FromFloat(1e-3)
		step := fixed.Min(md, airMaxSpeed(u.Meta))
		if md > eps {
			st.Pos.X = fixed.Wrap32(st.Pos.X + mdx.Div(md).Mul(step))
			st.Pos.Z = fixed.Wrap32(st.Pos.Z + mdz.Div(md).Mul(step))
			st.Speed = airMaxSpeed(u.Meta)
		} else {
			st.Speed = 0
		}
		turnToward(st, bearing, u.Meta) // face the target while strafing
		return
	}

	// Fixed-wing fly-by.
	egressDist := fixed.Max(fixed.FromInt(30), rangeF.Mul(fixed.FromFloat(0.4)))
	if bomberMode {
		egressDist = fixed.FromInt(30)
	}
	if u.atkPhase != atkEgress {
		u.atkPhase = atkApproach
		// Hold heading inside the drop zone (bomber + within passthrough of the
		// target); steer to the target bearing everywhere else.
		want := bearing
		if bomberMode && dist <= passthrough {
			want = st.Heading
		}
		w.flyForwardAir(u, want)
		// Past-target test: forward . (target - pos) negative => crossed the aim
		// point. Bombers also wait until clear of the far drop-zone edge.
		sinH, cosH := headingVec(st.Heading)
		dot := sinH.Mul(dx) + cosH.Mul(dz)
		pastTarget := dot < 0
		var triggerEgress bool
		if bomberMode {
			triggerEgress = pastTarget && dist >= passthrough
		} else {
			triggerEgress = dist < egressDist
		}
		if triggerEgress {
			u.atkPhase = atkEgress
			sx := cosH
			sz := -sinH
			if u.flybySide == 0 {
				u.flybySide = 1
			}
			u.flybySide = -u.flybySide
			lead := fixed.Max(fixed.FromInt(180), rangeF.Mul(fixed.FromFloat(1.2)))
			lat := fixed.Max(fixed.FromInt(120), rangeF.Mul(fixed.FromFloat(0.7))).Mul(fixed.FromInt(int(u.flybySide)))
			u.egX = tx + sinH.Mul(lead) + sx.Mul(lat)
			u.egZ = tz + cosH.Mul(lead) + sz.Mul(lat)
		}
		return
	}
	// Egress: fly to the turn-around point, then come back for another run.
	ex := u.egX - st.Pos.X
	ez := u.egZ - st.Pos.Z
	w.flyForwardAir(u, fixed.FromInt(int(fixed.Atan2(ex, ez))))
	if (fixed.Vec2{X: ex, Z: ez}).Len() < fixed.FromInt(40) {
		u.atkPhase = atkApproach
	}
}
