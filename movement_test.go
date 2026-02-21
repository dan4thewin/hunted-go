package main

import (
	"math"
	"math/rand"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
)

// ═══════════════════════════════════════════════════════════════════════════
// Test Helpers
// ═══════════════════════════════════════════════════════════════════════════
//
// These helpers construct minimal Game and GameObject instances that satisfy
// the dependencies of movement.go without requiring raylib initialisation.
// The key trick is leaving SoundsLoaded=false so that PlaySfx and
// PlayExplosion (called transitively by MobCollide → Crunch → Explode)
// skip all raylib audio calls.

// newTestGame builds a Game with a seeded RNG and standard wall tile config.
// No tile map is set — callers attach one via g.TileMap = makeTileMap(...).
func newTestGame() *Game {
	return &Game{
		Rng: rand.New(rand.NewSource(42)),
		TileConfig: TileConfig{
			FirstWallTile: 10,
			WallTileCnt:   3, // wall tiles are 10, 11, 12
			RockTile:      20,
			BlackTile:     1,
		},
	}
}

// newTileMap creates a rows×cols grid filled with `fill`.
func newTileMap(rows, cols, fill int) [][]int {
	m := make([][]int, rows)
	for r := range m {
		m[r] = make([]int, cols)
		for c := range m[r] {
			m[r][c] = fill
		}
	}
	return m
}

// newObj creates a simple 8×8 object at (x,y) with speed=1 and default
// movement bounds matching the game world (Y: 0–159, X: 0–1024).
func newObj(x, y float64) *GameObject {
	return &GameObject{
		X: x, Y: y,
		Width: 8, Height: 8,
		Speed:     1,
		YMax:      YMax,
		XMaxLimit: XMax + 1,
	}
}

// testDeadDef is a minimal SpriteDef used as the "Final" (death sprite) for
// objects that need Crunch to succeed.  It must have at least one Idle frame
// so Emplace → SetIdle → ResetPose doesn't index an empty slice.
var testDeadDef = &SpriteDef{Name: "dead", Width: 8, Height: 8, Idle: []int{0}}

// approxEqual compares two float64 values within a tolerance.
func approxEqual(a, b, eps float64) bool {
	return math.Abs(a-b) < eps
}

// ═══════════════════════════════════════════════════════════════════════════
// IsWall
// ═══════════════════════════════════════════════════════════════════════════
//
// IsWall looks up a tile coordinate (col, row) in the tile map and returns
// true if the tile is a wall or a rock.  Out-of-bounds coordinates return
// false — objects flying off the map edge are not blocked by phantom walls.
// The function has four early-return guards (negative row, negative col,
// row past end, col past end) before the actual tile comparison.

func TestIsWall(t *testing.T) {
	g := newTestGame()
	// 3-row map: row 0 = open, row 1 = wall tiles 10/11/12 and an open tile,
	// row 2 = rock tile (20) then open.
	g.TileMap = [][]int{
		{0, 0, 0, 0},
		{10, 11, 12, 0},
		{20, 0, 0, 0},
	}

	cases := []struct {
		name     string
		col, row int
		want     bool
	}{
		// Guard clauses: each out-of-bounds condition returns false.
		{"NegativeRow", 0, -1, false},
		{"NegativeCol", -1, 0, false},
		{"RowPastEnd", 0, 3, false},
		{"ColPastEnd", 4, 0, false},

		// Open floor tile (value 0) — not a wall.
		{"OpenFloor", 0, 0, false},

		// Wall tile range: FirstWallTile (10) through FirstWallTile+WallTileCnt-1 (12).
		{"WallTile0", 0, 1, true},
		{"WallTile1", 1, 1, true},
		{"WallTile2", 2, 1, true},

		// Same row but outside the wall tile range → open.
		{"OpenNextToWall", 3, 1, false},

		// Rock tile (20) — also blocks movement like a wall.
		{"RockTile", 0, 2, true},

		// Open tile next to rock.
		{"OpenNextToRock", 1, 2, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := g.IsWall(tc.col, tc.row); got != tc.want {
				t.Errorf("IsWall(%d,%d) = %v, want %v", tc.col, tc.row, got, tc.want)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Bounds
// ═══════════════════════════════════════════════════════════════════════════
//
// Bounds computes the axis-aligned bounding box (AABB) of a game object,
// incorporating pixel offsets (XOff, YOff) that shift the collision area
// relative to the sprite's draw origin.  Callers can optionally override
// the X and/or Y position via pointers — this lets Obstacle and Hazard
// test hypothetical positions without mutating the object.

func TestBounds(t *testing.T) {
	t.Run("NilOverrides_UsesObjectPosition", func(t *testing.T) {
		// With both overrides nil, Bounds uses the object's own X,Y.
		// An 8×8 object at (10,20) produces a box from (10,20) to (17,27).
		o := &GameObject{X: 10, Y: 20, Width: 8, Height: 8}
		x, y, x2, y2 := o.Bounds(nil, nil)
		if x != 10 || y != 20 || x2 != 17 || y2 != 27 {
			t.Errorf("got (%.0f,%.0f,%.0f,%.0f), want (10,20,17,27)", x, y, x2, y2)
		}
	})

	t.Run("XOverrideOnly", func(t *testing.T) {
		// Passing an X override repositions the box horizontally while
		// keeping the object's stored Y.  Used when probing whether a
		// purely horizontal move is safe.
		o := &GameObject{X: 10, Y: 20, Width: 8, Height: 8}
		nx := 50.0
		x, y, x2, y2 := o.Bounds(&nx, nil)
		if x != 50 || y != 20 || x2 != 57 || y2 != 27 {
			t.Errorf("got (%.0f,%.0f,%.0f,%.0f), want (50,20,57,27)", x, y, x2, y2)
		}
	})

	t.Run("YOverrideOnly", func(t *testing.T) {
		// Y override repositions vertically, keeping the object's X.
		o := &GameObject{X: 10, Y: 20, Width: 8, Height: 8}
		ny := 40.0
		x, y, x2, y2 := o.Bounds(nil, &ny)
		if x != 10 || y != 40 || x2 != 17 || y2 != 47 {
			t.Errorf("got (%.0f,%.0f,%.0f,%.0f), want (10,40,17,47)", x, y, x2, y2)
		}
	})

	t.Run("BothOverrides", func(t *testing.T) {
		// Both overrides: completely ignores the object's stored position.
		// This is the most common call site — Obstacle always passes both.
		o := &GameObject{X: 10, Y: 20, Width: 8, Height: 8}
		nx, ny := 100.0, 200.0
		x, y, x2, y2 := o.Bounds(&nx, &ny)
		if x != 100 || y != 200 || x2 != 107 || y2 != 207 {
			t.Errorf("got (%.0f,%.0f,%.0f,%.0f), want (100,200,107,207)", x, y, x2, y2)
		}
	})

	t.Run("WithPixelOffsets", func(t *testing.T) {
		// XOff and YOff shift the collision box relative to the sprite
		// origin.  Common for sprites whose hitbox is smaller or offset
		// from their visual area (e.g. a 16×16 sprite with an 8×8 hitbox).
		o := &GameObject{X: 10, Y: 20, Width: 4, Height: 6, XOff: 2, YOff: 3}
		x, y, x2, y2 := o.Bounds(nil, nil)
		if x != 12 || y != 23 || x2 != 15 || y2 != 28 {
			t.Errorf("got (%.0f,%.0f,%.0f,%.0f), want (12,23,15,28)", x, y, x2, y2)
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// Obstacle
// ═══════════════════════════════════════════════════════════════════════════
//
// Obstacle checks whether an object's bounding box, optionally shifted by
// (dx, dy) and/or repositioned via (nx, ny), overlaps any wall tiles.
// It converts pixel coords to tile coords and scans the rectangular
// tile region.  This is the core "can I move here?" test used by Walk,
// slideAxis, and bounceDir.

func TestObstacle(t *testing.T) {
	g := newTestGame()
	// 4×4 tile map with a single wall at (col=2, row=1).
	g.TileMap = newTileMap(4, 4, 0)
	g.TileMap[1][2] = 10

	t.Run("NoWall", func(t *testing.T) {
		// Object bounds fall entirely in open tiles — no obstacle.
		o := newObj(0, 0) // bounds (0,0)-(7,7), tile (0,0) — open
		if g.Obstacle(o, 0, 0, nil, nil) {
			t.Error("expected no obstacle in open area")
		}
	})

	t.Run("WallHit", func(t *testing.T) {
		// Object at (9,8): bounds (9,8)-(16,15) span tiles (1,1)-(2,1).
		// Tile (2,1) is a wall → obstacle detected.
		o := newObj(9, 8)
		if !g.Obstacle(o, 0, 0, nil, nil) {
			t.Error("expected obstacle when bounds overlap wall tile")
		}
	})

	t.Run("DeltaShiftsIntoWall", func(t *testing.T) {
		// Object at (8,8): bounds (8,8)-(15,15), tile (1,1) — clear.
		// dx=1 shifts right edge to 16, entering tile (2,1) — wall.
		o := newObj(8, 8)
		if g.Obstacle(o, 0, 0, nil, nil) {
			t.Error("should be clear without delta")
		}
		if !g.Obstacle(o, 1, 0, nil, nil) {
			t.Error("dx=1 should shift into wall")
		}
	})

	t.Run("PositionOverride", func(t *testing.T) {
		// Using nx/ny to probe a different position entirely.
		o := newObj(0, 0)
		nx, ny := 9.0, 8.0
		if !g.Obstacle(o, 0, 0, &nx, &ny) {
			t.Error("position override should place object in wall")
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// Overlap
// ═══════════════════════════════════════════════════════════════════════════
//
// Overlap is a pure AABB intersection test.  Two rectangles overlap unless
// one is entirely to the right, left, below, or above the other.  Touching
// edges (ax == bx2) count as overlapping — this matches the game's pixel-
// grid collision model where sharing an edge pixel means contact.

func TestOverlap(t *testing.T) {
	cases := []struct {
		name                               string
		ax, ay, ax2, ay2, bx, by, bx2, by2 float64
		want                               bool
	}{
		// Two identical boxes — obviously overlapping.
		{"Identical", 0, 0, 10, 10, 0, 0, 10, 10, true},

		// Separated on each axis in turn — exercises each of the four
		// "no overlap" sub-expressions (ax > bx2, ax2 < bx, ay > by2, ay2 < by).
		{"SeparatedRight", 0, 0, 5, 5, 10, 0, 15, 5, false},
		{"SeparatedLeft", 10, 0, 15, 5, 0, 0, 5, 5, false},
		{"SeparatedBelow", 0, 0, 5, 5, 0, 10, 5, 15, false},
		{"SeparatedAbove", 0, 10, 5, 15, 0, 0, 5, 5, false},

		// Edge-touching — overlapping in pixel coordinates.
		{"TouchRight", 0, 0, 10, 10, 10, 0, 20, 10, true},
		{"TouchBottom", 0, 0, 10, 10, 0, 10, 10, 20, true},

		// Partial overlap and full containment.
		{"PartialOverlap", 0, 0, 10, 10, 5, 5, 15, 15, true},
		{"Containment", 0, 0, 20, 20, 5, 5, 10, 10, true},

		// Degenerate: single-pixel boxes.
		{"PointSame", 5, 5, 5, 5, 5, 5, 5, 5, true},
		{"PointDiff", 5, 5, 5, 5, 6, 6, 6, 6, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Overlap(tc.ax, tc.ay, tc.ax2, tc.ay2, tc.bx, tc.by, tc.bx2, tc.by2)
			if got != tc.want {
				t.Errorf("Overlap = %v, want %v", got, tc.want)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Hazard
// ═══════════════════════════════════════════════════════════════════════════
//
// Hazard scans g.Objects for the first one whose bounding box overlaps the
// querying object at a hypothetical position (x, y).  A filter selects
// eligible candidates (e.g. NoGhost skips intangible objects).  The object
// always excludes itself.
//
// In the game, MobCollide calls Hazard to find what an object would bump
// into if it moved to a candidate position.

func TestHazard(t *testing.T) {
	g := newTestGame()

	t.Run("EmptyWorld", func(t *testing.T) {
		// No objects at all — nothing to collide with.
		g.Objects = nil
		p := newObj(10, 10)
		if got := g.Hazard(p, 10, 10, NoGhost); got != nil {
			t.Error("expected nil hazard in empty world")
		}
	})

	t.Run("SkipSelf", func(t *testing.T) {
		// The querying object is in g.Objects but must not match itself.
		p := newObj(10, 10)
		g.Objects = []*GameObject{p}
		if got := g.Hazard(p, 10, 10, NoGhost); got != nil {
			t.Error("should skip self")
		}
	})

	t.Run("FilterExcludes", func(t *testing.T) {
		// q overlaps but is Ignored (ghosted) — NoGhost rejects it.
		// This models an unarmed bomb that hasn't activated yet.
		p := newObj(10, 10)
		q := newObj(10, 10)
		q.Ignore = true
		g.Objects = []*GameObject{p, q}
		if got := g.Hazard(p, 10, 10, NoGhost); got != nil {
			t.Error("filter should exclude ignored object")
		}
	})

	t.Run("OverlapFound", func(t *testing.T) {
		// q is at the same position and passes the filter → returned.
		p := newObj(10, 10)
		q := newObj(10, 10)
		g.Objects = []*GameObject{p, q}
		if got := g.Hazard(p, 10, 10, NoGhost); got != q {
			t.Error("should return overlapping object")
		}
	})

	t.Run("NoOverlap", func(t *testing.T) {
		// q is far away — bounding boxes don't intersect.
		p := newObj(10, 10)
		q := newObj(100, 100)
		g.Objects = []*GameObject{p, q}
		if got := g.Hazard(p, 10, 10, NoGhost); got != nil {
			t.Error("should return nil when no overlap")
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// Distance
// ═══════════════════════════════════════════════════════════════════════════
//
// Distance computes straight-line Euclidean distance between two objects'
// draw positions (X,Y — not bounding box centres).  Used by the AI to
// decide target priority and engagement range.

func TestDistance(t *testing.T) {
	t.Run("Same", func(t *testing.T) {
		if d := Distance(newObj(10, 10), newObj(10, 10)); d != 0 {
			t.Errorf("same position: got %f, want 0", d)
		}
	})
	t.Run("Horizontal", func(t *testing.T) {
		if d := Distance(newObj(0, 0), newObj(10, 0)); d != 10 {
			t.Errorf("got %f, want 10", d)
		}
	})
	t.Run("Vertical", func(t *testing.T) {
		if d := Distance(newObj(0, 0), newObj(0, 7)); d != 7 {
			t.Errorf("got %f, want 7", d)
		}
	})
	t.Run("Diagonal_345", func(t *testing.T) {
		// 3-4-5 right triangle.
		if d := Distance(newObj(0, 0), newObj(3, 4)); !approxEqual(d, 5, 1e-9) {
			t.Errorf("got %f, want 5", d)
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// NoGhost and AvoidFilter
// ═══════════════════════════════════════════════════════════════════════════
//
// NoGhost returns true for objects that are NOT ghosted (Ignore=false).
// Used as a filter for MobCollide — ghosted objects (Ignore=true) are
// intangible.  Bombs and mines start Ignored until they arm themselves.
//
// AvoidFilter returns true for objects the AI pathfinder should steer
// around (e.g. active bombs/mines).

func TestNoGhost(t *testing.T) {
	if !NoGhost(&GameObject{Ignore: false}) {
		t.Error("non-ignored should return true")
	}
	if NoGhost(&GameObject{Ignore: true}) {
		t.Error("ignored should return false")
	}
}

func TestAvoidFilter(t *testing.T) {
	if !AvoidFilter(&GameObject{Avoid: true}) {
		t.Error("avoidable should return true")
	}
	if AvoidFilter(&GameObject{Avoid: false}) {
		t.Error("non-avoidable should return false")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Hazard2
// ═══════════════════════════════════════════════════════════════════════════
//
// Hazard2 probes for objects exactly 1 pixel away in each cardinal
// direction.  The AI uses this to detect obstacles that are immediately
// adjacent without actually overlapping, answering "which directions
// are blocked?"  It returns up to four pointers — one per direction.
//
// The 1-pixel probes work because game objects move in whole or fractional
// pixels, so being 1 pixel away means "the next step would cause contact."

func TestHazard2(t *testing.T) {
	g := newTestGame()

	// Central object at (10,10), 8×8.  bounds = (10,10)-(17,17).
	// Neighbours are positioned exactly 1 pixel away in each direction:
	//   up:    y2=9  aligns with py-1=9   → overlap on expanded up probe
	//   right: x=18  aligns with px2+1=18 → overlap on expanded right probe
	//   down:  y=18  aligns with py2+1=18 → overlap on expanded down probe
	//   left:  x2=9  aligns with px-1=9   → overlap on expanded left probe
	p := newObj(10, 10)
	up := newObj(10, 2)
	right := newObj(18, 10)
	down := newObj(10, 18)
	left := newObj(2, 10)

	g.Objects = []*GameObject{p, up, right, down, left}
	all := func(*GameObject) bool { return true }

	gotU, gotR, gotD, gotL := g.Hazard2(p, all)
	if gotU != up {
		t.Error("expected up hazard")
	}
	if gotR != right {
		t.Error("expected right hazard")
	}
	if gotD != down {
		t.Error("expected down hazard")
	}
	if gotL != left {
		t.Error("expected left hazard")
	}

	t.Run("FilterExcludes", func(t *testing.T) {
		// A filter that rejects everything → all four nil.
		none := func(*GameObject) bool { return false }
		u, r, d, l := g.Hazard2(p, none)
		if u != nil || r != nil || d != nil || l != nil {
			t.Error("filter should exclude all")
		}
	})

	t.Run("NoNeighbours", func(t *testing.T) {
		g.Objects = []*GameObject{p}
		u, r, d, l := g.Hazard2(p, all)
		if u != nil || r != nil || d != nil || l != nil {
			t.Error("no neighbours should return all nil")
		}
	})

	t.Run("FarObjectNoProbe", func(t *testing.T) {
		// An object that passes the filter but is too far away to match
		// any directional probe.  This exercises the false branch of
		// the left-direction Overlap check (the last else-if), which
		// gobco flagged as "once true but never false."
		far := newObj(100, 100)
		g.Objects = []*GameObject{p, far}
		u, r, d, l := g.Hazard2(p, all)
		if u != nil || r != nil || d != nil || l != nil {
			t.Error("far object should not match any probe")
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// MobCollide
// ═══════════════════════════════════════════════════════════════════════════
//
// MobCollide resolves the game-logic consequences of two objects occupying
// the same space.  It dispatches based on Kind:
//
//   - Ignored object → no collision (nil early return)
//   - No hazard → nil
//   - Friendly opens a Crate → destroy crate, award random items
//   - Two friendlies → Player passes through waif (nil);
//                       waif is blocked by player (returns blocker)
//   - Hostile encounter → both take damage (Crunch); explosion if either
//                         is destroyed

func TestMobCollide(t *testing.T) {
	// ── Why mock? ──────────────────────────────────────────────────────
	// MobCollide delegates to four methods defined outside movement.go:
	//   OpenCrate (game.go)   — awards items, calls Crunch internally
	//   Crunch    (game.go)   — kills an object, returns success bool
	//   Explode   (game.go)   — creates visual sparks + screen shake
	//   Log       (config.go) — debug logging
	// Without mocking, each test must construct the full state those
	// methods expect (testDeadDef, seeded Rng, HeartCount, Threat,
	// Playing flag, etc.) — none of which is relevant to MobCollide's
	// own branching logic.  Mocking lets us test MobCollide's decision
	// tree in isolation: which branch fires, which delegate is called,
	// and what MobCollide returns.
	//
	// gomonkey patches methods at the type level via runtime code
	// rewriting.  Tests MUST be run with -gcflags=all=-l (disable
	// inlining) for patches to take effect.

	t.Run("IgnoredObject", func(t *testing.T) {
		// Ghosted objects (e.g. unarmed bombs) short-circuit before
		// Hazard is even called — no mocks needed.
		g := &Game{}
		p := &GameObject{Ignore: true, Width: 8, Height: 8}
		if g.MobCollide(p, 0, 0) != nil {
			t.Error("ignored object should not collide")
		}
	})

	t.Run("NoHazard", func(t *testing.T) {
		// Empty Objects list → Hazard returns nil → early return.
		// No external calls made, so no mocks needed.
		g := &Game{}
		p := &GameObject{Width: 8, Height: 8}
		if g.MobCollide(p, 0, 0) != nil {
			t.Error("expected nil with no objects")
		}
	})

	t.Run("FriendlyOpensCrate", func(t *testing.T) {
		// Player walks onto a crate → OpenCrate is called with the
		// crate, then MobCollide returns nil (walk succeeds).
		// Before mocking: needed testDeadDef, seeded Rng, then
		// verified BombCount/MineCount deltas and crate reaping.
		// With mock: just verify OpenCrate is called with the crate.
		g := &Game{}
		player := &GameObject{Kind: Player, X: 10, Y: 10, Width: 8, Height: 8}
		crate := &GameObject{Kind: Crate, X: 10, Y: 10, Width: 8, Height: 8}
		g.Objects = []*GameObject{player, crate}

		var opened *GameObject
		patches := gomonkey.ApplyMethodFunc(g, "OpenCrate", func(q *GameObject) {
			opened = q
		})
		defer patches.Reset()

		if g.MobCollide(player, 10, 10) != nil {
			t.Error("crate collision should return nil")
		}
		if opened != crate {
			t.Error("OpenCrate should have been called with the crate")
		}
	})

	t.Run("PlayerIgnoresWaif", func(t *testing.T) {
		// Friendly-friendly with Player → returns nil (walk proceeds).
		// Pure branching — no external calls, no mocks needed.
		g := &Game{}
		player := &GameObject{Kind: Player, X: 10, Y: 10, Width: 8, Height: 8}
		waif := &GameObject{Kind: Waif, X: 10, Y: 10, Width: 8, Height: 8}
		g.Objects = []*GameObject{player, waif}
		if g.MobCollide(player, 10, 10) != nil {
			t.Error("player should pass through waif")
		}
	})

	t.Run("WaifBlockedByPlayer", func(t *testing.T) {
		// Friendly-friendly with non-Player → returns the blocker.
		// Pure branching — no external calls, no mocks needed.
		g := &Game{}
		waif := &GameObject{Kind: Waif, X: 10, Y: 10, Width: 8, Height: 8}
		player := &GameObject{Kind: Player, X: 10, Y: 10, Width: 8, Height: 8}
		g.Objects = []*GameObject{waif, player}
		if g.MobCollide(waif, 10, 10) != player {
			t.Error("waif should be blocked by player")
		}
	})

	t.Run("HostileWithExplosion", func(t *testing.T) {
		// Player collides with thug → hostile branch.  Both Crunch
		// calls return true → Explode fires.
		// Before mocking: needed testDeadDef on both objects, HeartCount=1,
		// Playing=true, Threat=1, then checked g.Sparks and g.ShakeFactor.
		// With mock: just verify Crunch is called twice and Explode once.
		g := &Game{}
		player := &GameObject{Kind: Player, X: 50, Y: 50, Width: 8, Height: 8, Name: "hero"}
		thug := &GameObject{Kind: Thug, X: 50, Y: 50, Width: 8, Height: 8, Name: "thug"}
		g.Objects = []*GameObject{player, thug}

		crunchCount := 0
		var exploded bool
		patches := gomonkey.ApplyMethodFunc(g, "Crunch", func(_ *GameObject) bool {
			crunchCount++
			return true
		})
		patches.ApplyMethodFunc(g, "Explode", func(_, _ float64) {
			exploded = true
		})
		defer patches.Reset()

		if g.MobCollide(player, 50, 50) != nil {
			t.Error("hostile collision should return nil")
		}
		if crunchCount != 2 {
			t.Errorf("Crunch called %d times, want 2", crunchCount)
		}
		if !exploded {
			t.Error("Explode should fire when both Crunch return true")
		}
	})

	t.Run("HostileNoExplosion", func(t *testing.T) {
		// Both Crunch return false → no explosion.  a||b is false.
		// Before mocking: both objects had Final=nil so Crunch naturally
		// returned false, then checked g.Sparks empty.
		// With mock: explicitly return false, verify Explode NOT called.
		g := &Game{}
		thug1 := &GameObject{Kind: Thug, X: 50, Y: 50, Width: 8, Height: 8, Name: "t1"}
		thug2 := &GameObject{Kind: Thug, X: 50, Y: 50, Width: 8, Height: 8, Name: "t2"}
		g.Objects = []*GameObject{thug1, thug2}

		patches := gomonkey.ApplyMethodFunc(g, "Crunch", func(_ *GameObject) bool {
			return false
		})
		patches.ApplyMethodFunc(g, "Explode", func(_, _ float64) {
			t.Error("Explode should not be called when both Crunch return false")
		})
		defer patches.Reset()

		g.MobCollide(thug1, 50, 50)
	})

	t.Run("ThugHitsCrate", func(t *testing.T) {
		// Thug (not Friendly) collides with Crate → the crate-open
		// guard (p.Kind&Friendly != 0) is false, so it falls through
		// to the hostile branch.  Covers the false branch of the
		// Friendly check in the crate condition.
		g := &Game{}
		thug := &GameObject{Kind: Thug, X: 10, Y: 10, Width: 8, Height: 8, Name: "thug"}
		crate := &GameObject{Kind: Crate, X: 10, Y: 10, Width: 8, Height: 8, Name: "crate"}
		g.Objects = []*GameObject{thug, crate}

		patches := gomonkey.ApplyMethodFunc(g, "Crunch", func(_ *GameObject) bool {
			return false
		})
		defer patches.Reset()

		if g.MobCollide(thug, 10, 10) != nil {
			t.Error("hostile collision should return nil")
		}
	})

	t.Run("HostileOnlyMoverDies", func(t *testing.T) {
		// Crunch(q)=false, Crunch(p)=true → a=false, b=true → Explode fires.
		// Covers the true branch of `b` in `a || b`.
		// Before mocking: target had Final=nil (Crunch→false), mover had
		// testDeadDef+Threat (Crunch→true).
		// With mock: explicit sequential return values via counter.
		g := &Game{}
		mover := &GameObject{Kind: Thug, X: 50, Y: 50, Width: 8, Height: 8, Name: "m"}
		target := &GameObject{Kind: Thug, X: 50, Y: 50, Width: 8, Height: 8, Name: "t"}
		g.Objects = []*GameObject{mover, target}

		call := 0
		var exploded bool
		patches := gomonkey.ApplyMethodFunc(g, "Crunch", func(_ *GameObject) bool {
			call++
			return call == 2 // first call (q): false, second call (p): true
		})
		patches.ApplyMethodFunc(g, "Explode", func(_, _ float64) {
			exploded = true
		})
		defer patches.Reset()

		g.MobCollide(mover, 50, 50)
		if !exploded {
			t.Error("Explode should fire when only mover dies (b=true)")
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// dirDelta
// ═══════════════════════════════════════════════════════════════════════════
//
// dirDelta converts a direction bitmask into normalised (dx, dy) deltas.
// Cardinal directions produce unit deltas; diagonals use ≈0.7 (roughly
// 1/√2) so diagonal speed matches cardinal speed in magnitude.
// Invalid masks (0, or opposing bits that were never cancelled) return
// ok=false, telling the walk loop there is no movement to perform.

func TestDirDelta(t *testing.T) {
	cases := []struct {
		dir          int
		wantX, wantY float64
		wantOK       bool
	}{
		{DirLeft, -1, 0, true},
		{DirRight, 1, 0, true},
		{DirUp, 0, -1, true},
		{DirDown, 0, 1, true},
		{DirLeft | DirUp, -0.7, -0.7, true},
		{DirRight | DirUp, 0.7, -0.7, true},
		{DirLeft | DirDown, -0.7, 0.7, true},
		{DirRight | DirDown, 0.7, 0.7, true},
		// No bits set → nothing to do.
		{0, 0, 0, false},
		// Opposing bits (Left|Right) — not a valid game input.
		{DirLeft | DirRight, 0, 0, false},
	}
	for _, tc := range cases {
		x, y, ok := dirDelta(tc.dir)
		if x != tc.wantX || y != tc.wantY || ok != tc.wantOK {
			t.Errorf("dirDelta(%d) = (%.1f,%.1f,%v), want (%.1f,%.1f,%v)",
				tc.dir, x, y, ok, tc.wantX, tc.wantY, tc.wantOK)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// snapDiag
// ═══════════════════════════════════════════════════════════════════════════
//
// When an object transitions from cardinal to diagonal movement, snapDiag
// rounds its position to the nearest integer pixel.  This prevents
// sub-pixel drift from accumulating across direction changes.
//
// Snapping direction depends on movement:
//   - Moving right (x>0) or down (y>0) → Floor (snap toward origin)
//   - Moving left  (x<0) or up  (y<0) → Ceil  (snap away from origin)
//
// If the object is ALREADY diagonal (Diag=true), the snap is skipped.
// If the movement is cardinal (x==0 or y==0), Diag is cleared.

func TestSnapDiag(t *testing.T) {
	t.Run("CardinalClearsDiag", func(t *testing.T) {
		// Moving straight down (x=0): Diag flag is cleared because
		// this is now cardinal movement.  Position is NOT snapped.
		p := &GameObject{X: 5.5, Y: 3.3, Diag: true}
		snapDiag(p, 0, 1)
		if p.Diag {
			t.Error("cardinal should clear Diag")
		}
		if p.X != 5.5 || p.Y != 3.3 {
			t.Error("cardinal should not snap position")
		}
	})

	t.Run("AlreadyDiagonalNoop", func(t *testing.T) {
		// Already moving diagonally — don't re-snap mid-movement.
		p := &GameObject{X: 5.5, Y: 3.3, Diag: true}
		snapDiag(p, 0.7, 0.7)
		if p.X != 5.5 || p.Y != 3.3 {
			t.Error("already-diagonal should not re-snap")
		}
	})

	t.Run("NewDiagonalPositive", func(t *testing.T) {
		// Right+Down (both positive): Floor both axes.
		p := &GameObject{X: 5.7, Y: 3.9, Diag: false}
		snapDiag(p, 0.7, 0.7)
		if !p.Diag {
			t.Error("should set Diag")
		}
		if p.X != 5 || p.Y != 3 {
			t.Errorf("positive diag should Floor: got (%.1f,%.1f), want (5,3)", p.X, p.Y)
		}
	})

	t.Run("NewDiagonalNegative", func(t *testing.T) {
		// Left+Up (both negative): Ceil both axes.
		p := &GameObject{X: 5.3, Y: 3.1, Diag: false}
		snapDiag(p, -0.7, -0.7)
		if !p.Diag {
			t.Error("should set Diag")
		}
		if p.X != 6 || p.Y != 4 {
			t.Errorf("negative diag should Ceil: got (%.1f,%.1f), want (6,4)", p.X, p.Y)
		}
	})

	t.Run("MixedDiagonal", func(t *testing.T) {
		// Right+Up: x positive → Floor, y negative → Ceil.
		p := &GameObject{X: 5.7, Y: 3.1, Diag: false}
		snapDiag(p, 0.7, -0.7)
		if p.X != 5 || p.Y != 4 {
			t.Errorf("mixed diag: got (%.1f,%.1f), want (5,4)", p.X, p.Y)
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// bounceDir
// ═══════════════════════════════════════════════════════════════════════════
//
// bounceDir computes the new direction after a bouncing object (thrown bomb)
// hits a wall.
//
// Cardinal (non-diagonal): simply reverse the direction.
//
// Diagonal: examine which wall surface was hit:
//   Case 1 – wall at (lx, ny): Y direction caused the hit; bounce off
//            horizontal surface → keep LR, flip UD.
//   Case 2 – wall at (nx, ly): X direction caused the hit; bounce off
//            vertical surface → flip LR, keep UD.
//   Case 3 – neither alone is blocked (corner): keep direction.  The
//            caller's next attempt (attempt>0) will fail, stopping the
//            object.

func TestBounceDir(t *testing.T) {
	t.Run("CardinalFlip", func(t *testing.T) {
		// Non-diagonal: each cardinal direction simply reverses.
		g := newTestGame()
		g.TileMap = newTileMap(4, 4, 0)
		p := newObj(0, 0)
		p.Diag = false

		for _, tc := range []struct{ dir, want int }{
			{DirRight, DirLeft},
			{DirLeft, DirRight},
			{DirUp, DirDown},
			{DirDown, DirUp},
		} {
			got := g.bounceDir(p, tc.dir, 0, 0, 0, 0)
			if got != tc.want {
				t.Errorf("bounce dir=%d: got %d, want %d", tc.dir, got, tc.want)
			}
		}
	})

	t.Run("DiagonalHorizontalSurface", func(t *testing.T) {
		// Bomb moving right+down hits a horizontal wall (floor).
		// The wall blocks the Y component → bounce off the horizontal
		// surface: keep rightward, flip down→up.
		//
		// Tile map: wall spans all of row 1.
		// Object: 2×2 at (5, 6.5), Diag=true.
		// (lx,ny) = (5, 7.2): bounds (5,7.2)-(6,8.2), row 1 wall → HIT
		// → Case 1: DirRight | DirUp.
		g := newTestGame()
		g.TileMap = newTileMap(3, 4, 0)
		for c := 0; c < 4; c++ {
			g.TileMap[1][c] = 10
		}
		p := &GameObject{
			X: 5, Y: 6.5, Width: 2, Height: 2,
			Speed: 1, Diag: true, YMax: YMax, XMaxLimit: XMax + 1,
		}
		got := g.bounceDir(p, DirRight|DirDown, 5, 6.5, 5.7, 7.2)
		if want := DirRight | DirUp; got != want {
			t.Errorf("horizontal surface: got %d, want %d", got, want)
		}
	})

	t.Run("DiagonalVerticalSurface", func(t *testing.T) {
		// Bomb moving right+down hits a vertical wall (side).
		// (lx,ny) is clear but (nx,ly) is blocked → bounce off
		// vertical surface: flip LR, keep UD.
		//
		// Wall at (col=2, row=1) only.
		// Object: 2×2 at (14.9, 7), Diag=true.
		// (lx,ny) = (14.9, 7.7): tiles (1,0)-(1,1) — clear
		// (nx,ly) = (15.6, 7):   tiles (1,0)-(2,1), wall at (2,1) → HIT
		// → Case 2: DirLeft | DirDown.
		g := newTestGame()
		g.TileMap = newTileMap(3, 4, 0)
		g.TileMap[1][2] = 10
		p := &GameObject{
			X: 14.9, Y: 7, Width: 2, Height: 2,
			Speed: 1, Diag: true, YMax: YMax, XMaxLimit: XMax + 1,
		}
		got := g.bounceDir(p, DirRight|DirDown, 14.9, 7, 15.6, 7.7)
		if want := DirLeft | DirDown; got != want {
			t.Errorf("vertical surface: got %d, want %d", got, want)
		}
	})

	t.Run("DiagonalCorner", func(t *testing.T) {
		// True corner hit: the wall is only at the diagonal target tile.
		// Neither axis alone hits it.  bounceDir returns the original
		// direction; the next attempt (attempt>0) will fail.
		//
		// Wall at (col=2, row=1).
		// Object: 2×2 at (14.9, 6.5), Diag=true.
		// (lx,ny) = (14.9, 7.2): tiles (1,0)-(1,1) — clear
		// (nx,ly) = (15.6, 6.5): tiles (1,0)-(2,0) — clear
		// → Case 3: keep DirRight|DirDown.
		g := newTestGame()
		g.TileMap = newTileMap(3, 4, 0)
		g.TileMap[1][2] = 10
		p := &GameObject{
			X: 14.9, Y: 6.5, Width: 2, Height: 2,
			Speed: 1, Diag: true, YMax: YMax, XMaxLimit: XMax + 1,
		}
		got := g.bounceDir(p, DirRight|DirDown, 14.9, 6.5, 15.6, 7.2)
		if want := DirRight | DirDown; got != want {
			t.Errorf("corner: got %d, want %d", got, want)
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// slideWalk
// ═══════════════════════════════════════════════════════════════════════════
//
// slideWalk handles corner-rounding for pure cardinal movement.  When a
// player walks straight into a wall corner, slideWalk probes ±3 pixels on
// the perpendicular axis for a gap.  If found, it walks sideways to slip
// around the corner.  Diagonal movement is never eligible for sliding.
//
// Returns (slid, result): slid=true means a slide was attempted and
// result is whether the recursive walk succeeded.

func TestSlideWalk(t *testing.T) {
	t.Run("NoObstacle", func(t *testing.T) {
		// No obstacle at target → no slide needed.
		g := newTestGame()
		g.TileMap = newTileMap(4, 4, 0)
		p := newObj(8, 8)
		slid, _ := g.slideWalk(p, 0, 1, 8, 8, 8, 9)
		if slid {
			t.Error("should not slide when no obstacle")
		}
	})

	t.Run("DiagonalSkipped", func(t *testing.T) {
		// Obstacle exists but movement is diagonal (dx!=0 && dy!=0) →
		// slideWalk skips entirely.  Diagonal axis reversion is handled
		// by the caller in walk(), not here.
		g := newTestGame()
		g.TileMap = newTileMap(4, 4, 0)
		g.TileMap[2][0] = 10 // wall at (col=0, row=2)
		p := newObj(7, 8)
		slid, _ := g.slideWalk(p, 0.7, 0.7, 7, 8, 7.7, 8.7)
		if slid {
			t.Error("should not slide during diagonal movement")
		}
	})

	t.Run("SlideRightY", func(t *testing.T) {
		// Pure vertical movement into a wall that only covers column 0.
		// Probe X+3 = lx+3 = 10 clears the wall → slide right.
		//
		// Wall at (col=0, row=2).  Object at (7,8), 8×8.
		// Obstacle at (lx=7, ny=9): tiles (0,1)-(1,2), wall at (0,2) → true.
		// Probe X=10: Obstacle(10, 9) → tiles (1,1)-(2,2) → clear.
		// Recursive walk(DirRight, slide=true) → moves X from 7 to 8.
		g := newTestGame()
		g.TileMap = newTileMap(4, 4, 0)
		g.TileMap[2][0] = 10
		p := newObj(7, 8)
		slid, result := g.slideWalk(p, 0, 1, 7, 8, 7, 9)
		if !slid {
			t.Error("expected slide right")
		}
		if !result {
			t.Error("recursive walk should succeed")
		}
		if p.X != 8 {
			t.Errorf("X should be 8 after slide right, got %.1f", p.X)
		}
	})

	t.Run("SlideLeftY", func(t *testing.T) {
		// Positive probe is blocked, forcing the negative (left) slide.
		//
		// Walls at (col=2, row=1) and (col=3, row=1).  Object: 3×3 at (15,5).
		// Obstacle at (lx=15, ny=6): tiles (1,0)-(2,1), wall at (2,1) → true.
		// Probe X=18: tiles (2,0)-(2,1), wall at (2,1) → blocked.
		// Probe X=12: tiles (1,0)-(1,1), clear → slide left.
		// Recursive walk(DirLeft, slide=true) → moves X from 15 to 14.
		g := newTestGame()
		g.TileMap = newTileMap(4, 4, 0)
		g.TileMap[1][2] = 10
		g.TileMap[1][3] = 10
		p := &GameObject{
			X: 15, Y: 5, Width: 3, Height: 3,
			Speed: 1, YMax: YMax, XMaxLimit: XMax + 1,
		}
		slid, result := g.slideWalk(p, 0, 1, 15, 5, 15, 6)
		if !slid {
			t.Error("expected slide left")
		}
		if !result {
			t.Error("recursive walk should succeed")
		}
		if p.X != 14 {
			t.Errorf("X should be 14 after slide left, got %.1f", p.X)
		}
	})

	t.Run("SlideUpX", func(t *testing.T) {
		// Pure horizontal movement into a wall.  Probe Y+3 blocked,
		// probe Y-3 clear → slide up.
		//
		// Wall at (col=2, row=1).  Object: 3×3 at (13,7).
		// Obstacle at (nx=14, ly=7): tiles (1,0)-(2,1), wall at (2,1) → true.
		// Probe Y=10: Obstacle(14, 10) → tiles (1,1)-(2,1), wall → blocked.
		// Probe Y=4:  Obstacle(14, 4)  → tiles (1,0)-(2,0), clear → slide up.
		// Recursive walk(DirUp, slide=true) → moves Y from 7 to 6.
		g := newTestGame()
		g.TileMap = newTileMap(4, 4, 0)
		g.TileMap[1][2] = 10
		p := &GameObject{
			X: 13, Y: 7, Width: 3, Height: 3,
			Speed: 1, YMax: YMax, XMaxLimit: XMax + 1,
		}
		slid, result := g.slideWalk(p, 1, 0, 13, 7, 14, 7)
		if !slid {
			t.Error("expected slide up")
		}
		if !result {
			t.Error("recursive walk should succeed")
		}
		if p.Y != 6 {
			t.Errorf("Y should be 6 after slide up, got %.1f", p.Y)
		}
	})

	t.Run("SlideDownX", func(t *testing.T) {
		// Pure horizontal movement into a wall.  Probe Y+3 finds a
		// gap below → slide down.  This covers the positive Y probe
		// in the horizontal slide branch.
		//
		// Wall at (col=2, row=0).  Object: 3×3 at (13, 1).
		// Obstacle at (nx=14, ly=1): tiles (1,0)-(2,0), wall at (2,0) → true.
		// Probe Y=4: Obstacle(14, 4) → tiles (1,0)-(2,0), wall → blocked?
		// Need wall only at row 0 so probe Y+3 clears it.
		// Actually: Object 3×3 at (13,1), probe Y=4.
		//   bounds (14,4)-(16,6), tiles (1,0)-(2,0) → wall at (2,0) → true.
		// That won't work. Let me use a bigger gap. Object at (13,5),
		// wall at (col=2, row=0). Obstacle at (14,5): bounds (14,5)-(16,7),
		// tiles (1,0)-(2,0) → wall at (2,0) → true.
		// Probe Y=8: bounds (14,8)-(16,10), tiles (1,1)-(2,1) → clear!
		// → slide Down.  Recursive walk moves Y from 5 to 6.
		g := newTestGame()
		g.TileMap = newTileMap(4, 4, 0)
		g.TileMap[0][2] = 10
		p := &GameObject{
			X: 13, Y: 5, Width: 3, Height: 3,
			Speed: 1, YMax: YMax, XMaxLimit: XMax + 1,
		}
		slid, result := g.slideWalk(p, 1, 0, 13, 5, 14, 5)
		if !slid {
			t.Error("expected slide down")
		}
		if !result {
			t.Error("recursive walk should succeed")
		}
		if p.Y != 6 {
			t.Errorf("Y should be 6 after slide down, got %.1f", p.Y)
		}
	})

	t.Run("BothBlockedNoSlide", func(t *testing.T) {
		// Wall spans the entire row → both ±3 probes hit walls → no slide.
		g := newTestGame()
		g.TileMap = newTileMap(4, 4, 0)
		for c := 0; c < 4; c++ {
			g.TileMap[1][c] = 10
		}
		p := &GameObject{
			X: 10, Y: 5, Width: 3, Height: 3,
			Speed: 1, YMax: YMax, XMaxLimit: XMax + 1,
		}
		slid, _ := g.slideWalk(p, 0, 1, 10, 5, 10, 6)
		if slid {
			t.Error("should not slide when both directions blocked")
		}
	})

	t.Run("BothBlockedNoSlideX", func(t *testing.T) {
		// Pure horizontal movement into a wall column that spans all
		// rows → both Y±3 probes hit walls → no slide.  Covers the
		// true branch of the negative Y probe (line 228).
		//
		// Wall at col 2 (all rows).  Object: 4×4 at (13,5), nx=14.
		// Obstacle(14, ly=5): bounds (14,5)-(17,8), cols 1-2 → wall ✓
		// Probe Y=8:  Obstacle(14, 8) → cols 1-2, row 1 → wall ✓
		// Probe Y=2:  Obstacle(14, 2) → cols 1-2, row 0 → wall ✓
		g := newTestGame()
		g.TileMap = newTileMap(4, 4, 0)
		for r := 0; r < 4; r++ {
			g.TileMap[r][2] = 10
		}
		p := &GameObject{
			X: 13, Y: 5, Width: 4, Height: 4,
			Speed: 1, YMax: YMax, XMaxLimit: XMax + 1,
		}
		slid, _ := g.slideWalk(p, 1, 0, 13, 5, 14, 5)
		if slid {
			t.Error("should not slide when both Y probes blocked")
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// Walk / walk
// ═══════════════════════════════════════════════════════════════════════════
//
// Walk is the public entry point: walk(p, dir, slide=false).
// walk implements the full movement pipeline:
//
//   1. dirDelta     – convert direction bits to (dx, dy)
//   2. snapDiag     – snap position when entering diagonal movement
//   3. Clamp        – restrict to world bounds (YMin/YMax, 0/XMaxLimit)
//   4. Bounce       – if Bounce flag, reverse direction on wall hit
//   5. Slide        – if not already sliding, try to nudge around corners
//   6. Blocked      – if position didn't change, report failure
//   7. MobCollide   – handle collisions with other objects
//   8. Commit       – update p.X, p.Y and return true
//
// Each test below targets one or two of these pipeline stages.

func TestWalkBasic(t *testing.T) {
	t.Run("InvalidDirection", func(t *testing.T) {
		// Direction 0 has no valid delta → walk returns false immediately.
		g := newTestGame()
		g.TileMap = newTileMap(4, 4, 0)
		p := newObj(10, 10)
		if g.Walk(p, 0) {
			t.Error("invalid direction should return false")
		}
		if p.X != 10 || p.Y != 10 {
			t.Error("position should not change")
		}
	})

	t.Run("Right", func(t *testing.T) {
		g := newTestGame()
		g.TileMap = newTileMap(4, 4, 0)
		p := newObj(10, 10)
		if !g.Walk(p, DirRight) {
			t.Error("should succeed in open space")
		}
		if p.X != 11 {
			t.Errorf("X = %.1f, want 11", p.X)
		}
	})

	t.Run("Left", func(t *testing.T) {
		g := newTestGame()
		g.TileMap = newTileMap(4, 4, 0)
		p := newObj(10, 10)
		if !g.Walk(p, DirLeft) {
			t.Error("should succeed")
		}
		if p.X != 9 {
			t.Errorf("X = %.1f, want 9", p.X)
		}
	})

	t.Run("Up", func(t *testing.T) {
		g := newTestGame()
		g.TileMap = newTileMap(4, 4, 0)
		p := newObj(10, 10)
		if !g.Walk(p, DirUp) {
			t.Error("should succeed")
		}
		if p.Y != 9 {
			t.Errorf("Y = %.1f, want 9", p.Y)
		}
	})

	t.Run("Down", func(t *testing.T) {
		g := newTestGame()
		g.TileMap = newTileMap(4, 4, 0)
		p := newObj(10, 10)
		if !g.Walk(p, DirDown) {
			t.Error("should succeed")
		}
		if p.Y != 11 {
			t.Errorf("Y = %.1f, want 11", p.Y)
		}
	})

	t.Run("Diagonal", func(t *testing.T) {
		// Diagonal movement uses 0.7×speed per axis and sets the Diag
		// flag.  snapDiag Floors the integer position (10→10) first,
		// then adds 0.7.
		g := newTestGame()
		g.TileMap = newTileMap(4, 4, 0)
		p := newObj(10, 10)
		if !g.Walk(p, DirRight|DirDown) {
			t.Error("diagonal should succeed")
		}
		if !p.Diag {
			t.Error("Diag flag should be set")
		}
		if !approxEqual(p.X, 10.7, 0.01) || !approxEqual(p.Y, 10.7, 0.01) {
			t.Errorf("position = (%.2f,%.2f), want (10.7,10.7)", p.X, p.Y)
		}
	})
}

func TestWalkBoundary(t *testing.T) {
	t.Run("LeftEdge", func(t *testing.T) {
		// At X=0, moving left clamps nx to 0 → nx==lx → blocked.
		g := newTestGame()
		g.TileMap = newTileMap(4, 4, 0)
		p := newObj(0, 10)
		if g.Walk(p, DirLeft) {
			t.Error("should be blocked at left boundary")
		}
	})

	t.Run("TopEdge", func(t *testing.T) {
		// At Y=YMin, moving up clamps ny → blocked.
		g := newTestGame()
		g.TileMap = newTileMap(4, 4, 0)
		p := newObj(10, 0)
		if g.Walk(p, DirUp) {
			t.Error("should be blocked at top boundary")
		}
	})

	t.Run("RightEdge", func(t *testing.T) {
		// At X=XMaxLimit, moving right clamps → blocked.
		g := newTestGame()
		g.TileMap = newTileMap(200, 200, 0)
		p := newObj(float64(XMax+1), 10)
		if g.Walk(p, DirRight) {
			t.Error("should be blocked at right boundary")
		}
	})

	t.Run("BottomEdge", func(t *testing.T) {
		// At Y=YMax, moving down clamps → blocked.
		g := newTestGame()
		g.TileMap = newTileMap(25, 4, 0)
		p := newObj(10, YMax)
		if g.Walk(p, DirDown) {
			t.Error("should be blocked at bottom boundary")
		}
	})
}

func TestWalkWallBlocked(t *testing.T) {
	// Object at (8,0) moves down into a solid wall spanning the entire row.
	// slideWalk tries ±3 nudges but both still hit the wall → fallback
	// to previous position → nx==lx and ny==ly → blocked.
	g := newTestGame()
	g.TileMap = newTileMap(4, 4, 0)
	for c := 0; c < 4; c++ {
		g.TileMap[1][c] = 10
	}
	p := newObj(8, 0)
	if g.Walk(p, DirDown) {
		t.Error("should be blocked by solid wall")
	}
	if p.Y != 0 {
		t.Errorf("Y should remain 0, got %.1f", p.Y)
	}
}

func TestWalkBounce(t *testing.T) {
	t.Run("CardinalBounce", func(t *testing.T) {
		// A bouncing bomb moves right into a wall at (2,1).  The
		// obstacle triggers a bounce → direction flips to DirLeft.
		// Second attempt with DirLeft succeeds in the open space.
		g := newTestGame()
		g.TileMap = newTileMap(4, 4, 0)
		g.TileMap[1][2] = 10

		p := newObj(9, 8)
		p.Bounce = true
		// Right: nx=10, bounds (10,8)-(17,15), tiles (1,1)-(2,1), wall → bounce.
		// bounceDir: cardinal → DirLeft.
		// Left: nx=8, bounds (8,8)-(15,15), tile (1,1) → clear → success.
		if !g.Walk(p, DirRight) {
			t.Error("bounce should succeed")
		}
		if p.Dir != DirLeft {
			t.Errorf("Dir should be DirLeft, got %d", p.Dir)
		}
		if p.X != 8 {
			t.Errorf("X = %.1f, want 8 (moved left after bounce)", p.X)
		}
	})

	t.Run("DoubleBounceBlocked", func(t *testing.T) {
		// Walls on both sides: first bounce reverses direction,
		// second attempt (attempt>0) returns false immediately.
		g := newTestGame()
		g.TileMap = newTileMap(4, 4, 0)
		g.TileMap[1][0] = 10 // blocks leftward
		g.TileMap[1][2] = 10 // blocks rightward

		p := newObj(8, 8)
		p.Bounce = true
		// Right: wall at (2,1) → bounce to left.
		// Left: nx=7, bounds (7,8)-(14,15), tiles (0,1)-(1,1),
		//   wall at (0,1) → attempt>0 → false.
		if g.Walk(p, DirRight) {
			t.Error("double bounce should fail")
		}
	})

	t.Run("DiagonalBounce", func(t *testing.T) {
		// Bomb moving right+down bounces off a horizontal wall.
		g := newTestGame()
		g.TileMap = newTileMap(3, 4, 0)
		for c := 0; c < 4; c++ {
			g.TileMap[1][c] = 10
		}
		p := &GameObject{
			X: 5, Y: 6.5, Width: 2, Height: 2,
			Speed: 1, Bounce: true, Diag: true,
			YMax: YMax, XMaxLimit: XMax + 1,
		}
		if !g.Walk(p, DirRight|DirDown) {
			t.Error("diagonal bounce should succeed")
		}
		if p.Dir != DirRight|DirUp {
			t.Errorf("Dir = %d, want %d (right+up)", p.Dir, DirRight|DirUp)
		}
	})
}

func TestWalkSlide(t *testing.T) {
	// The player walks straight down and their bounding box barely
	// clips a wall tile in the lower-left.  The slide system detects
	// that shifting right 3 pixels clears the wall, and recursively
	// walks the object right by 1 pixel.
	//
	// Wall at (col=0, row=2).  Object at (7,8), 8×8.
	// Moving DirDown: ny=9, bounds (7,9)-(14,16), tiles (0,1)-(1,2),
	//   wall at (0,2) → slideWalk fires.
	// probe X=10: bounds (10,9)-(17,16), tiles (1,1)-(2,2) → clear.
	// Recursive walk(DirRight, slide=true): moves X 7→8.
	g := newTestGame()
	g.TileMap = newTileMap(4, 4, 0)
	g.TileMap[2][0] = 10

	p := newObj(7, 8)
	if !g.Walk(p, DirDown) {
		t.Error("slide should succeed")
	}
	// The recursive walk moved X right by 1.
	if p.X != 8 {
		t.Errorf("X = %.1f, want 8 (slid right)", p.X)
	}
}

func TestWalkDiagonalAxisRevert(t *testing.T) {
	// Diagonal walk (DirRight|DirDown) where the X axis is blocked
	// by a wall but the Y axis is clear.  walk()'s per-axis obstacle
	// check reverts nx→lx while keeping ny, so the object moves
	// vertically only.  This exercises the X-axis reversion in walk().
	//
	// Wall at (col=2, row=0).  Object: 3×3 at (13,5), speed=2.
	// dx=0.7, dy=0.7 → nx=14.4, ny=6.4.
	// Obstacle(lx=13, ny=6.4): bounds (13,6.4)-(15,8.4), cols 1-1, rows 0-1 → clear ✓
	// Obstacle(nx=14.4, ly=5): bounds (14.4,5)-(16.4,7), cols 1-2, rows 0-0 → wall at (2,0) → X blocked!
	// nx reverts to 13.  Object moves to (13, 6.4).
	g := newTestGame()
	g.TileMap = newTileMap(4, 4, 0)
	g.TileMap[0][2] = 10
	p := &GameObject{
		X: 13, Y: 5, Width: 3, Height: 3,
		Speed: 2, YMax: YMax, XMaxLimit: XMax + 1,
	}
	if !g.Walk(p, DirRight|DirDown) {
		t.Error("diagonal with one axis blocked should still succeed on the other")
	}
	if p.X != 13 {
		t.Errorf("X should remain 13 (blocked), got %.2f", p.X)
	}
	if !approxEqual(p.Y, 6.4, 0.01) {
		t.Errorf("Y should be ≈6.4, got %.2f", p.Y)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Walk + MobCollide integration
// ═══════════════════════════════════════════════════════════════════════════

func TestWalkBlockedByFriendly(t *testing.T) {
	// A waif tries to walk right into the player.  MobCollide detects
	// the waif–player overlap and returns the player (non-nil), which
	// causes walk to return false WITHOUT updating the waif's position.
	// This prevents waifs from walking through the player.
	g := newTestGame()
	g.TileMap = newTileMap(4, 4, 0)
	player := newObj(11, 10)
	player.Kind = Player
	waif := newObj(10, 10)
	waif.Kind = Waif
	waif.Speed = 1
	g.Objects = []*GameObject{waif, player}

	if g.Walk(waif, DirRight) {
		t.Error("waif should be blocked by player")
	}
	if waif.X != 10 {
		t.Errorf("waif X = %.1f, should not have moved", waif.X)
	}
}

func TestWalkOpensCrate(t *testing.T) {
	// Player walks right into a crate.  MobCollide detects the crate,
	// calls OpenCrate (destroying the crate and awarding items),
	// returns nil → walk updates the player's position.
	g := newTestGame()
	g.TileMap = newTileMap(4, 4, 0)
	player := newObj(10, 10)
	player.Kind = Player
	player.Speed = 1
	crate := newObj(11, 10)
	crate.Kind = Crate
	crate.Final = testDeadDef
	g.Objects = []*GameObject{player, crate}

	if !g.Walk(player, DirRight) {
		t.Error("should walk through crate")
	}
	if player.X != 11 {
		t.Errorf("X = %.1f, want 11", player.X)
	}
	// Crate should be reaped.
	for _, obj := range g.Objects {
		if obj == crate {
			t.Error("crate should have been reaped")
		}
	}
}
