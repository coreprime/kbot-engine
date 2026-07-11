# KBot engine

The deterministic KBot simulation engine — the Go sim core (`engine/…`) and the
shared games meta bridge (`games`), building on the
[coreprime/kbot-io](https://github.com/coreprime/kbot-io) format library and
compiled to WebAssembly — packaged with a small ESM loader for browsers and
Node.

The engine owns game state, orders, movement, combat and the COB script VM.
It emits per-tick render snapshots (unit poses + packed piece transforms +
events) that a renderer such as the KBot Studio sandbox draws. It performs no
I/O and no rendering itself, which is what makes it deterministic: the same
seed plus the same order stream produces the same world hash everywhere.

## Install

This package is published publicly on npmjs.org, so no registry or auth
configuration is needed to consume it:

```
npm install @coreprime/kbot-engine
```

(Publishing needs a token — CI authenticates with
`//registry.npmjs.org/:_authToken=${NODE_AUTH_TOKEN}`, sourced from the
environment; never commit a literal token.)

## Use

```js
import { loadEngine } from '@coreprime/kbot-engine'

const engine = await loadEngine() // bundled engine.wasm; pass { wasmUrl } to override
const session = engine.createSession({ seed: 42 })

const id = session.addUnit({ name: 'probe', canMove: true, maxVelocity: 1.5, acceleration: 0.5, brakeRate: 1, turnRate: 800 }, 100, 100)
session.submitMove([id], 100, 400)
const snapshot = session.step() // one tick; snapshot.units[].x/y/z etc.

// Replay / scripted driving: seek forward to an exact tick, and pin a unit
// to an authoritative pose (only the keys you pass are applied).
const at120 = session.stepTo(120)
session.setUnitState(id, { pos: { x: 250, y: 0, z: 300 }, heading: Math.PI / 2, vel: 1.2, hp: 80, build: 100 })
```

`loadEngine()` is a singleton per JS realm (the wasm module registers one
global API); run multiple simulations by creating multiple sessions. In Node
the parked wasm runtime keeps the event loop alive — exit explicitly when done.

Every heading crossing this API follows the game's convention (the one
recordings carry): a uint16 TA angle maps 0 → facing −Z (map north), 0x4000 →
−X (west), 65536 per turn; radians parameters/fields are that angle times
2π/65536 with **no offsets** — convert with `@coreprime/kbot-game3d`'s
`headingToRadians`. Snapshot units carry `piecesPacked` (stride-7 Float32:
move x/y/z in world units, turn x/y/z in TA angles, visible flag) indexed by
the unit's COB piece table (`session.pieceNames(unitId)`); apply it by NAME
via `@coreprime/kbot-game3d`'s `applyPackedPieces` — COB table order is not the model
hierarchy order.

## Building from source

The wasm binary is generated from the repo's Go code and is not committed:

1. `npm run build` (in `packages-js/engine/`) compiles `cmd/engine-wasm` with
   `GOOS=js GOARCH=wasm` into `wasm/engine.wasm` and copies the matching
   `wasm_exec.js` from the local Go toolchain — a Go toolchain is required.
2. `npm pack` / `npm publish` run the build automatically via `prepack`.

The studio's own copy is built separately by `task build-wasm` at the repo
root; the two are the same Go program.

## Sandbox parity harness

`sandboxverify/` grades the sim against the canonical mechanics specifications.
A scenario file (`scenarios/sandbox/*.yaml`) declares a tiny world built from
real game data, a run length, and named observables sampled at spec-time ticks
and compared against hand-derived expected values. The harness measures
divergence — it never adjusts the sim to pass. The full matrix is 38 scenarios
/ 91 checks (all faithful).

There are two entry points:

- **CI golden gate — no game install.** `TestParityCISubset` builds a 13-scenario
  subset (27 checks, spanning locomotion / economy / combat / specials) from a
  minimal fixture tree committed under `sandboxverify/testdata/fixtures/ta`
  (the real `armflash` and `armmex` FBI + COB, the `TANKSH2` movement class,
  and the `EMG` weapon). It grades them tick-exact and asserts the result
  matches `sandboxverify/golden/ci-subset.json` byte-for-byte, so any engine
  change that shifts a simulated value on a covered mechanic fails CI with zero
  game assets on the runner. Regenerate the golden after an intended change with
  `UPDATE_GOLDEN=1 go test ./sandboxverify/...` and review the diff.
- **Full matrix — local only.** `TestParityFull` and the `cmd/sandbox-verify`
  CLI run all 38 scenarios against a real flattened install
  (`$TA_UNPACKED_PATH` / `$TAK_UNPACKED_PATH`). With `ALLOW_SKIP_ASSETS=true`
  (the CI setting) the full test skips cleanly; locally with the roots set it
  reports the complete 91-check gap matrix.
