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

// firstStrikeCover ranks reachable nearby positions by whether they break
// muzzle LOS from observed opponents. It is intentionally conservative:
// a cover position buys enough time for the game's five-second regen delay.
func (p *Pilot) firstStrikeCover(o *Obs) V2 {
	me := &o.Self
	pos := xy(me.Pos)
	if p.w == nil {
		return p.safeSpot(o)
	}
	// Refresh often as enemies move, but don't oscillate goals every tick.
	if o.T < p.safeUntil && p.zoneDistance(o.Zone, p.safe) < -8 {
		return p.safe
	}
	best := pos
	bestScore := -1e15
	for i := 0; i < 49; i++ {
		q := pos
		if i > 0 {
			j := i - 1
			radius := 14.0 + float64(j/12)*15.0
			ang := (float64(j%12)/12)*2*math.Pi + float64(p.h.Slot)*0.13
			q = pos.Add(V2{math.Cos(ang), math.Sin(ang)}.Mul(radius))
		}
		if !p.w.walk(q[0], q[1]) || p.zoneDistance(o.Zone, q) > -8 {
			continue
		}
		travel := q.Sub(pos).Len()
		score := -travel * 1.0
		if p.w.nav != nil && !p.w.nav.direct(pos, q) {
			score -= 55 // circuitous route may cost the whole regen window
		}
		for _, foe := range o.Mechs {
			if foe.Kind != "mech" {
				continue
			}
			d := q.Sub(xy(foe.Pos)).Len()
			if d < 52 {
				score -= 7 * (52 - d)
			}
			threat := p.threatValue(o, foe)
			if p.exposedFrom(me, q, foe) {
				score -= 300 * math.Max(0.35, threat)
			} else {
				score += 105 * math.Max(0.35, threat)
			}
		}
		if score > bestScore {
			bestScore, best = score, q
		}
	}
	p.safe, p.safeUntil = best, o.T+1.2
	return best
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
	// Hysteresis: once badly wounded, stop dealing damage until mostly healed.
	// Regen starts 5 s after the last damage dealt OR taken, at 45 HP/s.
	// We enter slightly before the 35% critical threshold to leave one-tick
	// margin against the next projectile; 82% exit prevents oscillation.
	if hp <= 0.38 {
		p.firstStrikeHealing = true
	}
	if p.firstStrikeHealing && hp >= 0.82 && o.T-p.lastDamage >= 5 && o.T-p.lastDealt >= 5 {
		p.firstStrikeHealing = false
	}
	mode, goal := "roam", p.getRoam(o)
	lootKind := ""
	lootGoal := V2{}
	urgent := p.zoneUrgency(o)
	if p.firstStrikeHealing {
		// Zone movement takes priority over holding cover: no HP regen outside.
		if urgent {
			mode, goal = "zone", p.nearZoneGoal(o)
		} else {
			goal = p.firstStrikeCover(o)
			mode = "retreat"
			if goal.Sub(pos).Len() < 5 {
				mode = "heal"
			}
		}
	} else if urgent {
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
	if p.firstStrikeHealing {
		// Moving is fine while healing, but unnecessary movement can leave cover.
		if mode == "heal" {
			a.Move, a.Boost, a.Jump = V2{}, false, false
		}
		// A healing mech must not trigger gun/laser/lightning or farm resources:
		// dealing ANY damage resets the 5-second regeneration countdown.
		// It may still drop a weapon if a pickup action was already chosen.
		a.Reload = false
		if target != nil {
			a.Aim = p.aimAngles(me, targetPoint(*target))
		} else if rec := p.memory[p.lastThreat]; rec != nil {
			a.Aim = p.aimAngles(me, targetPoint(rec.actor))
		}
		return a
	}
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
