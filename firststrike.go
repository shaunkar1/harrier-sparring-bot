package main

import "math"

// Firststrike: an unvalidated but reproducible long-range sparring adversary.
// Its policy is independent of the competition bot and based on recorded
// opening patterns, not on copying any other team's implementation.
const firstStrikeSpin = 2 * math.Pi / 1.8

func (p *Pilot) firstStrikeLaser(o *Obs) (V2, bool) {
	if p.has(&o.Self, "laser") {
		return V2{}, false
	}
	pos := xy(o.Self.Pos)
	best := V2{}
	bestDist := 1e9
	for _, it := range o.Pickups {
		if it.Kind != "laser" {
			continue
		}
		q := xy(it.Pos)
		d := q.Sub(pos).Len()
		if d > 250 || math.Abs(it.Pos[2]-o.Self.Pos[2]) > 4 || p.zoneDistance(o.Zone, q) > -5 {
			continue
		}
		if p.w != nil && (!p.w.walk(q[0], q[1]) || p.w.nav == nil) {
			continue
		}
		if d < bestDist {
			bestDist, best = d, q
		}
	}
	return best, bestDist < 1e8
}

func (p *Pilot) firstStrikeEnemy(o *Obs) (*Actor, float64) {
	me := &o.Self
	pos := xy(me.Pos)
	var best *Actor
	bs := -1e9
	danger := 0.0
	for i := range o.Mechs {
		e := &o.Mechs[i]
		if e.Kind != "mech" {
			continue
		}
		d := xy(e.Pos).Sub(pos).Len()
		danger += p.threatValue(o, *e)
		if d > 270 || !p.los(me, *e) {
			continue
		}
		frac := e.HP / math.Max(1, e.MaxHP)
		s := 260 - d*0.35 + 360*(1-frac)
		if d >= 110 && d <= 260 {
			s += 220
		}
		brg := angle(pos.Sub(xy(e.Pos)))
		if math.Abs(wrap(e.Aim[0]-brg)) > 0.5 {
			s += 150
		}
		if !e.Has("laser") {
			s += 95
		}
		if e.ID == p.lastThreat && o.T-p.lastDamage < 7 {
			s += 280
		}
		if e.ID == p.targetID {
			s += 80
		}
		if frac < 0.3 {
			s += 290
		}
		if s > bs {
			bs, best = s, e
		}
	}
	return best, danger
}

func (p *Pilot) firstStrikeAct(o *Obs) Action {
	me := &o.Self
	p.remember(o)
	a := Action{Tick: o.Tick, Aim: me.Aim}
	p.upgrade(o, &a)
	p.pickup(o, &a)
	target, danger := p.firstStrikeEnemy(o)
	p.targetID = -1
	if target != nil {
		p.targetID = target.ID
	}
	pos := xy(me.Pos)
	hp := me.HP / math.Max(1, me.MaxHP)
	mode, goal := "roam", p.getRoam(o)
	lootKind := ""
	lootGoal := V2{}
	urgent := p.zoneUrgency(o)
	if urgent {
		mode, goal = "zone", p.nearZoneGoal(o)
	} else if target != nil && target.HP/math.Max(1, target.MaxHP) >= 0.30 && pos.Sub(xy(target.Pos)).Len() < 90 {
		mode = "retreat"
		away := pos.Sub(xy(target.Pos)).Unit()
		if away.Len() < 0.5 {
			away = V2{1, 0}
		}
		goal = pos.Add(away.Mul(100))
		if p.w != nil && !p.w.walk(goal[0], goal[1]) {
			goal = p.safeSpot(o)
		}
	} else if hp < 0.52 && o.T-p.lastDamage < 7 && danger > 0.65 {
		mode, goal = "retreat", p.safeSpot(o)
	} else if q, ok := p.firstStrikeLaser(o); ok {
		mode, goal = "loot", q
		lootKind, lootGoal = "weapon", q
	} else if target != nil {
		mode = "fight"
		away := pos.Sub(xy(target.Pos)).Unit()
		if away.Len() < 0.5 {
			away = V2{1, 0}
		}
		standoff := 145.0
		if target.HP/math.Max(1, target.MaxHP) < 0.3 {
			standoff = 65
		}
		goal = xy(target.Pos).Add(away.Mul(standoff)).Add(perp(away).Mul(22*math.Sin(o.T*0.6+float64(p.h.Slot))))
	} else {
		if q, kind, value := p.chooseLoot(o); value > 0 {
			mode, goal = "loot", q
			lootKind, lootGoal = kind, q
			if kind == "crate" || kind == "npc" {
				pref := 22.0
				if kind == "npc" {
					pref = 35
				}
				from := pos.Sub(goal)
				if from.Len() < pref {
					if from.Len() < 0.5 {
						from = V2{1, 0}
					}
					goal = goal.Add(from.Unit().Mul(pref))
				}
			}
		}
	}
	if gun, _ := p.weapon(me, "gun"); gun != nil && gun.Reload <= 0 {
		if gun.Ammo <= 0 || (gun.Ammo < num(gun.Stats, "mag", 12)*0.35 && target == nil && lootKind != "crate" && lootKind != "npc") {
			a.Reload = true
		}
	}
	a.Move, a.Boost, a.Jump = p.move(o, goal, mode, target, danger)
	if l, _ := p.weapon(me, "laser"); l != nil && l.Charge > 0 && !urgent {
		a.Boost = false
	}
	if target != nil && p.los(me, *target) && xy(target.Pos).Sub(pos).Len() <= 260 {
		// Continue firing while kiting; do not surrender free laser/gun damage
		// merely because a healthy short-range enemy has closed on us.
		p.attack(o, &a, target, "fight", "", V2{})
	} else if mode == "loot" && (lootKind == "crate" || lootKind == "npc") {
		p.leaderFarm(o, &a, lootKind, lootGoal)
	} else if o.T-p.lastDamage < 6 && p.lastThreat >= 0 {
		if rec := p.memory[p.lastThreat]; rec != nil {
			a.Aim = p.aimAngles(me, targetPoint(rec.actor))
		} else {
			a.Aim = V2{wrap(o.T*firstStrikeSpin + float64(p.h.Slot)*0.31), -0.05}
		}
	} else {
		a.Aim = V2{wrap(o.T*firstStrikeSpin + float64(p.h.Slot)*0.31), -0.05}
	}
	return a
}
