package main

import "math"

// Leader is a deliberately aggressive sparring policy, modeled on the
// *reported* AT/Mustikas behaviors rather than copied from any team's code.
// It is an opponent test fixture, NOT a validated reproduction of either team.
const leaderSpin = 2 * math.Pi / 1.8 // full observation sweep in ~1.8 s

func (p *Pilot) leaderEnemy(o *Obs) (*Actor, bool, float64) {
	me := &o.Self
	pos := xy(me.Pos)
	best := -1e9
	var target *Actor
	danger := 0.0
	for i := range o.Mechs {
		a := &o.Mechs[i]
		if a.Kind != "mech" {
			continue
		}
		d := pos.Sub(xy(a.Pos)).Len()
		if d > 260 {
			continue
		}
		danger += p.threatValue(o, *a)
		if !p.los(me, *a) && d > 75 {
			continue
		}
		weak := (a.HP + 80) / math.Max(1, a.MaxHP)
		meFrac := (me.HP + 80) / math.Max(1, me.MaxHP)
		score := 170 - 0.63*d + 310*(1-weak) + 16*math.Min(a.Cubes, 10)
		if a.ID == p.targetID {
			score += 95
		}
		if a.ID == p.lastThreat && o.T-p.lastDamage < 7 {
			score += 160
		}
		if rec := p.memory[a.ID]; rec != nil && o.T-rec.damaged < 2.8 {
			score += 110
		}
		if weak > meFrac*1.2 && a.Cubes >= me.Cubes+3 {
			score -= 180
		}
		if score > best {
			best, target = score, a
		}
	}
	if target == nil {
		return nil, false, danger
	}
	d := pos.Sub(xy(target.Pos)).Len()
	weak := target.HP < 0.8*target.MaxHP || target.HP < me.HP*0.87
	threatening := target.ID == p.lastThreat && o.T-p.lastDamage < 5
	armed := p.has(me, "laser") || p.has(me, "lightning")
	// A leader challenges peers when armed; avoids feeding cubes to a huge snowball.
	good := d < 205 && (weak || threatening || (armed && me.HP > 0.56*me.MaxHP && target.Cubes <= me.Cubes+3))
	if danger > 3.7 && !threatening && target.HP > 180 {
		good = false
	}
	if me.HP < 0.35*me.MaxHP && !threatening {
		good = false
	}
	return target, good, danger
}

func (p *Pilot) leaderLoot(o *Obs) (V2, string, float64) {
	me := &o.Self
	pos := xy(me.Pos)
	best := -1e9
	var point V2
	kind := ""
	eval := func(pos3 V3, k string, value float64) {
		q := xy(pos3)
		d := q.Sub(pos).Len()
		if d > 165 || p.zoneDistance(o.Zone, q) > -5 {
			return
		}
		if p.w != nil && (math.Abs(p.w.ground(q[0], q[1])-pos3[2]) > 3.3 || !p.w.walk(q[0], q[1])) {
			return
		}
		s := value - d*1.15
		for _, a := range o.Mechs {
			if a.Kind != "mech" {
				continue
			}
			ad := q.Sub(xy(a.Pos)).Len()
			if ad < 65 && a.HP > me.HP*1.3 {
				s -= (65 - ad) * 2.3
			}
		}
		if s > best {
			best, kind, point = s, k, q
		}
	}
	for _, a := range o.Mechs {
		if a.Kind == "npc" && a.HP > 0 && me.HP > 0.48*me.MaxHP {
			eval(a.Pos, "npc", 248)
		}
	}
	for _, c := range o.Crates {
		if me.HP > 0.35*me.MaxHP && c.HP > 0 {
			eval(c.Pos, "crate", 190)
		}
	}
	for _, c := range o.Cubes {
		eval(c.Pos, "cube", 420)
	}
	for _, w := range o.Pickups {
		switch w.Kind {
		case "laser":
			if !p.has(me, "laser") {
				eval(w.Pos, "weapon", 430)
			}
		case "lightning":
			if !p.has(me, "lightning") {
				eval(w.Pos, "weapon", 405)
			}
		}
	}
	for _, d := range o.Drops {
		if d.LandsIn < 22 && (d.Holder == nil || *d.Holder == me.ID) {
			eval(d.Pos, "drop", 320)
		}
	}
	if best < 1 {
		return V2{}, "", best
	}
	return point, kind, best
}

func (p *Pilot) leaderFarm(o *Obs, a *Action, kind string, goal V2) bool {
	me := &o.Self
	var pt V3
	var actor *Actor
	found := false
	if kind == "npc" {
		for i := range o.Mechs {
			n := &o.Mechs[i]
			if n.Kind == "npc" && xy(n.Pos).Sub(goal).Len() < 20 && p.los(me, *n) {
				actor = n
				pt = p.lead(me, *n, 252)
				found = true
				break
			}
		}
	} else if kind == "crate" {
		for _, c := range o.Crates {
			if xy(c.Pos).Sub(goal).Len() < 12 {
				pt = V3{c.Pos[0], c.Pos[1], c.Pos[2] + 1.2}
				found = true
				break
			}
		}
	}
	if !found || !p.clearShot(me, pt) {
		return false
	}
	d := xy(pt).Sub(xy(me.Pos)).Len()
	cmd := p.aimAngles(me, pt)
	a.Aim = cmd
	las, li := p.weapon(me, "laser")
	if las != nil && li >= 0 && d < num(las.Stats, "range", 280)-5 {
		if las.Charge > 0 {
			a.Fire[li] = true
			if las.Charge >= num(las.Stats, "charge", 0.75)-0.08 && p.aligned(me, cmd, math.Max(0.02, math.Atan2(1.1, d))) {
				a.Fire[li] = false
			}
		} else if las.Cooldown <= 0.11 {
			a.Fire[li] = true
		}
	}
	if gun, gi := p.weapon(me, "gun"); gun != nil && gi >= 0 && gun.Ammo > 0 && gun.Reload <= 0 && d < num(gun.Stats, "range", 230)-5 && p.aligned(me, cmd, math.Max(0.03, math.Atan2(1.5, d))) && (las == nil || las.Charge <= 0) {
		a.Fire[gi] = true
	}
	if ltn, ni := p.weapon(me, "lightning"); ltn != nil && ni >= 0 && d < num(ltn.Stats, "range", 45)-3 && ltn.Cooldown <= 0.1 && me.Energy > num(ltn.Stats, "energy", 3)+16 && p.aligned(me, cmd, 0.08) {
		a.Fire[ni] = true
	}
	_ = actor
	return true
}

func (p *Pilot) leaderAct(o *Obs) Action {
	me := &o.Self
	p.remember(o)
	a := Action{Tick: o.Tick, Aim: me.Aim}
	p.upgrade(o, &a)
	p.pickup(o, &a)
	enemy, engage, danger := p.leaderEnemy(o)
	p.targetID = -1
	if enemy != nil {
		p.targetID = enemy.ID
	}
	hp := me.HP / math.Max(1, me.MaxHP)
	if hp < 0.38 && (danger > 0.8 || o.T-p.lastDamage < 4) {
		p.retreatUntil = o.T + 5
	}
	if hp > 0.88 {
		p.retreatUntil = 0
	}
	goal := p.getRoam(o)
	mode := "roam"
	kind := ""
	lootGoal := V2{}
	if p.zoneUrgency(o) {
		mode = "zone"
		goal = p.nearZoneGoal(o)
	} else if o.T < p.retreatUntil || (hp < 0.65 && danger > 2.5) {
		mode = "retreat"
		goal = p.safeSpot(o)
	} else if engage && enemy != nil {
		mode = "fight"
		delta := xy(me.Pos).Sub(xy(enemy.Pos)).Unit()
		if delta.Len() < 0.1 {
			delta = V2{1, 0}
		}
		// Close to ~35m, within lightning's 45m range, but outside collision radius.
		goal = xy(enemy.Pos).Add(delta.Mul(34))
	} else {
		q, k, v := p.leaderLoot(o)
		if v > 0 {
			mode = "loot"
			goal = q
			lootGoal = q
			kind = k
		}
	}
	// Stand off from crates: trying to stand on one blocks the gun's downward pitch.
	if mode == "loot" && (kind == "crate" || kind == "npc") {
		d := xy(me.Pos).Sub(goal)
		preferred := 22.0
		if kind == "npc" {
			preferred = 36
		}
		if d.Len() < preferred {
			if d.Len() < 0.5 {
				d = V2{1, 0}
			}
			goal = goal.Add(d.Unit().Mul(preferred))
		}
	}
	if gun, _ := p.weapon(me, "gun"); gun != nil && gun.Reload <= 0 && gun.Ammo <= 0 {
		a.Reload = true
	}
	a.Move, a.Boost, a.Jump = p.move(o, goal, mode, enemy, danger)
	// Save energy for close-range lightning once combat is underway.
	if mode == "fight" && me.Energy < 42 {
		a.Boost = false
	}
	if las, _ := p.weapon(me, "laser"); las != nil && las.Charge > 0 && mode != "zone" {
		a.Boost = false
	}
	if enemy != nil && (engage || (enemy.ID == p.lastThreat && o.T-p.lastDamage < 4)) && mode != "retreat" {
		p.attack(o, &a, enemy, "fight", "", V2{})
	} else if mode == "loot" && (kind == "npc" || kind == "crate") {
		p.leaderFarm(o, &a, kind, lootGoal)
	} else {
		// 360-degree scan ~1.8s/revolution; disable while holding a charge.
		if las, li := p.weapon(me, "laser"); las != nil && las.Charge > 0 {
			a.Fire[li] = true
		}
		if enemy != nil && danger > 1.3 {
			a.Aim = p.aimAngles(me, targetPoint(*enemy))
		} else {
			a.Aim = V2{wrap(o.T*leaderSpin + float64(p.h.Slot)*0.57), -0.04}
		}
	}
	return a
}