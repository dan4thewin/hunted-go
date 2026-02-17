package main

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func readDirKeys() int {
	return cond(rl.IsKeyDown(rl.KeyLeft) || rl.IsKeyDown(rl.KeyA), DirLeft, 0) |
		cond(rl.IsKeyDown(rl.KeyRight) || rl.IsKeyDown(rl.KeyD), DirRight, 0) |
		cond(rl.IsKeyDown(rl.KeyUp) || rl.IsKeyDown(rl.KeyW), DirUp, 0) |
		cond(rl.IsKeyDown(rl.KeyDown) || rl.IsKeyDown(rl.KeyS), DirDown, 0)
}

func PlayerInput(g *Game, p *GameObject) int {
	if p.X > XMax {
		return 0
	}
	if !g.Playing {
		return DirRight
	}

	dir := readDirKeys()

	// Debug: Left+Right spawns a waif
	if g.Debug > 0 && dir&(DirLeft|DirRight) == (DirLeft|DirRight) {
		g.SpawnWaif()
	}

	// Cancel opposing directions
	if dir&(DirLeft|DirRight) == (DirLeft | DirRight) {
		dir &^= DirLeft
	}
	if dir&(DirUp|DirDown) == (DirUp | DirDown) {
		dir &^= DirUp
	}

	if dir != 0 {
		p.LastDir = dir
	}

	if rl.IsKeyPressed(rl.KeyZ) && p.Poses2 != nil {
		p.SetDashing()
	}
	if rl.IsKeyPressed(rl.KeyX) {
		g.ThrowBomb(p)
	}
	if rl.IsKeyPressed(rl.KeyC) {
		g.PlaceMine(p)
	}

	return dir
}

func main() {
	// Set config flags before window creation
	rl.SetConfigFlags(rl.FlagWindowResizable)

	// Create initial window (will be resized after getting monitor info)
	rl.InitWindow(512, 512, "Hunted")
	defer rl.CloseWindow()

	// Get monitor height and size window to half of it (square)
	monitorHeight := rl.GetMonitorHeight(rl.GetCurrentMonitor())
	windowSize := int32(monitorHeight / 2)
	rl.SetWindowSize(int(windowSize), int(windowSize))

	// Center window on screen
	monitorWidth := rl.GetMonitorWidth(rl.GetCurrentMonitor())
	rl.SetWindowPosition(int(monitorWidth)/2-int(windowSize)/2, int(monitorHeight)/2-int(windowSize)/2)

	rl.SetTargetFPS(60)
	rl.SetExitKey(0) // Disable default Escape-to-quit so we can use it for the menu

	game := NewGame()
	game.WindowSize = windowSize
	game.InitCamera()

	if err := game.LoadConfig("config.yaml"); err != nil {
		fmt.Printf("Fatal: %v\n", err)
		return
	}

	game.LoadOptions()

	if err := game.LoadResources(); err != nil {
		fmt.Printf("Warning: %v\n", err)
		// Continue without spritesheet - will use colored rectangles
	}

	game.PopulateTitleBackground()

	game.InitMusic()
	defer game.CleanupMusic()

	for !rl.WindowShouldClose() {
		// Handle window resize - maintain square aspect ratio
		if rl.IsWindowResized() {
			game.HandleResize()
		}
		game.Update()
		game.Draw()
	}

	if game.State == StatePlaying || (game.State == StateGameOver && game.Won) {
		game.SaveGame()
	}
	game.SaveOptions()
	rl.UnloadFont(game.Font)
}
