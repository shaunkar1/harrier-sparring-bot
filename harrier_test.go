package main

import (
	"math"
	"math/rand"
	"testing"
)

func randForTest() *rand.Rand { return rand.New(rand.NewSource(42)) }

func TestBallisticSolution(t *testing.T) {
	p := &Pilot{}
	me := &Self{Pos: V3{0, 0, 0}, MaxHP: 1400, Stats: Stats{"muzzle_height": 7}}
	a := Actor{Pos: V3{110, 0, 0}, Kind: "mech", HP: 900, MaxHP: 1400}
	w := &WState{Kind: "mortar", Stats: Stats{"speed": 72, "gravity": 22}}
	ang, ok := p.ballistic(me, a, w)
	if !ok {
		t.Fatal("expected mortar solution for 110m target")
	}
	if math.Abs(ang[0]) > 0.01 || ang[1] <= 0.1 || ang[1] >= 1.4 {
		t.Fatalf("unexpected mortar angles: %v", ang)
	}
}
func TestNoMortarLaserCarry(t *testing.T) {
	p := &Pilot{style: "artillery"}
	p.h.Config.Loot.PickupRadius = 4
	me := Self{Pos: V3{0, 0, 0}, Weapons: []*WState{{Kind: "gun"}, {Kind: "mortar"}}}
	o := &Obs{Self: me, Pickups: []Item{{ID: 8221, Kind: "laser", Pos: V3{2, 0, 0}}}}
	a := Action{}
	p.pickup(o, &a)
	if a.Take == nil || *a.Take != 8221 {
		t.Fatalf("expected laser pickup, got %v", a.Take)
	}
	if a.Drop == nil || *a.Drop != 1 {
		t.Fatalf("expected mortar replacement not gun replacement, got %v", a.Drop)
	}
}
func TestChargingLaserHeldWithoutTarget(t *testing.T) {
	p := &Pilot{style: "harrier", rng: randForTest(), memory: map[int]*remembered{}}
	me := Self{Pos: V3{0, 0, 0}, HP: 1400, MaxHP: 1400, Energy: 100, MaxEnergy: 100, Grounded: true, InZone: true,
		Weapons: []*WState{{Kind: "gun", Ammo: 12, Stats: Stats{"mag": 12}}, {Kind: "laser", Charge: 0.5, Stats: Stats{"charge": 0.75}}}, Stats: Stats{"walk_speed": 11}}
	o := &Obs{Self: me, Zone: Zone{NextCenter: V2{0, 0}, NextRadius: 380, ShrinkStart: 60, ShrinkEnd: 110}, Alive: 16, Tick: 2, T: 5}
	a := p.act(o)
	if !a.Fire[1] {
		t.Fatal("charging laser must be held without a target")
	}
}
func TestIncomingBulletDeflection(t *testing.T) {
	p := &Pilot{style: "harrier", rng: randForTest(), memory: map[int]*remembered{}}
	me := Self{Pos: V3{0, 0, 0}, HP: 1400, MaxHP: 1400, Energy: 100, MaxEnergy: 100, Grounded: true, InZone: true,
		Weapons: []*WState{{Kind: "gun", Ammo: 12, Stats: Stats{"mag": 12}}}, Stats: Stats{"walk_speed": 11}}
	o := &Obs{Self: me, Zone: Zone{NextCenter: V2{0, 0}, NextRadius: 380, ShrinkStart: 60, ShrinkEnd: 110}, Alive: 16, Tick: 2, T: 5,
		Projectiles: []Projectile{{Kind: "gun", Pos: V3{45, 0, 5}, Vel: V3{-252, 0, 0}}}}
	a := p.act(o)
	if math.IsNaN(a.Move[0]) || math.IsNaN(a.Move[1]) {
		t.Fatal("invalid movement")
	}
}
