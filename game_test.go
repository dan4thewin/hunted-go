package main

import (
	"testing"

	"github.com/agiledragon/gomonkey/v2"
)

// ===========================================================================
// PlaceMine
// ===========================================================================
//
// Decrements MineCount and emplaces a mine at the player's position.
// The mine's FrameFunc arms it on the second animation tick.

func TestPlaceMine(t *testing.T) {
	t.Run("HasMines", func(t *testing.T) {
		g := newObjGame()
		g.MineCount = 2
		p := newTestObj(50, 60, Player)
		g.PlaceMine(p)
		if g.MineCount != 1 {
			t.Errorf("expected MineCount=1, got %d", g.MineCount)
		}
		if len(g.Objects) != 1 {
			t.Fatal("should emplace a mine")
		}
		mine := g.Objects[0]
		if mine.X != 50 || mine.Y != 60 {
			t.Error("mine should be at player position")
		}
	})

	t.Run("NoMines", func(t *testing.T) {
		g := newObjGame()
		g.MineCount = 0
		p := newTestObj(50, 60, Player)
		g.PlaceMine(p)
		if len(g.Objects) != 0 {
			t.Error("should not emplace when no mines")
		}
	})

	t.Run("ArmingSequence", func(t *testing.T) {
		// The mine's FrameFunc first sets Armed, then sets Ignore=false.
		g := newObjGame()
		g.MineCount = 1
		p := newTestObj(50, 60, Player)
		g.PlaceMine(p)
		mine := g.Objects[0]
		if mine.FrameFunc == nil {
			t.Fatal("mine should have FrameFunc")
		}
		// First call: arms the mine.
		mine.FrameFunc(mine)
		if !mine.Armed {
			t.Error("first call should arm")
		}
		// Second call: makes it active (not ignored).
		mine.FrameFunc(mine)
		if mine.Ignore {
			t.Error("second call should clear Ignore")
		}
	})
}

// ===========================================================================
// ThrowBomb
// ===========================================================================
//
// Decrements BombCount, emplaces a bomb with the player's last direction.

func TestThrowBomb(t *testing.T) {
	t.Run("HasBombs", func(t *testing.T) {
		g := newObjGame()
		g.BombCount = 2
		p := newTestObj(50, 60, Player)
		p.LastDir = DirLeft
		g.ThrowBomb(p)
		if g.BombCount != 1 {
			t.Errorf("expected BombCount=1, got %d", g.BombCount)
		}
		if len(g.Objects) != 1 {
			t.Fatal("should emplace a bomb")
		}
		bomb := g.Objects[0]
		if bomb.Dir != DirLeft {
			t.Error("bomb should inherit player's LastDir")
		}
		// Exercise the bomb's FrameFunc: first call arms, second clears Ignore.
		bomb.FrameFunc(bomb)
		if !bomb.Armed {
			t.Error("first call should arm the bomb")
		}
		bomb.FrameFunc(bomb)
		if bomb.Ignore {
			t.Error("second call should clear Ignore")
		}
	})

	t.Run("NoBombs", func(t *testing.T) {
		g := newObjGame()
		g.BombCount = 0
		p := newTestObj(50, 60, Player)
		g.ThrowBomb(p)
		if len(g.Objects) != 0 {
			t.Error("should not emplace when no bombs")
		}
	})
}

// ===========================================================================
// OpenCrate
// ===========================================================================
//
// Calls Crunch to destroy the crate, then awards 1-2 random bombs/mines.

func TestOpenCrate(t *testing.T) {
	g := newObjGame()
	crate := g.Emplace(g.DefCrate, 50, 60)
	crate.Final = g.DefJunk
	startBombs := g.BombCount
	startMines := g.MineCount
	g.OpenCrate(crate)
	gained := (g.BombCount - startBombs) + (g.MineCount - startMines)
	if gained < 1 || gained > 2 {
		t.Errorf("should gain 1-2 items, got %d", gained)
	}
}

// ===========================================================================
// Crunch
// ===========================================================================
//
// Removes an object and adjusts score.  Spawns the object's Final sprite
// if the death occurs in the safe corridor (or for waifs/player death).

func TestCrunch(t *testing.T) {
	t.Run("NoFinal", func(t *testing.T) {
		// Object without Final → Crunch returns false.
		g := newObjGame()
		o := newTestObj(50, 50, Thug)
		o.Final = nil
		g.Objects = []*GameObject{o}
		ok := g.Crunch(o)
		if ok {
			t.Error("should return false without Final")
		}
	})

	t.Run("ThugScore", func(t *testing.T) {
		// Killing a thug during play awards score based on speed.
		g := newObjGame()
		g.Playing = true
		o := newTestObj(50, 50, Thug)
		o.Speed = 2
		o.Final = g.Thugs[0].Final
		g.Objects = []*GameObject{o}
		g.Crunch(o)
		expected := int(2 * float64(g.ThugScoreX))
		if g.Score != expected {
			t.Errorf("expected score=%d, got %d", expected, g.Score)
		}
	})

	t.Run("WaifPenalty", func(t *testing.T) {
		// Killing a waif during play subtracts score.
		g := newObjGame()
		g.Playing = true
		o := newTestObj(50, 50, Waif)
		o.Speed = 1
		o.Final = g.DefWaif.Final
		g.Objects = []*GameObject{o}
		g.Crunch(o)
		expected := -int(1 * float64(g.WaifPenaltyX))
		if g.Score != expected {
			t.Errorf("expected score=%d, got %d", expected, g.Score)
		}
	})

	t.Run("ThugInCorridor", func(t *testing.T) {
		// Thug within TopEdge..BottomEdge → spawns Final sprite.
		g := newObjGame()
		g.Playing = true
		o := newTestObj(50, float64(TopEdge+10), Thug)
		o.Speed = 1
		o.Final = g.Thugs[0].Final
		g.Objects = []*GameObject{o}
		g.Crunch(o)
		if len(g.Objects) != 1 {
			t.Fatal("should spawn Final")
		}
	})

	t.Run("ThugOutsideCorridor", func(t *testing.T) {
		// Thug outside the corridor → no Final sprite, returns false.
		g := newObjGame()
		g.Playing = true
		o := newTestObj(50, float64(TopEdge-1), Thug)
		o.Speed = 1
		o.Final = g.Thugs[0].Final
		g.Objects = []*GameObject{o}
		ok := g.Crunch(o)
		if ok {
			t.Error("should return false for thug outside corridor")
		}
	})

	t.Run("NotPlaying_NoScore", func(t *testing.T) {
		// Not playing → no score change.
		g := newObjGame()
		g.Playing = false
		o := newTestObj(50, 50, Thug)
		o.Speed = 2
		o.Final = g.Thugs[0].Final
		g.Objects = []*GameObject{o}
		g.Crunch(o)
		if g.Score != 0 {
			t.Error("score should not change when not playing")
		}
	})

	t.Run("PlayerDeath_NoHearts", func(t *testing.T) {
		// Player with 1 heart → Reap decrements to 0 → Crunch spawns Final death sprite.
		g := newObjGame()
		g.HeartCount = 1
		o := newTestObj(50, 50, Player)
		o.Speed = 1
		o.Final = g.DefPlayer.Final
		g.Objects = []*GameObject{o}
		ok := g.Crunch(o)
		if !ok {
			t.Error("should spawn Final for player death")
		}
	})

	t.Run("SpeedZero_NoScore", func(t *testing.T) {
		// Thug with Speed=0 → no score awarded even when Playing.
		// Covers the p.Speed > 0 false branch.
		g := newObjGame()
		g.Playing = true
		o := newTestObj(50, float64(TopEdge+10), Thug)
		o.Speed = 0
		o.Final = g.Thugs[0].Final
		g.Objects = []*GameObject{o}
		g.Crunch(o)
		if g.Score != 0 {
			t.Error("should not award score when speed is 0")
		}
	})

	t.Run("FinalIsJunk_NoSound", func(t *testing.T) {
		// When a thug's Final is DefJunk, the crunch sound is NOT played.
		// Covers the p.Final != g.DefJunk false branch.
		g := newObjGame()
		g.Playing = true
		o := newTestObj(50, float64(TopEdge+10), Thug)
		o.Speed = 1
		o.Final = g.DefJunk // junk death → no crunch sound
		g.Objects = []*GameObject{o}
		ok := g.Crunch(o)
		if !ok {
			t.Error("should still spawn Final")
		}
	})

	t.Run("ThugBelowCorridor", func(t *testing.T) {
		// Thug below BottomEdge → no Final. Covers p.Y <= BottomEdge false branch.
		g := newObjGame()
		o := newTestObj(50, float64(BottomEdge+1), Thug)
		o.Final = g.Thugs[0].Final
		g.Objects = []*GameObject{o}
		ok := g.Crunch(o)
		if ok {
			t.Error("should return false for thug below corridor")
		}
	})
}

// ===========================================================================
// Reap
// ===========================================================================
//
// Removes an object from g.Objects.  Adjusts counters by kind:
// - Player: decrements hearts, starts invulnerability or ends game
// - Waif: decrements WaifCount
// - Thug: decrements ThugCount and Threat

func TestReap(t *testing.T) {
	t.Run("Thug", func(t *testing.T) {
		g := newObjGame()
		o := newTestObj(50, 50, Thug)
		o.Threat = 1.5
		g.Objects = []*GameObject{o}
		g.ThugCount = 1
		g.Threat = 1.5
		g.Reap(o)
		if g.ThugCount != 0 {
			t.Error("ThugCount should be 0")
		}
		if g.Threat != 0 {
			t.Error("Threat should be 0")
		}
		if len(g.Objects) != 0 {
			t.Error("object should be removed")
		}
	})

	t.Run("Waif", func(t *testing.T) {
		g := newObjGame()
		o := newTestObj(50, 50, Waif)
		g.Objects = []*GameObject{o}
		g.WaifCount = 1
		g.Reap(o)
		if g.WaifCount != 0 {
			t.Error("WaifCount should be 0")
		}
	})

	t.Run("PlayerWithHearts", func(t *testing.T) {
		// Player with hearts > 1 → heart decremented, invulnerability starts.
		g := newObjGame()
		o := newTestObj(50, 50, Player)
		o.InvulnFrames = 60
		g.Objects = []*GameObject{o}
		g.HeartCount = 2
		g.Playing = true
		g.Reap(o)
		if g.HeartCount != 1 {
			t.Error("should decrement hearts")
		}
		if !o.IsInvuln {
			t.Error("should start invulnerability")
		}
		if len(g.Objects) != 1 {
			t.Error("player should NOT be removed (still has hearts)")
		}
	})

	t.Run("PlayerLastHeart", func(t *testing.T) {
		// Player with 1 heart → speed=0, Playing=false, removed.
		g := newObjGame()
		o := newTestObj(50, 50, Player)
		g.Objects = []*GameObject{o}
		g.HeartCount = 1
		g.Playing = true
		g.Reap(o)
		if g.HeartCount != 0 {
			t.Error("should decrement to 0")
		}
		if g.Playing {
			t.Error("should stop playing")
		}
		if o.Speed != 0 {
			t.Error("player speed should be 0")
		}
	})

	t.Run("InvulnPlayerIgnored", func(t *testing.T) {
		// Invulnerable player → Reap returns early.
		g := newObjGame()
		o := newTestObj(50, 50, Player)
		o.IsInvuln = true
		g.Objects = []*GameObject{o}
		g.HeartCount = 2
		g.Reap(o)
		if g.HeartCount != 2 {
			t.Error("invulnerable player should not lose hearts")
		}
		if len(g.Objects) != 1 {
			t.Error("player should remain in objects")
		}
	})
}

// ===========================================================================
// InvulnPreAct / StartInvuln
// ===========================================================================
//
// InvulnPreAct counts down the invulnerability frames.  When expired,
// clears the invulnerability flag and removes itself.

func TestInvulnPreAct(t *testing.T) {
	t.Run("Countdown", func(t *testing.T) {
		g := newObjGame()
		p := newTestObj(0, 0, Player)
		p.Delay = 5
		p.IsInvuln = true
		p.PreAct = InvulnPreAct
		proceed := InvulnPreAct(g, p)
		if !proceed {
			t.Error("should return true (allow movement)")
		}
		if p.Delay != 4 {
			t.Errorf("expected Delay=4, got %d", p.Delay)
		}
	})

	t.Run("Expired", func(t *testing.T) {
		g := newObjGame()
		p := newTestObj(0, 0, Player)
		p.Delay = 0
		p.IsInvuln = true
		p.PreAct = InvulnPreAct
		InvulnPreAct(g, p)
		if p.IsInvuln {
			t.Error("should clear invulnerability")
		}
		if p.PreAct != nil {
			t.Error("should clear PreAct")
		}
	})
}

func TestStartInvuln(t *testing.T) {
	g := newObjGame()
	p := newTestObj(0, 0, Player)
	p.InvulnFrames = 120
	g.StartInvuln(p)
	if !p.IsInvuln {
		t.Error("should set IsInvuln")
	}
	if p.Delay != 120 {
		t.Errorf("expected Delay=120, got %d", p.Delay)
	}
	if p.PreAct == nil {
		t.Error("should set PreAct")
	}
}

// ===========================================================================
// Act
// ===========================================================================
//
// Core game tick for one object: runs PreAct, gets direction from
// Controller or Dir, applies speed decay, walks, and advances animation.

func TestAct(t *testing.T) {
	t.Run("NilObject", func(t *testing.T) {
		g := newObjGame()
		if g.Act(nil) {
			t.Error("nil should return false")
		}
	})

	t.Run("PreActBlocks", func(t *testing.T) {
		// PreAct returns false → Act returns false.
		g := newObjGame()
		p := newTestObj(50, 50, Player)
		p.PreAct = func(g *Game, p *GameObject) bool { return false }
		if g.Act(p) {
			t.Error("should return false when PreAct blocks")
		}
	})

	t.Run("NoDirection_SpeedDecay", func(t *testing.T) {
		// Direction is 0 → speed decays toward 0.
		g := newObjGame()
		p := newTestObj(50, 50, Thug)
		p.Dir = 0
		p.Speed = 1.0
		p.CurrentPoses = []int{1, 2}
		p.CurrentFrames = 4
		g.Act(p)
		if p.Speed >= 1.0 {
			t.Error("speed should decay")
		}
	})

	t.Run("WithDirection_Moves", func(t *testing.T) {
		// Object has a direction → walks and updates position.
		g := newObjGame()
		p := newTestObj(50, 80, Player)
		p.Dir = DirRight
		p.Speed = 1.0
		p.CurrentPoses = []int{1, 2}
		p.CurrentFrames = 4
		startX := p.X
		g.Act(p)
		if p.X <= startX {
			t.Error("should have moved right")
		}
	})

	t.Run("Controller", func(t *testing.T) {
		// Object with Controller → direction comes from Controller.
		g := newObjGame()
		p := newTestObj(50, 80, Player)
		p.Controller = func(g *Game, p *GameObject) int {
			return DirLeft
		}
		p.CurrentPoses = []int{1, 2}
		p.CurrentFrames = 4
		startX := p.X
		g.Act(p)
		if p.X >= startX {
			t.Error("should have moved left from controller")
		}
	})

	t.Run("FlipOnLeft", func(t *testing.T) {
		// Moving left sets Flip to true.
		g := newObjGame()
		p := newTestObj(50, 80, Player)
		p.Dir = DirLeft
		p.CurrentPoses = []int{1, 2}
		p.CurrentFrames = 4
		g.Act(p)
		if !p.Flip {
			t.Error("should set Flip when moving left")
		}
	})

	t.Run("PostMove", func(t *testing.T) {
		// PostMove is called after successful Walk.
		g := newObjGame()
		called := false
		p := newTestObj(50, 80, Player)
		p.Dir = DirRight
		p.CurrentPoses = []int{1, 2}
		p.CurrentFrames = 4
		p.PostMove = func(g *Game, p *GameObject) { called = true }
		g.Act(p)
		if !called {
			t.Error("PostMove should be called after Walk")
		}
	})

	t.Run("RelaxedMode", func(t *testing.T) {
		// Mode 2 thugs use RelaxedSpeedX for base speed.
		g := newObjGame()
		g.Mode = 2
		p := newTestObj(50, 80, Thug)
		p.BaseSpeed = 2
		p.Dir = DirRight
		p.Speed = 5
		p.CurrentPoses = []int{1, 2}
		p.CurrentFrames = 4
		g.Act(p)
		// Speed should decay toward relaxed base (BaseSpeed * RelaxedSpeedX).
		relaxedBase := p.BaseSpeed * g.RelaxedSpeedX
		if p.Speed < relaxedBase {
			t.Errorf("speed should not go below relaxed base %f, got %f", relaxedBase, p.Speed)
		}
	})

	t.Run("WalkFails_SpeedZero", func(t *testing.T) {
		// When Walk fails (wall blocks all movement), speed is set to 0
		// and the object goes idle. This tests the else-branch at line 162.
		g := newObjGame()
		// Surround object with walls so Walk returns false.
		g.TileMap = newTileMap(4, 4, g.FirstWallTile)
		p := newTestObj(8, 8, Thug)
		p.Dir = DirRight
		p.Speed = 2.0
		p.CurrentPoses = []int{1, 2}
		p.CurrentFrames = 4
		g.Act(p)
		if p.Speed != 0 {
			t.Errorf("speed should be 0 after Walk failure, got %f", p.Speed)
		}
	})

	t.Run("PureVertical_NoFlip", func(t *testing.T) {
		// Pure Up/Down direction has no left/right component (lr==0),
		// so Flip is unchanged. Covers the lr != 0 false branch.
		g := newObjGame()
		p := newTestObj(50, 80, Player)
		p.Dir = DirUp
		p.Flip = true // should remain unchanged
		p.CurrentPoses = []int{1, 2}
		p.CurrentFrames = 4
		g.Act(p)
		if !p.Flip {
			t.Error("Flip should be unchanged for pure vertical movement")
		}
	})

	t.Run("PreActAllows", func(t *testing.T) {
		// PreAct returning true allows Act to continue.
		// Covers the p.PreAct(g,p) true branch.
		g := newObjGame()
		p := newTestObj(50, 80, Player)
		p.PreAct = func(g *Game, p *GameObject) bool { return true }
		p.Dir = DirRight
		p.CurrentPoses = []int{1, 2}
		p.CurrentFrames = 4
		startX := p.X
		g.Act(p)
		if p.X <= startX {
			t.Error("Act should proceed when PreAct returns true")
		}
	})
}

// ===========================================================================
// MakeSparks / UpdateSparks
// ===========================================================================
//
// Particle system for explosions.  MakeSparks creates 10-30 sparks with
// random velocities.  UpdateSparks moves them and removes expired ones.

func TestMakeSparks(t *testing.T) {
	g := newObjGame()
	g.MakeSparks(50, 60)
	if len(g.Sparks) < 10 || len(g.Sparks) > 30 {
		t.Errorf("expected 10-30 sparks, got %d", len(g.Sparks))
	}
	for _, s := range g.Sparks {
		if s.X != 50 || s.Y != 60 {
			t.Error("sparks should start at origin")
			break
		}
	}
}

func TestUpdateSparks(t *testing.T) {
	t.Run("Movement", func(t *testing.T) {
		g := newObjGame()
		g.Sparks = []Spark{{X: 10, Y: 10, DX: 1, DY: -1, Time: 5}}
		g.UpdateSparks()
		if len(g.Sparks) != 1 {
			t.Fatal("spark should survive")
		}
		if g.Sparks[0].X != 11 || g.Sparks[0].Y != 9 {
			t.Error("spark should move by DX/DY")
		}
		if g.Sparks[0].Time != 4 {
			t.Error("Time should decrement")
		}
	})

	t.Run("Expiration", func(t *testing.T) {
		g := newObjGame()
		g.Sparks = []Spark{{X: 10, Y: 10, DX: 1, DY: 0, Time: 1}}
		g.UpdateSparks()
		if len(g.Sparks) != 0 {
			t.Error("spark with Time=1 should expire after update")
		}
	})
}

// ===========================================================================
// MakePing / UpdatePings
// ===========================================================================
//
// Expanding ring effects.  MakePing creates one; UpdatePings expands
// radius by 2 per frame and removes when past PingMaxPx.

func TestMakePing(t *testing.T) {
	g := newObjGame()
	g.MakePing(50, 60, PicoRed)
	if len(g.Pings) != 1 {
		t.Fatal("should add one ping")
	}
	if g.Pings[0].Radius != 0 {
		t.Error("initial radius should be 0")
	}
}

func TestUpdatePings(t *testing.T) {
	t.Run("Expansion", func(t *testing.T) {
		g := newObjGame()
		g.Pings = []Ping{{X: 50, Y: 60, Radius: 0}}
		g.UpdatePings()
		if g.Pings[0].Radius != 2 {
			t.Errorf("expected radius=2, got %f", g.Pings[0].Radius)
		}
	})

	t.Run("Expiration", func(t *testing.T) {
		g := newObjGame()
		g.Pings = []Ping{{X: 50, Y: 60, Radius: g.PingMaxPx}}
		g.UpdatePings()
		if len(g.Pings) != 0 {
			t.Error("ping past PingMaxPx should expire")
		}
	})
}

// ===========================================================================
// Explode
// ===========================================================================
//
// Sets ShakeFactor, creates sparks, plays explosion sound.

func TestExplode(t *testing.T) {
	g := newObjGame()
	patches := gomonkey.ApplyMethodFunc(g, "PlayExplosion", func() {})
	defer patches.Reset()
	g.Explode(50, 60)
	if g.ShakeFactor != 1 {
		t.Error("ShakeFactor should be 1")
	}
	if len(g.Sparks) == 0 {
		t.Error("should create sparks")
	}
}

// ===========================================================================
// UpdateShake
// ===========================================================================
//
// Decays ShakeFactor by ShakeDecay each frame, clamped to 0.

func TestUpdateShake(t *testing.T) {
	t.Run("Decay", func(t *testing.T) {
		g := newObjGame()
		g.ShakeFactor = 0.5
		g.UpdateShake()
		expected := 0.5 - g.ShakeDecay
		if g.ShakeFactor != expected {
			t.Errorf("expected %f, got %f", expected, g.ShakeFactor)
		}
	})

	t.Run("ClampToZero", func(t *testing.T) {
		g := newObjGame()
		g.ShakeFactor = 0.01 // less than ShakeDecay
		g.UpdateShake()
		if g.ShakeFactor != 0 {
			t.Errorf("should clamp to 0, got %f", g.ShakeFactor)
		}
	})

	t.Run("AlreadyZero", func(t *testing.T) {
		g := newObjGame()
		g.ShakeFactor = 0
		g.UpdateShake()
		if g.ShakeFactor != 0 {
			t.Error("should stay 0")
		}
	})
}

// ===========================================================================
// StepSimulation
// ===========================================================================
//
// Acts all objects, reaps distant thugs, increments tick, spawns on
// schedule, and updates particles.

func TestStepSimulation(t *testing.T) {
	t.Run("ActsAllObjects", func(t *testing.T) {
		g := newObjGame()
		p := newTestObj(50, 80, Player)
		p.Dir = DirRight
		p.CurrentPoses = []int{1, 2}
		p.CurrentFrames = 4
		g.Objects = []*GameObject{p}
		g.Player = p
		startX := p.X
		g.StepSimulation()
		if p.X <= startX {
			t.Error("player should have moved")
		}
	})

	t.Run("ReapsDistantThugs", func(t *testing.T) {
		// Thug more than ThugReapPx behind the player is reaped.
		g := newObjGame()
		g.Player = newTestObj(500, 80, Player)
		thug := newTestObj(100, 80, Thug) // 400 px behind
		thug.Threat = 1
		g.Objects = []*GameObject{g.Player, thug}
		g.ThugCount = 1
		g.Threat = 1
		g.StepSimulation()
		if g.ThugCount != 0 {
			t.Error("distant thug should be reaped")
		}
	})

	t.Run("TickIncrements", func(t *testing.T) {
		g := newObjGame()
		g.Player = newTestObj(50, 80, Player)
		g.Objects = []*GameObject{g.Player}
		g.StepSimulation()
		if g.Tick != 1 {
			t.Errorf("expected Tick=1, got %d", g.Tick)
		}
	})

	t.Run("ReapCrumbOnSchedule", func(t *testing.T) {
		// When Tick is a multiple of RespawnF, the oldest crumb is reaped.
		g := newObjGame()
		g.Player = newTestObj(50, 80, Player)
		g.Objects = []*GameObject{g.Player}
		g.Crumbs = []*Crumb{{X: 1, Y: 1}, {X: 2, Y: 2}}
		g.Tick = g.RespawnF - 1 // will increment to RespawnF
		g.StepSimulation()
		if len(g.Crumbs) != 1 {
			t.Errorf("expected 1 crumb after reap, got %d", len(g.Crumbs))
		}
	})

	t.Run("WaifSpawnOnSchedule", func(t *testing.T) {
		// Waif spawns when Tick is a multiple of WaifSpawnF and chance passes.
		g := newObjGame()
		g.Player = newTestObj(50, 80, Player)
		g.Objects = []*GameObject{g.Player}
		g.Tick = g.WaifSpawnF - 1
		g.WaifSpawnPct = 100 // guarantee spawn
		g.CamX = 0           // not past FinalScreen
		initLen := len(g.Objects)
		g.StepSimulation()
		if len(g.Objects) <= initLen {
			t.Error("waif should have spawned")
		}
	})

	t.Run("RespawnOnSchedule", func(t *testing.T) {
		// Thug respawns when Tick is a multiple of RespawnF and chance passes.
		g := newObjGame()
		g.Player = newTestObj(50, 80, Player)
		g.Objects = []*GameObject{g.Player}
		g.Tick = g.RespawnF - 1
		g.RespawnPct = 100 // guarantee spawn
		g.ThreatMax = 100  // enough budget
		initLen := len(g.Objects)
		g.StepSimulation()
		if len(g.Objects) <= initLen {
			t.Error("thug should have respawned")
		}
	})

	t.Run("NilPlayer_SkipsReap", func(t *testing.T) {
		// With nil Player, the thug-reap loop is skipped.
		// Covers the g.Player != nil false branch.
		g := newObjGame()
		g.Player = nil
		g.Objects = nil
		g.StepSimulation() // should not panic
		if g.Tick != 1 {
			t.Errorf("expected Tick=1, got %d", g.Tick)
		}
	})
}

// ===========================================================================
// StartGame
// ===========================================================================
//
// Initializes a new game: resets state, creates player, populates the
// first screen.  After a win, preserves score and increases difficulty.

func TestStartGame(t *testing.T) {
	t.Run("NewGame", func(t *testing.T) {
		g := newObjGame()
		g.Won = false
		g.StartGame()
		if g.State != StatePlaying {
			t.Error("state should be StatePlaying")
		}
		if !g.Playing {
			t.Error("should be playing")
		}
		if g.Player == nil {
			t.Error("player should be created")
		}
		if g.HeartCount != g.StartHearts {
			t.Errorf("hearts should be %d, got %d", g.StartHearts, g.HeartCount)
		}
		if g.BombCount != g.StartBombs {
			t.Errorf("bombs should be %d, got %d", g.StartBombs, g.BombCount)
		}
		if g.Level != 0 {
			t.Error("level should be 0 on new game")
		}
	})

	t.Run("AfterWin", func(t *testing.T) {
		// Won=true → preserves score, increases difficulty.
		g := newObjGame()
		g.Won = true
		g.Score = 500
		g.BombCount = 5
		g.MineCount = 3
		g.HeartCount = 2
		g.ThreatMax = g.ThreatMaxStart
		g.StartGame()
		if g.Score != 500 {
			t.Error("score should be preserved after win")
		}
		if g.BombCount != 5 {
			t.Error("bombs should be preserved after win")
		}
		if g.Level != 1 {
			t.Error("level should increment after win")
		}
		if g.ThreatMax != g.ThreatMaxStart+g.ThreatMaxInc {
			t.Error("ThreatMax should increase after win")
		}
	})

	t.Run("TileReset", func(t *testing.T) {
		// StartGame clears non-wall, non-endzone tiles, then repopulates.
		// Verify walls and endzone survive the clearing step by checking
		// they still hold their original values after StartGame.
		// Also verify a non-wall, non-endzone tile (99) gets cleared.
		g := newObjGame()
		g.TileMap[5][6] = g.FirstWallTile // wall → should stay
		g.TileMap[5][7] = g.EndzoneTile   // endzone → should stay
		g.TileMap[5][8] = 99              // arbitrary → should be cleared
		g.StartGame()
		if g.TileMap[5][6] != g.FirstWallTile {
			t.Error("wall tiles should be preserved")
		}
		if g.TileMap[5][7] != g.EndzoneTile {
			t.Error("endzone tiles should be preserved")
		}
		if g.TileMap[5][8] == 99 {
			t.Error("non-wall, non-endzone tiles should be cleared")
		}
	})
}

// ===========================================================================
// UpdatePlaying
// ===========================================================================
//
// Main game loop: steps simulation, follows camera, checks win/loss.

func TestUpdatePlaying(t *testing.T) {
	t.Run("WinCondition", func(t *testing.T) {
		// Player past FinishLine → game won.
		g := newObjGame()
		g.Playing = true
		g.Player = newTestObj(FinishLine+1, 80, Player)
		g.Player.CurrentPoses = []int{1, 2}
		g.Player.CurrentFrames = 4
		g.Objects = []*GameObject{g.Player}
		g.Respite = 0
		g.UpdatePlaying()
		if !g.Won {
			t.Error("should set Won")
		}
		if g.Playing {
			t.Error("should stop playing")
		}
	})

	t.Run("Respite", func(t *testing.T) {
		// After game ends, Respite counts down before StateGameOver.
		g := newObjGame()
		g.Playing = false
		g.Respite = 2
		g.Player = newTestObj(50, 80, Player)
		g.Objects = []*GameObject{g.Player}
		g.UpdatePlaying()
		if g.Respite != 1 {
			t.Errorf("expected Respite=1, got %d", g.Respite)
		}
		if g.State == StateGameOver {
			t.Error("should not transition to GameOver yet")
		}
	})

	t.Run("RespiteExpired", func(t *testing.T) {
		g := newObjGame()
		g.Playing = false
		g.Respite = 0
		g.Player = newTestObj(50, 80, Player)
		g.Objects = []*GameObject{g.Player}
		g.UpdatePlaying()
		if g.State != StateGameOver {
			t.Error("should transition to StateGameOver")
		}
	})

	t.Run("HighScore", func(t *testing.T) {
		// When game ends, high score updates if current score is higher.
		g := newObjGame()
		g.Playing = false
		g.Score = 1000
		g.HighScore = 500
		g.Respite = 0
		g.Player = newTestObj(50, 80, Player)
		g.Objects = []*GameObject{g.Player}
		g.UpdatePlaying()
		if g.HighScore != 1000 {
			t.Errorf("expected HighScore=1000, got %d", g.HighScore)
		}
	})

	t.Run("SuperHotMode", func(t *testing.T) {
		// Mode 3 ("super hot"): simulation only advances when the player
		// actually moves. Other objects act only when Act(player) returns true.
		g := newObjGame()
		g.Playing = true
		g.Mode = 3
		g.Player = newTestObj(50, 80, Player)
		g.Player.Dir = DirRight
		g.Player.CurrentPoses = []int{1, 2}
		g.Player.CurrentFrames = 4
		// Add a second object to verify it also acts when player moves.
		other := newTestObj(100, 80, Thug)
		other.Dir = DirLeft
		other.CurrentPoses = []int{1, 2}
		other.CurrentFrames = 4
		g.Objects = []*GameObject{g.Player, other}
		startOther := other.X
		g.UpdatePlaying()
		if other.X >= startOther {
			t.Error("other objects should move in super hot mode when player moves")
		}
	})

	t.Run("CameraFollow", func(t *testing.T) {
		// Camera tracks the player, advancing CamX toward the player position.
		g := newObjGame()
		g.Playing = true
		g.CamX = 0
		g.CameraMaxPxf = 4
		g.Player = newTestObj(200, 80, Player)
		g.Player.CurrentPoses = []int{1, 2}
		g.Player.CurrentFrames = 4
		g.Objects = []*GameObject{g.Player}
		g.UpdatePlaying()
		if g.CamX <= 0 {
			t.Error("camera should advance toward player")
		}
	})
}
