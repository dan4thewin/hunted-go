package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Options menu items
const optionsItemCount = 10 // volume, music, song, sfx volume, sfx toggle, mode, threat, debug, back, quit

// titleMenuLabels returns the labels for the title menu
func (g *Game) titleMenuLabels() []string {
	if g.HasSaveGame() {
		return []string{"Continue", "Start", "Options", "Credits", "Quit"}
	}
	return []string{"Start", "Options", "Credits", "Quit"}
}

// Mode names for display
var modeNames = []string{"Normal", "Normal", "Relaxed", "Stepwise"}

// overlayDismissPressed returns true if any game/menu key is pressed to dismiss an overlay
func (g *Game) overlayDismissPressed() bool {
	return rl.IsKeyPressed(rl.KeyUp) || rl.IsKeyPressed(rl.KeyDown) ||
		rl.IsKeyPressed(rl.KeyLeft) || rl.IsKeyPressed(rl.KeyRight) ||
		rl.IsKeyPressed(rl.KeyZ) || rl.IsKeyPressed(rl.KeyX) || rl.IsKeyPressed(rl.KeyC) ||
		rl.IsKeyPressed(rl.KeyEscape) || rl.IsKeyPressed(rl.KeySpace) || rl.IsKeyPressed(rl.KeyEnter)
}

// mouseMoved returns true if the mouse has moved since the last frame, and updates LastMousePos
func (g *Game) mouseMoved(mouse rl.Vector2) bool {
	moved := mouse.X != g.LastMousePos.X || mouse.Y != g.LastMousePos.Y
	g.LastMousePos = mouse
	return moved
}

// --- Menu helpers ---
//
// Reusable helpers for options and title menu input handling.
//
//   menuNav         — arrow-key + mouse-hover navigation (wraps menuMouseHover)
//   menuMouseHover  — hover detection within a bounding box of menu items
//   cyclePick       — left/right/enter/click dispatch for cyclic selectors
//   activatePressed — returns true on Enter, X, or mouse click
//
// Shared parameters:
//   index    *int   — pointer to the current menu selection index
//   count    int    — total number of menu items
//   rectFunc        — returns (x, y, w, h) for item i; menuItemRect or titleMenuRect

// menuNav handles arrow-key and mouse-hover navigation for a menu.
func (g *Game) menuNav(index *int, count int, rectFunc func(int) (int32, int32, int32, int32)) {
	if rl.IsKeyPressed(rl.KeyUp) {
		*index = (*index - 1 + count) % count
	} else if rl.IsKeyPressed(rl.KeyDown) {
		*index = (*index + 1) % count
	} else {
		mouse := rl.GetMousePosition()
		if g.mouseMoved(mouse) {
			g.menuMouseHover(index, count, rectFunc)
		}
	}
}

func (g *Game) menuMouseHover(index *int, count int, rectFunc func(int) (int32, int32, int32, int32)) {
	mouse := g.LastMousePos
	scale := float32(g.WindowSize) / 512.0
	margin := float32(19 * scale)
	firstX, firstY, firstW, _ := rectFunc(0)
	_, lastY, _, lastH := rectFunc(count - 1)
	if mouse.X < float32(firstX)-margin || mouse.X > float32(firstX+firstW)+margin ||
		mouse.Y < float32(firstY)-margin || mouse.Y > float32(lastY+lastH)+margin {
		return
	}
	for i := 0; i < count; i++ {
		_, itemY, _, itemH := rectFunc(i)
		if mouse.Y >= float32(itemY) && mouse.Y < float32(itemY+itemH) {
			*index = i
			return
		}
	}
}

// cyclePick handles left/right/enter/click input for a cyclic selector.
func (g *Game) cyclePick(menuIndex int, fn func(int)) {
	if rl.IsKeyPressed(rl.KeyRight) || rl.IsKeyPressed(rl.KeyEnter) {
		fn(1)
	}
	if rl.IsKeyPressed(rl.KeyLeft) {
		fn(-1)
	}
	if rl.IsMouseButtonPressed(rl.MouseLeftButton) {
		mx, _, mw, _ := g.menuItemRect(menuIndex)
		if rl.GetMousePosition().X < float32(mx)+float32(mw)*0.5 {
			fn(-1)
		} else {
			fn(1)
		}
	}
}

// activatePressed returns true when the user presses Enter, X, or clicks.
func activatePressed() bool {
	return rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeyX) || rl.IsMouseButtonPressed(rl.MouseLeftButton)
}

func (g *Game) UpdateOptions() {
	mouse := rl.GetMousePosition()
	g.menuNav(&g.MenuIndex, optionsItemCount, g.menuItemRect)

	switch g.MenuIndex {
	case 0:
		g.updateVolumeOption(mouse)
	case 1:
		g.updateMusicOption(mouse)
	case 2:
		g.updateSongOption()
	case 3:
		g.updateSfxVolumeOption(mouse)
	case 4:
		g.updateSfxToggle()
	case 5:
		g.cyclePick(5, func(dir int) { g.cycleMode(dir) })
	case 6:
		g.updateThreatOption(mouse)
	case 7:
		g.cyclePick(7, func(dir int) { g.Debug = (g.Debug + dir + 16) % 16 })
	case 8:
		g.updateBackOption()
	case 9:
		g.updateQuitOption()
	}
}

func (g *Game) updateVolumeOption(mouse rl.Vector2) {
	g.updateVolumeSlider(mouse)
	if rl.IsKeyPressed(rl.KeyEnter) {
		if g.MusicVolume > 0 {
			g.MusicVolume = 0
		} else {
			g.MusicVolume = 0.5
		}
		if g.MusicPlaying {
			rl.SetMusicVolume(g.Music, g.MusicVolume)
		}
	}
}

func (g *Game) updateSfxVolumeOption(mouse rl.Vector2) {
	g.updateSfxVolumeSlider(mouse)
	if rl.IsKeyPressed(rl.KeyEnter) {
		if g.SfxVolume > 0 {
			g.SfxVolume = 0
		} else {
			g.SfxVolume = 1.0
		}
	}
}

func (g *Game) updateThreatOption(mouse rl.Vector2) {
	g.updateThreatSlider(mouse)
	if rl.IsKeyPressed(rl.KeyEnter) {
		g.ThreatMax = g.ThreatMaxStart
		g.ThreatMaxBase = g.ThreatMaxStart
	}
}

func (g *Game) updateSfxToggle() {
	if activatePressed() || rl.IsKeyPressed(rl.KeyLeft) || rl.IsKeyPressed(rl.KeyRight) {
		g.SfxEnabled = !g.SfxEnabled
	}
}

func (g *Game) updateBackOption() {
	if activatePressed() {
		g.Paused = false
		g.SaveOptions()
	}
}

func (g *Game) toggleMusicEnabled() {
	g.MusicEnabled = !g.MusicEnabled
	if !g.MusicEnabled {
		g.StopMusic()
	} else if len(g.Songs) > 0 {
		g.PlaySong(g.SongIndex)
	}
}

func (g *Game) updateMusicOption(mouse rl.Vector2) {
	if rl.IsMouseButtonPressed(rl.MouseLeftButton) {
		mx, _, mw, _ := g.menuItemRect(1)
		if mouse.X >= float32(mx)+float32(mw)*0.5 {
			g.SongRepeat = !g.SongRepeat
		} else {
			g.toggleMusicEnabled()
		}
	} else if rl.IsKeyPressed(rl.KeyRight) {
		g.SongRepeat = !g.SongRepeat
	} else if rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeyX) || rl.IsKeyPressed(rl.KeyLeft) {
		g.toggleMusicEnabled()
	}
}

func (g *Game) updateSongOption() {
	if len(g.Songs) == 0 {
		return
	}
	g.cyclePick(2, func(dir int) {
		if dir > 0 {
			g.NextSong()
		} else {
			g.PlaySong((g.SongIndex - 1 + len(g.Songs)) % len(g.Songs))
		}
	})
}

func (g *Game) updateQuitOption() {
	if !activatePressed() {
		return
	}
	if g.State == StatePlaying || (g.State == StateGameOver && g.Won) {
		g.SaveGame()
	}
	g.SaveOptions()
	rl.CloseWindow()
	os.Exit(0)
}

func (g *Game) updateThreatSlider(mouse rl.Vector2) {
	sliderX, sliderY, sliderW, sliderH := g.threatSliderRect()

	// Keyboard
	if rl.IsKeyPressed(rl.KeyRight) {
		g.ThreatMax = min(g.ThreatMax+g.ThreatStep, g.ThreatCapHi)
		g.ThreatMaxBase = g.ThreatMax
	}
	if rl.IsKeyPressed(rl.KeyLeft) {
		g.ThreatMax = max(g.ThreatMax-g.ThreatStep, g.ThreatCapLo)
		g.ThreatMaxBase = g.ThreatMax
	}

	// Mouse drag on slider
	if rl.IsMouseButtonDown(rl.MouseLeftButton) {
		if mouse.Y >= float32(sliderY) && mouse.Y < float32(sliderY+sliderH) &&
			mouse.X >= float32(sliderX) && mouse.X <= float32(sliderX+sliderW) {
			raw := float64(mouse.X-float32(sliderX)) / float64(sliderW)
			g.ThreatMax = g.ThreatCapLo + raw*(g.ThreatCapHi-g.ThreatCapLo)
			g.ThreatMax = max(g.ThreatCapLo, min(g.ThreatCapHi, g.ThreatMax))
			g.ThreatMaxBase = g.ThreatMax
		}
	}
}

func (g *Game) threatSliderRect() (int32, int32, int32, int32) {
	menuX, y, menuW, rowH := g.menuItemRect(6)
	labelW := int32(float32(menuW) * 0.35)
	sliderX := menuX + labelW
	sliderW := menuW - labelW - int32(20*float32(g.WindowSize)/512.0)
	sliderH := int32(float32(rowH) * 0.4)
	sliderY := y + (rowH-sliderH)/2
	return sliderX, sliderY, sliderW, sliderH
}

func (g *Game) cycleMode(dir int) {
	// Cycle through 0 (Normal), 2 (Relaxed), 3 (Stepwise)
	order := []int{0, 2, 3}
	cur := 0
	for i, m := range order {
		if m == g.Mode {
			cur = i
			break
		}
	}
	cur = (cur + dir + len(order)) % len(order)
	g.Mode = order[cur]
}

func (g *Game) updateVolumeSlider(mouse rl.Vector2) {
	sliderX, sliderY, sliderW, sliderH := g.volumeSliderRect()

	// Keyboard
	if rl.IsKeyPressed(rl.KeyRight) {
		g.MusicVolume = min(g.MusicVolume+0.05, 1.0)
	}
	if rl.IsKeyPressed(rl.KeyLeft) {
		g.MusicVolume = max(g.MusicVolume-0.05, 0.0)
	}

	// Mouse drag on slider
	if rl.IsMouseButtonDown(rl.MouseLeftButton) {
		if mouse.Y >= float32(sliderY) && mouse.Y < float32(sliderY+sliderH) &&
			mouse.X >= float32(sliderX) && mouse.X <= float32(sliderX+sliderW) {
			g.MusicVolume = (mouse.X - float32(sliderX)) / float32(sliderW)
			g.MusicVolume = max(0, min(1, g.MusicVolume))
		}
	}

	if g.MusicPlaying {
		rl.SetMusicVolume(g.Music, g.MusicVolume)
	}
}

func (g *Game) menuItemRect(index int) (int32, int32, int32, int32) {
	scale := float32(g.WindowSize) / 512.0
	rowH := int32(42 * scale)
	menuX := int32(80 * scale)
	menuW := g.WindowSize - 2*menuX
	baseY := int32(68 * scale)
	y := baseY + int32(index)*rowH
	return menuX, y, menuW, rowH
}

func (g *Game) titleMenuRect(index int) (int32, int32, int32, int32) {
	scale := float32(g.WindowSize) / 512.0
	menuCount := len(g.titleMenuLabels())
	rowH := cond(menuCount > 4, int32(40*scale), int32(50*scale))
	menuX := int32(140 * scale)
	menuW := g.WindowSize - 2*menuX
	baseY := int32(float32(g.WindowSize)/2 + 40*scale)
	y := baseY + int32(index)*rowH
	return menuX, y, menuW, rowH
}

func (g *Game) volumeSliderRect() (int32, int32, int32, int32) {
	menuX, y, menuW, rowH := g.menuItemRect(0)
	labelW := int32(float32(menuW) * 0.35)
	sliderX := menuX + labelW
	sliderW := menuW - labelW - int32(20*float32(g.WindowSize)/512.0)
	sliderH := int32(float32(rowH) * 0.4)
	sliderY := y + (rowH-sliderH)/2
	return sliderX, sliderY, sliderW, sliderH
}

func (g *Game) updateSfxVolumeSlider(mouse rl.Vector2) {
	sliderX, sliderY, sliderW, sliderH := g.sfxVolumeSliderRect()

	if rl.IsKeyPressed(rl.KeyRight) {
		g.SfxVolume = min(g.SfxVolume+0.05, 1.0)
	}
	if rl.IsKeyPressed(rl.KeyLeft) {
		g.SfxVolume = max(g.SfxVolume-0.05, 0.0)
	}

	if rl.IsMouseButtonDown(rl.MouseLeftButton) {
		if mouse.Y >= float32(sliderY) && mouse.Y < float32(sliderY+sliderH) &&
			mouse.X >= float32(sliderX) && mouse.X <= float32(sliderX+sliderW) {
			g.SfxVolume = (mouse.X - float32(sliderX)) / float32(sliderW)
			g.SfxVolume = max(0, min(1, g.SfxVolume))
		}
	}
}

func (g *Game) sfxVolumeSliderRect() (int32, int32, int32, int32) {
	menuX, y, menuW, rowH := g.menuItemRect(3)
	labelW := int32(float32(menuW) * 0.35)
	sliderX := menuX + labelW
	sliderW := menuW - labelW - int32(20*float32(g.WindowSize)/512.0)
	sliderH := int32(float32(rowH) * 0.4)
	sliderY := y + (rowH-sliderH)/2
	return sliderX, sliderY, sliderW, sliderH
}

func (g *Game) UpdateTitle() {
	g.Tick++
	labels := g.titleMenuLabels()
	menuCount := len(labels)
	if g.TitleMenuIndex >= menuCount {
		g.TitleMenuIndex = 0
	}

	g.menuNav(&g.TitleMenuIndex, menuCount, g.titleMenuRect)

	if !activatePressed() {
		return
	}

	switch labels[g.TitleMenuIndex] {
	case "Continue":
		if err := g.LoadGame(); err != nil {
			fmt.Printf("Failed to load: %v\n", err)
		}
	case "Start":
		if g.FirstLaunch {
			g.FirstLaunch = false
			g.ShowHelp = true
		}
		g.StartGame()
	case "Options":
		g.Paused = true
	case "Credits":
		g.ShowCredits = true
	case "Quit":
		rl.CloseWindow()
		os.Exit(0)
	}
}

// --- DrawOptions helpers ---
//
//   drawOptionSlider   — draws a labeled slider with knob and value text
//   drawOptionCheckbox — draws a labeled checkbox with On/Off text
//   drawOptionSelector — draws a labeled < value > selector

func (g *Game) drawOptionSlider(label string, sx, sy, sw, sh, y, rowH int32,
	frac float32, valueText string, lx, ly, fontSize, scale float32, labelColor, valueColor rl.Color) {
	rl.DrawTextEx(g.Font, label, rl.Vector2{X: lx, Y: ly}, fontSize, 2, labelColor)
	rl.DrawRectangle(sx, sy, sw, sh, SliderTrack)
	fillW := int32(float32(sw) * frac)
	rl.DrawRectangle(sx, sy, fillW, sh, valueColor)
	knobX := sx + fillW
	knobH := int32(float32(rowH) * 0.6)
	knobY := y + (rowH-knobH)/2
	rl.DrawRectangle(knobX-int32(2*scale), knobY, int32(4*scale), knobH, rl.White)
	pctX := float32(sx+sw) + 8*scale
	rl.DrawTextEx(g.Font, valueText, rl.Vector2{X: pctX, Y: ly}, fontSize, 2, valueColor)
}

func (g *Game) drawOptionCheckbox(label string, checked bool, cbX float32,
	y, rowH int32, lx, ly, fontSize, scale float32, labelColor, valueColor rl.Color) float32 {
	rl.DrawTextEx(g.Font, label, rl.Vector2{X: lx, Y: ly}, fontSize, 2, labelColor)
	cbSize := int32(fontSize * 0.7)
	cbY := float32(y) + float32(rowH-cbSize)/2
	rl.DrawRectangleLines(int32(cbX), int32(cbY), cbSize, cbSize, valueColor)
	if checked {
		pad := int32(3 * scale)
		rl.DrawRectangle(int32(cbX)+pad, int32(cbY)+pad, cbSize-2*pad, cbSize-2*pad, valueColor)
	}
	stateText := cond(checked, "On", "Off")
	rl.DrawTextEx(g.Font, stateText, rl.Vector2{X: cbX + float32(cbSize) + 10*scale, Y: ly}, fontSize, 2, valueColor)
	return cbX + float32(cbSize) + 60*scale
}

func (g *Game) drawOptionSelector(label, value string, menuX, menuW int32,
	lx, ly, fontSize, scale float32, labelColor, valueColor rl.Color) {
	rl.DrawTextEx(g.Font, label, rl.Vector2{X: lx, Y: ly}, fontSize, 2, labelColor)
	arrX := float32(menuX) + float32(menuW)*0.35
	rl.DrawTextEx(g.Font, "<", rl.Vector2{X: arrX, Y: ly}, fontSize, 2, valueColor)
	rl.DrawTextEx(g.Font, value, rl.Vector2{X: arrX + 20*scale, Y: ly}, fontSize, 2, valueColor)
	endX := float32(menuX+menuW) - 30*scale
	rl.DrawTextEx(g.Font, ">", rl.Vector2{X: endX, Y: ly}, fontSize, 2, valueColor)
}

func (g *Game) DrawOptions() {
	rl.DrawRectangle(0, 0, g.WindowSize, g.WindowSize, OverlayDim)

	scale := float32(g.WindowSize) / 512.0
	fontSize := 28 * scale
	smallFont := 22 * scale

	// Title — show "PAUSED" only during gameplay
	title := cond(g.State == StatePlaying || g.State == StateGameOver, "PAUSED", "OPTIONS")
	titleX := float32(g.WindowSize)/2 - cond(title == "PAUSED", 50*scale, 60*scale)
	rl.DrawTextEx(g.Font, title, rl.Vector2{X: titleX, Y: 12 * scale}, 55*scale, 2, PicoOrange)

	for i := 0; i < optionsItemCount; i++ {
		menuX, y, menuW, rowH := g.menuItemRect(i)
		selected := i == g.MenuIndex
		if selected {
			rl.DrawRectangle(menuX, y, menuW, rowH, HighlightOrange)
		}

		labelColor := cond(selected, PicoOrange, PicoBrown)
		valueColor := cond(selected, rl.White, PicoOrange)
		lx := float32(menuX) + 10*scale
		ly := float32(y) + float32(rowH-int32(fontSize))/2

		switch i {
		case 0: // Volume slider
			sx, sy, sw, sh := g.volumeSliderRect()
			g.drawOptionSlider("Volume", sx, sy, sw, sh, y, rowH,
				g.MusicVolume, fmt.Sprintf("%d%%", int(g.MusicVolume*100+0.5)),
				lx, ly, fontSize, scale, labelColor, valueColor)
		case 1: // Music + Repeat checkboxes
			alignX := float32(menuX) + float32(menuW)*0.35
			nextX := g.drawOptionCheckbox("Music", g.MusicEnabled, alignX,
				y, rowH, lx, ly, fontSize, scale, labelColor, valueColor)
			repLabelW := rl.MeasureTextEx(g.Font, "Repeat", fontSize, 2).X
			g.drawOptionCheckbox("Repeat", g.SongRepeat, nextX+repLabelW+10*scale,
				y, rowH, nextX, ly, fontSize, scale, valueColor, valueColor)
		case 2: // Song
			rl.DrawTextEx(g.Font, "Song", rl.Vector2{X: lx, Y: ly}, fontSize, 2, labelColor)
			if len(g.Songs) > 0 {
				songName := filepath.Base(g.Songs[g.SongIndex])
				songName = strings.TrimSuffix(songName, filepath.Ext(songName))
				arrX := float32(menuX) + float32(menuW)*0.35
				rl.DrawTextEx(g.Font, "<", rl.Vector2{X: arrX, Y: ly}, fontSize, 2, valueColor)
				rl.DrawTextEx(g.Font, songName, rl.Vector2{X: arrX + 20*scale, Y: ly}, smallFont, 1, valueColor)
				endX := float32(menuX+menuW) - 30*scale
				rl.DrawTextEx(g.Font, ">", rl.Vector2{X: endX, Y: ly}, fontSize, 2, valueColor)
			}
		case 3: // SFX Volume slider
			sx, sy, sw, sh := g.sfxVolumeSliderRect()
			g.drawOptionSlider("SFX Vol", sx, sy, sw, sh, y, rowH,
				g.SfxVolume, fmt.Sprintf("%d%%", int(g.SfxVolume*100+0.5)),
				lx, ly, fontSize, scale, labelColor, valueColor)
		case 4: // SFX toggle
			alignX := float32(menuX) + float32(menuW)*0.35
			g.drawOptionCheckbox("SFX", g.SfxEnabled, alignX,
				y, rowH, lx, ly, fontSize, scale, labelColor, valueColor)
		case 5: // Mode
			curMode := "Normal"
			if g.Mode < len(modeNames) {
				curMode = modeNames[g.Mode]
			}
			g.drawOptionSelector("Mode", curMode, menuX, menuW, lx, ly, fontSize, scale, labelColor, valueColor)
		case 6: // Threat slider
			sx, sy, sw, sh := g.threatSliderRect()
			frac := float32((g.ThreatMax - g.ThreatCapLo) / (g.ThreatCapHi - g.ThreatCapLo))
			g.drawOptionSlider("Threat", sx, sy, sw, sh, y, rowH,
				frac, fmt.Sprintf("%.1f", g.ThreatMax),
				lx, ly, fontSize, scale, labelColor, valueColor)
		case 7: // Debug
			g.drawOptionSelector("Debug", fmt.Sprintf("%d", g.Debug), menuX, menuW, lx, ly, fontSize, scale, labelColor, valueColor)
		case 8: // Back
			rl.DrawTextEx(g.Font, "Back", rl.Vector2{X: lx, Y: ly}, fontSize, 2, labelColor)
		case 9: // Quit
			quitLabel := cond(g.State == StatePlaying || (g.State == StateGameOver && g.Won), "Save and Quit", "Quit")
			rl.DrawTextEx(g.Font, quitLabel, rl.Vector2{X: lx, Y: ly}, fontSize, 2, labelColor)
		}
	}
}

func (g *Game) DrawHelp() {
	rl.DrawRectangle(0, 0, g.WindowSize, g.WindowSize, OverlayDark)
	scale := float32(g.WindowSize) / 512.0
	cx := float32(g.WindowSize) / 2
	cy := float32(g.WindowSize)/2 - 40*scale

	rl.DrawTextEx(g.Font, "HOW TO PLAY", rl.Vector2{X: cx - 120*scale, Y: cy - 80*scale}, 60*scale, 2, PicoOrange)
	rl.DrawTextEx(g.Font, "Arrow keys to move", rl.Vector2{X: cx - 160*scale, Y: cy + 10*scale}, 30*scale, 2, PicoBrown)
	rl.DrawTextEx(g.Font, "Z to dash", rl.Vector2{X: cx - 160*scale, Y: cy + 50*scale}, 30*scale, 2, PicoBrown)
	rl.DrawTextEx(g.Font, "X to throw bomb", rl.Vector2{X: cx - 160*scale, Y: cy + 90*scale}, 30*scale, 2, PicoBrown)
	rl.DrawTextEx(g.Font, "C to place mine", rl.Vector2{X: cx - 160*scale, Y: cy + 130*scale}, 30*scale, 2, PicoBrown)
	rl.DrawTextEx(g.Font, "Press any key", rl.Vector2{X: cx - 120*scale, Y: cy + 200*scale}, 30*scale, 2, PicoOrange)
}

func (g *Game) DrawCredits() {
	rl.DrawRectangle(0, 0, g.WindowSize, g.WindowSize, OverlayDark)
	scale := float32(g.WindowSize) / 512.0
	cx := float32(g.WindowSize) / 2
	cy := float32(g.WindowSize)/2 - 40*scale

	rl.DrawTextEx(g.Font, "CREDITS", rl.Vector2{X: cx - 80*scale, Y: cy - 80*scale}, 60*scale, 2, PicoOrange)
	rl.DrawTextEx(g.Font, "Game by DuelingMachine", rl.Vector2{X: cx - 180*scale, Y: cy + 10*scale}, 30*scale, 2, PicoBrown)
	rl.DrawTextEx(g.Font, "Sprites by Hoyb and Q-Bit", rl.Vector2{X: cx - 200*scale, Y: cy + 60*scale}, 30*scale, 2, PicoBrown)
	rl.DrawTextEx(g.Font, "Music by Gruber", rl.Vector2{X: cx - 130*scale, Y: cy + 110*scale}, 30*scale, 2, PicoBrown)
	rl.DrawTextEx(g.Font, "Press any key", rl.Vector2{X: cx - 120*scale, Y: cy + 200*scale}, 30*scale, 2, PicoOrange)
}

func (g *Game) DrawTitle() {
	scale := float32(g.WindowSize) / 512.0
	cx := float32(g.WindowSize)/2 + 48
	cy := float32(g.WindowSize)/2 - 48
	rl.DrawTextEx(g.Font, "HUNTED", rl.Vector2{X: cx - 120, Y: cy - 80 + 32}, 80, 2, PicoOrange)

	// Title menu
	labels := g.titleMenuLabels()
	menuCount := len(labels)
	for i := 0; i < menuCount; i++ {
		menuX, y, menuW, rowH := g.titleMenuRect(i)
		selected := i == g.TitleMenuIndex

		if selected {
			rl.DrawRectangle(menuX, y, menuW, rowH, HighlightOrange)
		}

		color := cond(selected, PicoOrange, PicoBrown)

		lx := float32(menuX) + float32(menuW)/2 - 40*scale
		ly := float32(y) + float32(rowH-int32(40*scale))/2
		rl.DrawTextEx(g.Font, labels[i], rl.Vector2{X: lx, Y: ly}, 40*scale, 2, color)
	}
}
