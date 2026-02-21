package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
	"gopkg.in/yaml.v3"
)

// NewGame creates a new game instance
func NewGame() *Game {
	g := &Game{
		State:     StateTitle,
		Mode:      1,
		Debug:     0,
		CamY:      16,
		Path:      make([][]int, PathCols),
		MenuState: MenuState{FirstLaunch: true},
	}
	for i := range g.Path {
		g.Path[i] = make([]int, PathRows)
	}
	return g
}

func (g *Game) Log(msg string, obj *GameObject) {
	if g.Debug&1 == 0 {
		return
	}
	s := fmt.Sprintf("%d", g.Tick)
	if obj != nil {
		s += fmt.Sprintf(" %s%d %.0f,%.0f", obj.Name, obj.ID, obj.X, obj.Y)
	}
	s += " " + msg
	fmt.Println(s)
	g.DebugMsg = s
}

func (g *Game) InitCamera() {
	// Camera target is the center of the visible area (offset by CamX, CamY)
	// The visible area is 16x16 tiles (128x128 pixels), with top 2 rows off screen
	// So CamY=16 means we start viewing from row 2 (y=16)
	g.Camera = rl.Camera2D{
		Offset:   rl.Vector2{X: float32(g.WindowSize) / 2, Y: float32(g.WindowSize) / 2},
		Target:   rl.Vector2{X: float32(g.CamX) + VisibleSize/2, Y: float32(g.CamY) + VisibleSize/2},
		Rotation: 0,
		Zoom:     float32(g.WindowSize) / VisibleSize,
	}
}

func (g *Game) HandleResize() {
	width := int32(rl.GetScreenWidth())
	height := int32(rl.GetScreenHeight())

	// Use the smaller dimension to maintain square aspect ratio
	newSize := cond(height < width, height, width)

	g.WindowSize = newSize
	// Update camera zoom to fit the visible area in the new window size
	g.Camera.Zoom = float32(g.WindowSize) / VisibleSize
	g.Camera.Offset = rl.Vector2{X: float32(g.WindowSize) / 2, Y: float32(g.WindowSize) / 2}
}

func (g *Game) LoadResources() error {
	// Load spritesheet
	img := rl.LoadImage(g.Assets.Spritesheet)
	if img.Data == nil {
		return fmt.Errorf("failed to load %s", g.Assets.Spritesheet)
	}
	g.Spritesheet = rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)

	// Pre-calculate sprite rectangles (16x16 grid of 8x8 sprites)
	g.SpriteRects = make([]rl.Rectangle, 256)
	for i := 0; i < 256; i++ {
		col := i % 16
		row := i / 16
		g.SpriteRects[i] = rl.Rectangle{
			X:      float32(col * TileSize),
			Y:      float32(row * TileSize),
			Width:  TileSize,
			Height: TileSize,
		}
	}
	// Expand rects for multi-tile sprites defined in config
	for _, def := range g.Defs {
		if def.SprW <= 1 && def.SprH <= 1 {
			continue
		}
		for _, idx := range def.Idle {
			if idx < 0 {
				idx = -idx
			}
			g.expandSpriteRect(idx, def.SprW, def.SprH)
		}
		for _, idx := range def.Poses {
			if idx < 0 {
				idx = -idx
			}
			g.expandSpriteRect(idx, def.SprW, def.SprH)
		}
	}
	// Title screen decoration sprites (not in config)
	g.expandSpriteRect(205, 3, 3)
	g.expandSpriteRect(175, 1, 2)

	// Load invulnerability flash shader
	g.InvulnShader = rl.LoadShader("", g.Assets.InvulnShader)
	g.BrightnessLoc = rl.GetShaderLocation(g.InvulnShader, "brightness")

	// Load palette swap shader for waif recoloring
	g.PaletteSwapShader = rl.LoadShader("", g.Assets.PaletteShader)
	g.PaletteIndexLoc = rl.GetShaderLocation(g.PaletteSwapShader, "paletteIndex")

	// Load custom font at size 80 for crisp rendering
	g.Font = rl.LoadFontEx(g.Assets.Font, 80, nil)

	return nil
}

func (g *Game) expandSpriteRect(idx, w, h int) {
	if idx >= 0 && idx < len(g.SpriteRects) {
		g.SpriteRects[idx].Width = float32(w * TileSize)
		g.SpriteRects[idx].Height = float32(h * TileSize)
	}
}

func parseKind(s string) Kind {
	switch s {
	case "player":
		return Player
	case "waif":
		return Waif
	case "thug":
		return Thug
	case "crate":
		return Crate
	case "heart":
		return Heart
	default:
		return 0
	}
}

func (y *SpriteDefYAML) toSpriteDef(key string) *SpriteDef {
	name := cond(y.Name != "", y.Name, key)
	w := cond(y.Width > 0, y.Width, TileSize)
	h := cond(y.Height > 0, y.Height, TileSize)
	return &SpriteDef{
		Name:         name,
		Idle:         y.Idle,
		IdleFrames:   y.IdleFrames,
		Poses:        y.Poses,
		Frames:       y.Frames,
		Poses2:       y.Poses2,
		Frames2:      y.Frames2,
		Speed:        y.Speed,
		Speed2:       y.Speed2,
		Width:        w,
		Height:       h,
		SprW:         (w + TileSize - 1) / TileSize,
		SprH:         (h + TileSize - 1) / TileSize,
		XOff:         y.XOff,
		YOff:         y.YOff,
		Avoid:        y.Avoid,
		Ignore:       y.Ignore,
		Decay:        y.Decay,
		Bounce:       y.Bounce,
		Linger:       y.Linger,
		Kind:         parseKind(y.Kind),
		YMin:         y.YMin,
		YMax:         y.YMax,
		XMaxLimit:    y.XMaxLimit,
		InvulnFrames: y.InvulnFrames,
	}
}

func (g *Game) LoadConfig(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var cfg ConfigData
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}

	// Store asset paths and sound paths
	g.Assets = cfg.Assets
	g.SoundPaths = cfg.Sounds

	// Store config values
	g.GameConfig = cfg.Config
	g.ThreatMaxBase = cfg.Config.ThreatMaxStart
	g.ThreatMax = cfg.Config.ThreatMaxStart
	g.StartHearts = cfg.Config.HeartCount
	g.StartBombs = cfg.Config.BombCount
	g.StartMines = cfg.Config.MineCount
	g.Audio.MusicVolume = cfg.Config.DefaultMusicVolume
	g.Audio.SfxVolume = cfg.Config.DefaultSfxVolume
	g.Audio.SfxEnabled = true
	g.Audio.MusicEnabled = true
	g.Audio.SongIndex = cfg.Config.DefaultSongIndex

	// Store tile config
	g.BlackTile = cfg.Tiles.BlackTile
	g.RockTile = cfg.Tiles.RockTile
	g.EndzoneTile = cfg.Tiles.EndzoneTile
	g.FirstWallTile = cfg.Tiles.FirstWallTile
	g.WallTileCnt = cfg.Tiles.WallTileCnt
	g.FirstAccentTile = cfg.Tiles.FirstAccentTile
	g.AccentTileCnt = cfg.Tiles.AccentTileCnt
	g.AccentPct = cfg.Tiles.AccentPct

	// Parse tile map
	rows := strings.Split(strings.TrimSpace(cfg.Tiles.Map), "\n")
	g.TileMap = make([][]int, len(rows))
	for rowIdx, line := range rows {
		g.TileMap[rowIdx] = make([]int, len(line))
		for colIdx, ch := range line {
			val := int(ch - '0')
			switch val {
			case 1:
				g.TileMap[rowIdx][colIdx] = g.FirstWallTile + rand.Intn(g.WallTileCnt)
			case 2:
				g.TileMap[rowIdx][colIdx] = g.EndzoneTile
			default:
				g.TileMap[rowIdx][colIdx] = 0
			}
		}
	}

	// Parse sprite definitions into a local map for resolution
	defs := make(map[string]*SpriteDef, len(cfg.Sprites))
	for key, ydef := range cfg.Sprites {
		defs[key] = ydef.toSpriteDef(key)
	}

	// Resolve Final references for sprites
	for key, def := range defs {
		if f := cfg.Sprites[key].Final; f != "" {
			def.Final = defs[f]
		}
	}

	// Keep full map for save/load restore
	g.Defs = defs

	// Build thug list from sprite references
	g.Thugs = make([]*SpriteDef, len(cfg.Thugs))
	for i, name := range cfg.Thugs {
		g.Thugs[i] = defs[name]
	}

	// Resolve named defs used at runtime
	g.DefPlayer = defs["player"]
	g.DefWaif = defs["waif"]
	g.DefBomb = defs["bomb"]
	g.DefMine = defs["mine"]
	g.DefCrate = defs["crate"]
	g.DefJunk = defs["junk"]
	g.DefHeart = defs["heart"]

	return nil
}
