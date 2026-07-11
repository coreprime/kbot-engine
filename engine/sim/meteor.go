package sim

// Meteor weather (world.md §1.9). Meteor showers are an ambient map hazard, but
// their most important property for a lockstep sandbox is the RNG bookkeeping:
// meteor_strike_begin runs on a fixed cadence on EVERY map — even one with no
// meteor weapon configured — and burns four CRT draws each time, plus once at
// game start. Because the CRT stream is shared with the wind re-roll interval,
// a faithful sandbox must advance it identically, so these "dead draws" are
// reproduced here whether or not any meteor actually falls.
//
// When meteors are enabled, each strike opens an active window during which
// impacts fire on an interval, each drawing two more CRT values (radius and an
// even heading). The falling-meteor projectile and its damage need the meteor
// weapon definition the sandbox asset feed does not surface, so the impact path
// here advances the exact CRT draws and counts the impacts without spawning the
// ownerless projectile — a documented seam, kept out of the RNG contract.

// Stock meteor cadence (world.md §1.9): gamedata/meteor.tdf [Default] is
// Meteor/300/2/5/60 — density 2 impacts/s, duration 5 s, interval 60 s. The
// tick conversions are ftol(30/density), ftol(duration·30), ftol(interval·30).
const (
	meteorDefaultIntervalTicks = 15   // ftol(30 / 2 density)
	meteorDefaultDurationTicks = 150  // ftol(5 duration · 30)
	meteorDefaultWarmupTicks   = 1800 // ftol(60 interval · 30)
)

// meteorState tracks the meteor-weather schedule and its RNG cadence. enabled
// gates the active impact window (and, in a fuller build, the projectiles); the
// strike-begin dead draws run regardless. strikes/impacts count the CRT-draw
// events for inspection and the parity harness.
type meteorState struct {
	enabled                                   bool
	intervalTicks, durationTicks, warmupTicks uint64
	nextStrike                                uint64
	active                                    bool
	activeUntil                               uint64
	nextHit                                   uint64
	strikes                                   uint64
	impacts                                   uint64
}

// initMeteors seeds the meteor schedule with the stock cadence. A configured
// enable flag opens the active impact windows; the strike-begin dead draws fire
// at the stock period on every map either way. nextStrike 0 lands the first
// strike-begin on the opening tick (the engines' "once at game start").
func (w *World) initMeteors(enabled bool) {
	w.meteor = meteorState{
		enabled:       enabled,
		intervalTicks: meteorDefaultIntervalTicks,
		durationTicks: meteorDefaultDurationTicks,
		warmupTicks:   meteorDefaultWarmupTicks,
		nextStrike:    0,
	}
}

// stepMeteors advances the meteor cadence one tick (the engines' phase 9, after
// wind). A due strike-begin burns four CRT draws — the dead draws every map
// pays — and, when meteors are enabled, opens an active window whose impacts
// each burn two more CRT draws on the interval.
func (w *World) stepMeteors() {
	m := &w.meteor
	period := m.warmupTicks + m.durationTicks
	if period == 0 {
		return
	}
	for w.tick >= m.nextStrike {
		// meteor_strike_begin: four dead CRT draws (target_z, target_x, and the
		// z/x offsets), taken even when meteors are disabled.
		w.crt.Next()
		w.crt.Next()
		w.crt.Next()
		w.crt.Next()
		m.strikes++
		if m.enabled {
			m.active = true
			m.activeUntil = w.tick + m.durationTicks
			m.nextHit = w.tick
		}
		m.nextStrike += period
	}
	if !m.active {
		return
	}
	if w.tick > m.activeUntil {
		m.active = false
		return
	}
	for w.tick >= m.nextHit {
		// Each impact draws the radius and an even heading from the CRT stream.
		// The ownerless falling projectile + damage need the meteor weapon def
		// the sandbox does not feed (a documented seam); the draws still land so
		// the CRT stream stays aligned.
		w.crt.Next()
		w.crt.Next()
		m.impacts++
		m.nextHit += m.intervalTicks
	}
}

// MeteorStrikes / MeteorImpacts report the cumulative strike-begin and impact
// counts for inspection and the parity harness.
func (w *World) MeteorStrikes() uint64 { return w.meteor.strikes }
func (w *World) MeteorImpacts() uint64 { return w.meteor.impacts }

// CrtDraws reports the CRT stream's cumulative draw count (the wind re-roll
// interval plus the meteor cadence).
func (w *World) CrtDraws() uint64 { return w.crt.Draws() }
