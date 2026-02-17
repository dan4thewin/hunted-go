package main

import (
	"fmt"
	"math"
	"strconv"

	rl "github.com/gen2brain/raylib-go/raylib"
)

var (
	PicoBlack      = rl.Color{R: 0, G: 0, B: 0, A: 255}
	PicoDarkBlue   = rl.Color{R: 29, G: 43, B: 83, A: 255}
	PicoDarkPurple = rl.Color{R: 126, G: 37, B: 83, A: 255}
	PicoDarkGreen  = rl.Color{R: 0, G: 135, B: 81, A: 255}
	PicoBrown      = rl.Color{R: 171, G: 82, B: 54, A: 255}
	PicoDarkGrey   = rl.Color{R: 95, G: 87, B: 79, A: 255}
	PicoLightGrey  = rl.Color{R: 194, G: 195, B: 199, A: 255}
	PicoWhite      = rl.Color{R: 255, G: 241, B: 232, A: 255}
	PicoRed        = rl.Color{R: 255, G: 0, B: 77, A: 255}
	PicoOrange     = rl.Color{R: 255, G: 163, B: 0, A: 255}
	PicoYellow     = rl.Color{R: 255, G: 236, B: 39, A: 255}
	PicoGreen      = rl.Color{R: 0, G: 228, B: 54, A: 255}
	PicoBlue       = rl.Color{R: 41, G: 173, B: 255, A: 255}
	PicoIndigo     = rl.Color{R: 131, G: 118, B: 156, A: 255}
	PicoPink       = rl.Color{R: 255, G: 119, B: 168, A: 255}
	PicoPeach      = rl.Color{R: 255, G: 204, B: 170, A: 255}

	// UI overlay colors
	OverlayDim      = rl.Color{R: 0, G: 0, B: 0, A: 200}
	OverlayDark     = rl.Color{R: 0, G: 0, B: 0, A: 220}
	HighlightOrange = rl.Color{R: 255, G: 163, B: 0, A: 40}
	SliderTrack     = rl.Color{R: 80, G: 80, B: 80, A: 255}

	// Pico8Colors indexed palette for dynamic lookups
	Pico8Colors = [16]rl.Color{
		PicoBlack, PicoDarkBlue, PicoDarkPurple, PicoDarkGreen,
		PicoBrown, PicoDarkGrey, PicoLightGrey, PicoWhite,
		PicoRed, PicoOrange, PicoYellow, PicoGreen,
		PicoBlue, PicoIndigo, PicoPink, PicoPeach,
	}
)

func (g *Game) Draw() {
	rl.BeginDrawing()
	rl.ClearBackground(rl.Black)

	switch g.State {
	case StateTitle:
		// Draw map tiles from column 82, row 3 as background (matching Lua's map(82,3))
		const titleMapX = 82 * TileSize // world x = 656
		const titleMapY = 3 * TileSize  // world y = 24
		cam := rl.Camera2D{
			Offset:   rl.Vector2{X: float32(g.WindowSize) / 2, Y: float32(g.WindowSize) / 2},
			Target:   rl.Vector2{X: titleMapX + VisibleSize/2, Y: titleMapY + VisibleSize/2},
			Rotation: 0,
			Zoom:     float32(g.WindowSize) / VisibleSize,
		}
		rl.BeginMode2D(cam)
		g.DrawTiles()
		// Oversized sprites at screen-relative positions within the visible area
		g.DrawSpriteScaled(205, titleMapX+34+24, titleMapY+28-8, 3, 3)
		g.DrawSpriteScaled(175, titleMapX+62+24, titleMapY+38-8, 1, 2)
		rl.EndMode2D()
		// Title text overlay in screen coordinates
		g.DrawTitle()
	case StatePlaying, StateGameOver:
		// Update camera target with shake effect
		shakeX := float32(0)
		shakeY := float32(0)
		if g.ShakeFactor > 0 && g.Rng != nil && !g.Paused {
			shakeX = float32((g.Rng.Float64()*2 - 1) * g.ShakeFactor)
			shakeY = float32((g.Rng.Float64()*2 - 1) * g.ShakeFactor)
		}
		g.Camera.Target = rl.Vector2{
			X: float32(g.CamX) + VisibleSize/2 + shakeX,
			Y: float32(g.CamY) + VisibleSize/2 + shakeY,
		}

		rl.BeginMode2D(g.Camera)
		g.DrawGame()
		rl.EndMode2D()

		// Draw UI outside of camera mode (screen space)
		g.DrawUI()

		if g.State == StateGameOver {
			g.DrawGameOver()
		}
	}

	if g.Paused {
		g.DrawOptions()
	}

	if g.ShowHelp {
		g.DrawHelp()
	}

	if g.ShowCredits {
		g.DrawCredits()
	}

	rl.EndDrawing()
}

func (g *Game) DrawTiles() {
	for row := 0; row < len(g.TileMap); row++ {
		for col := 0; col < len(g.TileMap[row]); col++ {
			tile := g.TileMap[row][col]
			if tile > 0 {
				worldX := float32(col * TileSize)
				worldY := float32(row * TileSize)
				g.DrawSprite(tile, worldX, worldY, false)
			}
		}
	}
}

func (g *Game) DrawSpriteScaled(index int, x, y float32, w, h int) {
	if index < 0 || index >= len(g.SpriteRects) {
		return
	}
	// Source rect: expand from the sprite's position to cover w x h tiles in the spritesheet
	src := g.SpriteRects[index]
	src.Width = float32(w * TileSize)
	src.Height = float32(h * TileSize)
	dest := rl.Rectangle{
		X:      x,
		Y:      y,
		Width:  float32(w * TileSize),
		Height: float32(h * TileSize),
	}
	rl.DrawTexturePro(g.Spritesheet, src, dest, rl.Vector2{}, 0, rl.White)
}

func (g *Game) DrawGame() {
	g.DrawTiles()

	// Debug: draw pathfinding grid (debug&8)
	if g.Debug&8 != 0 {
		g.DrawPathGrid()
	}

	// Debug: draw breadcrumb trail (debug&2)
	if g.Debug&2 != 0 {
		g.DrawCrumbs()
	}

	// Draw objects (back to front)
	for i := len(g.Objects) - 1; i >= 0; i-- {
		obj := g.Objects[i]
		worldX := float32(obj.X)
		worldY := float32(obj.Y)

		sprite := obj.CurrentSprite
		flip := obj.Flip
		if sprite < 0 {
			sprite = -sprite
			flip = !flip
		}

		// Flash through lighter shades during invulnerability using shader
		if obj.IsInvuln {
			// Cycle brightness: use sine wave for smooth transition 0->1->0
			phase := float32(obj.Delay) * 0.15
			brightness := float32(math.Abs(math.Sin(float64(phase))))
			rl.BeginShaderMode(g.InvulnShader)
			rl.SetShaderValue(g.InvulnShader, g.BrightnessLoc, []float32{brightness}, rl.ShaderUniformFloat)
			g.DrawSprite(sprite, worldX, worldY, flip)
			rl.EndShaderMode()
		} else if obj.PalSwap {
			// Use palette swap shader for waifs - each waif has a random PalIndex (0, 1, or 2)
			rl.BeginShaderMode(g.PaletteSwapShader)
			rl.SetShaderValue(g.PaletteSwapShader, g.PaletteIndexLoc, []float32{float32(obj.PalIndex)}, rl.ShaderUniformFloat)
			g.DrawSprite(sprite, worldX, worldY, flip)
			rl.EndShaderMode()
		} else {
			g.DrawSprite(sprite, worldX, worldY, flip)
		}

		// Debug: show direction dot (debug&2)
		if g.Debug&2 != 0 {
			g.DrawDirection(obj)
			g.DrawTargetLine(obj)
		}

		// Debug: show bounding box (debug&4)
		if g.Debug&4 != 0 {
			g.DrawBoundsRect(obj)
		}
	}

	// Draw sparks
	for _, s := range g.Sparks {
		// Randomized orange-yellow for spark shimmer
		color := rl.Color{R: 255, G: uint8(200 + g.Rng.Intn(55)), B: 0, A: 255}
		rl.DrawRectangle(int32(s.X), int32(s.Y), 2, 2, color)
	}

	// Draw pings
	for _, p := range g.Pings {
		rl.DrawCircleLines(int32(p.X), int32(p.Y), float32(p.Radius), p.Color)
	}
}

func (g *Game) DrawCrumbs() {
	for _, c := range g.Crumbs {
		rl.DrawPixel(int32(c.X), int32(c.Y), PicoOrange)
	}
}

func (g *Game) DrawDirection(obj *GameObject) {
	if obj.Dot != 0 {
		c := obj.Dot
		if c < 0 || c > 15 {
			c = 7 // white fallback
		}
		rl.DrawPixel(int32(obj.X)+3, int32(obj.Y)+3, Pico8Colors[c])
	}
}

func (g *Game) DrawTargetLine(obj *GameObject) {
	if obj.Kind != Thug {
		return
	}
	ox, oy := int32(obj.X), int32(obj.Y)
	if obj.TargetCrumb != nil {
		rl.DrawLine(ox, oy, int32(obj.TargetCrumb.X), int32(obj.TargetCrumb.Y), PicoLightGrey)
	}
	if obj.Target != nil {
		color := PicoBlue // blue default
		switch obj.Target.Kind {
		case Player:
			color = PicoRed // red for player
		case Waif:
			color = PicoPink // pink for waif
		}
		rl.DrawLine(ox, oy, int32(obj.Target.X), int32(obj.Target.Y), color)
	}
}

func (g *Game) DrawBoundsRect(obj *GameObject) {
	bx, by, bx2, by2 := obj.Bounds(nil, nil)
	rl.DrawRectangleLines(int32(bx), int32(by), int32(bx2-bx+1), int32(by2-by+1), PicoDarkGreen)
}

func (g *Game) DrawPathGrid() {
	if g.Player == nil {
		return
	}
	tx := math.Floor(g.Player.X) + float64(g.Player.XOff)
	mapc := int(math.Floor(max(0, tx-ScrollWidth) / TileSize))
	camCol := int(math.Floor((g.CamX - float64(mapc*TileSize)) / TileSize))

	for i := 0; i < 16; i++ {
		for j := 0; j < 20; j++ {
			x := int32(g.CamX) + int32(i*TileSize)
			y := int32(j * TileSize)
			rl.DrawRectangleLines(x, y, TileSize, TileSize, PicoDarkBlue)

			pathCol := camCol + i + 1 // 1-based, matches Lua: path[c+i]
			if pathCol >= 1 && pathCol <= 32 && j+1 >= 1 && j+1 <= 20 {
				val := g.Path[pathCol-1][j] // Go is 0-based
				if val < MaxN {
					// Draw small number
					txt := strconv.Itoa(val)
					rl.DrawText(txt, x+1, y+1, 5, PicoOrange)
				}
			}
		}
	}
}

func (g *Game) DrawSprite(index int, x, y float32, flip bool) {
	if index < 0 || index >= len(g.SpriteRects) {
		return
	}

	src := g.SpriteRects[index]
	dest := rl.Rectangle{
		X:      x,
		Y:      y,
		Width:  TileSize,
		Height: TileSize,
	}

	if flip {
		src.Width = -src.Width
	}

	rl.DrawTexturePro(g.Spritesheet, src, dest, rl.Vector2{}, 0, rl.White)
}

func (g *Game) DrawSpriteUI(index int, x, y float32) {
	if index < 0 || index >= len(g.SpriteRects) {
		return
	}
	scale := float32(g.WindowSize) / VisibleSize
	src := g.SpriteRects[index]
	dest := rl.Rectangle{
		X:      x * scale,
		Y:      y * scale,
		Width:  float32(TileSize) * scale,
		Height: float32(TileSize) * scale,
	}
	rl.DrawTexturePro(g.Spritesheet, src, dest, rl.Vector2{}, 0, rl.White)
}

func (g *Game) DrawExtraGrid(sx, sy, scale float32, n int) {
	px := scale // one scaled pixel
	for i := 0; i < n; i++ {
		col := float32(i % 3)
		row := float32(i / 3)
		rl.DrawRectangleV(
			rl.Vector2{X: sx + col*2*px, Y: sy + row*2*px},
			rl.Vector2{X: px, Y: px},
			PicoOrange,
		)
	}
}

func (g *Game) DrawUI() {
	scale := float32(g.WindowSize) / VisibleSize
	color := cond(g.Playing, PicoOrange, PicoBrown)

	// Score
	fontSize := float32(TileSize) * scale
	rl.DrawTextEx(g.Font, strconv.Itoa(g.Score), rl.Vector2{X: 2 * scale, Y: 0}, fontSize, 1, color)

	// Hearts
	heartSprite := g.DefHeart.Idle[0]
	for i := 1; i <= 3; i++ {
		x := float32(20 + i*8)
		sprIdx := cond(i < g.HeartCount, heartSprite, heartSprite+1)
		g.DrawSpriteUI(sprIdx, x, -1)
	}
	// Plus sign if hearts > 4
	if g.HeartCount > 4 {
		cx := (float32(20-1+4*8) + 0.5) * scale
		cy := (float32(1) + 0.5) * scale
		rl.DrawCircle(int32(cx), int32(cy), 1.5*scale, rl.White)
	}

	// Bombs
	bombSprite := g.DefBomb.Idle[0]
	cnt := min(g.BombCount, 3)
	for i := 1; i <= cnt; i++ {
		x := float32(54 + i*4)
		g.DrawSpriteUI(bombSprite, x, 0)
	}
	// Extra bombs as 3x3 grid of orange pixels
	if g.BombCount > 3 {
		g.DrawExtraGrid(float32(54+2+4*4)*scale, 1*scale, scale, min(g.BombCount-3, 9))
	}

	// Mines
	mineSprite := g.DefMine.Idle[0]
	cnt = min(g.MineCount, 3)
	for i := 1; i <= cnt; i++ {
		x := float32(72 + i*6)
		g.DrawSpriteUI(mineSprite, x, 0)
	}
	// Extra mines as 3x3 grid of orange pixels
	if g.MineCount > 3 {
		g.DrawExtraGrid(float32(72+4*6)*scale, 1*scale, scale, min(g.MineCount-3, 9))
	}

	// Distance to finish
	dist := max(0, FinishLine-int(g.Player.X))
	rl.DrawTextEx(g.Font, strconv.Itoa(dist), rl.Vector2{X: 104 * scale, Y: 0}, fontSize, 1, color)
	rl.DrawTextEx(g.Font, ">>>", rl.Vector2{X: 117 * scale, Y: 0}, fontSize, 1, color)

	// Debug overlay (debug&1)
	if g.Debug&1 != 0 {
		smallFont := fontSize * 0.75
		// Show object/thug/waif/threat counts
		info := fmt.Sprintf("%d/%d/%d/%.1f", len(g.Objects), g.ThugCount, g.WaifCount, g.Threat)
		rl.DrawTextEx(g.Font, info, rl.Vector2{X: 80 * scale, Y: 9 * scale}, smallFont, 1, color)
		// Show debug message at bottom of screen
		if g.DebugMsg != "" {
			rl.DrawTextEx(g.Font, g.DebugMsg, rl.Vector2{X: 2 * scale, Y: 120 * scale}, smallFont, 1, PicoRed)
		}
		// Show FPS
		fpsText := fmt.Sprintf("fps:%d", rl.GetFPS())
		rl.DrawTextEx(g.Font, fpsText, rl.Vector2{X: 2 * scale, Y: 9 * scale}, smallFont, 1, color)
	}
}

func (g *Game) DrawGameOver() {
	centerX := g.WindowSize/2 + 48
	centerY := g.WindowSize/2 - 48

	if g.Won {
		rl.DrawTextEx(g.Font, "SAFE!", rl.Vector2{X: float32(centerX - 100), Y: float32(centerY - 40)}, 80, 2, rl.Green)
	} else {
		rl.DrawTextEx(g.Font, "GAME OVER", rl.Vector2{X: float32(centerX - 200), Y: float32(centerY - 40)}, 80, 2, rl.Red)
	}

	rl.DrawTextEx(g.Font, fmt.Sprintf("Score: %d", g.Score), rl.Vector2{X: float32(centerX - 120), Y: float32(centerY + 60)}, 40, 2, PicoOrange)
	rl.DrawTextEx(g.Font, fmt.Sprintf("High Score: %d", g.HighScore), rl.Vector2{X: float32(centerX - 160), Y: float32(centerY + 120)}, 40, 2, PicoOrange)
	if g.Won {
		rl.DrawTextEx(g.Font, "Press X to continue", rl.Vector2{X: float32(centerX - 280), Y: float32(centerY + 200)}, 40, 2, PicoBrown)
	} else {
		rl.DrawTextEx(g.Font, "Press X to play again", rl.Vector2{X: float32(centerX - 280), Y: float32(centerY + 200)}, 40, 2, PicoBrown)
	}
}
