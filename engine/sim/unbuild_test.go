package sim

import (
	"reflect"
	"testing"

	"github.com/coreprime/kbot-engine/engine/fixed"
	"github.com/coreprime/kbot-engine/engine/order"
)

// TestUnbuildDequeuesCount pins the count semantics: unbuild removes up to N
// pending copies, and asking for more than remain just empties the queue.
func TestUnbuildDequeuesCount(t *testing.T) {
	w := factoryWorld()
	fac := w.AddUnit("factory", factoryMeta(), nil, fixed.Vec2{}, 0, 0)
	for i := 0; i < 5; i++ {
		w.ApplyOrder(order.Build(fac, "tank", fixed.Vec2{}, 0))
	}
	u := w.UnitByID(fac)
	if len(u.prodQueue) != 5 {
		t.Fatalf("setup: queue should hold 5, got %d", len(u.prodQueue))
	}
	// Cancel two: the queue drops to three.
	w.ApplyOrder(order.Unbuild(fac, "tank", 2))
	if len(u.prodQueue) != 3 {
		t.Fatalf("unbuild 2 should leave 3, got %d", len(u.prodQueue))
	}
	// Over-cancel: removing more than remain empties the queue.
	w.ApplyOrder(order.Unbuild(fac, "tank", 10))
	if len(u.prodQueue) != 0 {
		t.Fatalf("unbuild 10 should empty the queue, got %d", len(u.prodQueue))
	}
}

// TestUnbuildRemovesNewestFirst pins the ordering: cancellation peels entries
// off the back of the queue, matching the named type, and an absent type is a
// no-op.
func TestUnbuildRemovesNewestFirst(t *testing.T) {
	w := factoryWorld()
	fac := w.AddUnit("factory", factoryMeta(), nil, fixed.Vec2{}, 0, 0)
	for _, name := range []string{"tank", "tank", "jeep", "tank"} {
		w.ApplyOrder(order.Build(fac, name, fixed.Vec2{}, 0))
	}
	u := w.UnitByID(fac)
	// The newest tank (the trailing entry) goes first; the jeep and the two
	// earlier tanks stay in click order.
	w.ApplyOrder(order.Unbuild(fac, "tank", 1))
	want := []string{"tank", "tank", "jeep"}
	if !reflect.DeepEqual(u.prodQueue, want) {
		t.Fatalf("newest tank should be cancelled, leaving %v, got %v", want, u.prodQueue)
	}
	// A type that isn't queued changes nothing.
	w.ApplyOrder(order.Unbuild(fac, "bomber", 3))
	if !reflect.DeepEqual(u.prodQueue, want) {
		t.Fatalf("unbuild of an absent type must not touch the queue: %v", u.prodQueue)
	}
}

// TestUnbuildLeavesInProgressFrame guards the reclaim-not-dequeue rule: once a
// copy is raising on the pad it lives in buildState, not prodQueue, so even an
// over-large unbuild clears only the pending copies and leaves the in-progress
// frame to finish.
func TestUnbuildLeavesInProgressFrame(t *testing.T) {
	w := factoryWorld()
	fac := w.AddUnit("factory", factoryMeta(), nil, fixed.Vec2{}, 0, 0)
	for i := 0; i < 3; i++ {
		w.ApplyOrder(order.Build(fac, "tank", fixed.Vec2{}, 0))
	}
	u := w.UnitByID(fac)
	// Step until the factory pulls the head onto the pad and starts raising it.
	for i := 0; i < 40 && u.buildState == buildIdle; i++ {
		w.Step(nil)
	}
	if u.buildState == buildIdle || u.buildeeID == 0 {
		t.Fatalf("setup: factory never started a frame (state=%d)", u.buildState)
	}
	if len(u.prodQueue) == 0 {
		t.Fatalf("setup: expected copies still queued behind the in-progress frame")
	}
	frame := u.buildeeID
	// Cancel far more than are queued: the pending copies clear, but the frame
	// already raising on the pad is untouched — it is not in prodQueue.
	w.ApplyOrder(order.Unbuild(fac, "tank", 10))
	if len(u.prodQueue) != 0 {
		t.Fatalf("unbuild should have emptied the pending queue, got %d", len(u.prodQueue))
	}
	if u.buildState == buildIdle || u.buildeeID != frame {
		t.Fatalf("unbuild destroyed the in-progress frame (state=%d buildee=%d, was %d)",
			u.buildState, u.buildeeID, frame)
	}
	if b := w.UnitByID(frame); b == nil || b.Dead {
		t.Fatalf("in-progress buildee was removed by unbuild")
	}
}
