package main

import "math"

type V2 [2]float64
type V3 [3]float64

func (a V2) Add(b V2) V2      { return V2{a[0] + b[0], a[1] + b[1]} }
func (a V2) Sub(b V2) V2      { return V2{a[0] - b[0], a[1] - b[1]} }
func (a V2) Mul(s float64) V2 { return V2{a[0] * s, a[1] * s} }
func (a V2) Len() float64     { return math.Hypot(a[0], a[1]) }
func (a V2) Unit() V2 {
	d := a.Len()
	if d < 1e-6 {
		return V2{}
	}
	return a.Mul(1 / d)
}
func (a V2) Dot(b V2) float64         { return a[0]*b[0] + a[1]*b[1] }
func xy(v V3) V2                      { return V2{v[0], v[1]} }
func clamp(x, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, x)) }
func angle(a V2) float64              { return math.Atan2(a[1], a[0]) }
func wrap(a float64) float64          { return math.Atan2(math.Sin(a), math.Cos(a)) }
func perp(v V2) V2                    { return V2{-v[1], v[0]} }
func num(m map[string]float64, k string, def float64) float64 {
	if v := m[k]; v != 0 {
		return v
	}
	return def
}

type Stats map[string]float64
type WState struct {
	Kind     string  `json:"kind"`
	Cooldown float64 `json:"cooldown"`
	Ammo     float64 `json:"ammo"`
	Reload   float64 `json:"reload"`
	Charge   float64 `json:"charge"`
	Stats    Stats   `json:"stats"`
}
type Self struct {
	ID        int       `json:"id"`
	Pos       V3        `json:"pos"`
	Vel       V3        `json:"vel"`
	Aim       V2        `json:"aim"`
	HP        float64   `json:"hp"`
	MaxHP     float64   `json:"max_hp"`
	Energy    float64   `json:"energy"`
	MaxEnergy float64   `json:"max_energy"`
	Grounded  bool      `json:"grounded"`
	InZone    bool      `json:"in_zone"`
	Cubes     float64   `json:"cubes"`
	Weapons   []*WState `json:"weapons"`
	Stats     Stats     `json:"stats"`
}
type Actor struct {
	ID       int      `json:"id"`
	Kind     string   `json:"kind"`
	Pos      V3       `json:"pos"`
	Vel      V3       `json:"vel"`
	Aim      V2       `json:"aim"`
	HP       float64  `json:"hp"`
	MaxHP    float64  `json:"max_hp"`
	Cubes    float64  `json:"cubes"`
	Weapons  []string `json:"weapons"`
	Charging bool     `json:"charging"`
}

func (e Actor) Has(kind string) bool {
	for _, w := range e.Weapons {
		if w == kind {
			return true
		}
	}
	return false
}

type Projectile struct {
	Kind string `json:"kind"`
	Pos  V3     `json:"pos"`
	Vel  V3     `json:"vel"`
}
type Item struct {
	ID   int     `json:"id"`
	Kind string  `json:"kind"`
	Pos  V3      `json:"pos"`
	HP   float64 `json:"hp"`
}
type Drop struct {
	ID       int     `json:"id"`
	Pos      V3      `json:"pos"`
	Radius   float64 `json:"radius"`
	LandsIn  float64 `json:"lands_in"`
	CaptureS float64 `json:"capture_s"`
	Holder   *int    `json:"holder"`
	Progress float64 `json:"progress"`
}
type Zone struct {
	Center      V2      `json:"center"`
	Radius      float64 `json:"radius"`
	NextCenter  V2      `json:"next_center"`
	NextRadius  float64 `json:"next_radius"`
	ShrinkStart float64 `json:"shrink_start"`
	ShrinkEnd   float64 `json:"shrink_end"`
	DPS         float64 `json:"dps"`
}
type Event struct {
	Type    string   `json:"type"`
	Amount  float64  `json:"amount"`
	Weapon  string   `json:"weapon"`
	By      *int     `json:"by"`
	Target  int      `json:"target"`
	Bearing *float64 `json:"bearing"`
}
type Offer struct {
	Choices []struct {
		ID string `json:"id"`
	} `json:"choices"`
}
type Obs struct {
	Type        string       `json:"type"`
	Tick        int          `json:"tick"`
	T           float64      `json:"t"`
	Alive       int          `json:"alive"`
	Self        Self         `json:"self"`
	Mechs       []Actor      `json:"mechs"`
	Projectiles []Projectile `json:"projectiles"`
	Crates      []Item       `json:"crates"`
	Cubes       []Item       `json:"cubes"`
	Pickups     []Item       `json:"pickups"`
	Drops       []Drop       `json:"drops"`
	Events      []Event      `json:"events"`
	Zone        Zone         `json:"zone"`
	Offer       *Offer       `json:"offer"`
}
type Hello struct {
	Type string `json:"type"`
	Seed int64  `json:"seed"`
	Slot int    `json:"slot"`
	Map  struct {
		Dir string `json:"dir"`
	} `json:"map"`
	Config struct {
		Mech    Stats            `json:"mech"`
		Weapons map[string]Stats `json:"weapons"`
		Loot    struct {
			PickupRadius float64 `json:"pickup_radius"`
		} `json:"loot"`
	} `json:"config"`
}
type Action struct {
	Tick   int     `json:"tick"`
	Move   V2      `json:"move"`
	Boost  bool    `json:"boost"`
	Jump   bool    `json:"jump"`
	Aim    V2      `json:"aim"`
	Fire   [2]bool `json:"fire"`
	Reload bool    `json:"reload"`
	Pick   *int    `json:"pick,omitempty"`
	Take   *int    `json:"take,omitempty"`
	Drop   *int    `json:"drop,omitempty"`
}
