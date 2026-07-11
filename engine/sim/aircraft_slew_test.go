package sim

import (
	"testing"

	"github.com/coreprime/kbot-engine/engine/fixed"
)

// TestAircraftVerticalSlewRate pins the exact §6.1 vertical slew law:
// vy = clamp(targetY − posY, ±max(speed>>2, 1 wu/frame)). The climb authority
// is at least one world unit per frame and grows with horizontal speed.
func TestAircraftVerticalSlewRate(t *testing.T) {
	cases := []struct {
		name     string
		speed    fixed.Fixed
		wantSlew fixed.Fixed
	}{
		{"idle floors at 1 wu/frame", 0, fixed.One},
		{"slow still floors at 1", fixed.FromInt(2), fixed.One},        // 2>>2 = 0.5 < 1
		{"fast climbs at speed/4", fixed.FromInt(8), fixed.FromInt(2)}, // 8>>2 = 2
		{"faster climbs at speed/4", fixed.FromInt(20), fixed.FromInt(5)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := New(Config{Seed: 1})
			m := &UnitMeta{Name: "flyer", CanMove: true, IsAircraft: true, CruiseAltitude: fixed.FromInt(100)}
			id := w.AddUnit("flyer", m, nil, fixed.Vec2{}, 0, 0)
			u := w.units[id]
			u.IsMoving = true // airborne
			u.loco.Speed = c.speed
			u.PosY = 0 // far below the 100 cruise target
			before := u.PosY
			w.stepAltitude(u)
			if got := u.PosY - before; got != c.wantSlew {
				t.Fatalf("vertical slew = %v, want %v (speed %v)", got, c.wantSlew, c.speed)
			}
		})
	}
}

// TestAircraftSlewSnapsToTarget verifies the slew snaps to the altitude target
// when the remaining gap is under one frame's authority (no overshoot).
func TestAircraftSlewSnapsToTarget(t *testing.T) {
	w := New(Config{Seed: 1})
	m := &UnitMeta{Name: "flyer", CanMove: true, IsAircraft: true, CruiseAltitude: fixed.FromInt(100)}
	id := w.AddUnit("flyer", m, nil, fixed.Vec2{}, 0, 0)
	u := w.units[id]
	u.IsMoving = true
	u.loco.Speed = 0                                   // slew authority 1 wu/frame
	u.PosY = fixed.FromInt(100) - fixed.FromFloat(0.3) // 0.3 wu below cruise
	w.stepAltitude(u)
	if u.PosY != fixed.FromInt(100) {
		t.Fatalf("slew did not snap to cruise target: PosY = %v, want 100", u.PosY)
	}
}
