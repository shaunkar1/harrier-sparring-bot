package main

import "math"

func (p *Pilot) pickup(o *Obs, a *Action) {
	me := &o.Self
	maxDist := p.h.Config.Loot.PickupRadius
	if maxDist < 1 {
		maxDist = 4
	}
	score := 0.0
	bestID := -1
	swap := -1
	for _, item := range o.Pickups {
		d := xy(item.Pos).Sub(xy(me.Pos)).Len()
		if d >= maxDist-0.25 || math.Abs(item.Pos[2]-me.Pos[2]) > 3.5 {
			continue
		}
		if p.has(me, item.Kind) || item.Kind == "gun" {
			continue
		}
		value := 0.0
		switch item.Kind {
		case "laser":
			value = 100
		case "lightning":
			value = 85
		case "mortar":
			value = 40
		}
		if p.style == "artillery" && item.Kind == "mortar" {
			value = 120
		}
		if (p.style == "stalker" || p.style == "leader") && item.Kind == "lightning" {
			value = 145
		}
		if item.Kind == "mortar" && (p.has(me, "laser") || p.style == "leader") {
			continue
		} // avoid cookoff
		if item.Kind == "laser" && p.has(me, "mortar") {
			value += 50
		}
		slot := -1
		replaceValue := 1000.0
		for j, w := range me.Weapons {
			if w == nil {
				slot = -1
				replaceValue = -1
				break
			}
			v := 30.0
			switch w.Kind {
			case "gun":
				v = 38
			case "mortar":
				v = 45
			case "lightning":
				v = 83
			case "laser":
				v = 90
			}
			if p.style == "artillery" && w.Kind == "mortar" {
				v = 125
			}
			if p.style == "stalker" && w.Kind == "lightning" {
				v = 130
			}
			if w.Kind == "laser" && w.Charge > 0 {
				v = 200
			}
			if v < replaceValue {
				replaceValue = v
				slot = j
			}
		}
		// A laser + mortar combination is vulnerable to the laser cook-off
		// modifier. Swap out the mortar even if artillery would usually keep it.
		if item.Kind == "laser" && p.has(me, "mortar") {
			_, slot = p.weapon(me, "mortar")
			replaceValue = 0
		}
		if replaceValue > value {
			continue
		}
		if p.style == "harrier" && item.Kind == "lightning" && p.has(me, "laser") {
			value += 35
		}
		if value > score {
			score = value
			bestID = item.ID
			swap = slot
		}
	}
	if bestID >= 0 {
		a.Take = &bestID
		if swap >= 0 {
			a.Drop = &swap
		}
	}
}
func (p *Pilot) upgrade(o *Obs, a *Action) {
	if o.Offer == nil || len(o.Offer.Choices) == 0 {
		return
	}
	prio := []string{"plating", "armor", "nanites", "laser_charge", "laser_damage", "gun_rate", "servos", "lightning_range", "lightning_efficiency", "laser_pierce", "gun_damage", "reactor", "jets", "gyros", "sensors", "capacitor", "gun_mag", "mortar_splash", "mortar_reload"}
	if p.style == "leader" {
		prio = []string{"laser_damage", "laser_charge", "lightning_efficiency", "lightning_range", "plating", "armor", "nanites", "servos", "reactor", "capacitor", "gun_damage", "gun_rate", "jets", "gyros"}
	}
	if p.style == "stalker" {
		prio = []string{"lightning_range", "lightning_efficiency", "plating", "armor", "servos", "nanites", "laser_charge", "laser_damage", "reactor", "gun_rate", "jets", "gyros"}
	}
	if p.style == "artillery" {
		prio = []string{"mortar_splash", "mortar_reload", "mortar_velocity", "plating", "armor", "nanites", "gun_rate", "gun_damage", "servos", "reactor", "jets", "gyros"}
	}
	bestIdx := 0
	best := 999
	for i, c := range o.Offer.Choices {
		rank := 500
		for j, k := range prio {
			if c.ID == k {
				rank = j
				break
			}
		}
		if rank < best {
			best, bestIdx = rank, i
		}
	}
	a.Pick = &bestIdx
}
func (p *Pilot) aimAngles(me *Self, pt V3) V2 {
	m := p.muzzle(me)
	delta := V3{pt[0] - m[0], pt[1] - m[1], pt[2] - m[2]}
	return V2{math.Atan2(delta[1], delta[0]), math.Atan2(delta[2], math.Hypot(delta[0], delta[1]))}
}
func (p *Pilot) lead(me *Self, act Actor, speed float64) V3 {
	target := targetPoint(act)
	origin := p.muzzle(me)
	d := math.Hypot(target[0]-origin[0], target[1]-origin[1])
	if speed < 5 {
		speed = 252
	}
	t := clamp(d/speed+0.08, 0.04, 1.5)
	for j := 0; j < 2; j++ {
		target = targetPoint(act)
		target[0] += act.Vel[0] * t
		target[1] += act.Vel[1] * t
		target[2] += act.Vel[2] * t
		d = math.Hypot(target[0]-origin[0], target[1]-origin[1])
		t = clamp(d/speed+0.08, 0.04, 1.5)
	}
	return target
}
func (p *Pilot) aligned(me *Self, cmd V2, tol float64) bool {
	rate := num(me.Stats, "turn_rate", 4)
	prate := num(me.Stats, "pitch_rate", 4)
	// use predicted torso after the next decision window, not desired yaw only
	yawGap := math.Abs(wrap(cmd[0] - me.Aim[0]))
	pitchGap := math.Abs(cmd[1] - me.Aim[1])
	return yawGap <= tol+rate*0.07 && pitchGap <= tol+prate*0.07
}
func (p *Pilot) clearShot(me *Self, pt V3) bool {
	if p.w == nil {
		return true
	}
	return p.w.clear(p.muzzle(me), pt)
}
func (p *Pilot) ballistic(me *Self, a Actor, ws *WState) (V2, bool) {
	muzzle := p.muzzle(me)
	v := num(ws.Stats, "speed", 72)
	g := num(ws.Stats, "gravity", 22)
	if v <= 5 || g <= 0 {
		return V2{}, false
	}
	for _, high := range []bool{false, true} {
		tFlight := 0.0
		target := targetPoint(a)
		var theta, d float64
		ok := false
		for it := 0; it < 3; it++ {
			dx, dy := target[0]-muzzle[0], target[1]-muzzle[1]
			d = math.Hypot(dx, dy)
			if d < 24 || d > 210 {
				break
			}
			h := target[2] - muzzle[2]
			disc := v*v*v*v - g*(g*d*d+2*h*v*v)
			if disc < 0 {
				break
			}
			root := math.Sqrt(disc)
			tan := (v*v - root) / (g * d)
			if high {
				tan = (v*v + root) / (g * d)
			}
			theta = math.Atan(tan)
			if theta < 0.01 || theta > 1.45 {
				break
			}
			tFlight = d / (v * math.Cos(theta))
			if tFlight > 4.1 || (high && tFlight > 3.25) {
				break
			}
			target = targetPoint(a)
			target[0] += a.Vel[0] * tFlight
			target[1] += a.Vel[1] * tFlight
			ok = true
		}
		if !ok {
			continue
		}
		cmd := V2{math.Atan2(target[1]-muzzle[1], target[0]-muzzle[0]), theta}
		// ballistic trajectory, clearing obstacles, with muzzle offset 3m
		c, s := math.Cos(cmd[0]), math.Sin(cmd[0])
		ct, st := math.Cos(theta), math.Sin(theta)
		pathOk := true
		for t := 0.09; t < tFlight-0.12; t += 0.12 {
			pos := V3{muzzle[0] + c*(3+v*ct*t), muzzle[1] + s*(3+v*ct*t), muzzle[2] + v*st*t - 0.5*g*t*t}
			if p.w != nil && p.w.solid(pos[0], pos[1], pos[2]) {
				pathOk = false
				break
			}
		}
		if pathOk {
			return cmd, true
		}
	}
	return V2{}, false
}
func (p *Pilot) attack(o *Obs, a *Action, enemy *Actor, mode string, lootKind string, lootGoal V2) {
	me := &o.Self
	// Turning away during retreat exposes rear armor. Keep the nearest danger in front.
	a.Aim = V2{math.Mod(o.T*0.37+float64(p.h.Slot), 2*math.Pi) - math.Pi, -0.05}
	if enemy != nil {
		a.Aim = p.aimAngles(me, targetPoint(*enemy))
	}
	if enemy == nil {
		// NPCs and crates support a different fighting style from tracking opposing mechs.
		if lootKind == "npc" {
			var chosen *Actor
			bd := 1e9
			for i := range o.Mechs {
				m := &o.Mechs[i]
				if m.Kind == "npc" {
					d := xy(m.Pos).Sub(lootGoal).Len()
					if d < bd {
						bd, chosen = d, m
					}
				}
			}
			if chosen != nil && bd < 25 && p.los(me, *chosen) {
				enemy = chosen
			}
		} else if lootKind == "crate" {
			var chosen *Item
			bd := 1e9
			for i := range o.Crates {
				m := &o.Crates[i]
				d := xy(m.Pos).Sub(lootGoal).Len()
				if d < bd {
					bd, chosen = d, m
				}
			}
			if chosen != nil && bd < 20 {
				pt := V3{chosen.Pos[0], chosen.Pos[1], chosen.Pos[2] + 1.3}
				a.Aim = p.aimAngles(me, pt)
				w, i := p.weapon(me, "gun")
				if w != nil && w.Reload <= 0 && w.Ammo > 0 && p.clearShot(me, pt) && p.aligned(me, a.Aim, 0.035) {
					a.Fire[i] = true
				}
			}
			return
		}
	}
	if enemy == nil {
		return
	}
	d := xy(enemy.Pos).Sub(xy(me.Pos)).Len()
	if mode == "heal" || (mode == "retreat" && me.HP/me.MaxHP < 0.7 && o.T-p.lastDamage > 1) {
		return
	}
	// Do not interrupt 5-second repair timer just for low-value NPC damage.
	if enemy.Kind == "npc" && me.HP/me.MaxHP < 0.75 {
		return
	}
	laser, li := p.weapon(me, "laser")
	mortar, mi := p.weapon(me, "mortar")
	gun, gi := p.weapon(me, "gun")
	lightning, ni := p.weapon(me, "lightning")
	los := p.los(me, *enemy)
	// Lightning causes a short on laser users; always use it before they release a charge.
	if lightning != nil && ni >= 0 && lightning.Cooldown <= 0.11 && me.Energy > num(lightning.Stats, "energy", 3)+8 && d < num(lightning.Stats, "range", 45)-3 && los {
		pt := targetPoint(*enemy)
		cmd := p.aimAngles(me, pt)
		if math.Abs(wrap(cmd[0]-me.Aim[0])) < 0.36 || p.aligned(me, cmd, 0.08) {
			a.Aim = cmd
			a.Fire[ni] = true
		}
	}
	// Laser: start charging before a favorable opening; never shoot into a roof edge.
	if laser != nil && li >= 0 {
		chargeT := num(laser.Stats, "charge", 0.75)
		pt := p.lead(me, *enemy, 3000)
		cmd := p.aimAngles(me, pt)
		if laser.Charge > 0 {
			a.Fire[li] = true
			if d < num(laser.Stats, "range", 280)-7 && los && p.clearShot(me, pt) {
				a.Aim = cmd
				if laser.Charge >= chargeT-0.09 && p.aligned(me, cmd, math.Max(0.019, math.Atan2(2.0, d)*0.75)) {
					a.Fire[li] = false
				}
			}
		} else if enemy.Kind == "mech" && d < 260 && los && laser.Cooldown <= 0.09 && mode != "heal" && mode != "retreat" {
			a.Aim = cmd
			a.Fire[li] = true
		}
	}
	// Mortar takes high arcs over cover; use it as a separate alternative opponent style.
	if mortar != nil && mi >= 0 && mortar.Cooldown <= 0.11 && enemy.Kind == "mech" && d > 32 && d < 205 {
		cmd, ok := p.ballistic(me, *enemy, mortar)
		if ok && (mode == "fight" || p.style == "artillery") && p.aligned(me, cmd, 0.03) {
			if laser == nil || laser.Charge <= 0 {
				a.Aim = cmd
				a.Fire[mi] = true
			}
		} else if ok && laser == nil && mode == "fight" {
			a.Aim = cmd
		}
	}
	// Gun: lead moving targets and check muzzle line for direct fire.
	if gun != nil && gi >= 0 && gun.Reload <= 0 && gun.Ammo > 0 && d < num(gun.Stats, "range", 230)-4 {
		pt := p.lead(me, *enemy, num(gun.Stats, "speed", 252))
		cmd := p.aimAngles(me, pt)
		if los && p.clearShot(me, pt) {
			if mortar == nil || !a.Fire[mi] {
				if laser == nil || laser.Charge <= 0 {
					a.Aim = cmd
				}
			}
			if p.aligned(me, cmd, math.Max(0.025, math.Atan2(1.7, d))) && math.Abs(wrap(a.Aim[0]-cmd[0])) < 0.08 {
				a.Fire[gi] = true
			}
		}
	}
}
func (p *Pilot) act(o *Obs) Action {
	if p.style == "leader" { return p.leaderAct(o) }
	me := &o.Self
	p.remember(o)
	a := Action{Tick: o.Tick, Aim: me.Aim}
	p.upgrade(o, &a)
	p.pickup(o, &a)

	enemy, good, danger := p.pickEnemy(o)
	p.targetID = -1
	if enemy != nil {
		p.targetID = enemy.ID
	}
	hp := me.HP / math.Max(me.MaxHP, 1)
	if hp < 0.43 && (danger > 0.8 || o.T-p.lastDamage < 6) {
		p.retreatUntil = o.T + 5.5
	}
	if danger > 3.2 && hp < 0.8 {
		p.retreatUntil = math.Max(p.retreatUntil, o.T+2.5)
	}
	if hp > 0.9 && danger < 0.9 {
		p.retreatUntil = 0
	}
	goal := p.getRoam(o)
	mode := "roam"
	lootKind := ""
	lootGoal := V2{}
	urgent := p.zoneUrgency(o)
	healing := hp < 0.78 && (o.T-p.lastDamage > 1.3 || o.T < p.retreatUntil)
	switch {
	case urgent:
		mode = "zone"
		goal = p.nearZoneGoal(o)
	case o.T < p.retreatUntil:
		mode = "retreat"
		goal = p.safeSpot(o)
	case healing && danger > 0.3:
		mode = "heal"
		goal = p.safeSpot(o)
	case good && enemy != nil:
		mode = "fight"
		goal = xy(enemy.Pos)
		if p.style == "harrier" && p.has(me, "laser") && !p.has(me, "lightning") {
			// The harrier uses cover and medium range instead of point-blank chases.
			away := xy(me.Pos).Sub(xy(enemy.Pos)).Unit()
			goal = xy(enemy.Pos).Add(away.Mul(100))
		}
	default:
		q, k, s := p.chooseLoot(o)
		if s > 0 {
			mode = "loot"
			goal = q
			lootKind = k
			lootGoal = q
		}
	}
	if mode == "loot" && lootKind == "drop" {
		for _, d := range o.Drops {
			if xy(d.Pos).Sub(goal).Len() < 4 && d.LandsIn <= 0 && xy(me.Pos).Sub(goal).Len() < math.Max(1, d.Radius-2) {
				goal = xy(me.Pos)
				break
			}
		}
	}
	// Reload in downtime rather than interrupting a live fight or crate farm.
	if gun, _ := p.weapon(me, "gun"); gun != nil && gun.Reload <= 0 {
		ammo := gun.Ammo
		mag := num(gun.Stats, "mag", 12)
		if ammo <= 0 || (ammo < mag*0.7 && mode != "fight" && mode != "zone" && lootKind != "crate" && lootKind != "npc" && danger < 0.7) {
			a.Reload = true
		}
	}
	if mode == "fight" && enemy != nil && danger > 2.8 && hp < 0.8 {
		mode = "retreat"
		goal = p.safeSpot(o)
	}
	a.Move, a.Boost, a.Jump = p.move(o, goal, mode, enemy, danger)
	// Never sustain boost during laser charging: it wastes energy with half speed.
	if laser, _ := p.weapon(me, "laser"); laser != nil && laser.Charge > 0 && mode != "zone" && mode != "retreat" {
		a.Boost = false
	}
	var fireTarget *Actor
	if (mode == "fight" || mode == "zone") && enemy != nil && p.los(me, *enemy) {
		fireTarget = enemy
	}
	if enemy != nil && enemy.ID == p.lastThreat && o.T-p.lastDamage < 2.0 && mode != "heal" {
		fireTarget = enemy
	}
	if mode == "loot" && lootKind == "npc" {
		fireTarget = nil
	}
	p.attack(o, &a, fireTarget, mode, lootKind, lootGoal)
	// Releasing the trigger on a charging laser fires immediately. If a
	// target vanishes behind cover, keep the charge rather than wasting it.
	if las, li := p.weapon(me, "laser"); las != nil && las.Charge > 0 && fireTarget == nil {
		a.Fire[li] = true
	}
	// If no target in view, keep torso oriented toward a known hostile to guard rear armor.
	if fireTarget == nil && enemy != nil && mode != "loot" {
		a.Aim = p.aimAngles(me, targetPoint(*enemy))
	}
	if a.Reload {
		if gun, idx := p.weapon(me, "gun"); gun != nil && a.Fire[idx] {
			a.Reload = false
		}
	}
	return a
}
