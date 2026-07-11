package sim

import (
	"testing"

	"github.com/coreprime/kbot-engine/engine/fixed"
	"github.com/coreprime/kbot-engine/engine/frame"
	"github.com/coreprime/kbot-engine/engine/order"
)

// countingAimBinding is a minimal weapon binding that exposes a per-slot aim
// script and counts how many times the world (re)issues it. It always reports
// its aim as complete, so the only variable under test is how often the world
// restarts the aim thread while a slot holds a standing target.
type countingAimBinding struct {
	aimStarts int
	nextID    int32
}

func (b *countingAimBinding) HasScript(name string) bool {
	return name == "AimTertiary"
}

func (b *countingAimBinding) Start(string, ...int)       {}
func (b *countingAimBinding) Restart(string, ...int)     {}
func (b *countingAimBinding) Pieces() []frame.PieceState { return nil }

func (b *countingAimBinding) StartAim(name string, _ ...int) int32 {
	if name == "AimTertiary" {
		b.aimStarts++
	}
	b.nextID++
	return b.nextID
}

// AimStatus always reports the latest thread as a completed, successful aim so
// fire is never blocked by the aim latch.
func (b *countingAimBinding) AimStatus(int32) (bool, int32) { return true, 1 }

// TestCommandFireAimNotReissuedOnCadence pins the fix: a command-fire weapon
// held on a target it cannot reach (out of range, so it never discharges and
// never clears) must issue its aim thread once, not restart it every
// aimRefreshMs. Restarting the aim script re-runs the disintegrator's
// aim/fire-latch animation, which reads in the studio as the D-gun replaying
// its fire pose while the shot sits gated.
func TestCommandFireAimNotReissuedOnCadence(t *testing.T) {
	b := &countingAimBinding{}
	w := New(Config{Seed: 1})

	m := &UnitMeta{Name: "com", CanMove: true, MaxHealth: fixed.FromInt(3000)}
	m.FootprintX, m.FootprintZ = 2, 2
	// Slot 2 is the command-fire disintegrator; short range so a distant force-
	// fire point stays out of reach and the slot never discharges.
	m.Weapons[2] = WeaponMeta{Name: "ARM_DISINTEGRATOR", Range: fixed.FromInt(50), Burst: 1, CommandFire: true, Present: true}

	id := w.AddUnit("com", m, b, fixed.Vec2{}, 0, 0)

	// Force-fire slot 2 at a point well beyond its 50 wu range: the aim latch
	// tracks but the range gate blocks fire, so the slot holds its target.
	w.ApplyOrder(order.Order{Kind: order.KindFire, UnitID: id, Slot: 2, Target: fixed.Vec2{X: fixed.FromInt(400)}})

	// Run for six seconds of sim time — six aimRefreshMs windows.
	for i := 0; i < 6*TickHz; i++ {
		w.Step(nil)
	}

	if b.aimStarts != 1 {
		t.Fatalf("command-fire aim thread issued %d times; want exactly 1 (no periodic cadence re-issue)", b.aimStarts)
	}
}
