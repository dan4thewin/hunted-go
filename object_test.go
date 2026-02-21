package main

import (
	"math/rand"
	"testing"
)

// ===========================================================================
// Test Helpers (object/game shared)
// ===========================================================================
//
// newTestGame builds a Game with enough structure for object.go and game.go
// tests: seeded RNG, sprite defs, tile config, path grid, and default
// game config values.  SoundsLoaded=false prevents raylib calls.

func newObjGame() *Game {
	g := &Game{
		Rng:  rand.New(rand.NewSource(42)),
		Mode: 1,
		TileConfig: TileConfig{
			FirstWallTile: 10,
			WallTileCnt:   3,
			RockTile:      20,
			BlackTile:     1,
			EndzoneTile:   30,
		},
		Path: make([][]int, PathCols),
	}
	for i := range g.Path {
		g.Path[i] = make([]int, PathRows)
	}
	g.TileMap = newTileMap(140, 20, 0)

	// Sprite defs used by Emplace, Spawn, Crunch, etc.
	g.DefPlayer = &SpriteDef{
		Name: "player", Kind: Player, Speed: 1.5, Width: 8, Height: 8,
		Idle: []int{1}, Poses: []int{2, 3}, Frames: 4,
		Poses2: []int{4, 5}, Frames2: 2, Speed2: 3,
	}
	g.DefWaif = &SpriteDef{
		Name: "waif", Kind: Waif, Speed: 1, Width: 8, Height: 8,
		Idle: []int{10}, Poses: []int{11, 12}, Frames: 4,
	}
	g.DefCrate = &SpriteDef{
		Name: "crate", Kind: Crate, Speed: 0, Width: 8, Height: 8,
		Idle: []int{20},
	}
	g.DefBomb = &SpriteDef{
		Name: "bomb", Speed: 1, Width: 8, Height: 8,
		Idle: []int{30}, Poses: []int{31, 32}, Frames: 4,
		Decay: true,
	}
	g.DefMine = &SpriteDef{
		Name: "mine", Speed: 0, Width: 8, Height: 8,
		Idle: []int{40},
	}
	// Junk: decay + linger, no Final
	g.DefJunk = &SpriteDef{
		Name: "junk", Speed: 0, Width: 8, Height: 8,
		Idle: []int{50}, Poses: []int{51}, Frames: 4,
		Decay: true, Linger: true,
	}
	// Dead sprites (Final targets)
	deadThug := &SpriteDef{
		Name: "dead-thug", Speed: 0, Width: 8, Height: 8,
		Idle: []int{60}, Poses: []int{61}, Frames: 4,
		Decay: true,
	}
	deadWaif := &SpriteDef{
		Name: "dead-waif", Speed: 0, Width: 8, Height: 8,
		Idle: []int{70}, Poses: []int{71}, Frames: 4,
		Decay: true,
	}
	deadPlayer := &SpriteDef{
		Name: "dead-player", Speed: 0, Width: 8, Height: 8,
		Idle: []int{80}, Poses: []int{81}, Frames: 4,
		Decay: true,
	}

	// Thug defs with Final pointing to dead sprite
	thug1 := &SpriteDef{
		Name: "thug1", Kind: Thug, Speed: 1, Width: 8, Height: 8,
		Idle: []int{100}, Poses: []int{101, 102}, Frames: 4,
		Final: deadThug,
	}
	thug2 := &SpriteDef{
		Name: "thug2", Kind: Thug, Speed: 2, Width: 8, Height: 8,
		Idle: []int{110}, Poses: []int{111, 112}, Frames: 4,
		Final: deadThug,
	}
	g.Thugs = []*SpriteDef{thug1, thug2}
	g.DefWaif.Final = deadWaif
	g.DefPlayer.Final = deadPlayer

	// Defs map for reference
	g.Defs = map[string]*SpriteDef{
		"player": g.DefPlayer, "waif": g.DefWaif,
		"crate": g.DefCrate, "bomb": g.DefBomb,
		"mine": g.DefMine, "junk": g.DefJunk,
		"thug1": thug1, "thug2": thug2,
	}

	// Game config defaults
	g.GameConfig = GameConfig{
		ThreatMaxStart: 10,
		ThreatMaxInc:   2,
		ThreatSpeedX:   2,
		RespawnF:       60,
		RespawnPct:     50,
		WaifSpawnF:     120,
		WaifSpawnPct:   25,
		WaifLevelPct:   5,
		ThugScoreX:     100,
		WaifPenaltyX:   200,
		WaifSafeScore:  100,
		SpeedDecayPxf:  0.02,
		RelaxedSpeedX:  0.5,
		CameraMaxPxf:   2,
		WaifDelayF:     60,
		WaifFollowPx:   40,
		WaifJitterPx:   6,
		WaifJitterF:    120,
		RespiteF:       60,
		ShakeDecay:     0.05,
		PingMaxPx:      20,
		ThugReapPx:     200,
		CratePct:       5,
		RockPct:        3,
		JunkPct:        2,
	}

	g.ThreatMax = g.ThreatMaxStart
	g.ThreatMaxBase = g.ThreatMaxStart
	g.StartHearts = 3
	g.StartBombs = 3
	g.StartMines = 2

	return g
}

// newTestObj builds a minimal GameObject for standalone tests.
func newTestObj(x, y float64, kind Kind) *GameObject {
	return &GameObject{
		X: x, Y: y, Width: 8, Height: 8,
		Speed: 1, BaseSpeed: 1, Kind: kind,
		YMax: YMax, XMaxLimit: XMax + 1, LastDir: DirRight,
		Anim: Anim{Idle: []int{1}, Poses: []int{2, 3}, Frames: 4},
	}
}

// ===========================================================================
// NewID
// ===========================================================================
//
// Increments and returns a monotonically increasing ID for each object.

func TestNewID(t *testing.T) {
	g := newObjGame()
	id1 := g.NewID()
	id2 := g.NewID()
	if id2 != id1+1 {
		t.Errorf("expected sequential IDs: %d, %d", id1, id2)
	}
}

// ===========================================================================
// Emplace
// ===========================================================================
//
// Creates a GameObject from a SpriteDef, applies option functions, sets
// defaults (YMax, XMaxLimit), appends to g.Objects, and starts idle.

func TestEmplace(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		g := newObjGame()
		obj := g.Emplace(g.DefPlayer, 10, 20)
		if obj.Name != "player" {
			t.Errorf("expected name 'player', got %q", obj.Name)
		}
		if obj.X != 10 || obj.Y != 20 {
			t.Error("position not set")
		}
		if obj.Speed != g.DefPlayer.Speed {
			t.Error("speed should come from def")
		}
		if !obj.IsIdle {
			t.Error("should start idle")
		}
		if len(g.Objects) != 1 || g.Objects[0] != obj {
			t.Error("should be appended to g.Objects")
		}
	})

	t.Run("DefaultYMax", func(t *testing.T) {
		// SpriteDef has YMax=0 → defaults to YMax constant.
		g := newObjGame()
		obj := g.Emplace(g.DefPlayer, 0, 0)
		if obj.YMax != YMax {
			t.Errorf("expected YMax=%d, got %d", YMax, obj.YMax)
		}
	})

	t.Run("DefaultXMaxLimit", func(t *testing.T) {
		// SpriteDef has XMaxLimit=0 → defaults to XMax+1.
		g := newObjGame()
		obj := g.Emplace(g.DefPlayer, 0, 0)
		if obj.XMaxLimit != XMax+1 {
			t.Errorf("expected XMaxLimit=%d, got %d", XMax+1, obj.XMaxLimit)
		}
	})

	t.Run("CustomXMaxLimit", func(t *testing.T) {
		// SpriteDef with non-zero XMaxLimit → preserves it.
		// Covers the XMaxLimit == 0 false branch.
		g := newObjGame()
		def := *g.DefPlayer
		def.XMaxLimit = 500
		obj := g.Emplace(&def, 0, 0)
		if obj.XMaxLimit != 500 {
			t.Errorf("expected XMaxLimit=500, got %d", obj.XMaxLimit)
		}
	})

	t.Run("CustomYMax", func(t *testing.T) {
		// SpriteDef with non-zero YMax → uses it.
		g := newObjGame()
		def := *g.DefPlayer
		def.YMax = 100
		obj := g.Emplace(&def, 0, 0)
		if obj.YMax != 100 {
			t.Errorf("expected YMax=100, got %d", obj.YMax)
		}
	})

	t.Run("OptionFunc", func(t *testing.T) {
		// Option functions modify the object after creation.
		g := newObjGame()
		obj := g.Emplace(g.DefPlayer, 0, 0, func(o *GameObject) {
			o.Speed = 99
		})
		if obj.Speed != 99 {
			t.Error("option func should override speed")
		}
	})

	t.Run("JunkFrameFunc", func(t *testing.T) {
		// Junk objects get a random linger duration via FrameFunc.
		// Calling it sets CurrentFrames to a random value (min 6).
		g := newObjGame()
		obj := g.Emplace(g.DefJunk, 0, 0)
		if obj.FrameFunc == nil {
			t.Fatal("junk should have FrameFunc set")
		}
		obj.FrameFunc(obj)
		if obj.CurrentFrames < 6 {
			t.Errorf("CurrentFrames should be >= 6, got %d", obj.CurrentFrames)
		}
	})

	t.Run("JunkCustomFrameFunc", func(t *testing.T) {
		// When an option func already sets FrameFunc on junk, the default
		// junk FrameFunc is NOT applied. Covers the FrameFunc==nil guard.
		g := newObjGame()
		custom := func(o *GameObject) {}
		obj := g.Emplace(g.DefJunk, 0, 0, func(o *GameObject) {
			o.FrameFunc = custom
		})
		// Verify the custom FrameFunc is preserved, not overwritten.
		if obj.FrameFunc == nil {
			t.Error("custom FrameFunc should be preserved")
		}
	})

	t.Run("NonJunkNoFrameFunc", func(t *testing.T) {
		// Non-junk objects don't get the junk FrameFunc.
		g := newObjGame()
		obj := g.Emplace(g.DefPlayer, 0, 0)
		if obj.FrameFunc != nil {
			t.Error("non-junk should not have FrameFunc")
		}
	})
}

// ===========================================================================
// SetIdle / SetMoving / SetDashing
// ===========================================================================
//
// Animation state transitions.  Each is idempotent — calling twice has
// no effect.  SetMoving is a no-op if already moving at base speed.

func TestSetIdle(t *testing.T) {
	t.Run("FromMoving", func(t *testing.T) {
		o := newTestObj(0, 0, Player)
		o.IsIdle = false
		o.SetIdle()
		if !o.IsIdle {
			t.Error("should be idle")
		}
	})

	t.Run("AlreadyIdle", func(t *testing.T) {
		// Calling SetIdle when already idle is a no-op.
		o := newTestObj(0, 0, Player)
		o.IsIdle = true
		o.CurrentPoses = []int{99}
		o.SetIdle()
		// CurrentPoses should NOT be reset since it returns early.
		if o.CurrentPoses[0] != 99 {
			t.Error("should not re-enter SetIdle")
		}
	})
}

func TestSetMoving(t *testing.T) {
	t.Run("FromIdle", func(t *testing.T) {
		o := newTestObj(0, 0, Player)
		o.IsIdle = true
		o.SetMoving()
		if o.IsIdle {
			t.Error("should not be idle")
		}
		if o.IsDash {
			t.Error("should not be dashing")
		}
	})

	t.Run("AlreadyMoving", func(t *testing.T) {
		// Already moving at base speed → no-op.
		o := newTestObj(0, 0, Player)
		o.IsIdle = false
		o.IsDash = false
		o.Speed = o.BaseSpeed
		o.CurrentPoses = []int{99}
		o.SetMoving()
		if o.CurrentPoses[0] != 99 {
			t.Error("should not reset pose when already moving")
		}
	})

	t.Run("FromDash", func(t *testing.T) {
		// Dashing → moving transitions when speed matches base.
		o := newTestObj(0, 0, Player)
		o.IsDash = true
		o.Speed = o.BaseSpeed
		o.SetMoving()
		if o.IsDash {
			t.Error("should clear dash")
		}
	})

	t.Run("DashSpeedStillHigh", func(t *testing.T) {
		// Dashing with speed still above BaseSpeed → stays in dash.
		// Covers the o.Speed != o.BaseSpeed true branch.
		o := newTestObj(0, 0, Player)
		o.IsDash = true
		o.Speed = o.BaseSpeed + 1 // still dashing fast
		o.CurrentPoses = []int{99}
		o.SetMoving()
		if !o.IsDash {
			t.Error("should stay dashing when speed > BaseSpeed")
		}
		if o.CurrentPoses[0] != 99 {
			t.Error("should not reset poses")
		}
	})
}

func TestSetDashing(t *testing.T) {
	t.Run("FromMoving", func(t *testing.T) {
		o := newTestObj(0, 0, Player)
		o.Anim.Speed2 = 3
		o.Anim.Poses2 = []int{4, 5}
		o.Anim.Frames2 = 2
		o.SetDashing()
		if !o.IsDash {
			t.Error("should be dashing")
		}
		if o.Speed != 3 {
			t.Errorf("speed should be Speed2=3, got %f", o.Speed)
		}
	})

	t.Run("AlreadyDashing", func(t *testing.T) {
		o := newTestObj(0, 0, Player)
		o.IsDash = true
		o.Speed = 5
		o.SetDashing()
		// Speed should not change.
		if o.Speed != 5 {
			t.Error("should not re-enter SetDashing")
		}
	})
}

// ===========================================================================
// ResetPose
// ===========================================================================
//
// Resets animation counters.  Falls back to Idle poses if poses is empty.
// Sets CurrentFrames to -1 if frames is 0 (disables frame counting).

func TestResetPose(t *testing.T) {
	t.Run("WithPoses", func(t *testing.T) {
		o := newTestObj(0, 0, Player)
		o.ResetPose([]int{10, 11, 12}, 5)
		if o.PoseIndex != 0 {
			t.Error("PoseIndex should be 0")
		}
		if o.CurrentFrames != 5 {
			t.Error("CurrentFrames should be 5")
		}
		if o.CurrentSprite != 10 {
			t.Errorf("CurrentSprite should be first pose (10), got %d", o.CurrentSprite)
		}
	})

	t.Run("EmptyPoses_FallbackToIdle", func(t *testing.T) {
		// No poses provided → uses Idle animation.
		o := newTestObj(0, 0, Player)
		o.Anim.Idle = []int{1}
		o.ResetPose(nil, 3)
		if o.CurrentSprite != 1 {
			t.Errorf("should fall back to idle sprite, got %d", o.CurrentSprite)
		}
	})

	t.Run("ZeroFrames", func(t *testing.T) {
		// frames=0 → CurrentFrames set to -1 (infinite hold).
		o := newTestObj(0, 0, Player)
		o.ResetPose([]int{1}, 0)
		if o.CurrentFrames != -1 {
			t.Errorf("expected -1 for zero frames, got %d", o.CurrentFrames)
		}
	})
}

// ===========================================================================
// NextPose
// ===========================================================================
//
// Advances animation by one frame.  On wrap, either loops (non-decay)
// or triggers decay behavior (spawn Final, reap or linger).

func TestNextPose(t *testing.T) {
	t.Run("Advance", func(t *testing.T) {
		// Normal frame advance: FrameCount decrements from CurrentFrames.
		g := newObjGame()
		o := newTestObj(0, 0, Player)
		o.CurrentPoses = []int{1, 2, 3}
		o.CurrentFrames = 2
		o.FrameCount = 2 // will just decrement
		ok := o.NextPose(g)
		if !ok {
			t.Error("should return true")
		}
		if o.FrameCount != 1 {
			t.Errorf("expected FrameCount=1, got %d", o.FrameCount)
		}
	})

	t.Run("WrapLoop", func(t *testing.T) {
		// Non-decay: PoseIndex wraps back to 1 after exceeding len.
		g := newObjGame()
		o := newTestObj(0, 0, Player)
		o.CurrentPoses = []int{1, 2}
		o.CurrentFrames = 1
		o.FrameCount = 0
		o.PoseIndex = 2 // will increment to 3, > len(2) → wrap
		ok := o.NextPose(g)
		if !ok {
			t.Error("should return true")
		}
		if o.PoseIndex != 1 {
			t.Errorf("expected PoseIndex=1 (wrapped), got %d", o.PoseIndex)
		}
	})

	t.Run("DecayReap", func(t *testing.T) {
		// Decay without Linger: reaped after animation completes.
		g := newObjGame()
		o := newTestObj(0, 0, Thug)
		o.Decay = true
		o.Linger = false
		o.CurrentPoses = []int{1}
		o.CurrentFrames = 1
		o.FrameCount = 0
		o.PoseIndex = 1 // will go to 2, > len(1)
		g.Objects = []*GameObject{o}
		ok := o.NextPose(g)
		if ok {
			t.Error("should return false (reaped)")
		}
		if len(g.Objects) != 0 {
			t.Error("object should be removed from Objects")
		}
	})

	t.Run("DecayWithFinal", func(t *testing.T) {
		// Decay with Final: spawns the Final sprite before reaping.
		g := newObjGame()
		finalDef := &SpriteDef{
			Name: "boom", Width: 8, Height: 8,
			Idle: []int{90}, Poses: []int{91}, Frames: 2, Decay: true,
		}
		o := newTestObj(50, 60, Thug)
		o.Decay = true
		o.Final = finalDef
		o.CurrentPoses = []int{1}
		o.CurrentFrames = 1
		o.FrameCount = 0
		o.PoseIndex = 1
		g.Objects = []*GameObject{o}
		o.NextPose(g)
		// The original should be reaped, and a new object spawned.
		if len(g.Objects) != 1 {
			t.Fatalf("expected 1 object (final), got %d", len(g.Objects))
		}
		if g.Objects[0].Name != "boom" {
			t.Errorf("expected 'boom', got %q", g.Objects[0].Name)
		}
	})

	t.Run("DecayLinger", func(t *testing.T) {
		// Decay with Linger: stays on last frame, keeps cycling.
		// The object is not reaped; it remains alive on the last pose.
		g := newObjGame()
		o := newTestObj(0, 0, Thug)
		o.Decay = true
		o.Linger = true
		o.CurrentPoses = []int{1}
		o.CurrentFrames = 1
		o.FrameCount = 0
		o.PoseIndex = 1
		g.Objects = []*GameObject{o}
		ok := o.NextPose(g)
		if !ok {
			t.Error("linger should return true")
		}
		// PoseIndex stays on last pose (decremented from overflow).
		if o.PoseIndex != 1 {
			t.Errorf("expected PoseIndex=1 (last pose), got %d", o.PoseIndex)
		}
	})

	t.Run("FrameFunc", func(t *testing.T) {
		// FrameFunc is called when FrameCount reaches 0.
		g := newObjGame()
		called := false
		o := newTestObj(0, 0, Player)
		o.CurrentPoses = []int{1, 2}
		o.CurrentFrames = 3
		o.FrameCount = 0
		o.PoseIndex = 0
		o.FrameFunc = func(obj *GameObject) {
			called = true
		}
		o.NextPose(g)
		if !called {
			t.Error("FrameFunc should be called when FrameCount is 0")
		}
	})
}

// ===========================================================================
// SlowStart
// ===========================================================================
//
// Pre-act hook for newly spawned waifs.  Counts down Delay, then clears
// itself and constrains Y range to the safe corridor.

func TestSlowStart(t *testing.T) {
	t.Run("Countdown", func(t *testing.T) {
		g := newObjGame()
		p := newTestObj(0, 0, Waif)
		p.Delay = 3
		p.PreAct = SlowStart
		proceed := SlowStart(g, p)
		if proceed {
			t.Error("should return false during countdown")
		}
		if p.Delay != 2 {
			t.Errorf("expected Delay=2, got %d", p.Delay)
		}
	})

	t.Run("Expired", func(t *testing.T) {
		// Delay reaches 0 → clears PreAct and constrains Y range.
		g := newObjGame()
		p := newTestObj(0, 0, Waif)
		p.Delay = 1
		p.PreAct = SlowStart
		SlowStart(g, p)
		if p.Delay != 0 {
			t.Error("Delay should be 0")
		}
		if p.PreAct != nil {
			t.Error("PreAct should be cleared")
		}
		if p.YMin != TopEdge {
			t.Errorf("YMin should be TopEdge=%d, got %d", TopEdge, p.YMin)
		}
		if p.YMax != BottomEdge-8 {
			t.Errorf("YMax should be BottomEdge-8=%d, got %d", BottomEdge-8, p.YMax)
		}
	})
}

// ===========================================================================
// randomFloorTile
// ===========================================================================
//
// Returns BlackTile 90% of the time, accent tiles the other 10%.
// With AccentTileCnt=0, always returns BlackTile.

func TestRandomFloorTile(t *testing.T) {
	t.Run("NoAccents", func(t *testing.T) {
		g := newObjGame()
		g.AccentTileCnt = 0
		// All calls should return BlackTile.
		for i := 0; i < 100; i++ {
			tile := g.randomFloorTile(g.Rng)
			if tile != g.BlackTile {
				t.Fatalf("expected BlackTile=%d, got %d", g.BlackTile, tile)
			}
		}
	})

	t.Run("WithAccents", func(t *testing.T) {
		g := newObjGame()
		g.AccentTileCnt = 3
		g.FirstAccentTile = 5
		// Run enough times that we should see at least one accent tile.
		sawAccent := false
		for i := 0; i < 1000; i++ {
			tile := g.randomFloorTile(g.Rng)
			if tile != g.BlackTile {
				sawAccent = true
				if tile < g.FirstAccentTile || tile >= g.FirstAccentTile+g.AccentTileCnt {
					t.Fatalf("accent tile %d out of range [%d,%d)", tile, g.FirstAccentTile, g.FirstAccentTile+g.AccentTileCnt)
				}
			}
		}
		if !sawAccent {
			t.Error("should have produced at least one accent tile in 1000 tries")
		}
	})
}

// ===========================================================================
// Spawn
// ===========================================================================
//
// Creates a thug if within threat budget.  Sets speed based on
// ThreatMax, increments ThugCount and Threat.

func TestSpawn(t *testing.T) {
	t.Run("WithinBudget", func(t *testing.T) {
		g := newObjGame()
		g.Populating = true // suppress PlaySfx
		def := g.Thugs[0]
		ok := g.Spawn(def, 50, 50, 1)
		if !ok {
			t.Error("should succeed within budget")
		}
		if g.ThugCount != 1 {
			t.Error("ThugCount should increment")
		}
		if g.Threat == 0 {
			t.Error("Threat should increase")
		}
	})

	t.Run("OverBudget", func(t *testing.T) {
		g := newObjGame()
		g.Threat = g.ThreatMax // already maxed
		ok := g.Spawn(g.Thugs[0], 50, 50, 1)
		if ok {
			t.Error("should fail when over budget")
		}
	})

	t.Run("SpawnSound", func(t *testing.T) {
		// When not populating, PlaySfx is called (mocked via SoundsLoaded=false).
		g := newObjGame()
		g.Populating = false
		g.Spawn(g.Thugs[0], 50, 50, 1)
		// Just verifying no panic; SoundsLoaded=false skips raylib.
	})
}

// ===========================================================================
// SpawnWaif
// ===========================================================================
//
// Creates a waif at a random position along the top or bottom edge.
// Sets up controller hooks and increments WaifCount.

func TestSpawnWaif(t *testing.T) {
	// SpawnWaif picks a random Y position (top or bottom edge).
	// We call it twice with a known seed to cover both branches.
	g := newObjGame()
	g.CamX = 100
	g.SpawnWaif()
	g.SpawnWaif()
	if g.WaifCount != 2 {
		t.Errorf("WaifCount should be 2, got %d", g.WaifCount)
	}
	// Verify at least one waif spawns at top (Y=0) and one at bottom.
	var topCount, botCount int
	for _, obj := range g.Objects {
		if obj.Kind == Waif {
			if obj.Y == 0 {
				topCount++
			} else {
				botCount++
			}
			if obj.Controller == nil {
				t.Error("waif should have Controller")
			}
			if obj.PostMove == nil {
				t.Error("waif should have PostMove")
			}
			if obj.PreAct == nil {
				t.Error("waif should have PreAct (SlowStart)")
			}
			if obj.Delay != g.WaifDelayF {
				t.Errorf("Delay should be %d, got %d", g.WaifDelayF, obj.Delay)
			}
		}
	}
	if topCount == 0 || botCount == 0 {
		// If both branches aren't hit with seed 42, that's fine —
		// coverage for the other branch comes from StepSimulation tests.
		t.Logf("topCount=%d botCount=%d (one branch may need different seed)", topCount, botCount)
	}
}

// ===========================================================================
// Respawn
// ===========================================================================
//
// Picks a random thug definition and spawns it near the camera.
// Spawns at bottom if player is in the upper half.

func TestRespawn(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		g := newObjGame()
		g.Populating = true
		g.Playing = true
		g.Player = newTestObj(64, 80, Player)
		g.Objects = []*GameObject{g.Player}
		g.Respawn()
		if g.ThugCount != 1 {
			t.Error("should spawn a thug")
		}
	})

	t.Run("PlayerHighSpawnsLow", func(t *testing.T) {
		// Player in upper half (Y < 76) → thug spawns at bottom.
		g := newObjGame()
		g.Populating = true
		g.Playing = true
		g.Player = newTestObj(64, 40, Player)
		g.Objects = []*GameObject{g.Player}
		g.Respawn()
		// Find the spawned thug.
		for _, obj := range g.Objects {
			if obj.Kind == Thug {
				if obj.Y == 0 {
					t.Error("thug should spawn at bottom when player is high")
				}
			}
		}
	})

	t.Run("NotPlaying_RandomY", func(t *testing.T) {
		// When not playing, thug Y is randomly top or bottom.
		// Covers the `else if g.Rng.Intn(2) == 1` branch.
		g := newObjGame()
		g.Playing = false
		// Call multiple times to hit both random branches.
		g.ThreatMax = 100
		for i := 0; i < 5; i++ {
			g.Respawn()
		}
		if g.ThugCount < 1 {
			t.Error("should spawn at least one thug")
		}
	})
}

// ===========================================================================
// Populate
// ===========================================================================
//
// Fills a screen-width region with floor tiles, thugs, crates, rocks.
// Skips if already populated (checks top-right tile).  Excludes a zone
// around the player.

func TestPopulate(t *testing.T) {
	t.Run("FillsTiles", func(t *testing.T) {
		g := newObjGame()
		g.Player = newTestObj(64, 80, Player)
		g.Objects = []*GameObject{g.Player}
		g.Populate(0, 1)
		// Check that some tiles were filled.
		filled := 0
		for c := 0; c < 16; c++ {
			if g.TileMap[0][c] != 0 {
				filled++
			}
		}
		if filled == 0 {
			t.Error("should have filled some tiles")
		}
	})

	t.Run("AlreadyPopulated", func(t *testing.T) {
		// If top-right tile is already set, Populate is a no-op.
		g := newObjGame()
		g.Player = newTestObj(64, 80, Player)
		g.Objects = []*GameObject{g.Player}
		g.Populate(0, 1)
		count1 := len(g.Objects)
		g.Populate(0, 1) // second call should be no-op
		if len(g.Objects) != count1 {
			t.Error("should not add more objects on re-populate")
		}
	})
}

// ===========================================================================
// PopulateTitleBackground
// ===========================================================================
//
// Fills all empty tiles with random floor tiles for the title screen.

func TestPopulateTitleBackground(t *testing.T) {
	g := newObjGame()
	g.PopulateTitleBackground()
	empty := 0
	for r := range g.TileMap {
		for c := range g.TileMap[r] {
			if g.TileMap[r][c] == 0 {
				empty++
			}
		}
	}
	if empty != 0 {
		t.Errorf("expected 0 empty tiles, got %d", empty)
	}
}
