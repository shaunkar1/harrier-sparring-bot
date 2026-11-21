package main

import (
  "math"
  "math/rand"
  "testing"
)

func firstTestPilot() *Pilot {
  return &Pilot{style:"firststrike",h:Hello{Seed:1,Slot:1},rng:rand.New(rand.NewSource(1)),memory:map[int]*remembered{},lastThreat:-1,targetID:-1}
}
func firstTestObs(t float64) *Obs {
  return &Obs{Tick:int(t*10),T:t,Alive:16,
    Self:Self{ID:1,Pos:V3{0,0,0},HP:1400,MaxHP:1400,Energy:100,MaxEnergy:100,Grounded:true,InZone:true,
      Weapons:[]*WState{{Kind:"gun",Stats:Stats{"range":230,"speed":252,"mag":12},Ammo:12},nil},
      Stats:Stats{"walk_speed":11.7,"ground_accel":58.5,"muzzle_height":7,"turn_rate":4}},
    Zone:Zone{Center:V2{0,0},Radius:300,NextCenter:V2{0,0},NextRadius:300,ShrinkStart:180,ShrinkEnd:220}}
}
func TestFirstStrikeNearestLaser(t *testing.T) {
  p:=firstTestPilot();o:=firstTestObs(5)
  o.Pickups=[]Item{{ID:20,Kind:"laser",Pos:V3{130,0,0}},{ID:21,Kind:"laser",Pos:V3{44,0,0}},{ID:22,Kind:"mortar",Pos:V3{10,0,0}}}
  g,ok:=p.firstStrikeLaser(o);if !ok||g!=(V2{44,0}){t.Fatalf("nearest laser goal: %v %v",g,ok)}
  o.Self.Weapons[1]=&WState{Kind:"laser"};if _,ok=p.firstStrikeLaser(o);ok{t.Fatal("already has a laser")}
}
func TestFirstStrikeTargetsDistantExposedMech(t *testing.T) {
  p:=firstTestPilot();o:=firstTestObs(8)
  o.Mechs=[]Actor{{ID:2,Kind:"mech",Pos:V3{160,0,0},Aim:V2{math.Pi,0},HP:1400,MaxHP:1400},{ID:3,Kind:"mech",Pos:V3{290,0,0},HP:1400,MaxHP:1400}}
  target,_:=p.firstStrikeEnemy(o);if target==nil||target.ID!=2{t.Fatalf("distant visible target: %+v",target)}
  a:=p.firstStrikeAct(o);if a.Tick!=80{t.Fatalf("wrong tick %d",a.Tick)}
}
func TestFirstStrikeScanAndLastAttacker(t *testing.T) {
  p:=firstTestPilot();o:=firstTestObs(3)
  a:=p.firstStrikeAct(o);expected:=wrap(o.T*firstStrikeSpin+float64(p.h.Slot)*0.31)
  if math.Abs(wrap(a.Aim[0]-expected))>1e-5{t.Fatalf("scan yaw: %v want %.3f",a.Aim,expected)}
  p.memory[9]=&remembered{actor:Actor{ID:9,Kind:"mech",Pos:V3{100,20,0},MaxHP:1400,HP:1400},last:3}
  by:=9;o=firstTestObs(3.1);o.Events=[]Event{{Type:"damaged",By:&by,Weapon:"gun",Amount:100}}
  a=p.firstStrikeAct(o);if math.Abs(wrap(a.Aim[0]-math.Atan2(20,100)))>0.04{t.Fatalf("not facing attacker: %+v",a.Aim)}
}
func TestFirstStrikeNoBlindRushInside90(t *testing.T) {
  p:=firstTestPilot();o:=firstTestObs(6)
  o.Mechs=[]Actor{{ID:2,Kind:"mech",Pos:V3{50,0,0},Aim:V2{0,0},HP:1400,MaxHP:1400}}
  a:=p.firstStrikeAct(o)
  if a.Move[0]>0.9&&math.Abs(a.Move[1])<0.1{t.Fatalf("closing straight into healthy enemy: %v",a.Move)}
}


 // Regen is impossible if the bot keeps dealing damage. A low-health escape
 // must persist even if the target disappears.
 func TestFirstStrikeRegenRetreatHysteresis(t *testing.T) {
	p := firstTestPilot()
	o := firstTestObs(10)
	o.Self.HP = 490 // 35% HP
	o.Mechs = []Actor{{ID: 2, Kind: "mech", Pos: V3{150, 0, 0}, HP: 1400, MaxHP: 1400, Aim: V2{math.Pi, 0}}}
	a := p.firstStrikeAct(o)
	if !p.firstStrikeHealing { t.Fatal("low HP did not enter regen retreat") }
	if a.Fire != ([2]bool{}) { t.Fatalf("fired while entering regen: %v", a.Fire) }

	o = firstTestObs(16)
	o.Self.HP = 900 // 64%, damage had stopped long ago
	a = p.firstStrikeAct(o)
	if !p.firstStrikeHealing { t.Fatal("exited heal state before recovering above 82%") }
	if a.Fire != ([2]bool{}) { t.Fatalf("fired while healing: %v", a.Fire) }

	o = firstTestObs(32)
	o.Self.HP = 1150 // >82%
	_ = p.firstStrikeAct(o)
	if p.firstStrikeHealing { t.Fatal("failed to resume normal tactics after recovery") }
 }

 func TestFirstStrikeHealingOverridesLaserTarget(t *testing.T) {
	p := firstTestPilot()
	o := firstTestObs(10)
	o.Self.HP = 420
	o.Self.Weapons[1] = &WState{Kind: "laser", Stats: Stats{"range": 280, "charge": 0.75}, Cooldown: 0}
	o.Mechs = []Actor{{ID: 2, Kind: "mech", Pos: V3{110, 0, 0}, HP: 100, MaxHP: 1400, Aim: V2{math.Pi, 0}}}
	a := p.firstStrikeAct(o)
	if !p.firstStrikeHealing { t.Fatal("should heal, even with easy kill opportunity") }
	if a.Fire != ([2]bool{}) { t.Fatalf("heal policy must suppress attacks: %v", a.Fire) }
 }
