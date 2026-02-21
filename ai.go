package main

import (
	"slices"
)

func (g *Game) innerFollow(p *GameObject, tx, ty float64, hasLast bool) (int, bool) {
	x, y := int(p.X), int(p.Y)
	itx, ity := int(tx), int(ty)
	if hasLast && itx == x && ity == y {
		return 0, true
	}
	hu, hr, hd, hl := g.Hazard2(p, AvoidFilter)

	// X axis: move left or right toward target if no hazard and no obstacle
	var bits int
	if itx < x && hl == nil && !g.Obstacle(p, -1, 0, nil, nil) {
		bits |= DirLeft
	} else if itx > x && hr == nil && !g.Obstacle(p, 1, 0, nil, nil) {
		bits |= DirRight
	}
	// Y axis: move up or down toward target if no hazard and no obstacle
	if ity < y && hu == nil && !g.Obstacle(p, 0, -1, nil, nil) {
		bits |= DirUp
	} else if ity > y && hd == nil && !g.Obstacle(p, 0, 1, nil, nil) {
		bits |= DirDown
	}
	return bits, false
}

func (g *Game) resetPathGrid() {
	for i := range g.Path {
		for j := range g.Path[i] {
			g.Path[i][j] = MaxN
		}
	}
}

func (g *Game) bfsPath(startC, startR, mapC int) {
	type cell struct{ c, r int }
	queue := []cell{{startC, startR}}
	for len(queue) > 0 {
		c, r := queue[0].c, queue[0].r
		queue = queue[1:]
		if c < 0 || r < 0 || c >= PathCols || r >= PathRows {
			continue
		}
		cost := g.Path[c][r] + 1
		for _, n := range [4]cell{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nc, nr := c+n.c, r+n.r
			if nc >= 0 && nc < PathCols && nr >= 0 && nr < PathRows && g.Path[nc][nr] == MaxN {
				if g.IsWall(mapC+nc, nr) {
					g.Path[nc][nr] = WallCost
				} else {
					g.Path[nc][nr] = cost
					queue = append(queue, cell{nc, nr})
				}
			}
		}
	}
}

func (g *Game) UpdatePath(p *GameObject) {
	px := int(p.X) + p.XOff
	mapC := max(0, px-GameWidth) / TileSize
	c := (px - mapC*TileSize) / TileSize
	r := (int(p.Y) + p.YOff) / TileSize

	if mapC+c == p.PathC && r == p.PathR {
		return
	}
	p.PathC = mapC + c
	p.PathR = r

	g.resetPathGrid()

	if c >= 0 && c < PathCols && r >= 0 && r < PathRows {
		g.Path[c][r] = 0
	}

	g.bfsPath(c, r, mapC)
}

func (g *Game) Retarget(p *GameObject) *GameObject {
	var target *GameObject
	minDist := float64(MaxN)
	for _, q := range g.Objects {
		if p == q {
			continue
		}
		if (q.Kind&Friendly != 0 && q.X < FinishLine) || (q.Kind == Thug && !g.Playing) {
			d := Distance(p, q)
			if d < minDist {
				target = q
				minDist = d
			}
		}
	}
	return target
}

func Follow(g *Game, p *GameObject) int {
	tc := p.TargetCrumb

	for {
		// If no crumb target or crumb was reaped, retarget
		var t *GameObject
		if tc == nil || tc.Old {
			p.TargetCrumb = nil
			tc = nil
			t = g.Retarget(p)
			p.Target = t
			if t != nil && t.Kind == Waif && !t.Spotted {
				g.PlaySfx(g.SndSpotted, "spotted")
				t.Spotted = true
			}
		}

		if t == nil && tc == nil {
			return 0
		}

		// Follow the current target
		var ret int
		var arrived bool
		if tc != nil {
			ret, arrived = g.innerFollow(p, tc.X, tc.Y, tc.Last != nil)
		} else {
			ret, arrived = g.innerFollow(p, t.X, t.Y, false)
		}

		// Arrived at breadcrumb — loop to retarget
		if arrived {
			p.TargetCrumb = nil
			tc = nil
			continue
		}

		// Stuck — try following the player's breadcrumb trail
		if g.Playing && ret == 0 && len(g.Crumbs) > 0 {
			ret, p.TargetCrumb = g.followCrumbs(p)
		}

		p.Dot = ret
		return ret
	}
}

func (g *Game) followCrumbs(p *GameObject) (int, *Crumb) {
	c := p.TargetCrumb
	if c != nil {
		c = c.Last
	}
	if c == nil {
		c = g.Crumbs[len(g.Crumbs)-1]
	}
	for c != nil {
		ret, _ := g.innerFollow(p, c.X, c.Y, c.Last != nil)
		if ret != 0 {
			return ret, c
		}
		c = c.Last
	}
	return 0, nil
}

func dirToward(px, py, tx, ty int) int {
	return cond(tx < px, DirLeft, cond(tx > px, DirRight, 0)) |
		cond(ty < py, DirUp, cond(ty > py, DirDown, 0))
}

func (g *Game) bestPathNeighbor(c, r int) (int, int, int) {
	pmin := MaxN
	if c >= 0 && c < PathCols && r >= 0 && r < PathRows {
		pmin = g.Path[c][r]
	}
	nc, nr := c, r
	for i := max(0, c-1); i <= min(PathCols-1, c+1); i++ {
		for j := max(0, r-1); j <= min(PathRows-1, r+1); j++ {
			if !(i == c && j == r) && g.Path[i][j] < pmin {
				pmin = g.Path[i][j]
				nc, nr = i, j
			}
		}
	}
	return nc, nr, pmin
}

func FollowPath(g *Game, p *GameObject) int {
	px, py := int(p.X), int(p.Y)

	if px >= FinishLine || g.Won {
		return DirRight
	}
	if !g.Playing {
		return DirRight | cond(py > 80, DirUp, cond(py < 78, DirDown, 0))
	}
	if g.Player == nil {
		return DirRight
	}

	tx := int(g.Player.X) + g.Player.XOff
	ty := int(g.Player.Y) + g.Player.YOff

	// Too far from player — use simple direction
	if abs(tx-px) >= GameWidth {
		ret := dirToward(px, py, tx, ty)
		p.Dot = ret
		return ret
	}

	// Use path grid for navigation
	mapC := max(0, tx-GameWidth) / TileSize
	c := (px - mapC*TileSize) / TileSize
	r := py / TileSize
	nc, nr, pmin := g.bestPathNeighbor(c, r)

	nx := float64((mapC + nc) * TileSize)
	ny := float64(nr * TileSize)

	// Jitter when close to player to avoid stacking
	if pmin <= 1 {
		if p.JitterTime == 0 {
			p.JitterX = float64(g.Rng.Intn(g.WaifJitterPx*2+1) - g.WaifJitterPx)
			p.JitterY = float64(g.Rng.Intn(g.WaifJitterPx*2+1) - g.WaifJitterPx)
			p.JitterTime = g.WaifJitterF
		} else {
			p.JitterTime--
		}
		nx = float64(tx) + p.JitterX
		ny = float64(ty) + p.JitterY
	} else {
		p.JitterTime = 0
	}

	ret := dirToward(px, py, int(nx), int(ny))
	p.Dot = ret
	return ret
}

func PlayerPostMove(g *Game, p *GameObject) {
	g.UpdatePath(p)
	g.AddCrumb(p)
}

func WaifPostMove(g *Game, p *GameObject) {
	// Score when crossing finish line (only once, only during play/win)
	if p.X > FinishLine && !p.Scored && (g.Playing || g.Won) {
		g.Score += g.WaifSafeScore
		p.Scored = true
		p.Ignore = true
		g.PlaySfx(g.SndSafe, "safe")
		return
	}
	// Remove when past the right edge
	if p.X > XMax {
		g.Reap(p)
		return
	}
	// Speed-match player when close enough
	if g.Player != nil && Distance(p, g.Player) < g.WaifFollowPx && g.Player.Speed > 0 {
		p.Speed = g.Player.Speed
	}
}

func (g *Game) AddCrumb(p *GameObject) {
	if len(g.Crumbs) > 0 {
		if len(g.Crumbs) > MaxCrumbs {
			g.ReapCrumb()
		}
		last := g.Crumbs[len(g.Crumbs)-1]
		dx := p.X - last.X
		dy := p.Y - last.Y
		if dx*dx+dy*dy > CrumbMinDist {
			g.CrumbID++
			g.Crumbs = append(g.Crumbs, &Crumb{X: p.X, Y: p.Y, Last: last, ID: g.CrumbID})
		}
	} else {
		g.CrumbID = 1
		g.Crumbs = append(g.Crumbs, &Crumb{X: p.X, Y: p.Y, ID: g.CrumbID})
	}
}

func (g *Game) ReapCrumb() {
	if len(g.Crumbs) > 1 {
		g.Crumbs[1].Last = nil
	}
	if len(g.Crumbs) > 0 {
		g.Crumbs[0].Old = true
		g.Crumbs = slices.Delete(g.Crumbs, 0, 1)
	}
}
