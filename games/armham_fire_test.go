package games_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coreprime/kbot-engine/engine/fixed"
	"github.com/coreprime/kbot-engine/engine/frame"
	"github.com/coreprime/kbot-engine/engine/order"
	"github.com/coreprime/kbot-engine/engine/script"
	"github.com/coreprime/kbot-engine/engine/sim"
	"github.com/coreprime/kbot-engine/games"
	"github.com/coreprime/kbot-io/formats/gamedata/ta"
	"github.com/coreprime/kbot-io/formats/scripting"
	"github.com/coreprime/kbot-io/formats/tdf"
)

// TestArmhamFiresBallisticArc drives the ARM Hammer (armham) — a ballistic
// artillery kbot whose retail AimPrimary turns the torso then plays a
// multi-second barrel-open animation before it returns TRUE — against an
// in-range target. The long aim script must be allowed to run to completion:
// the weapon SM's cadence refresh must not supersede the still-running aim and
// stall the shot. It regresses the "opens barrels but never fires" bug.
func TestArmhamFiresBallisticArc(t *testing.T) {
	root := os.Getenv("TA_UNPACKED_PATH")
	if root == "" {
		t.Skip("set TA_UNPACKED_PATH")
	}
	fbi, err := os.ReadFile(filepath.Join(root, "units", "armham.fbi"))
	if err != nil {
		t.Fatal(err)
	}
	resolve := func(ref string) (ta.Weapon, bool) {
		entries, _ := os.ReadDir(filepath.Join(root, "weapons"))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".tdf") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(root, "weapons", e.Name()))
			if err != nil {
				continue
			}
			var ws []ta.Weapon
			if err := tdf.Unmarshal(data, &ws); err != nil {
				continue
			}
			for _, w := range ws {
				if strings.EqualFold(w.Key, ref) {
					return w, true
				}
			}
		}
		return ta.Weapon{}, false
	}
	meta, err := games.UnitMetaFromFBI("armham", fbi, resolve)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("weapons[0]: %+v", meta.Weapons[0])

	rt := script.NewRuntime(41)
	data, err := os.ReadFile(filepath.Join(root, "scripts", "armham.cob"))
	if err != nil {
		t.Fatal(err)
	}
	cob, err := scripting.LoadFromReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	prog, err := script.FromCOB(cob)
	if err != nil {
		t.Fatal(err)
	}
	bind := rt.NewUnit(prog, nil)

	w := sim.New(sim.Config{Seed: 41})
	atk := w.AddUnit("armham", meta, bind, fixed.Vec2{}, 0, 0)
	defMeta := &sim.UnitMeta{Name: "def", MaxHealth: fixed.FromInt(800)}
	def := w.AddUnit("def", defMeta, nil, fixed.Vec2{X: fixed.FromInt(200)}, 0, 1)
	w.ApplyOrder(order.Attack([]uint32{atk}, def))

	fires := 0
	for i := 0; i < 900; i++ {
		w.Step(rt)
		for _, ev := range w.Snapshot().Events {
			if ev.Kind == frame.EvFire && ev.UnitID == atk {
				fires++
			}
		}
	}
	t.Logf("fires=%d defHealth=%v", fires, w.UnitByID(def).Health)
	if fires == 0 {
		t.Fatal("armham never fired with real FBI meta")
	}
}
