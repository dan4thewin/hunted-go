package main

import (
	"math"
	"math/rand"
	"slices"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func (g *Game) PlaceMine(p *GameObject) {
	if g.MineCount > 0 {
		g.MineCount--
		g.Emplace(g.DefMine, p.X, p.Y, func(obj *GameObject) {
			obj.FrameFunc = func(o *GameObject) {
				if !o.Armed {
					o.Armed = true
				} else {
					o.Ignore = false
				}
			}
		})
	}
}

func (g *Game) ThrowBomb(p *GameObject) {
	if g.BombCount > 0 {
		g.BombCount--
		g.Emplace(g.DefBomb, p.X, p.Y, func(obj *GameObject) {
			obj.Dir = p.LastDir
			obj.FrameFunc = func(o *GameObject) {
				if !o.Armed {
					o.Armed = true
				} else {
					o.Ignore = false
				}
			}
		})
		g.PlaySfx(g.SndThrow, "throw")
	}
}

func (g *Game) OpenCrate(p *GameObject) {
	g.Crunch(p)
	count := g.Rng.Intn(2) + 1
	for i := 0; i < count; i++ {
		if g.Rng.Intn(2) == 0 {
			g.BombCount++
		} else {
			g.MineCount++
		}
	}
}

func (g *Game) Crunch(p *GameObject) bool {
	if p.Final == nil {
		return false
	}
	g.Reap(p)
	if g.Playing && p.Speed > 0 {
		switch p.Kind {
		case Thug:
			g.Score += int(p.Speed * float64(g.ThugScoreX))
		case Waif:
			g.Score -= int(p.Speed * float64(g.WaifPenaltyX))
		}
	}
	if (p.Kind == Thug && p.Y >= TopEdge && p.Y <= BottomEdge) || (p.Kind == Player && g.HeartCount == 0) || p.Kind == Waif {
		g.Emplace(p.Final, p.X, p.Y, func(obj *GameObject) {
			obj.PalIndex = p.PalIndex
			obj.PalSwap = p.PalSwap
		})
		// Play crunch for dead bodies (not junk-producing deaths)
		if p.Final != g.DefJunk {
			g.PlaySfx(g.SndCrunch, "crunch")
		}
		return true
	}
	return false
}

func (g *Game) Reap(p *GameObject) {
	switch p.Kind {
	case Player:
		if p.IsInvuln {
			return
		}
		g.HeartCount--
		if g.HeartCount > 0 {
			g.StartInvuln(p)
			return
		}
		p.Speed = 0
		g.Playing = false
	case Waif:
		g.WaifCount--
	case Thug:
		g.ThugCount--
		g.Threat -= p.Threat
	}

	// Use slices.Delete for Go 1.25
	for i, obj := range g.Objects {
		if obj == p {
			g.Objects = slices.Delete(g.Objects, i, i+1)
			break
		}
	}
}

func InvulnPreAct(g *Game, p *GameObject) bool {
	if p.Delay == 0 {
		p.IsInvuln = false
		p.PreAct = nil
	} else {
		p.Delay--
	}
	return true
}

func (g *Game) StartInvuln(p *GameObject) {
	p.IsInvuln = true
	p.Delay = p.InvulnFrames
	p.PreAct = InvulnPreAct
}

func (g *Game) Act(p *GameObject) bool {
	if p == nil {
		return false
	}

	// Pre-action hook (invulnerability countdown, waif slow start)
	if p.PreAct != nil && !p.PreAct(g, p) {
		return false
	}

	// Get direction
	var dir int
	if p.Controller != nil {
		dir = p.Controller(g, p)
	} else {
		dir = p.Dir
	}

	if dir == 0 {
		p.Speed = max(0, p.Speed-g.SpeedDecayPxf)
	} else {
		base := cond(p.Kind == Thug && g.Mode == 2, p.BaseSpeed*g.RelaxedSpeedX, p.BaseSpeed)
		p.Speed = max(base, p.Speed-g.SpeedDecayPxf)

		lr := dir & (DirLeft | DirRight)
		if lr != 0 {
			p.Flip = lr == DirLeft
		}

		if g.Walk(p, dir) {
			p.SetMoving()
			if p.PostMove != nil {
				p.PostMove(g, p)
			}
		} else {
			p.Speed = 0
		}
	}

	if p.Speed == 0 {
		p.SetIdle()
	}

	return p.NextPose(g) && dir != 0
}

func (g *Game) Explode(x, y float64) {
	g.ShakeFactor = 1
	g.MakeSparks(x, y)
	g.PlayExplosion()
}

func (g *Game) MakeSparks(x, y float64) {
	count := g.Rng.Intn(20) + 10
	for i := 0; i < count; i++ {
		g.Sparks = append(g.Sparks, Spark{
			X:    x,
			Y:    y,
			DX:   g.Rng.Float64()*2 - 1,
			DY:   g.Rng.Float64()*2 - 1,
			Time: g.Rng.Intn(40) + 20,
		})
	}
}

func (g *Game) UpdateSparks() {
	newSparks := g.Sparks[:0]
	for _, s := range g.Sparks {
		s.X += s.DX
		s.Y += s.DY
		s.Time--
		if s.Time > 0 {
			newSparks = append(newSparks, s)
		}
	}
	g.Sparks = newSparks
}

func (g *Game) MakePing(x, y float64, color rl.Color) {
	g.Pings = append(g.Pings, Ping{X: x, Y: y, Radius: 0, Color: color})
}

func (g *Game) UpdatePings() {
	newPings := g.Pings[:0]
	for _, p := range g.Pings {
		p.Radius += 2
		if p.Radius <= g.PingMaxPx {
			newPings = append(newPings, p)
		}
	}
	g.Pings = newPings
}

func (g *Game) Update() {
	// Update music stream
	g.UpdateMusic()

	// Help overlay dismissal
	if g.ShowHelp {
		if g.overlayDismissPressed() {
			g.ShowHelp = false
		}
		return
	}

	// Credits overlay dismissal
	if g.ShowCredits {
		if g.overlayDismissPressed() {
			g.ShowCredits = false
		}
		return
	}

	// Escape toggles options menu from any state
	if rl.IsKeyPressed(rl.KeyEscape) {
		g.Paused = !g.Paused
	}

	if g.Paused {
		g.UpdateOptions()
		return
	}

	switch g.State {
	case StateTitle:
		g.UpdateTitle()
	case StatePlaying:
		g.UpdatePlaying()
	case StateGameOver:
		g.UpdateGameOver()
	}
}

func (g *Game) UpdatePlaying() {
	// Update all objects
	if g.Playing && g.Mode == 3 {
		// Super hot mode - only move when player moves
		if g.Act(g.Player) {
			for i := 1; i < len(g.Objects); i++ {
				g.Act(g.Objects[i])
			}
			g.Tick++
		}
	} else {
		g.StepSimulation()
	}

	// Camera follow and populate new screens
	if g.Playing && g.Player != nil {
		targetCamX := math.Floor(g.Player.X)
		targetCamX = max(0, min(targetCamX-math.Mod(targetCamX, ScrollWidth), FinalScreen))
		g.Populate(targetCamX, 2)
		diff := targetCamX - g.CamX
		g.CamX += max(-g.CameraMaxPxf, min(g.CameraMaxPxf, diff))
	}

	// Check game over
	if g.Player != nil && g.Player.X >= FinishLine {
		g.Playing = false
		g.Won = true
	}
	if !g.Playing {
		if g.Score > g.HighScore {
			g.HighScore = g.Score
		}
		if g.Respite > 0 {
			g.Respite--
		} else {
			g.State = StateGameOver
		}
	}
}

func (g *Game) UpdateGameOver() {
	if rl.IsKeyPressed(rl.KeyX) || rl.IsKeyPressed(rl.KeyEnter) {
		g.StartGame()
		return
	}
	g.StepSimulation()
}

func (g *Game) StepSimulation() {
	for _, obj := range g.Objects {
		g.Act(obj)
	}

	// Silently reap thugs more than a screen width behind the player
	if g.Player != nil {
		px := g.Player.X
		for i := len(g.Objects) - 1; i >= 0; i-- {
			obj := g.Objects[i]
			if obj.Kind == Thug && px-obj.X > g.ThugReapPx {
				g.ThugCount--
				g.Threat -= obj.Threat
				g.Objects = slices.Delete(g.Objects, i, i+1)
			}
		}
	}

	g.Tick++

	if g.Tick%g.RespawnF == 0 {
		g.ReapCrumb()
	}
	if g.Tick%g.WaifSpawnF == 0 && g.CamX < FinalScreen && g.Rng.Float64()*100 < g.WaifSpawnPct+float64(g.Level)*g.WaifLevelPct {
		g.SpawnWaif()
	}
	if g.Tick%g.RespawnF == 0 && g.Rng.Float64()*100 < g.RespawnPct {
		g.Log("respawn", nil)
		g.Respawn()
	}

	g.UpdatePings()
	g.UpdateSparks()
	g.UpdateShake()
}

func (g *Game) UpdateShake() {
	if g.ShakeFactor > 0 {
		g.ShakeFactor = max(0, g.ShakeFactor-g.ShakeDecay)
	}
}

func (g *Game) StartGame() {
	won := g.Won

	g.State = StatePlaying
	g.Playing = true
	g.Won = false
	g.Respite = g.RespiteF
	g.CamX = 0
	g.CamY = 16
	g.ShakeFactor = 0
	g.Tick = 0
	g.Threat = 0
	g.ThugCount = 0
	g.WaifCount = 0
	g.NextID = 0

	// Reset populated tiles back to unpopulated
	for r := range g.TileMap {
		for c := range g.TileMap[r] {
			tile := g.TileMap[r][c]
			if tile != 0 && tile != g.EndzoneTile &&
				!(tile >= g.FirstWallTile && tile < g.FirstWallTile+g.WallTileCnt) {
				g.TileMap[r][c] = 0
			}
		}
	}

	if won {
		// Continue: preserve score, bombs, mines, hearts; increase difficulty
		g.ThreatMax += g.ThreatMaxInc
		g.Level++
	} else {
		// Game over: reset everything
		g.HeartCount = g.StartHearts
		g.BombCount = g.StartBombs
		g.MineCount = g.StartMines
		g.Score = 0
		g.ThreatMax = g.ThreatMaxBase
		g.Level = 0
	}

	g.Seed = time.Now().UnixNano()
	g.Rng = rand.New(rand.NewSource(g.Seed))

	g.Objects = nil
	g.Crumbs = nil
	g.Sparks = nil
	g.Pings = nil

	// Reset path
	g.resetPathGrid()

	// Create player
	g.Player = g.Emplace(g.DefPlayer, 64, 80, func(obj *GameObject) {
		obj.PathC = -1
		obj.PathR = -1
		obj.Controller = PlayerInput
		obj.PostMove = PlayerPostMove
	})

	// Initial population
	g.Populate(0, 1)
	if g.Player != nil {
		g.UpdatePath(g.Player)
	}
}
