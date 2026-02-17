package main

import (
	"fmt"
	"math"
)

func (g *Game) IsWall(col, row int) bool {
	if row < 0 || col < 0 {
		return false
	}
	if row >= len(g.TileMap) {
		return false
	}
	if col >= len(g.TileMap[row]) {
		return false
	}
	tile := g.TileMap[row][col]
	return (tile >= g.FirstWallTile && tile < g.FirstWallTile+g.WallTileCnt) ||
		tile == g.RockTile
}

func (o *GameObject) Bounds(nx, ny *float64) (x, y, x2, y2 float64) {
	px := o.X
	py := o.Y
	if nx != nil {
		px = *nx
	}
	if ny != nil {
		py = *ny
	}
	x = px + float64(o.XOff)
	y = py + float64(o.YOff)
	x2 = x + float64(o.Width) - 1
	y2 = y + float64(o.Height) - 1
	return
}

func (g *Game) Obstacle(o *GameObject, dx, dy float64, nx, ny *float64) bool {
	px, py, px2, py2 := o.Bounds(nx, ny)
	px += dx
	px2 += dx
	py += dy
	py2 += dy

	c := int(px / TileSize)
	r := int(py / TileSize)
	c2 := int(px2 / TileSize)
	r2 := int(py2 / TileSize)

	for i := c; i <= c2; i++ {
		for j := r; j <= r2; j++ {
			if g.IsWall(i, j) {
				return true
			}
		}
	}
	return false
}

func Overlap(ax, ay, ax2, ay2, bx, by, bx2, by2 float64) bool {
	return !(ax > bx2 || ax2 < bx || ay > by2 || ay2 < by)
}

func (g *Game) Hazard(p *GameObject, x, y float64, filter func(*GameObject) bool) *GameObject {
	px, py, px2, py2 := p.Bounds(&x, &y)
	for _, q := range g.Objects {
		if p != q && filter(q) {
			qx, qy, qx2, qy2 := q.Bounds(nil, nil)
			if Overlap(px, py, px2, py2, qx, qy, qx2, qy2) {
				return q
			}
		}
	}
	return nil
}

func Distance(a, b *GameObject) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return math.Sqrt(dx*dx + dy*dy)
}

func NoGhost(p *GameObject) bool {
	return !p.Ignore
}

func AvoidFilter(p *GameObject) bool {
	return p.Avoid
}

func (g *Game) Hazard2(p *GameObject, filter func(*GameObject) bool) (u, r, d, l *GameObject) {
	px, py, px2, py2 := p.Bounds(nil, nil)
	for _, q := range g.Objects {
		if p != q && filter(q) {
			qx, qy, qx2, qy2 := q.Bounds(nil, nil)
			if Overlap(px, py-1, px2, py2, qx, qy, qx2, qy2) {
				u = q
			} else if Overlap(px, py, px2+1, py2, qx, qy, qx2, qy2) {
				r = q
			} else if Overlap(px, py, px2, py2+1, qx, qy, qx2, qy2) {
				d = q
			} else if Overlap(px-1, py, px2, py2, qx, qy, qx2, qy2) {
				l = q
			}
		}
	}
	return
}

func (g *Game) MobCollide(p *GameObject, x, y float64) *GameObject {
	if p.Ignore {
		return nil
	}
	q := g.Hazard(p, x, y, NoGhost)
	if q == nil {
		return nil
	}
	if q.Kind == Crate && p.Kind&Friendly != 0 {
		g.OpenCrate(q)
	} else if p.Kind&Friendly != 0 && q.Kind&Friendly != 0 {
		if p.Kind != Player {
			return q
		}
		return nil
	} else {
		g.Log(fmt.Sprintf("hits %s%d", q.Name, q.ID), p)
		a := g.Crunch(q)
		b := g.Crunch(p)
		if a || b {
			g.Explode(p.X, p.Y)
		}
	}
	return nil
}

func (g *Game) Walk(p *GameObject, dir int) bool {
	return g.walk(p, dir, false)
}

func dirDelta(dir int) (float64, float64, bool) {
	switch dir {
	case DirLeft:
		return -1, 0, true
	case DirRight:
		return 1, 0, true
	case DirUp:
		return 0, -1, true
	case DirDown:
		return 0, 1, true
	case DirLeft | DirUp:
		return -0.7, -0.7, true
	case DirRight | DirUp:
		return 0.7, -0.7, true
	case DirLeft | DirDown:
		return -0.7, 0.7, true
	case DirRight | DirDown:
		return 0.7, 0.7, true
	}
	return 0, 0, false
}

func snapDiag(p *GameObject, x, y float64) {
	if x == 0 || y == 0 {
		p.Diag = false
		return
	}
	if p.Diag {
		return
	}
	snapX, snapY := math.Floor, math.Floor
	if x < 0 {
		snapX = math.Ceil
	}
	if y < 0 {
		snapY = math.Ceil
	}
	p.X = snapX(p.X)
	p.Y = snapY(p.Y)
	p.Diag = true
}

func (g *Game) bounceDir(p *GameObject, dir int, lx, ly, nx, ny float64) int {
	lr := dir & (DirLeft | DirRight)
	ud := dir & (DirUp | DirDown)
	// lr and ud are each a single bit or zero; XOR swaps Left↔Right / Up↔Down
	flipLR := cond(lr != 0, (DirLeft|DirRight)^lr, 0)
	flipUD := cond(ud != 0, (DirUp|DirDown)^ud, 0)
	if !p.Diag {
		return flipLR | flipUD
	}
	if g.Obstacle(p, 0, 0, &lx, &ny) {
		return lr | flipUD
	}
	if g.Obstacle(p, 0, 0, &nx, &ly) {
		return flipLR | ud
	}
	return lr | ud
}

func (g *Game) slideAxis(p *GameObject, otherDelta float64, move, fixed, fallback float64,
	obstA, obstB *float64, posDir, negDir int) (float64, bool, bool) {
	if !g.Obstacle(p, 0, 0, obstA, obstB) {
		return move, false, false
	}
	if otherDelta == 0 {
		testPos := fixed + 3
		if !g.Obstacle(p, 0, 0, &testPos, obstB) {
			return move, true, g.walk(p, posDir, true)
		}
		testNeg := fixed - 3
		if !g.Obstacle(p, 0, 0, &testNeg, obstB) {
			return move, true, g.walk(p, negDir, true)
		}
	}
	return fallback, false, false
}

func (g *Game) slideWalk(p *GameObject, x, y, lx, ly, nx, ny float64) (float64, float64, bool, bool) {
	if y != 0 {
		adjY, slid, result := g.slideAxis(p, x, ny, lx, ly, &lx, &ny, DirRight, DirLeft)
		if slid {
			return nx, adjY, true, result
		}
		ny = adjY
	}
	if x != 0 {
		adjX, slid, result := g.slideAxis(p, y, nx, ly, lx, &nx, &ly, DirDown, DirUp)
		if slid {
			return adjX, ny, true, result
		}
		nx = adjX
	}
	return nx, ny, false, false
}

func (g *Game) walk(p *GameObject, dir int, slide bool) bool {
	for attempt := 0; ; attempt++ {
		x, y, ok := dirDelta(dir)
		if !ok {
			return false
		}
		snapDiag(p, x, y)

		lx, ly := p.X, p.Y
		s := p.Speed
		ny := max(float64(p.YMin), min(ly+y*s, float64(p.YMax)))
		nx := max(0, min(lx+x*s, float64(p.XMaxLimit)))

		if p.Bounce && g.Obstacle(p, 0, 0, &nx, &ny) {
			if attempt > 0 {
				return false
			}
			dir = g.bounceDir(p, dir, lx, ly, nx, ny)
			p.Dir = dir
			continue
		}

		if !slide {
			var slid, result bool
			nx, ny, slid, result = g.slideWalk(p, x, y, lx, ly, nx, ny)
			if slid {
				return result
			}
		}

		if nx == lx && ny == ly {
			g.Log("blocked", p)
			return false
		}

		q := g.MobCollide(p, nx, ny)
		if q != nil {
			return false
		}

		p.X, p.Y = nx, ny
		return true
	}
}
