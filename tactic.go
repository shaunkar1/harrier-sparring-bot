package main

import (
	"math"
	"math/rand"
	"os"
)

type remembered struct {
	actor                 Actor
	last, lastHP, damaged float64
}
type Pilot struct {
	h            Hello
	w            *World
	rng          *rand.Rand
	style        string
	memory       map[int]*remembered
	lastDamage   float64
	lastDealt    float64
	retreatUntil float64
	targetID     int
	roam         V2
	roamUntil    float64
	safe         V2
	safeUntil    float64
	navTarget    V2
	navPoint     V2
	navUntil     float64
	lastMove     V2
	stuck        float64
	lastPos      V2
	lastTime     float64
	lastThreat   int
}

func newPilot(h Hello) *Pilot {
	mapDir := h.Map.Dir
	if override := os.Getenv("SPAR_MAP_DIR"); override != "" {
		mapDir = override // local simulators may record /arena as the map location
	}
	p := &Pilot{h: h, w: loadWorld(mapDir), rng: rand.New(rand.NewSource(h.Seed + int64(h.Slot*2971))), style: os.Getenv("SPAR_PROFILE"), memory: make(map[int]*remembered), targetID: -1, lastThreat: -1}
	if p.style != "artillery" && p.style != "stalker" && p.style != "harrier" {
		p.style = "harrier"
	}
	return p
}
func (p *Pilot) weapon(me *Self, kind string) (*WState, int) {
	for i, w := range me.Weapons {
		if w != nil && w.Kind == kind {
			return w, i
		}
	}
	return nil, -1
}
func (p *Pilot) remember(o *Obs) {
	for _, a := range o.Mechs {
		rec := p.memory[a.ID]
		if rec == nil {
			rec = &remembered{}
			p.memory[a.ID] = rec
		}
		if rec.last > 0 && a.HP < rec.actor.HP-10 {
			rec.damaged = o.T
		}
		rec.lastHP = rec.actor.HP
		rec.last = o.T
		rec.actor = a
	}
	for id, r := range p.memory {
		if o.T-r.last > 7 {
			delete(p.memory, id)
		}
	}
	for _, e := range o.Events {
		if e.Type == "damaged" && e.Weapon != "zone" {
			p.lastDamage = o.T
			if e.By != nil {
				p.lastThreat = *e.By
			}
		}
		if e.Type == "hit" {
			p.lastDealt = o.T
		}
		if e.Type == "kill" {
			delete(p.memory, e.Target)
		}
	}
}
func (p *Pilot) eye(me *Self) V3 {
	return V3{me.Pos[0], me.Pos[1], me.Pos[2] + num(me.Stats, "eye_height", 8)}
}
func (p *Pilot) muzzle(me *Self) V3 {
	return V3{me.Pos[0], me.Pos[1], me.Pos[2] + num(me.Stats, "muzzle_height", 7)}
}
func targetPoint(a Actor) V3 {
	h := 4.5
	if a.Kind == "npc" {
		h = 1.8
	} else {
		h *= 1 + 0.05*a.Cubes
	}
	return V3{a.Pos[0], a.Pos[1], a.Pos[2] + h}
}
func (p *Pilot) los(me *Self, a Actor) bool {
	return p.w == nil || p.w.clear(p.muzzle(me), targetPoint(a))
}
func (p *Pilot) zoneDistance(z Zone, pt V2) float64 { return pt.Sub(z.NextCenter).Len() - z.NextRadius }
func (p *Pilot) centerSoon(o *Obs) (V2, float64) {
	z := o.Zone
	if z.ShrinkEnd <= z.ShrinkStart {
		return z.NextCenter, z.NextRadius
	}
	if o.T < z.ShrinkStart {
		return z.Center, z.Radius
	}
	// zone.center/radius are current values, next is guaranteed later endpoint
	return z.Center, z.Radius
}
func (p *Pilot) nearZoneGoal(o *Obs) V2 {
	x := xy(o.Self.Pos)
	z := o.Zone
	margin := math.Min(24, math.Max(7, z.NextRadius*0.18))
	safeRadius := math.Max(5, z.NextRadius-margin)
	dist := x.Sub(z.NextCenter).Len()
	if dist < safeRadius {
		return x
	}
	return z.NextCenter.Add(x.Sub(z.NextCenter).Unit().Mul(math.Max(0, safeRadius-6)))
}
func (p *Pilot) zoneUrgency(o *Obs) bool {
	x := xy(o.Self.Pos)
	z := o.Zone
	d := x.Sub(z.NextCenter).Len() - z.NextRadius + 15
	if !o.Self.InZone {
		return true
	}
	if d < 0 {
		return false
	}
	available := math.Max(0, z.ShrinkEnd-o.T)
	if z.ShrinkEnd > o.T && z.ShrinkStart > o.T {
		available = math.Min(available, z.ShrinkStart-o.T+15)
	}
	return available < d/math.Max(7, num(o.Self.Stats, "walk_speed", 11.7))*1.8+17
}
func (p *Pilot) exposedFrom(me *Self, at V2, a Actor) bool {
	if p.w == nil {
		return true
	}
	origin := V3{a.Pos[0], a.Pos[1], a.Pos[2] + 8*(1+0.05*a.Cubes)}
	goal := V3{at[0], at[1], p.w.ground(at[0], at[1]) + num(me.Stats, "height", 9)*0.65}
	return p.w.clear(origin, goal)
}
func (p *Pilot) threatValue(o *Obs, a Actor) float64 {
	d := xy(a.Pos).Sub(xy(o.Self.Pos)).Len()
	if d > 280 {
		return 0
	}
	facing := math.Abs(wrap(a.Aim[0] - angle(xy(o.Self.Pos).Sub(xy(a.Pos)))))
	factor := 0.35
	if facing < 0.6 {
		factor = 1.25
	} else if facing < 1.35 {
		factor = 0.75
	}
	if a.Charging {
		factor += 0.9
	}
	if a.Has("lightning") && d < 52 {
		factor += 1.5
	}
	if a.Has("laser") && d < 270 {
		factor += 0.3
	}
	if a.ID == p.lastThreat && o.T-p.lastDamage < 8 {
		factor += 1.2
	}
	return (120 / (d + 35)) * factor
}
func (p *Pilot) pickEnemy(o *Obs) (*Actor, bool, float64) {
	var best *Actor
	bs := -1e9
	danger := 0.0
	me := &o.Self
	hpFrac := me.HP / math.Max(1, me.MaxHP)
	for i := range o.Mechs {
		a := &o.Mechs[i]
		if a.Kind != "mech" {
			continue
		}
		d := xy(a.Pos).Sub(xy(me.Pos)).Len()
		tv := p.threatValue(o, *a)
		danger += tv
		vis := p.los(me, *a)
		score := 0.0
		if d < 190 {
			score += 80 - d*0.45
		} else {
			score -= 100
		}
		if vis {
			score += 90
		} else {
			score -= 120
		}
		score += 480 * (1 - a.HP/math.Max(1, a.MaxHP))
		if p.memory[a.ID] != nil && o.T-p.memory[a.ID].damaged < 2.5 {
			score += 170
		} // already fighting another mech
		if a.ID == p.targetID {
			score += 90
		}
		if a.ID == p.lastThreat && o.T-p.lastDamage < 5 {
			score += 150
		}
		if a.Charging && a.Has("laser") {
			score -= 50
		} // watch the beam; don't walk into it
		if d < 45 && a.Has("lightning") && !me.Grounded {
			score -= 120
		}
		if score > bs {
			bs = score
			best = a
		}
	}
	if best == nil {
		return nil, false, danger
	}
	d := xy(best.Pos).Sub(xy(me.Pos)).Len()
	hpAdv := (me.HP + 150) / (best.HP + 150)
	weak := best.HP < 0.62*best.MaxHP
	thirdParty := p.memory[best.ID] != nil && o.T-p.memory[best.ID].damaged < 2.5
	isolated := danger < 2.35
	committed := best.ID == p.lastThreat && o.T-p.lastDamage < 6
	good := p.los(me, *best) && d < 210 && hpFrac > 0.36 && (weak || thirdParty || committed || (hpAdv > 1.25 && isolated))
	if o.Alive <= 3 {
		good = good && (committed || best.HP < 0.4*best.MaxHP || (o.Alive == 2 && hpFrac > 0.38))
	}
	if danger > 3.6 && !(weak && best.HP < 250) {
		good = false
	}
	if p.style == "stalker" && d < 75 && hpFrac > 0.65 && danger < 2.7 {
		good = true
	}
	if p.style == "artillery" && d > 75 && d < 190 && p.los(me, *best) && danger < 2.0 && hpFrac > 0.75 {
		good = true
	}
	return best, good, danger
}
func (p *Pilot) safeSpot(o *Obs) V2 {
	if o.T < p.safeUntil && p.zoneDistance(o.Zone, p.safe) < -7 {
		return p.safe
	}
	me := &o.Self
	x := xy(me.Pos)
	best := x
	bs := -1e9
	for i := 0; i < 64; i++ {
		ang := float64(i)*2.3999632 + float64(p.h.Slot)*0.3
		r := 12 + float64(i%8)*7.0
		q := x.Add(V2{math.Cos(ang) * r, math.Sin(ang) * r})
		if p.w != nil && !p.w.walk(q[0], q[1]) {
			continue
		}
		if p.zoneDistance(o.Zone, q) > -8 {
			continue
		}
		score := -r * 0.65
		for _, a := range o.Mechs {
			if a.Kind != "mech" {
				continue
			}
			d := q.Sub(xy(a.Pos)).Len()
			if d < 34 {
				score -= 200
			}
			if p.exposedFrom(me, q, a) {
				score -= 130 * p.threatValue(o, a)
			} else {
				score += 30
			}
		}
		if score > bs {
			bs, best = score, q
		}
	}
	p.safe = best
	p.safeUntil = o.T + 1.2
	return best
}
func (p *Pilot) chooseLoot(o *Obs) (V2, string, float64) {
	me := &o.Self
	x := xy(me.Pos)
	best := V2{}
	kind := ""
	bs := -1e9
	assess := func(pt V3, cat string, value float64) {
		q := xy(pt)
		d := q.Sub(x).Len()
		if d > 175 {
			return
		}
		if p.w != nil && math.Abs(p.w.ground(q[0], q[1])-pt[2]) > 3.0 {
			return
		} // no rooftop detours
		if p.zoneDistance(o.Zone, q) > -4 {
			return
		}
		score := value - d*1.2
		if o.T > 190 && cat == "crate" {
			score -= 100
		}
		for _, a := range o.Mechs {
			if a.Kind != "mech" {
				continue
			}
			ed := q.Sub(xy(a.Pos)).Len()
			if ed < 75 {
				score -= 120 * (1 - ed/90)
			}
		}
		if score > bs {
			best, kind, bs = q, cat, score
		}
	}
	haveLas, _ := p.weapon(me, "laser")
	haveLtn, _ := p.weapon(me, "lightning")
	haveMor, _ := p.weapon(me, "mortar")
	for _, i := range o.Pickups {
		val := 0.0
		switch i.Kind {
		case "laser":
			if haveLas == nil {
				val = 320
			}
		case "lightning":
			if haveLtn == nil {
				val = 280
			}
			if haveLas != nil {
				val = 370
			}
		case "mortar":
			if haveMor == nil {
				val = 210
			}
			if p.style == "artillery" {
				val = 360
			}
			if haveLas != nil {
				val = 0
			}
		}
		if val > 0 {
			assess(i.Pos, "weapon", val)
		}
	}
	for _, i := range o.Cubes {
		assess(i.Pos, "cube", 160)
	}
	if me.Cubes < 9 && me.HP/me.MaxHP > 0.58 {
		for _, i := range o.Crates {
			assess(i.Pos, "crate", 105)
		}
		for _, a := range o.Mechs {
			if a.Kind == "npc" && xy(a.Pos).Sub(x).Len() > 26 {
				assess(a.Pos, "npc", 165)
			}
		}
	}
	for _, d := range o.Drops {
		if d.LandsIn > 30 {
			continue
		}
		if d.Holder != nil && *d.Holder != me.ID {
			continue
		}
		value := 230.0 + 360*(1-me.HP/me.MaxHP)
		if d.LandsIn > 10 {
			value -= 60
		}
		assess(d.Pos, "drop", value)
	}
	if bs < 10 {
		return V2{}, "", bs
	}
	return best, kind, bs
}
func (p *Pilot) getRoam(o *Obs) V2 {
	if o.T < p.roamUntil && p.zoneDistance(o.Zone, p.roam) < -15 {
		return p.roam
	}
	z := o.Zone
	radius := math.Min(80, math.Max(20, z.NextRadius*0.58))
	for k := 0; k < 24; k++ {
		a := p.rng.Float64() * 2 * math.Pi
		q := z.NextCenter.Add(V2{math.Cos(a) * radius, math.Sin(a) * radius})
		if p.w == nil || p.w.walk(q[0], q[1]) {
			p.roam = q
			break
		}
	}
	if p.roam == (V2{}) {
		p.roam = z.NextCenter
	}
	p.roamUntil = o.T + 12
	return p.roam
}
func (p *Pilot) waypoint(o *Obs, goal V2) V2 {
	if p.w == nil || p.w.nav == nil {
		return goal
	}
	if o.T < p.navUntil && goal.Sub(p.navTarget).Len() < 9 && p.navPoint.Sub(xy(o.Self.Pos)).Len() > 5 {
		return p.navPoint
	}
	p.navPoint = p.w.nav.waypoint(xy(o.Self.Pos), goal)
	p.navTarget = goal
	p.navUntil = o.T + 1.0
	return p.navPoint
}
func (p *Pilot) move(o *Obs, goal V2, mode string, target *Actor, danger float64) (V2, bool, bool) {
	me := &o.Self
	xyMe := xy(me.Pos)
	wp := p.waypoint(o, goal)
	if wp.Sub(xyMe).Len() < 2 {
		wp = goal
	}
	heading := wp.Sub(xyMe).Unit()
	v := xy(me.Vel)
	best := -1e12
	mv := V2{}
	for i := 0; i < 20; i++ {
		theta := float64(i) * 2 * math.Pi / 19.0
		dir := V2{math.Cos(theta), math.Sin(theta)}
		if i == 19 {
			dir = V2{}
		}
		// 0.7s lookahead; grounded mechs accelerate rapidly but have inertia
		vPred := v.Add(dir.Mul(num(me.Stats, "ground_accel", 58.5) * 0.15))
		speed := num(me.Stats, "walk_speed", 11.7)
		if vPred.Len() > speed {
			vPred = vPred.Unit().Mul(speed)
		}
		q := xyMe.Add(vPred.Mul(0.7))
		if i == 19 {
			q = xyMe.Add(v.Mul(0.24))
		}
		score := heading.Dot(dir) * 48.0
		if i == 19 {
			score -= 20
		}
		if mode == "retreat" || mode == "heal" {
			score += heading.Dot(dir) * 35
		}
		if p.w != nil {
			if !p.w.walk(q[0], q[1]) {
				score -= 260
			}
			if !p.w.nav.direct(xyMe, q) {
				score -= 135
			}
		}
		futureRadius := o.Zone.NextRadius
		if futureRadius > 0 {
			out := q.Sub(o.Zone.NextCenter).Len() - futureRadius + 7
			if out > 0 {
				score -= math.Min(230, 6*out)
			}
		}
		for _, a := range o.Mechs {
			if a.Kind != "mech" {
				continue
			}
			d := q.Sub(xy(a.Pos)).Len()
			threat := p.threatValue(o, a)
			if d < 65 {
				score -= math.Max(0, 65-d) * threat * 0.85
			}
			if mode == "retreat" || mode == "heal" {
				if p.exposedFrom(me, q, a) {
					score -= 65 * threat
				} else {
					score += 48 * threat
				}
			}
			if a.Charging && a.Has("laser") && d < 250 {
				toward := angle(q.Sub(xy(a.Pos)))
				if math.Abs(wrap(a.Aim[0]-toward)) < 0.12 {
					score -= 280
				}
			}
		}
		if target != nil && mode == "fight" {
			d := q.Sub(xy(target.Pos)).Len()
			pref := 100.0
			if p.style == "stalker" {
				pref = 33
			}
			if p.style == "artillery" {
				pref = 150
			}
			if (p.has(me, "lightning") && me.Energy > 20) && p.style != "artillery" {
				pref = 34
			}
			score -= math.Abs(d-pref) * 0.55
			// Attack from the opponent's rear arc. The opposite vector of their facing is ideal.
			behind := V2{math.Cos(target.Aim[0]), math.Sin(target.Aim[0])}.Mul(-1)
			fromEnemy := q.Sub(xy(target.Pos)).Unit()
			score += 24 * fromEnemy.Dot(behind)
			if p.los(me, *target) && p.exposedFrom(me, q, *target) {
				score += 16
			}
		}
		// dodge predicted projectiles, including mortar landing circles
		for _, s := range o.Projectiles {
			delta := xyMe.Sub(xy(s.Pos))
			vel := xy(s.Vel)
			v2 := vel.Dot(vel)
			if s.Kind == "mortar" {
				grav := 22.0
				z0 := s.Pos[2] - me.Pos[2]
				vz := s.Vel[2]
				disc := vz*vz + 2*grav*z0
				if disc < 0 {
					continue
				}
				t := (vz + math.Sqrt(disc)) / grav
				if t > 0 && t < 1.5 {
					impact := xy(s.Pos).Add(vel.Mul(t))
					pred := xyMe.Add(vPred.Mul(t))
					if pred.Sub(impact).Len() < 14 {
						score -= 280 * (1 - pred.Sub(impact).Len()/14)
					}
				}
				continue
			}
			if v2 < 20 {
				continue
			}
			t := clamp(delta.Dot(vel)/v2, 0, 0.9)
			if t < 0.05 {
				continue
			}
			bullet := xy(s.Pos).Add(vel.Mul(t))
			pred := xyMe.Add(vPred.Mul(t))
			if bullet.Sub(pred).Len() < 8 && math.Abs(s.Pos[2]+s.Vel[2]*t-(me.Pos[2]+num(me.Stats, "height", 9)*0.5)) < 8 {
				score -= 300 * (1 - bullet.Sub(pred).Len()/8)
			}
		}
		score += p.lastMove.Dot(dir) * 5.0
		if score > best {
			best, mv = score, dir
		}
	}
	boost := false
	if (mode == "zone" || mode == "retreat") && wp.Sub(xyMe).Len() > 15 && me.Energy > 30 {
		boost = true
	}
	if mode == "fight" && p.style == "stalker" && target != nil && xy(target.Pos).Sub(xyMe).Len() > 55 && me.Energy > 55 {
		boost = true
	}
	if mode == "loot" && goal.Sub(xyMe).Len() > 75 && danger < 0.7 && me.Energy > 78 {
		boost = true
	}
	// trapped navigation: periodically jump and jet over obstacles
	moved := xyMe.Sub(p.lastPos).Len()
	if o.T-p.lastTime < 0.4 && mv.Len() > 0.5 && moved < 0.2 {
		p.stuck += 0.1
	} else {
		p.stuck = math.Max(0, p.stuck-0.1)
	}
	jump := p.stuck > 1.1 && me.Grounded && me.Energy > 40
	if p.stuck > 1.4 {
		p.navUntil = 0
		p.safeUntil = 0
		p.roamUntil = 0
		p.stuck = 0
	}
	if jump {
		boost = true
	}
	p.lastPos = xyMe
	p.lastTime = o.T
	p.lastMove = mv
	return mv, boost, jump
}
func (p *Pilot) has(me *Self, kind string) bool { w, _ := p.weapon(me, kind); return w != nil }
