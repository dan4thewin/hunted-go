package main

import (
	"math/rand"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
)

// ===========================================================================
// Test Helpers (AI-specific)
// ===========================================================================
//
// newAIGame builds a Game suitable for AI tests: seeded RNG, wall tile
// config, and a PathCols x PathRows grid (matching the real game dimensions).
// SoundsLoaded=false prevents PlaySfx from calling raylib.

func newAIGame() *Game {
	g := &Game{
		Rng: rand.New(rand.NewSource(42)),
		TileConfig: TileConfig{
			FirstWallTile: 10,
			WallTileCnt:   3,
			RockTile:      20,
			BlackTile:     1,
		},
		Path: make([][]int, PathCols),
	}
	for i := range g.Path {
		g.Path[i] = make([]int, PathRows)
	}
	return g
}

// newAIObj builds a minimal GameObject for AI tests.
func newAIObj(x, y float64) *GameObject {
	return &GameObject{
		X: x, Y: y, Width: 8, Height: 8,
		Speed: 1, YMax: YMax, XMaxLimit: XMax + 1,
	}
}

// ===========================================================================
// dirToward
// ===========================================================================
//
// Pure function: returns direction bitmask to move from (px,py) toward
// (tx,ty).  No game state needed.

func TestDirToward(t *testing.T) {
	tests := []struct {
		name                 string
		px, py, tx, ty, want int
	}{
		{"Same", 10, 10, 10, 10, 0},
		{"Left", 10, 10, 5, 10, DirLeft},
		{"Right", 10, 10, 15, 10, DirRight},
		{"Up", 10, 10, 10, 5, DirUp},
		{"Down", 10, 10, 10, 15, DirDown},
		{"UpLeft", 10, 10, 5, 5, DirLeft | DirUp},
		{"DownRight", 10, 10, 15, 15, DirRight | DirDown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := dirToward(tc.px, tc.py, tc.tx, tc.ty)
			if got != tc.want {
				t.Errorf("dirToward(%d,%d,%d,%d) = %d, want %d",
					tc.px, tc.py, tc.tx, tc.ty, got, tc.want)
			}
		})
	}
}

// ===========================================================================
// innerFollow
// ===========================================================================
//
// Main per-step movement decision for thugs.  Determines which direction(s)
// to move toward a target, checking hazards and obstacles on each axis.
// Returns direction bits and an "arrived" flag (true when hasLast &&
// already at target position).

func TestInnerFollow(t *testing.T) {
	t.Run("ArrivedAtCrumb", func(t *testing.T) {
		// Object is at the target position and hasLast=true (breadcrumb
		// chain continues) → signals "arrived" so Follow can advance.
		g := newAIGame()
		g.TileMap = newTileMap(4, 4, 0)
		p := newAIObj(10, 10)
		ret, arrived := g.innerFollow(p, 10, 10, true)
		if !arrived {
			t.Error("should signal arrived at crumb position")
		}
		if ret != 0 {
			t.Errorf("expected 0 direction at arrival, got %d", ret)
		}
	})

	t.Run("AtTargetNoLast", func(t *testing.T) {
		// At target position but hasLast=false (direct target, not crumb).
		// Does NOT signal arrived — returns 0 since itx==x and ity==y.
		g := newAIGame()
		g.TileMap = newTileMap(4, 4, 0)
		p := newAIObj(10, 10)
		ret, arrived := g.innerFollow(p, 10, 10, false)
		if arrived {
			t.Error("should not signal arrived without hasLast")
		}
		if ret != 0 {
			t.Errorf("expected 0 (at target), got %d", ret)
		}
	})

	t.Run("HasLastButNotArrived", func(t *testing.T) {
		// hasLast=true but NOT at target → does NOT signal arrived.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		p := newAIObj(8, 8)
		ret, arrived := g.innerFollow(p, 80, 80, true)
		if arrived {
			t.Error("should not signal arrived when not at target")
		}
		if ret == 0 {
			t.Error("should return direction toward target")
		}
	})

	t.Run("HasLastSameXDiffY", func(t *testing.T) {
		// hasLast=true, same X but different Y → itx==x is true but
		// ity==y is false, so NOT arrived.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		p := newAIObj(8, 8)
		ret, arrived := g.innerFollow(p, 8, 80, true)
		if arrived {
			t.Error("should not signal arrived with different Y")
		}
		if ret == 0 {
			t.Error("should move toward target Y")
		}
	})

	t.Run("FollowBothAxes", func(t *testing.T) {
		// Target is to the right and below → should get DirRight|DirDown.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		p := newAIObj(8, 8)
		ret, arrived := g.innerFollow(p, 80, 80, false)
		if arrived {
			t.Error("should not be arrived")
		}
		if ret != DirRight|DirDown {
			t.Errorf("expected DirRight|DirDown=%d, got %d", DirRight|DirDown, ret)
		}
	})

	t.Run("TargetLeft_Clear", func(t *testing.T) {
		// Target to the left, no hazard, no wall → DirLeft.
		g := newAIGame()
		g.TileMap = newTileMap(4, 4, 0)
		p := newAIObj(16, 8)
		ret, _ := g.innerFollow(p, 5, 8, false)
		if ret != DirLeft {
			t.Errorf("expected DirLeft, got %d", ret)
		}
	})

	t.Run("TargetRight_Clear", func(t *testing.T) {
		// Target to the right, no hazard, no wall → DirRight.
		g := newAIGame()
		g.TileMap = newTileMap(4, 4, 0)
		p := newAIObj(8, 8)
		ret, _ := g.innerFollow(p, 20, 8, false)
		if ret != DirRight {
			t.Errorf("expected DirRight, got %d", ret)
		}
	})

	t.Run("TargetLeft_ObstacleBlocks", func(t *testing.T) {
		// Target left but wall blocks → 0 on X axis.
		g := newAIGame()
		g.TileMap = newTileMap(4, 4, 0)
		g.TileMap[1][1] = 10 // wall
		p := newAIObj(16, 8)
		ret, _ := g.innerFollow(p, 5, 8, false)
		if ret != 0 {
			t.Errorf("expected 0 (blocked left), got %d", ret)
		}
	})

	t.Run("TargetRight_ObstacleBlocks", func(t *testing.T) {
		// Target right but wall blocks → 0 on X axis.
		g := newAIGame()
		g.TileMap = newTileMap(4, 4, 0)
		g.TileMap[1][2] = 10 // wall
		p := newAIObj(8, 8)
		ret, _ := g.innerFollow(p, 20, 8, false)
		if ret != 0 {
			t.Errorf("expected 0 (blocked right), got %d", ret)
		}
	})

	t.Run("TargetAbove", func(t *testing.T) {
		// Target is above the object → DirUp.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		p := newAIObj(8, 80)
		ret, _ := g.innerFollow(p, 8, 8, false)
		if ret != DirUp {
			t.Errorf("expected DirUp, got %d", ret)
		}
	})

	t.Run("TargetBelow", func(t *testing.T) {
		// Target is below the object → DirDown.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		p := newAIObj(8, 8)
		ret, _ := g.innerFollow(p, 8, 80, false)
		if ret != DirDown {
			t.Errorf("expected DirDown, got %d", ret)
		}
	})

	t.Run("HazardBlocksLeft", func(t *testing.T) {
		// Hazard to the left → hl != nil → can't go left.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		p := newAIObj(16, 8)
		hazard := newAIObj(8, 8)
		hazard.Avoid = true
		g.Objects = []*GameObject{p, hazard}
		ret, _ := g.innerFollow(p, 5, 8, false)
		if ret != 0 {
			t.Errorf("expected 0 (hazard blocks left), got %d", ret)
		}
	})

	t.Run("HazardBlocksRight", func(t *testing.T) {
		// Hazard to the right → hr != nil → can't go right.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		p := newAIObj(8, 8)
		hazard := newAIObj(16, 8)
		hazard.Avoid = true
		g.Objects = []*GameObject{p, hazard}
		ret, _ := g.innerFollow(p, 24, 8, false)
		if ret != 0 {
			t.Errorf("expected 0 (hazard blocks right), got %d", ret)
		}
	})

	t.Run("HazardBlocksUp", func(t *testing.T) {
		// Hazard above → hu != nil → can't go up.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		p := newAIObj(8, 16)
		hazard := newAIObj(8, 8)
		hazard.Avoid = true
		g.Objects = []*GameObject{p, hazard}
		ret, _ := g.innerFollow(p, 8, 5, false)
		if ret != 0 {
			t.Errorf("expected 0 (hazard blocks up), got %d", ret)
		}
	})

	t.Run("HazardBlocksDown", func(t *testing.T) {
		// Hazard below → hd != nil → can't go down.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		p := newAIObj(8, 8)
		hazard := newAIObj(8, 16)
		hazard.Avoid = true
		g.Objects = []*GameObject{p, hazard}
		ret, _ := g.innerFollow(p, 8, 24, false)
		if ret != 0 {
			t.Errorf("expected 0 (hazard blocks down), got %d", ret)
		}
	})

	t.Run("ObstacleBlocksUp", func(t *testing.T) {
		// Wall above blocks upward movement.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		// p at (8,16) → top edge at y=15 (16+0-1=15 with dy=-1).
		// Tile at row 1, col 1 covers y=[8,15].
		g.TileMap[1][1] = 10 // wall at col=1, row=1
		p := newAIObj(8, 16)
		ret, _ := g.innerFollow(p, 8, 5, false)
		if ret != 0 {
			t.Errorf("expected 0 (obstacle blocks up), got %d", ret)
		}
	})

	t.Run("ObstacleBlocksDown", func(t *testing.T) {
		// Wall below blocks downward movement.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		g.TileMap[2][1] = 10 // wall below
		p := newAIObj(8, 8)
		ret, _ := g.innerFollow(p, 8, 24, false)
		if ret != 0 {
			t.Errorf("expected 0 (obstacle blocks down), got %d", ret)
		}
	})
}

// ===========================================================================
//
// Fills every cell in the PathCols x PathRows grid with MaxN.

func TestResetPathGrid(t *testing.T) {
	g := newAIGame()
	g.Path[0][0] = 0
	g.Path[15][10] = 5
	g.Path[PathCols-1][PathRows-1] = 42
	g.resetPathGrid()
	for i := range g.Path {
		for j := range g.Path[i] {
			if g.Path[i][j] != MaxN {
				t.Errorf("Path[%d][%d] = %d, want MaxN (%d)", i, j, g.Path[i][j], MaxN)
			}
		}
	}
}

// ===========================================================================
// bfsPath
// ===========================================================================
//
// BFS flood fill on the PathCols x PathRows grid.  Starting cell should
// be 0 (set by UpdatePath before calling bfsPath).  Neighbors get cost+1.
// Walls get WallCost.

func TestBfsPath(t *testing.T) {
	t.Run("OpenGrid", func(t *testing.T) {
		// Open area with source at (1,1) 0-indexed.
		// After BFS, cells should have Manhattan-like distances.
		g := newAIGame()
		g.TileMap = newTileMap(40, 20, 0)
		g.resetPathGrid()
		g.Path[1][1] = 0
		g.bfsPath(1, 1, 0)
		if g.Path[2][1] != 1 {
			t.Errorf("right neighbor = %d, want 1", g.Path[2][1])
		}
		if g.Path[0][1] != 1 {
			t.Errorf("left neighbor = %d, want 1", g.Path[0][1])
		}
		if g.Path[1][2] != 1 {
			t.Errorf("below neighbor = %d, want 1", g.Path[1][2])
		}
		if g.Path[1][0] != 1 {
			t.Errorf("above neighbor = %d, want 1", g.Path[1][0])
		}
	})

	t.Run("WallBlocking", func(t *testing.T) {
		// Wall tile should get WallCost and not propagate further.
		g := newAIGame()
		g.TileMap = newTileMap(40, 20, 0)
		g.TileMap[1][2] = 10 // wall at col=2, row=1
		g.resetPathGrid()
		g.Path[1][1] = 0
		g.bfsPath(1, 1, 0)
		if g.Path[2][1] != WallCost {
			t.Errorf("wall cell = %d, want %d", g.Path[2][1], WallCost)
		}
	})

	t.Run("OutOfBounds", func(t *testing.T) {
		// Starting position outside the grid → BFS skips it.
		g := newAIGame()
		g.TileMap = newTileMap(40, 20, 0)
		g.resetPathGrid()
		g.bfsPath(-1, -1, 0)
		for i := range g.Path {
			for j := range g.Path[i] {
				if g.Path[i][j] != MaxN {
					t.Errorf("Path[%d][%d] = %d, want MaxN", i, j, g.Path[i][j])
				}
			}
		}
	})

	t.Run("EdgeBoundaries", func(t *testing.T) {
		// Source at bottom-right corner → BFS expands to the edges,
		// exercising c>=PathCols and r>=PathRows boundary checks.
		g := newAIGame()
		g.TileMap = newTileMap(40, 24, 0)
		g.resetPathGrid()
		g.Path[PathCols-1][PathRows-1] = 0
		g.bfsPath(PathCols-1, PathRows-1, 0)
		if g.Path[PathCols-2][PathRows-1] != 1 {
			t.Errorf("left neighbor = %d, want 1", g.Path[PathCols-2][PathRows-1])
		}
		if g.Path[PathCols-1][PathRows-2] != 1 {
			t.Errorf("above neighbor = %d, want 1", g.Path[PathCols-1][PathRows-2])
		}
	})

	t.Run("InitialCellOutOfBounds", func(t *testing.T) {
		// When the initial BFS cell is out of bounds, it's skipped.
		g := newAIGame()
		g.TileMap = newTileMap(40, 24, 0)
		g.resetPathGrid()

		// c < 0
		g.bfsPath(-1, 0, 0)
		if g.Path[0][0] != MaxN {
			t.Errorf("c<0: expected MaxN, got %d", g.Path[0][0])
		}

		// r < 0
		g.bfsPath(0, -1, 0)
		if g.Path[0][0] != MaxN {
			t.Errorf("r<0: expected MaxN, got %d", g.Path[0][0])
		}

		// c >= PathCols
		g.bfsPath(PathCols, 0, 0)
		if g.Path[PathCols-1][0] != MaxN {
			t.Errorf("c>=PathCols: expected MaxN, got %d", g.Path[PathCols-1][0])
		}

		// r >= PathRows
		g.bfsPath(0, PathRows, 0)
		if g.Path[0][PathRows-1] != MaxN {
			t.Errorf("r>=PathRows: expected MaxN, got %d", g.Path[0][PathRows-1])
		}
	})
}

// ===========================================================================
// UpdatePath
// ===========================================================================
//
// Computes the BFS path grid centered on a player's position.  Caches
// the grid position in PathC/PathR and skips recomputation if unchanged.

func TestUpdatePath(t *testing.T) {
	t.Run("FirstUpdate", func(t *testing.T) {
		// First call always computes the path grid.
		g := newAIGame()
		g.TileMap = newTileMap(40, 20, 0)
		p := newAIObj(16, 16)
		g.UpdatePath(p)
		// Source cell should be 0.
		if p.PathC == 0 && p.PathR == 0 {
			t.Error("PathC/PathR should be set after UpdatePath")
		}
		// At least one cell should have been set to a non-MaxN value.
		found := false
		for i := range g.Path {
			for j := range g.Path[i] {
				if g.Path[i][j] != MaxN {
					found = true
					break
				}
			}
		}
		if !found {
			t.Error("path grid should have non-MaxN cells after BFS")
		}
	})

	t.Run("CachedPosition", func(t *testing.T) {
		// Same position → PathC/PathR unchanged → early return.
		g := newAIGame()
		g.TileMap = newTileMap(40, 20, 0)
		p := newAIObj(16, 16)
		g.UpdatePath(p)
		pc, pr := p.PathC, p.PathR
		// Corrupt a cell to detect if grid is recomputed.
		g.Path[0][0] = 999
		g.UpdatePath(p)
		if g.Path[0][0] != 999 {
			t.Error("grid should not be recomputed for same position")
		}
		if p.PathC != pc || p.PathR != pr {
			t.Error("PathC/PathR should not change")
		}
	})

	t.Run("NewPosition", func(t *testing.T) {
		// Different position → grid is recomputed (corrupted cell reset).
		g := newAIGame()
		g.TileMap = newTileMap(40, 20, 0)
		p := newAIObj(16, 16)
		g.UpdatePath(p)
		g.Path[0][0] = 999
		p.X = 80 // move to a different tile
		g.UpdatePath(p)
		if g.Path[0][0] == 999 {
			t.Error("grid should be recomputed for new position")
		}
	})

	t.Run("SameColumn_DifferentRow", func(t *testing.T) {
		// Same column (mapC+c unchanged) but different row → grid is
		// recomputed.  Covers the false branch of r == p.PathR (line 69).
		g := newAIGame()
		g.TileMap = newTileMap(40, 20, 0)
		p := newAIObj(16, 16)
		g.UpdatePath(p)
		g.Path[0][0] = 999
		p.Y = 80 // move vertically within the same column
		g.UpdatePath(p)
		if g.Path[0][0] == 999 {
			t.Error("grid should be recomputed for new row")
		}
	})

	t.Run("OutOfBounds_CR", func(t *testing.T) {
		// Player at extreme position where c falls below 0 →
		// the source cell is not set to 0 but BFS still runs.
		g := newAIGame()
		g.TileMap = newTileMap(40, 20, 0)
		p := newAIObj(0, 0)
		p.XOff = -TileSize // push c below 0
		g.UpdatePath(p)
		allMax := true
		for i := range g.Path {
			for j := range g.Path[i] {
				if g.Path[i][j] != MaxN {
					allMax = false
				}
			}
		}
		if !allMax {
			t.Error("all cells should be MaxN when source is out of bounds")
		}
	})

	t.Run("OutOfBounds_R", func(t *testing.T) {
		// Player far below the screen → r >= PathRows.
		// r = (int(p.Y)+p.YOff)/TileSize.  Need r >= 20 → Y >= 160.
		g := newAIGame()
		g.TileMap = newTileMap(40, 25, 0)
		p := newAIObj(16, 160) // r = 160/8 = 20 → >= PathRows
		g.UpdatePath(p)
		allMax := true
		for i := range g.Path {
			for j := range g.Path[i] {
				if g.Path[i][j] != MaxN {
					allMax = false
				}
			}
		}
		if !allMax {
			t.Error("all cells should be MaxN when r is out of bounds")
		}
	})

	t.Run("OutOfBounds_LowR", func(t *testing.T) {
		// Player above screen → r < 0 (negative Y with offset).
		g := newAIGame()
		g.TileMap = newTileMap(40, 20, 0)
		p := newAIObj(16, 0)
		p.YOff = -TileSize // r = (0-8)/8 = -1 → < 0
		g.UpdatePath(p)
		allMax := true
		for i := range g.Path {
			for j := range g.Path[i] {
				if g.Path[i][j] != MaxN {
					allMax = false
				}
			}
		}
		if !allMax {
			t.Error("all cells should be MaxN when r is out of bounds")
		}
	})
}

// ===========================================================================
// bestPathNeighbor
// ===========================================================================
//
// Finds the neighbor in the Path grid with the lowest cost.  Used by
// FollowPath to determine which direction a waif should walk.

func TestBestPathNeighbor(t *testing.T) {
	t.Run("CenterLowest", func(t *testing.T) {
		// Center cell has cost 0, neighbors have cost 1+ → returns center.
		g := newAIGame()
		g.resetPathGrid()
		g.Path[4][4] = 0
		g.Path[5][4] = 1
		g.Path[3][4] = 2
		nc, nr, pmin := g.bestPathNeighbor(4, 4)
		if nc != 4 || nr != 4 {
			t.Errorf("expected center (4,4), got (%d,%d)", nc, nr)
		}
		if pmin != 0 {
			t.Errorf("expected pmin=0, got %d", pmin)
		}
	})

	t.Run("NeighborLower", func(t *testing.T) {
		// Neighbor has lower cost than center → returns neighbor.
		g := newAIGame()
		g.resetPathGrid()
		g.Path[4][4] = 5 // center
		g.Path[5][4] = 2 // right neighbor is cheaper
		nc, nr, pmin := g.bestPathNeighbor(4, 4)
		if nc != 5 || nr != 4 {
			t.Errorf("expected (5,4), got (%d,%d)", nc, nr)
		}
		if pmin != 2 {
			t.Errorf("expected pmin=2, got %d", pmin)
		}
	})

	t.Run("OutOfBoundsCenter", func(t *testing.T) {
		// Center outside valid range (-1,-1) → pmin starts at MaxN,
		// checks available neighbors only.
		g := newAIGame()
		g.resetPathGrid()
		g.Path[0][0] = 3
		nc, nr, pmin := g.bestPathNeighbor(-1, -1)
		if pmin != 3 {
			t.Errorf("expected pmin=3, got %d", pmin)
		}
		if nc != 0 || nr != 0 {
			t.Errorf("expected (0,0), got (%d,%d)", nc, nr)
		}
	})

	t.Run("EdgeCell", func(t *testing.T) {
		// Cell at grid corner (0,0) → only 3 neighbors exist.
		g := newAIGame()
		g.resetPathGrid()
		g.Path[0][0] = 0
		g.Path[1][0] = 5
		g.Path[0][1] = 5
		g.Path[1][1] = 5
		nc, nr, pmin := g.bestPathNeighbor(0, 0)
		if nc != 0 || nr != 0 {
			t.Errorf("expected (0,0), got (%d,%d)", nc, nr)
		}
		if pmin != 0 {
			t.Errorf("expected pmin=0, got %d", pmin)
		}
	})

	t.Run("FarCorner", func(t *testing.T) {
		// Cell past the far corner → outside valid range.
		// Covers the c >= PathCols false branch.
		g := newAIGame()
		g.resetPathGrid()
		g.Path[PathCols-1][PathRows-1] = 1
		nc, nr, pmin := g.bestPathNeighbor(PathCols, PathRows)
		if pmin != 1 {
			t.Errorf("expected pmin=1, got %d", pmin)
		}
		if nc != PathCols-1 || nr != PathRows-1 {
			t.Errorf("expected (%d,%d), got (%d,%d)", PathCols-1, PathRows-1, nc, nr)
		}
	})

	t.Run("ValidC_InvalidLowR", func(t *testing.T) {
		// c=4 (valid) but r=-1 → r>=0 is false.
		g := newAIGame()
		g.resetPathGrid()
		g.Path[4][0] = 2
		nc, nr, pmin := g.bestPathNeighbor(4, -1)
		if pmin != 2 {
			t.Errorf("expected pmin=2, got %d", pmin)
		}
		if nc != 4 || nr != 0 {
			t.Errorf("expected (4,0), got (%d,%d)", nc, nr)
		}
	})

	t.Run("ValidC_InvalidHighR", func(t *testing.T) {
		// c=4 (valid), r=PathRows → r<PathRows is false.
		g := newAIGame()
		g.resetPathGrid()
		g.Path[4][PathRows-1] = 3
		nc, nr, pmin := g.bestPathNeighbor(4, PathRows)
		if pmin != 3 {
			t.Errorf("expected pmin=3, got %d", pmin)
		}
		if nc != 4 || nr != PathRows-1 {
			t.Errorf("expected (4,%d), got (%d,%d)", PathRows-1, nc, nr)
		}
	})
}

// ===========================================================================
// Retarget
// ===========================================================================
//
// Finds the nearest valid target for a thug.  Valid targets are:
//   - Friendly (Player|Waif) with X < FinishLine (still in play)
//   - Thug when g.Playing is false (thugs hunt each other in demo mode)

func TestRetarget(t *testing.T) {
	t.Run("NoObjects", func(t *testing.T) {
		g := newAIGame()
		p := newAIObj(50, 50)
		p.Kind = Thug
		if g.Retarget(p) != nil {
			t.Error("expected nil with no objects")
		}
	})

	t.Run("SkipSelf", func(t *testing.T) {
		g := newAIGame()
		p := newAIObj(50, 50)
		p.Kind = Thug
		g.Objects = []*GameObject{p}
		if g.Retarget(p) != nil {
			t.Error("should skip self")
		}
	})

	t.Run("NearestFriendly", func(t *testing.T) {
		// Two friendlies at different distances → picks the closest.
		g := newAIGame()
		p := newAIObj(50, 50)
		p.Kind = Thug
		near := newAIObj(55, 50)
		near.Kind = Player
		far := newAIObj(100, 50)
		far.Kind = Waif
		g.Objects = []*GameObject{p, far, near}
		got := g.Retarget(p)
		if got != near {
			t.Error("should pick nearest friendly")
		}
	})

	t.Run("FriendlyPastFinishLine", func(t *testing.T) {
		// Friendly past FinishLine is not a valid target.
		g := newAIGame()
		p := newAIObj(50, 50)
		p.Kind = Thug
		scored := newAIObj(FinishLine+10, 50)
		scored.Kind = Waif
		g.Objects = []*GameObject{p, scored}
		if g.Retarget(p) != nil {
			t.Error("should not target friendly past FinishLine")
		}
	})

	t.Run("ThugTargetsThugWhenNotPlaying", func(t *testing.T) {
		// When g.Playing is false, thugs target other thugs (demo mode).
		g := newAIGame()
		g.Playing = false
		p := newAIObj(50, 50)
		p.Kind = Thug
		other := newAIObj(60, 50)
		other.Kind = Thug
		g.Objects = []*GameObject{p, other}
		got := g.Retarget(p)
		if got != other {
			t.Error("thug should target other thug when not playing")
		}
	})

	t.Run("ThugIgnoredWhenPlaying", func(t *testing.T) {
		// When g.Playing is true, thugs don't target other thugs.
		g := newAIGame()
		g.Playing = true
		p := newAIObj(50, 50)
		p.Kind = Thug
		other := newAIObj(60, 50)
		other.Kind = Thug
		g.Objects = []*GameObject{p, other}
		if g.Retarget(p) != nil {
			t.Error("thug should not target other thug when playing")
		}
	})

	t.Run("CloserSeenFirst", func(t *testing.T) {
		// Closer friendly appears first in g.Objects, so the farther
		// one is processed second and fails the d < minDist check.
		// Covers the false branch of d < minDist (line 93).
		g := newAIGame()
		p := newAIObj(50, 50)
		p.Kind = Thug
		near := newAIObj(55, 50)
		near.Kind = Player
		far := newAIObj(100, 50)
		far.Kind = Waif
		// near BEFORE far in the slice
		g.Objects = []*GameObject{p, near, far}
		got := g.Retarget(p)
		if got != near {
			t.Error("should still pick nearest friendly")
		}
	})
}

// ===========================================================================
// AddCrumb / ReapCrumb
// ===========================================================================
//
// Breadcrumbs form a linked list (Last pointers) that thugs follow when
// stuck.  AddCrumb appends if the player has moved far enough from the
// last crumb.  ReapCrumb removes the oldest and marks it Old so any
// thug targeting it will re-acquire.

func TestAddCrumb(t *testing.T) {
	t.Run("FirstCrumb", func(t *testing.T) {
		// Empty trail → creates the first crumb with ID=1, no Last.
		g := newAIGame()
		p := newAIObj(10, 10)
		g.AddCrumb(p)
		if len(g.Crumbs) != 1 {
			t.Fatalf("expected 1 crumb, got %d", len(g.Crumbs))
		}
		if g.Crumbs[0].ID != 1 {
			t.Errorf("first crumb ID = %d, want 1", g.Crumbs[0].ID)
		}
		if g.Crumbs[0].Last != nil {
			t.Error("first crumb should have nil Last")
		}
	})

	t.Run("TooClose", func(t *testing.T) {
		// Player hasn't moved far enough (dx²+dy² ≤ CrumbMinDist) → no new crumb.
		g := newAIGame()
		p := newAIObj(10, 10)
		g.AddCrumb(p) // first crumb at (10,10)
		p.X = 12      // distance² = 4, well under threshold
		g.AddCrumb(p)
		if len(g.Crumbs) != 1 {
			t.Errorf("expected 1 crumb (too close), got %d", len(g.Crumbs))
		}
	})

	t.Run("FarEnough", func(t *testing.T) {
		// Player moved far enough (dx²+dy² > CrumbMinDist) → new crumb
		// added with Last pointing to previous crumb.
		g := newAIGame()
		p := newAIObj(10, 10)
		g.AddCrumb(p)
		p.X = 20 // distance² = 100, well over threshold
		g.AddCrumb(p)
		if len(g.Crumbs) != 2 {
			t.Fatalf("expected 2 crumbs, got %d", len(g.Crumbs))
		}
		if g.Crumbs[1].Last != g.Crumbs[0] {
			t.Error("second crumb should link to first")
		}
		if g.Crumbs[1].ID != 2 {
			t.Errorf("second crumb ID = %d, want 2", g.Crumbs[1].ID)
		}
	})

	t.Run("OverflowTrimsOldest", func(t *testing.T) {
		// With > MaxCrumbs crumbs, AddCrumb calls ReapCrumb to remove
		// the oldest before appending the new one.
		g := newAIGame()
		for i := 0; i < MaxCrumbs+1; i++ {
			p := newAIObj(float64(i*10), 0)
			g.AddCrumb(p)
		}
		if len(g.Crumbs) != MaxCrumbs+1 {
			t.Fatalf("expected %d crumbs, got %d", MaxCrumbs+1, len(g.Crumbs))
		}
		// Adding one more should trigger reap then add.
		p := newAIObj(200, 0)
		g.AddCrumb(p)
		if len(g.Crumbs) != MaxCrumbs+1 {
			t.Errorf("expected %d crumbs after overflow trim, got %d", MaxCrumbs+1, len(g.Crumbs))
		}
	})
}

func TestReapCrumb(t *testing.T) {
	t.Run("EmptyList", func(t *testing.T) {
		// Reaping an empty list should not panic.
		g := newAIGame()
		g.ReapCrumb() // should be a no-op
		if len(g.Crumbs) != 0 {
			t.Error("should remain empty")
		}
	})

	t.Run("SingleCrumb", func(t *testing.T) {
		// Reaping a single crumb marks it Old and removes it.
		g := newAIGame()
		c := &Crumb{X: 10, Y: 10, ID: 1}
		g.Crumbs = []*Crumb{c}
		g.ReapCrumb()
		if !c.Old {
			t.Error("reaped crumb should be marked Old")
		}
		if len(g.Crumbs) != 0 {
			t.Errorf("expected 0 crumbs, got %d", len(g.Crumbs))
		}
	})

	t.Run("MultipleCrumbs", func(t *testing.T) {
		// Reaping with multiple crumbs: oldest is removed and marked Old,
		// the second crumb's Last pointer is set to nil (unlinked).
		g := newAIGame()
		c0 := &Crumb{X: 0, Y: 0, ID: 1}
		c1 := &Crumb{X: 10, Y: 10, Last: c0, ID: 2}
		c2 := &Crumb{X: 20, Y: 20, Last: c1, ID: 3}
		g.Crumbs = []*Crumb{c0, c1, c2}
		g.ReapCrumb()
		if !c0.Old {
			t.Error("reaped crumb should be marked Old")
		}
		if len(g.Crumbs) != 2 {
			t.Fatalf("expected 2 crumbs, got %d", len(g.Crumbs))
		}
		if g.Crumbs[0] != c1 {
			t.Error("c1 should be the new head")
		}
		if c1.Last != nil {
			t.Error("new head's Last should be nil after reap")
		}
	})
}

// ===========================================================================
// followCrumbs
// ===========================================================================
//
// When a thug is stuck (Follow returns 0), it falls back to following the
// player's breadcrumb trail.  Walks backward through the Last chain until
// it finds a crumb it can move toward.

func TestFollowCrumbs(t *testing.T) {
	t.Run("NoExistingCrumb", func(t *testing.T) {
		// p.TargetCrumb is nil → starts from the last crumb in the trail.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		p := newAIObj(8, 8)
		p.Kind = Thug
		c := &Crumb{X: 80, Y: 80}
		g.Crumbs = []*Crumb{c}
		ret, tc := g.followCrumbs(p)
		if ret == 0 {
			t.Error("should find a direction toward the crumb")
		}
		if tc != c {
			t.Error("should return the crumb being followed")
		}
	})

	t.Run("ExistingCrumb_FollowLast", func(t *testing.T) {
		// p.TargetCrumb is set → follow its Last pointer.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		p := newAIObj(8, 8)
		p.Kind = Thug
		c0 := &Crumb{X: 80, Y: 80}
		c1 := &Crumb{X: 50, Y: 50, Last: c0}
		p.TargetCrumb = c1
		g.Crumbs = []*Crumb{c0, c1}
		ret, tc := g.followCrumbs(p)
		if ret == 0 {
			t.Error("should find direction toward c0")
		}
		if tc != c0 {
			t.Error("should return c0 (c1's Last)")
		}
	})

	t.Run("AllBlocked", func(t *testing.T) {
		// All crumbs are at the same position as the thug → no movement.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		p := newAIObj(8, 8)
		p.Kind = Thug
		c := &Crumb{X: 8, Y: 8}
		g.Crumbs = []*Crumb{c}
		ret, tc := g.followCrumbs(p)
		if ret != 0 {
			t.Errorf("expected 0 (no movement possible), got %d", ret)
		}
		if tc != nil {
			t.Error("should return nil when all crumbs exhausted")
		}
	})

	t.Run("WalkChain", func(t *testing.T) {
		// First crumb in chain is at thug's position (no movement),
		// but its Last crumb is reachable → follows the chain.
		// Covers the true branch of c.Last != nil in the for loop.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		p := newAIObj(8, 8)
		p.Kind = Thug
		reachable := &Crumb{X: 80, Y: 80}
		blocked := &Crumb{X: 8, Y: 8, Last: reachable}
		g.Crumbs = []*Crumb{reachable, blocked}
		ret, tc := g.followCrumbs(p)
		if ret == 0 {
			t.Error("should find direction via chain")
		}
		if tc != reachable {
			t.Error("should return the reachable crumb in the chain")
		}
	})
}

// ===========================================================================
// Follow
// ===========================================================================
//
// Main thug AI controller.  Targets the nearest friendly, follows it,
// and falls back to breadcrumbs when stuck.  Handles crumb arrival
// (recursive retarget) and waif spotting (PlaySfx).

func TestFollow(t *testing.T) {
	t.Run("NoTarget", func(t *testing.T) {
		// No friendlies and no crumb target → returns 0 (idle).
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		p := newAIObj(50, 50)
		p.Kind = Thug
		g.Objects = []*GameObject{p}
		ret := Follow(g, p)
		if ret != 0 {
			t.Errorf("expected 0 (no target), got %d", ret)
		}
	})

	t.Run("FollowPlayer", func(t *testing.T) {
		// Thug targets a player to the right → Follow returns DirRight.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		thug := newAIObj(8, 8)
		thug.Kind = Thug
		player := newAIObj(80, 8)
		player.Kind = Player
		g.Objects = []*GameObject{thug, player}
		ret := Follow(g, thug)
		if ret&DirRight == 0 {
			t.Errorf("expected DirRight, got %d", ret)
		}
		if thug.Target != player {
			t.Error("thug should target the player")
		}
	})

	t.Run("SpottedWaif", func(t *testing.T) {
		// Thug acquires an unspotted waif → PlaySfx called, Spotted set.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		thug := newAIObj(8, 8)
		thug.Kind = Thug
		waif := newAIObj(80, 8)
		waif.Kind = Waif
		g.Objects = []*GameObject{thug, waif}

		var sfxPlayed bool
		patches := gomonkey.ApplyMethodFunc(g, "PlaySfx", func(_ interface{}, _ string) {
			sfxPlayed = true
		})
		defer patches.Reset()

		Follow(g, thug)
		if !waif.Spotted {
			t.Error("waif should be marked Spotted")
		}
		if !sfxPlayed {
			t.Error("PlaySfx should be called for spotted waif")
		}
	})

	t.Run("AlreadySpotted", func(t *testing.T) {
		// Waif already spotted → PlaySfx NOT called again.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		thug := newAIObj(8, 8)
		thug.Kind = Thug
		waif := newAIObj(80, 8)
		waif.Kind = Waif
		waif.Spotted = true
		g.Objects = []*GameObject{thug, waif}

		patches := gomonkey.ApplyMethodFunc(g, "PlaySfx", func(_ interface{}, _ string) {
			t.Error("PlaySfx should not be called for already-spotted waif")
		})
		defer patches.Reset()

		Follow(g, thug)
	})

	t.Run("CrumbTargetReaped", func(t *testing.T) {
		// TargetCrumb is set but marked Old (reaped) → clears crumb,
		// retargets to nearest friendly.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		thug := newAIObj(8, 8)
		thug.Kind = Thug
		thug.TargetCrumb = &Crumb{X: 50, Y: 50, Old: true}
		player := newAIObj(80, 8)
		player.Kind = Player
		g.Objects = []*GameObject{thug, player}
		Follow(g, thug)
		if thug.TargetCrumb != nil {
			t.Error("old crumb should be cleared")
		}
		if thug.Target != player {
			t.Error("should retarget to player")
		}
	})

	t.Run("FollowCrumb", func(t *testing.T) {
		// Thug has a valid TargetCrumb → follows it instead of retargeting.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		thug := newAIObj(8, 8)
		thug.Kind = Thug
		crumb := &Crumb{X: 80, Y: 80}
		thug.TargetCrumb = crumb
		g.Objects = []*GameObject{thug}
		ret := Follow(g, thug)
		if ret == 0 {
			t.Error("should follow crumb direction")
		}
	})

	t.Run("ArrivedAtCrumb_Recursive", func(t *testing.T) {
		// Thug arrives at its crumb target (same position, Last != nil) →
		// Follow clears crumb and recurses to retarget.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		thug := newAIObj(8, 8)
		thug.Kind = Thug
		parentCrumb := &Crumb{X: 80, Y: 80}
		crumb := &Crumb{X: 8, Y: 8, Last: parentCrumb}
		thug.TargetCrumb = crumb
		player := newAIObj(80, 8)
		player.Kind = Player
		g.Objects = []*GameObject{thug, player}
		ret := Follow(g, thug)
		// After arrival, it recurses → retargets to player → follows.
		if ret == 0 {
			t.Error("should follow player after arriving at crumb")
		}
		if thug.TargetCrumb != nil {
			t.Error("crumb should be cleared after arrival")
		}
	})

	t.Run("StuckFallsToCrumbs", func(t *testing.T) {
		// Thug has a target but can't move toward it (e.g. same X and Y
		// as target in integer terms but blocked).  Falls back to
		// breadcrumb trail.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		g.Playing = true
		thug := newAIObj(8, 8)
		thug.Kind = Thug
		// Player at same integer position → followAxis returns 0.
		player := newAIObj(8.5, 8.5)
		player.Kind = Player
		g.Objects = []*GameObject{thug, player}
		// Breadcrumb far away that the thug can follow.
		crumb := &Crumb{X: 80, Y: 80}
		g.Crumbs = []*Crumb{crumb}
		ret := Follow(g, thug)
		if ret == 0 {
			t.Error("should follow breadcrumbs when stuck")
		}
		if thug.TargetCrumb != crumb {
			t.Error("should set TargetCrumb to the crumb being followed")
		}
	})

	t.Run("StuckNoCrumbs", func(t *testing.T) {
		// Thug is stuck (ret==0) but no crumbs available →
		// the crumb fallback is skipped (len(g.Crumbs) == 0).
		// Covers the false branch of len(g.Crumbs) > 0.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		g.Playing = true
		thug := newAIObj(8, 8)
		thug.Kind = Thug
		player := newAIObj(8.5, 8.5)
		player.Kind = Player
		g.Objects = []*GameObject{thug, player}
		// No crumbs at all.
		ret := Follow(g, thug)
		if ret != 0 {
			t.Errorf("expected 0 (stuck, no crumbs), got %d", ret)
		}
	})

	t.Run("NotStuck_SkipsCrumbs", func(t *testing.T) {
		// Thug has a clear path to the target (ret != 0) → the crumb
		// fallback is not entered.  Covers the false branch of ret==0.
		g := newAIGame()
		g.TileMap = newTileMap(20, 20, 0)
		g.Playing = true
		thug := newAIObj(8, 8)
		thug.Kind = Thug
		player := newAIObj(80, 8)
		player.Kind = Player
		g.Objects = []*GameObject{thug, player}
		g.Crumbs = []*Crumb{{X: 50, Y: 50}}
		ret := Follow(g, thug)
		if ret == 0 {
			t.Error("should not be stuck")
		}
		if thug.TargetCrumb != nil {
			t.Error("should not set TargetCrumb when not stuck")
		}
	})
}

// ===========================================================================
// FollowPath
// ===========================================================================
//
// Main waif AI controller.  Uses the BFS path grid to navigate toward
// the player.  Has special cases for: past FinishLine, not playing,
// no player, far from player, and jittering when close.

func TestFollowPath(t *testing.T) {
	t.Run("PastFinishLine", func(t *testing.T) {
		// Waif past FinishLine → always move right (escaping).
		g := newAIGame()
		p := newAIObj(FinishLine+1, 80)
		ret := FollowPath(g, p)
		if ret != DirRight {
			t.Errorf("expected DirRight, got %d", ret)
		}
	})

	t.Run("Won", func(t *testing.T) {
		// Game won → move right regardless of position.
		g := newAIGame()
		g.Won = true
		p := newAIObj(100, 80)
		ret := FollowPath(g, p)
		if ret != DirRight {
			t.Errorf("expected DirRight when won, got %d", ret)
		}
	})

	t.Run("NotPlaying_Above80", func(t *testing.T) {
		// Not playing, py > 80 → DirRight | DirUp (center toward Y=80).
		g := newAIGame()
		g.Playing = false
		p := newAIObj(100, 81)
		ret := FollowPath(g, p)
		if ret != DirRight|DirUp {
			t.Errorf("expected DirRight|DirUp, got %d", ret)
		}
	})

	t.Run("NotPlaying_Below78", func(t *testing.T) {
		// Not playing, py < 78 → DirRight | DirDown.
		g := newAIGame()
		g.Playing = false
		p := newAIObj(100, 77)
		ret := FollowPath(g, p)
		if ret != DirRight|DirDown {
			t.Errorf("expected DirRight|DirDown, got %d", ret)
		}
	})

	t.Run("NotPlaying_Centered", func(t *testing.T) {
		// Not playing, py between 78 and 80 → just DirRight.
		g := newAIGame()
		g.Playing = false
		p := newAIObj(100, 79)
		ret := FollowPath(g, p)
		if ret != DirRight {
			t.Errorf("expected DirRight when centered, got %d", ret)
		}
	})

	t.Run("NoPlayer", func(t *testing.T) {
		// Playing but no player object → DirRight.
		g := newAIGame()
		g.Playing = true
		g.Player = nil
		p := newAIObj(100, 80)
		ret := FollowPath(g, p)
		if ret != DirRight {
			t.Errorf("expected DirRight with no player, got %d", ret)
		}
	})

	t.Run("FarFromPlayer", func(t *testing.T) {
		// Player is more than GameWidth pixels away → uses simple
		// dirToward instead of path grid.
		g := newAIGame()
		g.Playing = true
		g.Player = newAIObj(500, 50)
		p := newAIObj(100, 80)
		ret := FollowPath(g, p)
		if ret&DirRight == 0 {
			t.Error("should head right toward distant player")
		}
		if ret != p.Dot {
			t.Error("Dot should be set to the return value")
		}
	})

	t.Run("NearPlayer_PathGrid", func(t *testing.T) {
		// Player is within GameWidth → uses bestPathNeighbor for
		// navigation.  UpdatePath must be called first to populate grid.
		g := newAIGame()
		g.TileMap = newTileMap(40, 20, 0)
		g.Playing = true
		g.Player = newAIObj(80, 80)
		g.UpdatePath(g.Player)
		p := newAIObj(50, 80)
		ret := FollowPath(g, p)
		// Should produce some direction toward the player.
		if ret == 0 {
			t.Error("should produce a direction toward player via path grid")
		}
	})

	t.Run("Jitter", func(t *testing.T) {
		// When very close to player (pmin <= 1), waif applies random
		// jitter to avoid stacking on the player.
		g := newAIGame()
		g.TileMap = newTileMap(40, 20, 0)
		g.Playing = true
		g.WaifJitterPx = 5
		g.WaifJitterF = 10
		g.Player = newAIObj(80, 80)
		g.UpdatePath(g.Player)
		// Place waif on same tile as player.
		p := newAIObj(80, 80)
		p.JitterTime = 0
		FollowPath(g, p)
		if p.JitterTime == 0 {
			t.Error("JitterTime should be set when close to player")
		}
	})

	t.Run("JitterCountdown", func(t *testing.T) {
		// JitterTime > 0 → decrements instead of rerolling.
		g := newAIGame()
		g.TileMap = newTileMap(40, 20, 0)
		g.Playing = true
		g.WaifJitterPx = 5
		g.WaifJitterF = 10
		g.Player = newAIObj(80, 80)
		g.UpdatePath(g.Player)
		p := newAIObj(80, 80)
		p.JitterTime = 5
		p.JitterX = 3
		p.JitterY = -2
		FollowPath(g, p)
		if p.JitterTime != 4 {
			t.Errorf("JitterTime = %d, want 4 (decremented)", p.JitterTime)
		}
		// JitterX/Y should be unchanged (not rerolled).
		if p.JitterX != 3 || p.JitterY != -2 {
			t.Error("jitter values should not change during countdown")
		}
	})
}

// ===========================================================================
// PlayerPostMove / WaifPostMove
// ===========================================================================
//
// Post-movement hooks called after each step.
// PlayerPostMove updates the path grid and drops a breadcrumb.
// WaifPostMove handles scoring, reaping past XMax, and speed matching.

func TestPlayerPostMove(t *testing.T) {
	// PlayerPostMove calls UpdatePath + AddCrumb.  Verify both effects.
	g := newAIGame()
	g.TileMap = newTileMap(40, 20, 0)
	p := newAIObj(16, 16)
	PlayerPostMove(g, p)
	if p.PathC == 0 && p.PathR == 0 {
		t.Error("UpdatePath should set PathC/PathR")
	}
	if len(g.Crumbs) != 1 {
		t.Errorf("AddCrumb should add a crumb, got %d", len(g.Crumbs))
	}
}

func TestWaifPostMove(t *testing.T) {
	t.Run("Score", func(t *testing.T) {
		// Waif past FinishLine, playing, not scored → earns points.
		g := newAIGame()
		g.Playing = true
		g.WaifSafeScore = 100
		waif := newAIObj(FinishLine+1, 50)
		waif.Kind = Waif

		var sfxPlayed bool
		patches := gomonkey.ApplyMethodFunc(g, "PlaySfx", func(_ interface{}, _ string) {
			sfxPlayed = true
		})
		defer patches.Reset()

		WaifPostMove(g, waif)
		if g.Score != 100 {
			t.Errorf("Score = %d, want 100", g.Score)
		}
		if !waif.Scored {
			t.Error("waif should be marked Scored")
		}
		if !waif.Ignore {
			t.Error("scored waif should be Ignored (ghost mode)")
		}
		if !sfxPlayed {
			t.Error("safe sound should play")
		}
	})

	t.Run("ScoreWhenWon", func(t *testing.T) {
		// Waif past FinishLine, game won (not playing) → still scores.
		g := newAIGame()
		g.Won = true
		g.WaifSafeScore = 50
		waif := newAIObj(FinishLine+1, 50)
		waif.Kind = Waif

		patches := gomonkey.ApplyMethodFunc(g, "PlaySfx", func(_ interface{}, _ string) {})
		defer patches.Reset()

		WaifPostMove(g, waif)
		if g.Score != 50 {
			t.Errorf("Score = %d, want 50", g.Score)
		}
	})

	t.Run("AlreadyScored", func(t *testing.T) {
		// Already scored → no double scoring.
		g := newAIGame()
		g.Playing = true
		g.WaifSafeScore = 100
		waif := newAIObj(FinishLine+1, 50)
		waif.Kind = Waif
		waif.Scored = true
		WaifPostMove(g, waif)
		if g.Score != 0 {
			t.Error("should not double-score")
		}
	})

	t.Run("PastXMax_Reaped", func(t *testing.T) {
		// Waif past XMax → reaped from object list.
		g := newAIGame()
		waif := newAIObj(XMax+1, 50)
		waif.Kind = Waif
		g.Objects = []*GameObject{waif}

		var reaped bool
		patches := gomonkey.ApplyMethodFunc(g, "Reap", func(_ *GameObject) {
			reaped = true
		})
		defer patches.Reset()

		WaifPostMove(g, waif)
		if !reaped {
			t.Error("waif past XMax should be reaped")
		}
	})

	t.Run("SpeedMatch", func(t *testing.T) {
		// Waif close to player → matches player speed.
		g := newAIGame()
		g.WaifFollowPx = 50
		player := newAIObj(100, 50)
		player.Kind = Player
		player.Speed = 2
		g.Player = player
		waif := newAIObj(110, 50)
		waif.Kind = Waif
		waif.Speed = 1
		WaifPostMove(g, waif)
		if waif.Speed != 2 {
			t.Errorf("waif Speed = %.1f, want 2 (matched player)", waif.Speed)
		}
	})

	t.Run("SpeedMatch_PlayerStopped", func(t *testing.T) {
		// Player speed is 0 → waif does NOT match (guard: Speed > 0).
		g := newAIGame()
		g.WaifFollowPx = 50
		player := newAIObj(100, 50)
		player.Kind = Player
		player.Speed = 0
		g.Player = player
		waif := newAIObj(110, 50)
		waif.Kind = Waif
		waif.Speed = 1
		WaifPostMove(g, waif)
		if waif.Speed != 1 {
			t.Errorf("waif Speed = %.1f, want 1 (player stopped)", waif.Speed)
		}
	})

	t.Run("TooFarForSpeedMatch", func(t *testing.T) {
		// Waif too far from player → speed unchanged.
		g := newAIGame()
		g.WaifFollowPx = 10
		player := newAIObj(100, 50)
		player.Kind = Player
		player.Speed = 3
		g.Player = player
		waif := newAIObj(200, 50)
		waif.Kind = Waif
		waif.Speed = 1
		WaifPostMove(g, waif)
		if waif.Speed != 1 {
			t.Errorf("waif Speed = %.1f, want 1 (too far)", waif.Speed)
		}
	})
}
