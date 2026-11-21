package main

import (
	"math"
	"testing"
)

func TestLeaderWeaponSwap(t *testing.T) {
	p := &Pilot{style: "leader"}
	p.h.Config.Loot.PickupRadius = 4
	o := &Obs{Self: Self{Pos: V3{0, 0, 0}, Weapons: []*WState{{Kind: "gun", Ammo: 12}, {Kind: "laser"}}}, Pickups: []Item{{ID: 8080, Kind: "lightning", Pos: V3{2, 0, 0}}}}
	a := Action{}
	p.pickup(o, &a)
	if a.Take == nil || *a.Take != 8080 || a.Drop == nil || *a.Drop != 0 {
		t.Fatalf("leader must trade gun for lightning beside laser: %+v", a)
	}
}
func TestLeaderSkipsMortar(t *testing.T) {
	p := &Pilot{style: "leader"}
	p.h.Config.Loot.PickupRadius = 4
	o := &Obs{Self: Self{Pos: V3{0, 0, 0}, Weapons: []*WState{{Kind: "gun"}, nil}}, Pickups: []Item{{ID: 8111, Kind: "mortar", Pos: V3{2, 0, 0}}}}
	a := Action{}
	p.pickup(o, &a)
	if a.Take != nil {
		t.Fatalf("leader should not take mortar (laser cook-off): %+v", a)
	}
}
func TestLeaderScanningAndFarming(t *testing.T) {
	p := &Pilot{style: "leader", rng: randForTest(), memory: map[int]*remembered{}}
	me := Self{Pos: V3{0, 0, 0}, HP: 1400, MaxHP: 1400, Energy: 100, MaxEnergy: 100, Grounded: true, InZone: true, Weapons: []*WState{{Kind: "gun", Ammo: 12, Stats: Stats{"mag": 12}}, nil}, Stats: Stats{"walk_speed": 12}}
	zone := Zone{NextCenter: V2{0, 0}, NextRadius: 350, Center: V2{0, 0}, Radius: 350, ShrinkStart: 60, ShrinkEnd: 120}
	a := p.act(&Obs{T: 2, Tick: 20, Self: me, Zone: zone, Alive: 16})
	b := p.act(&Obs{T: 2.5, Tick: 25, Self: me, Zone: zone, Alive: 16})
	if math.Abs(wrap((b.Aim[0]-a.Aim[0])-leaderSpin*0.5)) > 0.02 {
		t.Fatalf("leader scan angular speed: got %.3f vs %.3f", wrap(b.Aim[0]-a.Aim[0]), leaderSpin*0.5)
	}
	p2 := &Pilot{style: "leader", rng: randForTest(), memory: map[int]*remembered{}}
	cr := Item{ID: 1001, Pos: V3{40, 0, 0}, HP: 300}
	o := &Obs{T: 2, Tick: 20, Self: me, Zone: zone, Alive: 16, Crates: []Item{cr}}
	a = p2.act(o)
	if !a.Fire[0] {
		t.Fatalf("leader should fire gun at visible crate on route: %+v", a)
	}
}