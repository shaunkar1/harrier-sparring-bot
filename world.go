package main

import (
	"container/heap"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
)

type bounds struct {
	MinX float64 `json:"minX"`
	MaxX float64 `json:"maxX"`
	MinY float64 `json:"minY"`
	MaxY float64 `json:"maxY"`
}
type terrainInfo struct {
	MinX float64 `json:"minX"`
	MinY float64 `json:"minY"`
	Step float64 `json:"step"`
	Nx   int     `json:"nx"`
	Ny   int     `json:"ny"`
}
type prism struct {
	Kind                   string      `json:"kind"`
	Base                   float64     `json:"base"`
	Top                    float64     `json:"top"`
	Ring                   []float64   `json:"ring"`
	Holes                  [][]float64 `json:"holes"`
	minx, maxx, miny, maxy float64
}
type World struct {
	Bounds  bounds      `json:"bounds"`
	Terrain terrainInfo `json:"terrain"`
	Prisms  []prism     `json:"prisms"`
	heights []int16
	nav     *Nav
}

func loadWorld(dir string) *World {
	w := &World{}
	raw, err := os.ReadFile(filepath.Join(dir, "v2/world.json"))
	if err != nil {
		return nil
	}
	if json.Unmarshal(raw, w) != nil {
		return nil
	}
	h, err := os.ReadFile(filepath.Join(dir, "v2/terrain.i16"))
	if err == nil && len(h) >= w.Terrain.Nx*w.Terrain.Ny*2 {
		w.heights = make([]int16, w.Terrain.Nx*w.Terrain.Ny)
		for i := range w.heights {
			w.heights[i] = int16(binary.LittleEndian.Uint16(h[i*2:]))
		}
	}
	for i := range w.Prisms {
		p := &w.Prisms[i]
		p.minx, p.miny = 1e9, 1e9
		p.maxx, p.maxy = -1e9, -1e9
		for j := 0; j+1 < len(p.Ring); j += 2 {
			p.minx = math.Min(p.minx, p.Ring[j])
			p.maxx = math.Max(p.maxx, p.Ring[j])
			p.miny = math.Min(p.miny, p.Ring[j+1])
			p.maxy = math.Max(p.maxy, p.Ring[j+1])
		}
	}
	w.nav = newNav(w)
	return w
}
func (w *World) ground(x, y float64) float64 {
	if w == nil || len(w.heights) == 0 {
		return 0
	}
	g := w.Terrain
	fx := clamp((x-g.MinX)/g.Step, 0, float64(g.Nx-1)-0.001)
	fy := clamp((y-g.MinY)/g.Step, 0, float64(g.Ny-1)-0.001)
	i, j := int(fx), int(fy)
	sx, sy := fx-float64(i), fy-float64(j)
	k := j*g.Nx + i
	a := float64(w.heights[k])*(1-sx) + float64(w.heights[k+1])*sx
	b := float64(w.heights[k+g.Nx])*(1-sx) + float64(w.heights[k+g.Nx+1])*sx
	return (a*(1-sy) + b*sy) * 0.01
}
func inPoly(x, y float64, ring []float64) bool {
	n := len(ring) / 2
	inside := false
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		xi, yi := ring[i*2], ring[i*2+1]
		xj, yj := ring[j*2], ring[j*2+1]
		if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
			inside = !inside
		}
	}
	return inside
}
func (w *World) solid(x, y, z float64) bool {
	if w == nil {
		return false
	}
	if x < w.Bounds.MinX+2 || x > w.Bounds.MaxX-2 || y < w.Bounds.MinY+2 || y > w.Bounds.MaxY-2 {
		return true
	}
	if z < w.ground(x, y)-0.05 {
		return true
	}
	for i := range w.Prisms {
		p := &w.Prisms[i]
		if x < p.minx || x > p.maxx || y < p.miny || y > p.maxy || z < p.Base || z > p.Top {
			continue
		}
		if !inPoly(x, y, p.Ring) {
			continue
		}
		hole := false
		for _, r := range p.Holes {
			if inPoly(x, y, r) {
				hole = true
				break
			}
		}
		if !hole {
			return true
		}
	}
	return false
}
func (w *World) clear(a, b V3) bool {
	if w == nil {
		return true
	}
	d := math.Sqrt(math.Pow(b[0]-a[0], 2) + math.Pow(b[1]-a[1], 2) + math.Pow(b[2]-a[2], 2))
	steps := int(d/3.0) + 1
	for k := 1; k < steps; k++ {
		t := float64(k) / float64(steps)
		if w.solid(a[0]+(b[0]-a[0])*t, a[1]+(b[1]-a[1])*t, a[2]+(b[2]-a[2])*t) {
			return false
		}
	}
	return true
}
func (w *World) walk(x, y float64) bool {
	if w == nil {
		return true
	}
	g := w.ground(x, y)
	if w.solid(x, y, g+4.5) {
		return false
	}
	for _, a := range [][2]float64{{3, 0}, {-3, 0}, {0, 3}, {0, -3}} {
		if math.Abs(w.ground(x+a[0], y+a[1])-g) > 2.7 {
			return false
		}
		if w.solid(x+a[0]*0.55, y+a[1]*0.55, g+4.5) {
			return false
		}
	}
	return true
}

type Nav struct {
	w      *World
	step   float64
	ox, oy float64
	nx, ny int
	ok     []bool
}

func newNav(w *World) *Nav {
	n := &Nav{w: w, step: 5, ox: w.Bounds.MinX + 3, oy: w.Bounds.MinY + 3}
	n.nx = int((w.Bounds.MaxX-n.ox)/n.step) + 1
	n.ny = int((w.Bounds.MaxY-n.oy)/n.step) + 1
	n.ok = make([]bool, n.nx*n.ny)
	for j := 0; j < n.ny; j++ {
		for i := 0; i < n.nx; i++ {
			x, y := n.pos(j*n.nx + i)
			n.ok[j*n.nx+i] = w.walk(x, y)
		}
	}
	return n
}
func (n *Nav) pos(k int) (float64, float64) {
	return n.ox + float64(k%n.nx)*n.step, n.oy + float64(k/n.nx)*n.step
}
func (n *Nav) idx(x, y float64) int {
	i := int(math.Round((x - n.ox) / n.step))
	j := int(math.Round((y - n.oy) / n.step))
	if i < 0 || j < 0 || i >= n.nx || j >= n.ny {
		return -1
	}
	return j*n.nx + i
}
func (n *Nav) nearest(p V2) int {
	k := n.idx(p[0], p[1])
	if k >= 0 && n.ok[k] {
		return k
	}
	best := -1
	bd := 1e12
	for j := -5; j <= 5; j++ {
		for i := -5; i <= 5; i++ {
			q := n.idx(p[0]+float64(i)*n.step, p[1]+float64(j)*n.step)
			if q < 0 || !n.ok[q] {
				continue
			}
			x, y := n.pos(q)
			d := math.Hypot(p[0]-x, p[1]-y)
			if d < bd {
				bd, best = d, q
			}
		}
	}
	return best
}
func (n *Nav) direct(a, b V2) bool {
	d := b.Sub(a).Len()
	steps := int(d/4) + 1
	for i := 1; i <= steps; i++ {
		p := a.Add(b.Sub(a).Mul(float64(i) / float64(steps)))
		k := n.idx(p[0], p[1])
		if k < 0 || !n.ok[k] {
			return false
		}
	}
	return true
}

type node struct {
	k int
	f float64
}
type pq []node

func (p pq) Len() int           { return len(p) }
func (p pq) Less(i, j int) bool { return p[i].f < p[j].f }
func (p pq) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p *pq) Push(x any)        { *p = append(*p, x.(node)) }
func (p *pq) Pop() any          { a := *p; v := a[len(a)-1]; *p = a[:len(a)-1]; return v }

// Bounded A*: practical navigation in ~1-3 ms when the direct path is obstructed.
func (n *Nav) waypoint(start, goal V2) V2 {
	if n.direct(start, goal) {
		return goal
	}
	from, to := n.nearest(start), n.nearest(goal)
	if from < 0 || to < 0 {
		return goal
	}
	N := len(n.ok)
	g := make([]float32, N)
	parent := make([]int32, N)
	closed := make([]bool, N)
	for i := range g {
		g[i] = 1e9
		parent[i] = -1
	}
	g[from] = 0
	q := &pq{{from, 0}}
	heap.Init(q)
	dirs := [8][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
	tx, ty := n.pos(to)
	nearest := from
	bestD := 1e9
	expanded := 0
	for q.Len() > 0 && expanded < 4000 {
		u := heap.Pop(q).(node).k
		if closed[u] {
			continue
		}
		closed[u] = true
		expanded++
		ux, uy := n.pos(u)
		hd := math.Hypot(tx-ux, ty-uy)
		if hd < bestD {
			bestD, nearest = hd, u
		}
		if u == to {
			nearest = u
			break
		}
		x, y := u%n.nx, u/n.nx
		for _, d := range dirs {
			xx, yy := x+d[0], y+d[1]
			if xx < 0 || yy < 0 || xx >= n.nx || yy >= n.ny {
				continue
			}
			v := yy*n.nx + xx
			if !n.ok[v] || closed[v] {
				continue
			}
			if d[0] != 0 && d[1] != 0 && (!n.ok[y*n.nx+xx] || !n.ok[yy*n.nx+x]) {
				continue
			}
			step := float32(1.0)
			if d[0] != 0 && d[1] != 0 {
				step = 1.4142
			}
			score := g[u] + step
			if score < g[v] {
				g[v] = score
				parent[v] = int32(u)
				vx, vy := n.pos(v)
				heap.Push(q, node{v, float64(score) + math.Hypot(tx-vx, ty-vy)/n.step})
			}
		}
	}
	if nearest == from {
		return goal
	}
	chain := []int{nearest}
	for p := nearest; p != from && len(chain) < 500; {
		p = int(parent[p])
		if p < 0 {
			break
		}
		chain = append(chain, p)
	}
	next := nearest
	for i := len(chain) - 2; i >= 0; i-- {
		k := chain[i]
		x, y := n.pos(k)
		if n.direct(start, V2{x, y}) {
			next = k
		} else {
			break
		}
	}
	x, y := n.pos(next)
	return V2{x, y}
}
