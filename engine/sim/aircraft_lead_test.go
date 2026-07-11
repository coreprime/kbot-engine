package sim

import (
	"testing"

	"github.com/coreprime/kbot-engine/engine/fixed"
)

// TestAirHorizontalVelocityMatch locks the step-5 velocity-match term of the
// §6.1 horizontal flight law: ax = dx*scale - (v - targetVel). A move
// destination carries no velocity, so its term is pure damping and the craft
// brakes as it closes on the point; an engaging craft's motion goal carries its
// own fly-through velocity (nose direction at cruise speed), so matching to it
// cancels the damping and the craft holds speed straight through the aim.
//
// The two regimes only diverge when the steering vector no longer saturates the
// acceleration clamp — i.e. when the aim is close. With a far aim the clamp
// absorbs the term and both regimes coincide (that far-aim cruise is what the
// engage-approach scenario pins). Here the aim sits a few world units ahead of a
// craft already at cruise speed, so the term is live and its sign decides
// whether the craft brakes or flies through.
func TestAirHorizontalVelocityMatch(t *testing.T) {
	// One frame from a shared cruise state with a close aim dead ahead.
	step := func(targetVel fixed.Vec2) fixed.Fixed {
		w := New(Config{Seed: 1})
		m := fighterMeta("f")
		id := w.AddUnit("f", m, nil,
			fixed.Vec2{X: fixed.FromInt(200), Z: fixed.FromInt(200)}, 0, 0)
		u := w.UnitByID(id)
		u.airVel = fixed.Vec3{Z: m.MaxVelocity} // at cruise up +Z
		aim := fixed.Vec2{X: u.loco.Pos.X, Z: u.loco.Pos.Z + fixed.FromInt(8)}
		w.stepAirHorizontal(u, aim, targetVel)
		return u.airVel.Z
	}

	m := fighterMeta("f")
	maxVel := m.MaxVelocity
	accel := m.Accel

	brake := step(fixed.Vec2{})                             // move: targetVel = 0
	flythrough := step(fixed.Vec2{Z: maxVel})               // engage: own-run velocity

	// Decisive sign: matching the own fly-through velocity keeps more speed than
	// the braking (zero-velocity) aim. A port that dropped the term, or matched
	// the wrong sign, would not separate these.
	if flythrough <= brake {
		t.Fatalf("velocity-match term inert: flythrough=%d not > brake=%d",
			flythrough, brake)
	}

	// Fly-through holds cruise speed: after drag sheds Accel and the clamped
	// steering adds it back toward the matched velocity, v returns to MaxVelocity
	// (a few LSB of fixed-point sqrt/atan2 residue).
	if d := (flythrough - maxVel).Abs(); d > fixed.FromInt(1)>>14 {
		t.Errorf("fly-through did not hold MaxVelocity: v=%d maxVel=%d (|d|=%d)",
			flythrough, maxVel, d)
	}

	// Braking sheds a second Accel: drag removes Accel, then the clamped steering
	// subtracts another Accel (pull back toward the static aim), so v settles at
	// MaxVelocity - 2*Accel.
	wantBrake := maxVel - accel - accel
	if d := (brake - wantBrake).Abs(); d > fixed.FromInt(1)>>14 {
		t.Errorf("brake did not settle at MaxVel-2*Accel: v=%d want~%d (|d|=%d)",
			brake, wantBrake, d)
	}

	// The whole separation is exactly the sign flip on one clamped Accel of
	// steering: +Accel toward the fly-through velocity vs -Accel toward the
	// static point => a 2*Accel gap.
	gap := flythrough - brake
	if d := (gap - (accel + accel)).Abs(); d > fixed.FromInt(1)>>14 {
		t.Errorf("fly-through/brake gap not ~2*Accel: gap=%d 2accel=%d (|d|=%d)",
			gap, accel+accel, d)
	}

	t.Logf("brake=%d flythrough=%d maxVel=%d accel=%d gap=%d",
		brake, flythrough, maxVel, accel, gap)
}
