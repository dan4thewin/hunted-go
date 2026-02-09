// Hunted - Go port of PICO-8 game
// by DuelingMachine

package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"gopkg.in/yaml.v3"
)

// Constants
const (
	GameWidth    = 128 // Game world width in pixels (16 tiles)
	GameHeight   = 160 // Game world height in pixels (20 tiles)
	TileSize     = 8
	VisibleTiles = 16                      // Number of tiles visible in each dimension
	VisibleSize  = VisibleTiles * TileSize // 128 pixels
	MaxN         = 32767
	ScrollWidth  = 112
	FinalScreen  = 896
	FinishLine   = 992
	TopEdge      = 16  // 2 tiles from top
	BottomEdge   = 144 // 2 tiles from bottom
	XMax         = 1023
	YMax         = 159
	BlackTile    = 9
	RockTile     = 241
)

// Direction flags
const (
	DirLeft  = 1
	DirRight = 2
	DirUp    = 4
	DirDown  = 8
)

const ThreatMaxStart = 1.375

// Game states
type GameState int

const (
	StateTitle GameState = iota
	StatePlaying
	StateGameOver
)

// Sprite definition
type SpriteDef struct {
	Name         string
	Idle         []int
	IdleFrames   int
	Poses        []int
	Frames       int
	Poses2       []int
	Frames2      int
	Speed        float64
	Speed2       float64
	Width        int
	Height       int
	XOff         int
	YOff         int
	Avoid        bool
	Ignore       bool
	Decay        bool
	Bounce       bool
	Final        *SpriteDef
	Linger       bool
	Crate        bool
	Heart        bool
	Thug         bool
	Player       bool
	Friendly     bool
	Waif         bool
	YMin         int
	YMax         int
	XMaxLimit    int
	InvulnFrames int
}

// Game object
type GameObject struct {
	ID        int
	Name      string
	X, Y      float64
	Speed     float64
	BaseSpeed float64
	Dir       int
	LastDir   int
	Flip      bool
	Width     int
	Height    int
	XOff      int
	YOff      int
	Avoid     bool
	Ignore    bool
	Decay     bool
	Bounce    bool
	Final     *SpriteDef
	Linger    bool
	Crate     bool
	Heart     bool
	Thug      bool
	Player    bool
	Friendly  bool
	Waif      bool
	Threat    float64
	Scored    bool
	YMin      int
	YMax      int
	XMaxLimit int
	Diag      bool
	Armed     bool
	Dot       int

	// Animation state
	Idle       []int
	IdleFrames int
	Poses      []int
	Frames     int
	Poses2     []int
	Frames2    int
	Speed2     float64

	// Current animation
	CurrentPoses  []int
	CurrentFrames int
	PoseIndex     int
	FrameCount    int
	CurrentSprite int
	IsIdle        bool
	IsDash        bool

	// Invulnerability
	InvulnFrames int
	InvulnCount  int
	IsInvuln     bool
	PalIndex     int  // for waif recoloring
	PalSwap      bool // draw with palette swap shader
	Spotted      bool // waif: spotted sound has played

	// Pathfinding
	Target       *GameObject
	TargetCrumb  *Crumb
	JitterX      float64
	JitterY      float64
	JitterTime   int
	PathC, PathR int
	Delay        int

	// Controller function
	Controller func(*Game, *GameObject) int
	PostMove   func(*Game, *GameObject)
	PreAct     func(*Game, *GameObject) bool
	FrameFunc  func(*GameObject) // called on animation frame reset, like Lua's framesfn
}

// Breadcrumb for pathfinding
type Crumb struct {
	X, Y float64
	Last *Crumb
	Old  bool
	ID   int
}

// Spark particle
type Spark struct {
	X, Y   float64
	DX, DY float64
	Time   int
}

// Ping visual effect
type Ping struct {
	X, Y   float64
	Radius float64
}

// Game state
type Game struct {
	State       GameState
	Playing     bool
	Won         bool
	Tick        int
	Score       int
	HighScore   int
	HeartCount  int
	BombCount   int
	MineCount   int
	CamX, CamY  float64
	ShakeFactor float64
	Seed        int64
	Rng         *rand.Rand
	Debug       int
	DebugMsg    string
	Mode        int
	ThreatMax   float64
	Threat      float64
	ThugCount   int
	WaifCount   int
	Respite     int
	Level       int
	Populating  bool // suppress individual spawn sounds during bulk populate

	// Game objects
	Objects []*GameObject
	Player  *GameObject
	Crumbs  []*Crumb
	CrumbID int
	Sparks  []Spark
	Pings   []Ping
	NextID  int

	// Pathfinding grid
	Path [][]int

	// Resources
	Spritesheet       rl.Texture2D
	TileMap           [][]int
	SpriteRects       []rl.Rectangle
	EndzoneTile       int
	FirstWallTile     int
	WallTileCnt       int
	FirstAccentTile   int
	AccentTileCnt     int
	AccentPct         int
	InvulnShader      rl.Shader
	BrightnessLoc     int32
	PaletteSwapShader rl.Shader
	PaletteIndexLoc   int32
	Camera            rl.Camera2D
	WindowSize        int32 // Square window dimension
	Font              rl.Font

	// Music
	Songs        []string
	SongIndex    int
	Music        rl.Music
	MusicPlaying bool
	MusicVolume  float32

	// Menu state
	Paused         bool
	MenuIndex      int        // options menu selected item
	ShowHelp       bool       // help overlay visible
	ShowCredits    bool       // credits overlay visible
	FirstLaunch    bool       // show help on first game start
	TitleMenuIndex int        // 0=start, 1=options, 2=credits, 3=quit
	LastMousePos   rl.Vector2 // previous frame mouse position for detecting movement

	// Sound effects
	SndExplosion [3]rl.Sound
	SndCrunch    rl.Sound
	SndPopulate  rl.Sound
	SndSafe      rl.Sound
	SndSpawn     rl.Sound
	SndSpawnWaif rl.Sound
	SndThrow     rl.Sound
	SndSpotted   rl.Sound
	SoundsLoaded bool
	SfxVolume    float32 // 0.0 to 1.0
	SfxEnabled   bool    // toggle sound effects on/off
}

// Sprite definitions
var (
	DefEmpty    = &SpriteDef{Name: "empty", Idle: []int{9}, IdleFrames: 2, Ignore: true, Decay: true}
	DefBoom     = &SpriteDef{Name: "boom", Idle: []int{199}, IdleFrames: 2, Decay: true}
	DefJunk     = &SpriteDef{Name: "junk", Idle: []int{3, 4, 5, 6, 7}, IdleFrames: 2, Avoid: true, Decay: true, XOff: 2, Width: 4, YOff: 3, Height: 4}
	DefMine     = &SpriteDef{Name: "mine", Idle: []int{1, 2}, IdleFrames: 30, Ignore: true, XOff: 2, Width: 3, YOff: 2, Height: 4, Final: DefEmpty}
	DefLiveBomb = &SpriteDef{Name: "livebomb", Idle: []int{236}, IdleFrames: 6, Speed: 1.5, Poses: []int{236}, Frames: 14, XOff: 3, Width: 2, YOff: 3, Height: 2, Decay: true, Final: DefBoom, Bounce: true}
	DefBomb     = &SpriteDef{Name: "bomb", Idle: []int{236}, IdleFrames: 6, Speed: 1.5, Poses: []int{236}, Frames: 6, XOff: 3, Width: 2, YOff: 3, Height: 2, Decay: true, Final: DefLiveBomb, Ignore: true, Bounce: true}
	DefCrate    = &SpriteDef{Name: "crate", Idle: []int{169}, IdleFrames: -1, Avoid: true, XOff: 1, Width: 7, YOff: 1, Height: 7, Final: DefEmpty, Crate: true}
	DefHeart    = &SpriteDef{Name: "heart", Idle: []int{170}, IdleFrames: -1, Avoid: true, Width: 7, YOff: 1, Height: 7, Final: DefEmpty, Heart: true}
	DefCorpse   = &SpriteDef{Name: "corpse", Idle: []int{27}, IdleFrames: -1, Ignore: true}
	DefPlayer   = &SpriteDef{
		Name: "player", Idle: []int{16, -16}, IdleFrames: 90, Width: 4, XOff: 2, Height: 5, YOff: 3,
		YMin: TopEdge, YMax: BottomEdge - 8, Speed: 0.25, Poses: []int{17, 18, 19, 20, 21, 22}, Frames: 4,
		Speed2: 1.375, Poses2: []int{23, 24, 25, 26}, Frames2: 2, Final: DefCorpse, Player: true, Friendly: true, InvulnFrames: 120,
	}
	DefWaif = &SpriteDef{
		Name: "waif", Idle: []int{16}, IdleFrames: -1, Width: 1, XOff: 4, Height: 4, YOff: 2,
		Speed: 0.25, Poses: []int{17, 18, 19, 20, 21, 22}, Frames: 4, Final: DefCorpse, Friendly: true, Waif: true,
	}

	// Dead thug sprites
	DefDeadChomper   = &SpriteDef{Name: "chX", Idle: []int{83, 84, 85}, IdleFrames: 16, Ignore: true, Decay: true, Linger: true}
	DefDeadIcedevil  = &SpriteDef{Name: "icX", Idle: []int{86, 87, 88}, IdleFrames: 16, Ignore: true, Decay: true, Linger: true}
	DefDeadCrabby    = &SpriteDef{Name: "crX", Idle: []int{147, 148, 149}, IdleFrames: 16, Ignore: true, Decay: true, Linger: true}
	DefDeadSluggy    = &SpriteDef{Name: "slX", Idle: []int{134, 135, 136}, IdleFrames: 10, Ignore: true, Decay: true}
	DefDeadFroggy    = &SpriteDef{Name: "frX", Idle: []int{160, 161, 162}, IdleFrames: 10, Ignore: true, Decay: true, Linger: true}
	DefDeadBatty     = &SpriteDef{Name: "baX", Idle: []int{166, 167}, IdleFrames: 10, Ignore: true, Decay: true}
	DefDeadThingy    = &SpriteDef{Name: "thX", Idle: []int{211, 212, 213}, IdleFrames: 16, Ignore: true, Decay: true, Linger: true}
	DefDeadBluedevil = &SpriteDef{Name: "blX", Idle: []int{224, 225, 226}, IdleFrames: 16, Ignore: true, Decay: true, Linger: true}
	DefDeadCopter    = &SpriteDef{Name: "coX", Idle: []int{198, 199, 200}, IdleFrames: 4, Ignore: true, Decay: true, Final: DefJunk}
	DefDeadTripod    = &SpriteDef{Name: "trX", Idle: []int{233, 234, 235}, IdleFrames: 4, Ignore: true, Decay: true, Final: DefJunk}
	DefDeadDrone     = &SpriteDef{Name: "drX", Idle: []int{243, 244, 245}, IdleFrames: 4, Ignore: true, Decay: true, Final: DefJunk}
	DefDeadCyclops   = &SpriteDef{Name: "cyX", Idle: []int{57, 58, 59, 73, 74, 75}, IdleFrames: 12, Ignore: true, Decay: true, Final: DefJunk}
	DefDeadWabbit    = &SpriteDef{Name: "waX", Idle: []int{105, 106, 107, 121, 122, 123}, IdleFrames: 12, Ignore: true, Decay: true, Final: DefJunk}

	// Thug definitions
	Thugs = []*SpriteDef{
		{Name: "mech", Speed: 0.1, Idle: []int{28}, Poses: []int{28, 30}, Frames: 12, Final: DefJunk, XOff: 2, Width: 12, YOff: 3, Height: 13, Thug: true},
		{Name: "pinkie", Speed: 0.25, Idle: []int{32}, Poses: []int{33, 34}, Frames: 12, Final: DefJunk, XOff: 2, Width: 4, Thug: true},
		{Name: "chomper", Speed: 0.25, Idle: []int{35, 36, 37}, IdleFrames: 12, Poses: []int{51, 52, 53, 67, 68, 69}, Frames: 6, Final: DefDeadChomper, Thug: true},
		{Name: "icedevil", Speed: 0.3, Idle: []int{38, 39, 40}, IdleFrames: 12, Poses: []int{54, 55, 56, 70, 71, 72}, Frames: 6, Final: DefDeadIcedevil, Thug: true},
		{Name: "zoid", Speed: 0.125, Idle: []int{48}, Poses: []int{49, 50}, Frames: 12, Final: DefJunk, Thug: true},
		{Name: "jazzy", Speed: 0.25, Idle: []int{64}, Poses: []int{65, 66}, Frames: 12, Final: DefJunk, Thug: true},
		{Name: "stretch", Speed: 0.3, Idle: []int{80}, Poses: []int{81, 82, 96, 97, 98, 112, 113, 114, 80}, Frames: 6, Final: DefJunk, Thug: true},
		{Name: "crabby", Speed: 0.25, Idle: []int{99, 100, 101}, IdleFrames: 12, Poses: []int{115, 116, 117}, Frames: 6, YOff: 3, Height: 5, Final: DefDeadCrabby, Thug: true},
		{Name: "sluggy", Speed: 0.05, Idle: []int{102, 103, 104}, IdleFrames: 12, Poses: []int{118, 119, 120}, Frames: 6, XOff: 3, Width: 3, YOff: 5, Height: 3, Final: DefDeadSluggy, Thug: true},
		{Name: "froggy", Speed: 0.25, Idle: []int{128, 129, 130}, IdleFrames: 12, Poses: []int{144, 145, 146}, Frames: 6, XOff: 2, Width: 4, YOff: 5, Height: 3, Final: DefDeadFroggy, Thug: true},
		{Name: "batty", Speed: 0.3, Idle: []int{150, 151, 152}, IdleFrames: 12, Poses: []int{150, 151, 152}, Frames: 6, Final: DefDeadBatty, Thug: true},
		{Name: "thingy", Speed: 0.15, Idle: []int{163, 164, 165}, IdleFrames: 12, Poses: []int{179, 180, 181, 195, 196, 197}, Frames: 6, Final: DefDeadThingy, Thug: true},
		{Name: "bluedevil", Speed: 0.3, Idle: []int{176, 177, 178}, IdleFrames: 12, Poses: []int{192, 192, 194, 208, 209, 210}, Frames: 6, Final: DefDeadBluedevil, Thug: true},
		{Name: "copter", Speed: 0.325, Idle: []int{182, 183, 184}, IdleFrames: 6, Poses: []int{182, 183, 184}, Frames: 6, Final: DefDeadCopter, Thug: true},
		{Name: "tripod", Speed: 0.3, Idle: []int{214, 215, 216}, IdleFrames: 6, Poses: []int{230, 231, 232}, Frames: 6, Final: DefDeadTripod, Thug: true},
		{Name: "drone", Speed: 0.3, Idle: []int{227, 228, 229}, IdleFrames: 6, Poses: []int{227, 228, 229}, Frames: 6, Final: DefDeadDrone, Thug: true},
		{Name: "cyclops", Speed: 0.25, Idle: []int{42, -42}, IdleFrames: 18, Poses: []int{41, 42, 43, 42}, Frames: 6, XOff: 2, Width: 3, Final: DefDeadCyclops, Thug: true},
		{Name: "wabbit", Speed: 0.25, Idle: []int{90, -90}, IdleFrames: 12, Poses: []int{89, 90, 91, 90}, Frames: 6, XOff: 2, Width: 4, Final: DefDeadWabbit, Thug: true},
	}
)

// NewGame creates a new game instance
func NewGame() *Game {
	g := &Game{
		State:       StateTitle,
		Mode:        1,
		ThreatMax:   ThreatMaxStart,
		Debug:       0,
		HeartCount:  4,
		BombCount:   3,
		FirstLaunch: true,
		SfxVolume:   1.0,
		SfxEnabled:  true,
		MineCount:   3,
		CamY:        16,
		Path:        make([][]int, 32),
	}
	for i := range g.Path {
		g.Path[i] = make([]int, 20)
	}
	return g
}

// Log prints a debug message to the console and sets the on-screen debug message (debug&1)
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

// InitCamera initializes the 2D camera for the game view
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

// HandleResize handles window resize to maintain square aspect ratio
func (g *Game) HandleResize() {
	width := int32(rl.GetScreenWidth())
	height := int32(rl.GetScreenHeight())

	// Use the smaller dimension to maintain square aspect ratio
	newSize := width
	if height < width {
		newSize = height
	}

	g.WindowSize = newSize
	// Update camera zoom to fit the visible area in the new window size
	g.Camera.Zoom = float32(g.WindowSize) / VisibleSize
	g.Camera.Offset = rl.Vector2{X: float32(g.WindowSize) / 2, Y: float32(g.WindowSize) / 2}
}

// LoadResources loads game resources
func (g *Game) LoadResources() error {
	// Load spritesheet
	img := rl.LoadImage("hunted.png")
	if img.Data == nil {
		return fmt.Errorf("failed to load hunted.png")
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

	// Load tile map from map.yaml
	if err := g.LoadTileMap("map.yaml"); err != nil {
		// Create empty map if file doesn't exist
		g.TileMap = make([][]int, YMax/TileSize+1)
		for i := range g.TileMap {
			g.TileMap[i] = make([]int, XMax/TileSize+1)
		}
	}

	// Load invulnerability flash shader
	g.InvulnShader = rl.LoadShader("", "invuln_flash.fs")
	g.BrightnessLoc = rl.GetShaderLocation(g.InvulnShader, "brightness")

	// Load palette swap shader for waif recoloring
	g.PaletteSwapShader = rl.LoadShader("", "palette_swap.fs")
	g.PaletteIndexLoc = rl.GetShaderLocation(g.PaletteSwapShader, "paletteIndex")

	// Load custom font at size 80 for crisp rendering
	g.Font = rl.LoadFontEx("fonts/hw.otf", 80, nil)

	return nil
}

// InitMusic initializes the audio device, loads sounds, scans for MP3 files and starts playing
func (g *Game) InitMusic() {
	rl.InitAudioDevice()
	g.LoadSounds()

	entries, err := os.ReadDir("music")
	if err != nil {
		fmt.Printf("No music directory: %v\n", err)
		return
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".mp3") {
			g.Songs = append(g.Songs, filepath.Join("music", e.Name()))
		}
	}
	sort.Strings(g.Songs)
	if len(g.Songs) == 0 {
		fmt.Println("No MP3 files found in music/")
		return
	}

	g.PlaySong(0)
}

// PlaySong loads and plays the song at the given index
func (g *Game) PlaySong(index int) {
	if len(g.Songs) == 0 {
		return
	}
	if g.MusicPlaying {
		rl.StopMusicStream(g.Music)
		rl.UnloadMusicStream(g.Music)
	}
	g.SongIndex = index % len(g.Songs)
	g.Music = rl.LoadMusicStream(g.Songs[g.SongIndex])
	g.MusicVolume = max(g.MusicVolume, 0.5)
	rl.SetMusicVolume(g.Music, g.MusicVolume)
	rl.PlayMusicStream(g.Music)
	g.MusicPlaying = true
	fmt.Printf("Now playing: %s\n", filepath.Base(g.Songs[g.SongIndex]))
}

// NextSong advances to the next song
func (g *Game) NextSong() {
	if len(g.Songs) == 0 {
		return
	}
	g.PlaySong(g.SongIndex + 1)
}

// StopMusic stops the current song
func (g *Game) StopMusic() {
	if g.MusicPlaying {
		rl.StopMusicStream(g.Music)
		rl.UnloadMusicStream(g.Music)
		g.MusicPlaying = false
		fmt.Println("Music stopped")
	}
}

// UpdateMusic updates the music stream buffer and auto-advances when a song ends
func (g *Game) UpdateMusic() {
	if !g.MusicPlaying {
		return
	}
	rl.UpdateMusicStream(g.Music)
	// Auto-advance to next song when current one finishes
	if !rl.IsMusicStreamPlaying(g.Music) {
		g.NextSong()
	}
}

// CleanupMusic unloads music resources and closes audio device
func (g *Game) CleanupMusic() {
	if g.MusicPlaying {
		rl.StopMusicStream(g.Music)
		rl.UnloadMusicStream(g.Music)
	}
	g.UnloadSounds()
	rl.CloseAudioDevice()
}

// LoadSounds loads all sound effects from the sounds/ directory
func (g *Game) LoadSounds() {
	if _, err := os.Stat("sounds"); os.IsNotExist(err) {
		fmt.Println("No sounds directory")
		return
	}
	g.SndExplosion[0] = rl.LoadSound("sounds/explosion1.wav")
	g.SndExplosion[1] = rl.LoadSound("sounds/explosion2.wav")
	g.SndExplosion[2] = rl.LoadSound("sounds/explosion3.wav")
	g.SndCrunch = rl.LoadSound("sounds/crunch.wav")
	g.SndPopulate = rl.LoadSound("sounds/populate.wav")
	g.SndSafe = rl.LoadSound("sounds/safe.wav")
	g.SndSpawn = rl.LoadSound("sounds/spawn.wav")
	g.SndSpawnWaif = rl.LoadSound("sounds/spawn-waif.wav")
	g.SndThrow = rl.LoadSound("sounds/throw.wav")
	g.SndSpotted = rl.LoadSound("sounds/spotted.wav")
	g.SoundsLoaded = true
}

// UnloadSounds frees all loaded sound effects
func (g *Game) UnloadSounds() {
	if !g.SoundsLoaded {
		return
	}
	for i := range g.SndExplosion {
		rl.UnloadSound(g.SndExplosion[i])
	}
	rl.UnloadSound(g.SndCrunch)
	rl.UnloadSound(g.SndPopulate)
	rl.UnloadSound(g.SndSafe)
	rl.UnloadSound(g.SndSpawn)
	rl.UnloadSound(g.SndSpawnWaif)
	rl.UnloadSound(g.SndThrow)
	rl.UnloadSound(g.SndSpotted)
}

// PlaySfx plays a sound effect if sound effects are enabled
func (g *Game) PlaySfx(s rl.Sound, name string) {
	if g.SoundsLoaded && g.SfxEnabled {
		rl.SetSoundVolume(s, g.SfxVolume)
		rl.PlaySound(s)
		g.Log("sfx:"+name, nil)
	}
}

// PlayExplosion plays a random explosion sound
func (g *Game) PlayExplosion() {
	if g.SoundsLoaded && g.Rng != nil {
		g.PlaySfx(g.SndExplosion[g.Rng.Intn(3)], "explosion")
	}
}

// MapData represents the YAML structure for the tile map
type MapData struct {
	EndzoneTile     int     `yaml:"endzone_tile"`
	FirstWallTile   int     `yaml:"first_wall_tile"`
	WallTileCnt     int     `yaml:"wall_tile_cnt"`
	FirstAccentTile int     `yaml:"first_accent_tile"`
	AccentTileCnt   int     `yaml:"accent_tile_cnt"`
	AccentPct       int     `yaml:"accent_pct"`
	Tiles           [][]int `yaml:"tiles"`
}

// LoadTileMap loads tile positions from map.yaml and converts values to sprite numbers
func (g *Game) LoadTileMap(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var mapData MapData
	if err := yaml.Unmarshal(data, &mapData); err != nil {
		return err
	}

	// Store tile config from YAML
	g.EndzoneTile = mapData.EndzoneTile
	g.FirstWallTile = mapData.FirstWallTile
	g.WallTileCnt = mapData.WallTileCnt
	g.FirstAccentTile = mapData.FirstAccentTile
	g.AccentTileCnt = mapData.AccentTileCnt
	g.AccentPct = mapData.AccentPct

	g.TileMap = make([][]int, len(mapData.Tiles))
	for rowIdx, srcRow := range mapData.Tiles {
		g.TileMap[rowIdx] = make([]int, len(srcRow))
		for colIdx, val := range srcRow {
			switch val {
			case 1:
				// Wall tile: random sprite from first_wall_tile to first_wall_tile + wall_tile_cnt - 1
				g.TileMap[rowIdx][colIdx] = g.FirstWallTile + rand.Intn(g.WallTileCnt)
			case 2:
				// Endzone tile
				g.TileMap[rowIdx][colIdx] = g.EndzoneTile
			default:
				// Floor: leave as 0, will be populated on first screen visit
				g.TileMap[rowIdx][colIdx] = 0
			}
		}
	}
	return nil
}

// IsWall checks if a tile position is a wall (wall tiles are FirstWallTile through FirstWallTile + WallTileCnt - 1)
func (g *Game) IsWall(col, row int) bool {
	if row < 0 || col < 0 {
		return false
	}
	if row >= len(g.TileMap) {
		return false
	}
	if col >= len(g.TileMap[row]) {
		return false
	}
	tile := g.TileMap[row][col]
	return (tile >= g.FirstWallTile && tile < g.FirstWallTile+g.WallTileCnt) ||
		tile == RockTile
}

// NewID generates a new unique ID
func (g *Game) NewID() int {
	g.NextID++
	return g.NextID
}

// Emplace creates a new game object from a sprite definition
func (g *Game) Emplace(def *SpriteDef, x, y float64, opts ...func(*GameObject)) *GameObject {
	obj := &GameObject{
		ID:           g.NewID(),
		Name:         def.Name,
		X:            x,
		Y:            y,
		Speed:        def.Speed,
		BaseSpeed:    def.Speed,
		Width:        def.Width,
		Height:       def.Height,
		XOff:         def.XOff,
		YOff:         def.YOff,
		Avoid:        def.Avoid,
		Ignore:       def.Ignore,
		Decay:        def.Decay,
		Bounce:       def.Bounce,
		Final:        def.Final,
		Linger:       def.Linger,
		Crate:        def.Crate,
		Heart:        def.Heart,
		Thug:         def.Thug,
		Player:       def.Player,
		Friendly:     def.Friendly,
		Waif:         def.Waif,
		YMin:         def.YMin,
		YMax:         def.YMax,
		XMaxLimit:    def.XMaxLimit,
		LastDir:      DirRight,
		InvulnFrames: def.InvulnFrames,
		Idle:         def.Idle,
		IdleFrames:   def.IdleFrames,
		Poses:        def.Poses,
		Frames:       def.Frames,
		Poses2:       def.Poses2,
		Frames2:      def.Frames2,
		Speed2:       def.Speed2,
	}

	// Set defaults
	if obj.Width == 0 {
		obj.Width = 8
	}
	if obj.Height == 0 {
		obj.Height = 8
	}
	if obj.YMax == 0 {
		obj.YMax = YMax
	}
	if obj.XMaxLimit == 0 {
		obj.XMaxLimit = XMax + 1
	}

	// Apply options
	for _, opt := range opts {
		opt(obj)
	}

	// Junk lingers with random duration: min 6 + random up to 240 frames (60fps)
	if def == DefJunk && obj.FrameFunc == nil {
		obj.FrameFunc = func(o *GameObject) {
			o.CurrentFrames = max(6, g.Rng.Intn(240))
		}
	}

	// Start idling
	obj.SetIdle()

	g.Objects = append(g.Objects, obj)
	return obj
}

// SetIdle sets object to idle animation
func (o *GameObject) SetIdle() {
	if o.IsIdle {
		return
	}
	o.IsIdle = true
	o.IsDash = false
	o.ResetPose(o.Idle, o.IdleFrames)
}

// SetMoving sets object to moving animation
func (o *GameObject) SetMoving() {
	if !o.IsIdle && (!o.IsDash || o.Speed != o.BaseSpeed) {
		return
	}
	o.IsIdle = false
	o.IsDash = false
	o.ResetPose(o.Poses, o.Frames)
}

// SetDashing sets object to dash animation
func (o *GameObject) SetDashing() {
	if o.IsDash {
		return
	}
	o.IsDash = true
	o.Speed = o.Speed2
	o.ResetPose(o.Poses2, o.Frames2)
}

// ResetPose resets animation state
func (o *GameObject) ResetPose(poses []int, frames int) {
	o.PoseIndex = 0
	o.FrameCount = 0
	if len(poses) > 0 {
		o.CurrentPoses = poses
	} else if len(o.Idle) > 0 {
		o.CurrentPoses = o.Idle
	}
	if frames != 0 {
		o.CurrentFrames = frames
	} else {
		o.CurrentFrames = -1
	}
	if len(o.CurrentPoses) > 0 {
		o.CurrentSprite = o.CurrentPoses[0]
	}
}

// NextPose advances to next animation frame
func (o *GameObject) NextPose(g *Game) bool {
	if o.FrameCount == 0 {
		if o.FrameFunc != nil {
			o.FrameFunc(o)
		}
		o.PoseIndex++
		if o.PoseIndex > len(o.CurrentPoses) {
			if o.Decay {
				if o.Final != nil {
					g.Emplace(o.Final, o.X, o.Y, func(obj *GameObject) {
						obj.Dir = o.Dir
					})
				}
				if !o.Linger {
					g.Reap(o)
					return false
				}
				o.PoseIndex--
				o.FrameCount = -1
			} else {
				o.PoseIndex = 1
			}
		}
		o.FrameCount = o.CurrentFrames
	} else if o.FrameCount > 0 {
		o.FrameCount--
	}
	if o.PoseIndex > 0 && o.PoseIndex <= len(o.CurrentPoses) {
		o.CurrentSprite = o.CurrentPoses[o.PoseIndex-1]
	}
	return true
}

// Bounds returns bounding box
func (o *GameObject) Bounds(nx, ny *float64) (x, y, x2, y2 float64) {
	px := o.X
	py := o.Y
	if nx != nil {
		px = *nx
	}
	if ny != nil {
		py = *ny
	}
	x = px + float64(o.XOff)
	y = py + float64(o.YOff)
	x2 = x + float64(o.Width) - 1
	y2 = y + float64(o.Height) - 1
	return
}

// Obstacle checks if position hits a wall
func (g *Game) Obstacle(o *GameObject, dx, dy float64, nx, ny *float64) bool {
	px, py, px2, py2 := o.Bounds(nx, ny)
	px += dx
	px2 += dx
	py += dy
	py2 += dy

	c := int(px / TileSize)
	r := int(py / TileSize)
	c2 := int(px2 / TileSize)
	r2 := int(py2 / TileSize)

	for i := c; i <= c2; i++ {
		for j := r; j <= r2; j++ {
			if g.IsWall(i, j) {
				return true
			}
		}
	}
	return false
}

// Overlap checks if two bounding boxes overlap
func Overlap(ax, ay, ax2, ay2, bx, by, bx2, by2 float64) bool {
	return !(ax > bx2 || ax2 < bx || ay > by2 || ay2 < by)
}

// Hazard finds an object that overlaps at position
func (g *Game) Hazard(p *GameObject, x, y float64, filter func(*GameObject) bool) *GameObject {
	px, py, px2, py2 := p.Bounds(&x, &y)
	for _, q := range g.Objects {
		if p != q && filter(q) {
			qx, qy, qx2, qy2 := q.Bounds(nil, nil)
			if Overlap(px, py, px2, py2, qx, qy, qx2, qy2) {
				return q
			}
		}
	}
	return nil
}

// Distance calculates distance between two objects
func Distance(a, b *GameObject) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return math.Sqrt(dx*dx + dy*dy)
}

// Input handles player input
func PlayerInput(g *Game, p *GameObject) int {
	if p.X > XMax {
		return 0
	}
	if !g.Playing {
		return DirRight
	}

	var dir int
	if rl.IsKeyDown(rl.KeyLeft) || rl.IsKeyDown(rl.KeyA) {
		dir |= DirLeft
	}
	if rl.IsKeyDown(rl.KeyRight) || rl.IsKeyDown(rl.KeyD) {
		dir |= DirRight
	}
	if rl.IsKeyDown(rl.KeyUp) || rl.IsKeyDown(rl.KeyW) {
		dir |= DirUp
	}
	if rl.IsKeyDown(rl.KeyDown) || rl.IsKeyDown(rl.KeyS) {
		dir |= DirDown
	}

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

	// Dash on Z
	if rl.IsKeyPressed(rl.KeyZ) && p.Poses2 != nil {
		p.SetDashing()
	}

	// Bomb on X (tap)
	if rl.IsKeyPressed(rl.KeyX) {
		g.ThrowBomb(p)
	}

	// Mine on C
	if rl.IsKeyPressed(rl.KeyC) {
		g.PlaceMine(p)
	}

	return dir
}

// PlaceMine places a mine at player position
func (g *Game) PlaceMine(p *GameObject) {
	if g.MineCount > 0 {
		g.MineCount--
		g.Emplace(DefMine, p.X, p.Y, func(obj *GameObject) {
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

// ThrowBomb throws a bomb
func (g *Game) ThrowBomb(p *GameObject) {
	if g.BombCount > 0 {
		g.BombCount--
		g.Emplace(DefBomb, p.X, p.Y, func(obj *GameObject) {
			obj.Dir = p.LastDir
		})
		g.PlaySfx(g.SndThrow, "throw")
	}
}

// OpenCrate opens a crate and gives items
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

// Crunch destroys an object
func (g *Game) Crunch(p *GameObject) bool {
	if p.Final == nil {
		return false
	}
	g.Reap(p)
	if g.Playing && p.Speed > 0 {
		if p.Thug {
			g.Score += int(p.Speed * 200)
		} else if p.Waif {
			g.Score -= int(p.Speed * 200)
		}
	}
	if (p.Thug && p.Y >= TopEdge && p.Y <= BottomEdge) || (p.Player && g.HeartCount == 0) || p.Waif {
		g.Emplace(p.Final, p.X, p.Y, func(obj *GameObject) {
			obj.PalIndex = p.PalIndex
			obj.PalSwap = p.PalSwap
		})
		// Play crunch for dead bodies (not junk-producing deaths)
		if p.Final != DefJunk {
			g.PlaySfx(g.SndCrunch, "crunch")
		}
		return true
	}
	return false
}

// Reap removes an object from the game
func (g *Game) Reap(p *GameObject) {
	if p.Player {
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
	} else if p.Friendly {
		g.WaifCount--
	} else if p.Thug {
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

// StartInvuln makes player invulnerable
func (g *Game) StartInvuln(p *GameObject) {
	p.IsInvuln = true
	p.InvulnCount = p.InvulnFrames
}

// UpdateInvuln updates invulnerability state
func (g *Game) UpdateInvuln(p *GameObject) bool {
	if !p.IsInvuln {
		return true
	}
	if p.InvulnCount == 0 {
		p.IsInvuln = false
		return true
	}
	p.InvulnCount--
	return true
}

// NoGhost filter for collision - returns true if object should be considered for collision
func NoGhost(p *GameObject) bool {
	return !p.Ignore
}

// AvoidFilter for pathfinding - returns true if object should be avoided
func AvoidFilter(p *GameObject) bool {
	return p.Avoid
}

// Hazard2 checks for avoidable objects adjacent in 4 directions (up, right, down, left)
func (g *Game) Hazard2(p *GameObject, filter func(*GameObject) bool) (u, r, d, l *GameObject) {
	px, py, px2, py2 := p.Bounds(nil, nil)
	for _, q := range g.Objects {
		if p != q && filter(q) {
			qx, qy, qx2, qy2 := q.Bounds(nil, nil)
			if Overlap(px, py-1, px2, py2, qx, qy, qx2, qy2) {
				u = q
			} else if Overlap(px, py, px2+1, py2, qx, qy, qx2, qy2) {
				r = q
			} else if Overlap(px, py, px2, py2+1, qx, qy, qx2, qy2) {
				d = q
			} else if Overlap(px-1, py, px2, py2, qx, qy, qx2, qy2) {
				l = q
			}
		}
	}
	return
}

// innerFollow returns direction toward target, checking obstacles and avoidable hazards.
// hasLast indicates the target is a breadcrumb with a predecessor (triggers arrival detection).
func (g *Game) innerFollow(p *GameObject, tx, ty float64, hasLast bool) (int, bool) {
	x, y := int(p.X), int(p.Y)
	itx, ity := int(tx), int(ty)
	if hasLast && itx == x && ity == y {
		return 0, true
	}

	hu, hr, hd, hl := g.Hazard2(p, AvoidFilter)

	var bits int
	if itx < x && hl == nil && !g.Obstacle(p, -1, 0, nil, nil) {
		bits |= DirLeft
	} else if itx > x && hr == nil && !g.Obstacle(p, 1, 0, nil, nil) {
		bits |= DirRight
	}
	if ity < y && hu == nil && !g.Obstacle(p, 0, -1, nil, nil) {
		bits |= DirUp
	} else if ity > y && hd == nil && !g.Obstacle(p, 0, 1, nil, nil) {
		bits |= DirDown
	}
	return bits, false
}

// MobCollide handles collision between objects
func (g *Game) MobCollide(p *GameObject, x, y float64) *GameObject {
	if p.Ignore {
		return nil
	}
	q := g.Hazard(p, x, y, NoGhost)
	if q == nil {
		return nil
	}
	if q.Crate && p.Friendly {
		g.OpenCrate(q)
	} else if p.Friendly && q.Friendly {
		if !p.Player {
			return q
		}
		return nil
	} else {
		g.Log(fmt.Sprintf("hits %s%d", q.Name, q.ID), p)
		a := g.Crunch(q)
		b := g.Crunch(p)
		if a || b {
			g.Explode(p.X, p.Y)
		}
	}
	return nil
}

// Walk moves an object in a direction
func (g *Game) Walk(p *GameObject, dir int) bool {
	var x, y float64
	switch dir {
	case DirLeft:
		x = -1
	case DirRight:
		x = 1
	case DirUp:
		y = -1
	case DirDown:
		y = 1
	case DirLeft | DirUp:
		x, y = -0.7, -0.7
	case DirRight | DirUp:
		x, y = 0.7, -0.7
	case DirLeft | DirDown:
		x, y = -0.7, 0.7
	case DirRight | DirDown:
		x, y = 0.7, 0.7
	default:
		return false
	}

	if x != 0 && y != 0 {
		if !p.Diag {
			p.X = math.Floor(p.X)
			p.Y = math.Floor(p.Y)
			p.Diag = true
		}
	} else {
		p.Diag = false
	}

	lx, ly := p.X, p.Y
	s := p.Speed

	// Using Go 1.25 builtin min/max
	ny := max(float64(p.YMin), min(ly+y*s, float64(p.YMax)))
	nx := max(0, min(lx+x*s, float64(p.XMaxLimit)))

	if p.Bounce && g.Obstacle(p, 0, 0, &nx, &ny) {
		lr := dir & (DirLeft | DirRight)
		ud := dir & (DirUp | DirDown)
		if !p.Diag {
			if lr == DirLeft {
				lr = DirRight
			} else if lr == DirRight {
				lr = DirLeft
			}
			if ud == DirUp {
				ud = DirDown
			} else if ud == DirDown {
				ud = DirUp
			}
		} else if g.Obstacle(p, 0, 0, &lx, &ny) {
			if ud == DirUp {
				ud = DirDown
			} else if ud == DirDown {
				ud = DirUp
			}
		} else if g.Obstacle(p, 0, 0, &nx, &ly) {
			if lr == DirLeft {
				lr = DirRight
			} else if lr == DirRight {
				lr = DirLeft
			}
		}
		p.Dir = lr | ud
		return g.Walk(p, p.Dir)
	}

	// Slide around corners
	if y != 0 && g.Obstacle(p, 0, 0, &lx, &ny) {
		testX := lx + 3
		if x == 0 && !g.Obstacle(p, 0, 0, &testX, &ny) {
			return g.Walk(p, DirRight)
		}
		testX = lx - 3
		if x == 0 && !g.Obstacle(p, 0, 0, &testX, &ny) {
			return g.Walk(p, DirLeft)
		}
		ny = ly
	}
	if x != 0 && g.Obstacle(p, 0, 0, &nx, &ly) {
		testY := ly + 3
		if y == 0 && !g.Obstacle(p, 0, 0, &nx, &testY) {
			return g.Walk(p, DirDown)
		}
		testY = ly - 3
		if y == 0 && !g.Obstacle(p, 0, 0, &nx, &testY) {
			return g.Walk(p, DirUp)
		}
		nx = lx
	}

	if nx == lx && ny == ly {
		g.Log("blocked", p)
		return false
	}

	q := g.MobCollide(p, nx, ny)
	if q != nil {
		//g.Log(fmt.Sprintf("bumped %s%d", q.Name, q.ID), p)
		return false
	}

	p.X, p.Y = nx, ny
	return true
}

// Act performs one tick of object behavior
func (g *Game) Act(p *GameObject) bool {
	if p == nil {
		return false
	}

	// Pre-action (invulnerability check)
	if p.PreAct != nil && !p.PreAct(g, p) {
		return false
	}
	if p.IsInvuln {
		g.UpdateInvuln(p)
	}

	// Get direction
	var dir int
	if p.Controller != nil {
		dir = p.Controller(g, p)
	} else {
		dir = p.Dir
	}

	if dir == 0 {
		p.Speed = max(0, p.Speed-0.05)
	} else {
		base := p.BaseSpeed
		if p.Thug && g.Mode == 2 {
			base *= 0.5
		}
		p.Speed = max(base, p.Speed-0.05)

		if dir&(DirLeft|DirRight) == DirLeft {
			p.Flip = true
		} else if dir&(DirLeft|DirRight) == DirRight {
			p.Flip = false
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

// Explode creates explosion effect
func (g *Game) Explode(x, y float64) {
	g.ShakeFactor = 1
	g.MakeSparks(x, y)
	g.PlayExplosion()
}

// MakeSparks creates spark particles
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

// UpdateSparks updates spark particles
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

// MakePing creates a ping effect
func (g *Game) MakePing(x, y float64) {
	g.Pings = append(g.Pings, Ping{X: x, Y: y, Radius: 0})
}

// UpdatePings updates ping effects
func (g *Game) UpdatePings() {
	newPings := g.Pings[:0]
	for _, p := range g.Pings {
		p.Radius += 2
		if p.Radius <= 80 {
			newPings = append(newPings, p)
		}
	}
	g.Pings = newPings
}

// UpdatePath updates pathfinding grid
func (g *Game) UpdatePath(p *GameObject) {
	px := int(p.X) + p.XOff
	mapC := max(0, px-GameWidth) / TileSize
	c := (px-mapC*TileSize)/TileSize + 1
	r := (int(p.Y)+p.YOff)/TileSize + 1

	if mapC+c == p.PathC && r == p.PathR {
		return
	}
	p.PathC = mapC + c
	p.PathR = r

	// Reset path grid
	for i := range g.Path {
		for j := range g.Path[i] {
			g.Path[i][j] = MaxN
		}
	}

	if c >= 1 && c <= 32 && r >= 1 && r <= 20 {
		g.Path[c-1][r-1] = 0
	}

	// BFS pathfinding
	queue := []struct{ c, r int }{{c, r}}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		cc, cr := curr.c-1, curr.r-1
		if cc < 0 || cr < 0 || cc >= 32 || cr >= 20 {
			continue
		}
		cost := g.Path[cc][cr] + 1

		// Check neighbors
		neighbors := []struct{ dc, dr int }{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
		for _, n := range neighbors {
			nc, nr := cc+n.dc, cr+n.dr
			if nc >= 0 && nc < 32 && nr >= 0 && nr < 20 && g.Path[nc][nr] == MaxN {
				if g.IsWall(mapC+nc, nr) {
					g.Path[nc][nr] = 99
				} else {
					g.Path[nc][nr] = cost
					queue = append(queue, struct{ c, r int }{nc + 1, nr + 1})
				}
			}
		}
	}
}

// Retarget finds nearest friendly target
func (g *Game) Retarget(p *GameObject) *GameObject {
	var target *GameObject
	minDist := float64(MaxN)
	for _, q := range g.Objects {
		if p == q {
			continue
		}
		if (q.Friendly && q.X < FinishLine) || (q.Thug && !g.Playing) {
			d := Distance(p, q)
			if d < minDist {
				target = q
				minDist = d
			}
		}
	}
	return target
}

// Follow is the thug AI controller - follows targets with breadcrumb fallback
func Follow(g *Game, p *GameObject) int {
	// p.TargetCrumb is the persistent target (nil or a crumb, matching Lua's p.t)
	tc := p.TargetCrumb

	// If no crumb target or crumb was reaped, retarget to nearest friendly
	var t *GameObject
	if tc == nil || tc.Old {
		p.TargetCrumb = nil
		tc = nil
		t = g.Retarget(p)
		p.Target = t
		if t != nil && t.Waif && !t.Spotted {
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

	// Arrived at breadcrumb - retarget to nearest friendly
	if arrived {
		p.TargetCrumb = nil
		return Follow(g, p)
	}

	// Stuck - try following the player's breadcrumb trail
	if g.Playing && ret == 0 && len(g.Crumbs) > 0 {
		var c *Crumb
		if p.TargetCrumb != nil {
			c = p.TargetCrumb.Last
		}
		if c == nil {
			c = g.Crumbs[len(g.Crumbs)-1]
		}
		for c != nil {
			ret, _ = g.innerFollow(p, c.X, c.Y, c.Last != nil)
			if ret != 0 {
				break
			}
			c = c.Last
		}
		p.TargetCrumb = c
	}

	p.Dot = ret
	return ret
}

// FollowPath is the waif AI controller that uses the Path grid
func FollowPath(g *Game, p *GameObject) int {
	px, py := int(p.X), int(p.Y)

	// At or past finish line, or game won - just go right
	if px >= FinishLine || g.Win() {
		return DirRight
	}

	// Not playing - head right and center vertically around y=80
	if !g.Playing {
		ret := DirRight
		if py > 80 {
			ret |= DirUp
		} else if py < 78 {
			ret |= DirDown
		}
		return ret
	}

	// Need player reference for pathfinding
	if g.Player == nil {
		return DirRight
	}

	tx := int(g.Player.X) + g.Player.XOff
	ty := int(g.Player.Y) + g.Player.YOff

	// If too far from player (beyond screen width), use simple direction towards player
	if abs(tx-px) >= GameWidth {
		var ret int
		if px < tx {
			ret |= DirRight
		} else {
			ret |= DirLeft
		}
		if py < ty {
			ret |= DirDown
		} else if py > ty {
			ret |= DirUp
		}
		p.Dot = ret
		return ret
	}

	// Use Path grid for navigation
	// Calculate map column offset based on player position (same as UpdatePath)
	mapC := max(0, tx-GameWidth) / TileSize

	// Calculate object's position in the path grid (1-indexed like Lua)
	c := (px-mapC*TileSize)/TileSize + 1
	r := py/TileSize + 1

	// Get current path value, default to MaxN if out of bounds
	pmin := MaxN
	if c >= 1 && c <= 32 && r >= 1 && r <= 20 {
		pmin = g.Path[c-1][r-1]
	}

	// Find neighboring cell with minimum path value
	nc, nr := c, r
	for i := max(1, c-1); i <= min(32, c+1); i++ {
		for j := max(1, r-1); j <= min(20, r+1); j++ {
			if !(i == c && j == r) && g.Path[i-1][j-1] < pmin {
				pmin = g.Path[i-1][j-1]
				nc, nr = i, j
			}
		}
	}

	// Calculate target position from the best cell
	nx := float64((mapC + nc - 1) * TileSize)
	ny := float64((nr - 1) * TileSize)

	// If very close to player (pmin <= 1), add jitter to avoid stacking on player
	if pmin <= 1 {
		if p.JitterTime == 0 {
			p.JitterX = float64(g.Rng.Intn(13) - 6)
			p.JitterY = float64(g.Rng.Intn(13) - 6)
			p.JitterTime = 120
		} else {
			p.JitterTime--
		}
		nx = float64(tx) + p.JitterX
		ny = float64(ty) + p.JitterY
	} else {
		p.JitterTime = 0
	}

	// Calculate direction towards target
	var ret int
	if nx < float64(px) {
		ret |= DirLeft
	} else if nx > float64(px) {
		ret |= DirRight
	}
	if ny < float64(py) {
		ret |= DirUp
	} else if ny > float64(py) {
		ret |= DirDown
	}

	p.Dot = ret
	return ret
}

// abs returns absolute value of an int
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// PlayerPostMove handles player post-movement
func PlayerPostMove(g *Game, p *GameObject) {
	g.UpdatePath(p)
	g.AddCrumb(p)
}

// WaifPostMove handles waif post-movement
func WaifPostMove(g *Game, p *GameObject) {
	if p.X > FinishLine && !p.Scored && (g.Playing || g.Win()) {
		g.Score += 100
		p.Scored = true
		p.Ignore = true
		g.PlaySfx(g.SndSafe, "safe")
	} else if p.X > XMax {
		g.Reap(p)
	} else if g.Player != nil && Distance(p, g.Player) < 40 && g.Player.Speed > 0 {
		p.Speed = g.Player.Speed
	}
}

// AddCrumb adds a breadcrumb
func (g *Game) AddCrumb(p *GameObject) {
	if len(g.Crumbs) > 0 {
		if len(g.Crumbs) > 11 {
			g.ReapCrumb()
		}
		last := g.Crumbs[len(g.Crumbs)-1]
		dx := p.X - last.X
		dy := p.Y - last.Y
		if dx*dx+dy*dy > 20 {
			g.CrumbID++
			g.Crumbs = append(g.Crumbs, &Crumb{X: p.X, Y: p.Y, Last: last, ID: g.CrumbID})
		}
	} else {
		g.CrumbID = 1
		g.Crumbs = append(g.Crumbs, &Crumb{X: p.X, Y: p.Y, ID: g.CrumbID})
	}
}

// ReapCrumb removes oldest breadcrumb
func (g *Game) ReapCrumb() {
	if len(g.Crumbs) > 1 {
		g.Crumbs[1].Last = nil
	}
	if len(g.Crumbs) > 0 {
		g.Crumbs[0].Old = true
		g.Crumbs = slices.Delete(g.Crumbs, 0, 1)
	}
}

// Spawn creates a new thug
func (g *Game) Spawn(def *SpriteDef, x, y float64, factor float64) bool {
	t := def.Speed
	s := (def.Speed - (def.Speed / (g.ThreatMax * 0.8)))

	if g.Threat+t > g.ThreatMax*factor {
		return false
	}

	g.Emplace(def, x, y, func(obj *GameObject) {
		obj.Speed = s
		obj.BaseSpeed = s
		obj.Threat = t
		obj.XMaxLimit = FinishLine - obj.Width - obj.XOff
		obj.Thug = true
		obj.Controller = Follow
	})

	g.ThugCount++
	g.Threat += t
	if !g.Populating {
		g.PlaySfx(g.SndSpawn, "spawn")
	}
	return true
}

// SpawnWaif creates a new waif
func (g *Game) SpawnWaif() {
	x := g.CamX + 32 + float64(g.Rng.Intn(56))
	var y float64
	if g.Rng.Intn(2) == 0 {
		y = 0
	} else {
		y = YMax - 8
	}

	g.MakePing(x, y)

	palIndex := g.Rng.Intn(3)
	g.Emplace(DefWaif, x, y, func(obj *GameObject) {
		obj.PalIndex = palIndex
		obj.PalSwap = true
		obj.JitterTime = 0
		obj.Controller = FollowPath
		obj.PostMove = WaifPostMove
		obj.Delay = 60
		obj.PreAct = SlowStart
	})

	g.WaifCount++
	g.PlaySfx(g.SndSpawnWaif, "spawn-waif")
}

// SlowStart delays waif movement
func SlowStart(g *Game, p *GameObject) bool {
	p.Delay--
	if p.Delay == 0 {
		p.PreAct = nil
		p.YMin = TopEdge
		p.YMax = BottomEdge - 8
	}
	return false
}

// Respawn creates a new enemy
func (g *Game) Respawn() {
	idx := g.Rng.Intn(len(Thugs))
	def := Thugs[idx]

	x := g.CamX + float64(g.Rng.Intn(120-def.Width))
	var y float64
	if g.Playing {
		if g.Player != nil && g.Player.Y < 76 {
			y = float64(YMax - def.Height)
		}
	} else if g.Rng.Intn(2) == 1 {
		y = float64(YMax - def.Height)
	}

	g.Spawn(def, x, y, 1)
}

// Populate fills a screen with floor tiles and objects on first visit.
// It matches the original Lua: iterates tiles in random order, assigns
// black or accent tiles, and spawns thugs/crates/rocks/junk outside a
// no-populate zone around the player.
func (g *Game) Populate(x float64, factor float64) {
	xb := int(x / TileSize)
	yb := 0
	xb2 := min(FinishLine/TileSize-1, xb+15)
	yb2 := YMax / TileSize

	// Guard: if top-right tile already populated, this screen was visited
	if xb2 >= 0 && xb2 < len(g.TileMap[yb]) && g.TileMap[yb][xb2] > 0 {
		return
	}

	g.PlaySfx(g.SndPopulate, "populate")

	g.Populating = true
	defer func() { g.Populating = false }()

	// Build shuffled coordinate list
	coords := make([][2]int, 0, (xb2-xb+1)*(yb2-yb+1))
	for i := xb; i <= xb2; i++ {
		for j := yb; j <= yb2; j++ {
			coords = append(coords, [2]int{i, j})
		}
	}
	for i := len(coords) - 1; i >= 1; i-- {
		j := g.Rng.Intn(i + 1)
		coords[i], coords[j] = coords[j], coords[i]
	}

	// Player tile position for nopop exclusion zone
	px, py := 0, 0
	if g.Player != nil {
		px = int(g.Player.X) / TileSize
		py = int(g.Player.Y) / TileSize
	}
	// nopop: exclude tiles within {left=3, up=3, right=6, down=3} of player
	const nopL, nopU, nopR, nopD = 3, 3, 6, 3

	for _, coord := range coords {
		c, r := coord[0], coord[1]
		if r < 0 || r >= len(g.TileMap) || c < 0 || c >= len(g.TileMap[r]) {
			continue
		}
		if g.TileMap[r][c] != 0 {
			continue
		}

		// Assign floor tile: mostly black, small chance of accent
		n := g.Rng.Float64() * 100
		if n < 90 {
			g.TileMap[r][c] = BlackTile
		} else {
			accentIdx := int((101 - n) / 2)
			if g.AccentTileCnt > 0 {
				if accentIdx >= g.AccentTileCnt {
					accentIdx = g.AccentTileCnt - 1
				}
				g.TileMap[r][c] = g.FirstAccentTile + accentIdx
			} else {
				g.TileMap[r][c] = BlackTile
			}
		}

		// Only spawn objects outside nopop zone
		if c < px-nopL || c > px+nopR || r < py-nopU || r > py+nopD {
			added := false
			if g.Threat < g.ThreatMax*factor {
				idx := g.Rng.Intn(len(Thugs)-1) + 1 // skip mech (index 0) due to 16x16
				def := Thugs[idx]
				added = g.Spawn(def, float64(c*TileSize), float64(r*TileSize), factor)
			}
			if !added && r > 1 && r < 18 {
				if n < 0.5 {
					g.Emplace(DefCrate, float64(c*TileSize), float64(r*TileSize))
				} else if n < 1 {
					g.TileMap[r][c] = RockTile
				} else if n < 1.5 {
					g.Emplace(DefJunk, float64(c*TileSize), float64(r*TileSize))
				}
			}
		}
	}
}

// Win checks if player won
func (g *Game) Win() bool {
	return g.Won
}

// PopulateTitleBackground fills empty tiles with black/accent tiles for the title screen
func (g *Game) PopulateTitleBackground() {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for r := range g.TileMap {
		for c := range g.TileMap[r] {
			if g.TileMap[r][c] != 0 {
				continue
			}
			n := rng.Float64() * 100
			if n < 90 {
				g.TileMap[r][c] = BlackTile
			} else if g.AccentTileCnt > 0 {
				accentIdx := int((101 - n) / 2)
				if accentIdx >= g.AccentTileCnt {
					accentIdx = g.AccentTileCnt - 1
				}
				g.TileMap[r][c] = g.FirstAccentTile + accentIdx
			} else {
				g.TileMap[r][c] = BlackTile
			}
		}
	}
}

// Update game state
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

// Options menu items
const optionsItemCount = 10 // volume, music, song, sfx volume, sfx toggle, mode, threat, debug, back, quit

// Title menu items
const titleMenuCount = 4 // start, options, credits, quit

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

// UpdateOptions handles input in the options menu
func (g *Game) UpdateOptions() {
	mouse := rl.GetMousePosition()

	// Arrow navigation — checked first so keyboard takes priority
	if rl.IsKeyPressed(rl.KeyUp) {
		g.MenuIndex = (g.MenuIndex - 1 + optionsItemCount) % optionsItemCount
	} else if rl.IsKeyPressed(rl.KeyDown) {
		g.MenuIndex = (g.MenuIndex + 1) % optionsItemCount
	} else if g.mouseMoved(mouse) {
		// Mouse hover detection — only when mouse moved and within margin
		scale := float32(g.WindowSize) / 512.0
		margin := float32(19 * scale) // ~0.5 cm dead zone
		firstX, firstY, firstW, _ := g.menuItemRect(0)
		_, lastY, _, lastH := g.menuItemRect(optionsItemCount - 1)
		if mouse.X >= float32(firstX)-margin && mouse.X <= float32(firstX+firstW)+margin &&
			mouse.Y >= float32(firstY)-margin && mouse.Y <= float32(lastY+lastH)+margin {
			for i := 0; i < optionsItemCount; i++ {
				_, itemY, _, itemH := g.menuItemRect(i)
				if mouse.Y >= float32(itemY) && mouse.Y < float32(itemY+itemH) {
					if g.MenuIndex != i {
						g.MenuIndex = i
					}
					break
				}
			}
		}
	}

	// Enter key interacts with current item
	enterPressed := rl.IsKeyPressed(rl.KeyEnter)

	switch g.MenuIndex {
	case 0: // Volume slider
		g.updateVolumeSlider(mouse)
		if enterPressed {
			if g.MusicVolume > 0 {
				g.MusicVolume = 0
			} else {
				g.MusicVolume = 0.5
			}
			if g.MusicPlaying {
				rl.SetMusicVolume(g.Music, g.MusicVolume)
			}
		}
	case 1: // Music checkbox
		if enterPressed || rl.IsKeyPressed(rl.KeyX) || rl.IsKeyPressed(rl.KeyLeft) || rl.IsKeyPressed(rl.KeyRight) || rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			if g.MusicPlaying {
				g.StopMusic()
			} else if len(g.Songs) > 0 {
				g.PlaySong(g.SongIndex)
			}
		}
	case 2: // Song select
		if len(g.Songs) > 0 {
			if rl.IsKeyPressed(rl.KeyRight) || enterPressed {
				g.NextSong()
			}
			if rl.IsKeyPressed(rl.KeyLeft) {
				g.PlaySong((g.SongIndex - 1 + len(g.Songs)) % len(g.Songs))
			}
			if rl.IsMouseButtonPressed(rl.MouseLeftButton) {
				mx, _, mw, _ := g.menuItemRect(2)
				if mouse.X < float32(mx)+float32(mw)*0.5 {
					g.PlaySong((g.SongIndex - 1 + len(g.Songs)) % len(g.Songs))
				} else {
					g.NextSong()
				}
			}
		}
	case 3: // SFX Volume slider
		g.updateSfxVolumeSlider(mouse)
		if enterPressed {
			if g.SfxVolume > 0 {
				g.SfxVolume = 0
			} else {
				g.SfxVolume = 1.0
			}
		}
	case 4: // SFX toggle
		if enterPressed || rl.IsKeyPressed(rl.KeyX) || rl.IsKeyPressed(rl.KeyLeft) || rl.IsKeyPressed(rl.KeyRight) || rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			g.SfxEnabled = !g.SfxEnabled
		}
	case 5: // Mode select
		if rl.IsKeyPressed(rl.KeyRight) || enterPressed {
			g.cycleMode(1)
		}
		if rl.IsKeyPressed(rl.KeyLeft) {
			g.cycleMode(-1)
		}
		if rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			mx, _, mw, _ := g.menuItemRect(5)
			if mouse.X < float32(mx)+float32(mw)*0.5 {
				g.cycleMode(-1)
			} else {
				g.cycleMode(1)
			}
		}
	case 6: // Threat slider
		g.updateThreatSlider(mouse)
		if enterPressed {
			// Reset to default
			g.ThreatMax = ThreatMaxStart
		}
	case 7: // Debug
		if rl.IsKeyPressed(rl.KeyRight) || enterPressed {
			g.Debug = (g.Debug + 1) % 16
		}
		if rl.IsKeyPressed(rl.KeyLeft) {
			g.Debug = (g.Debug - 1 + 16) % 16
		}
		if rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			mx, _, mw, _ := g.menuItemRect(7)
			if mouse.X < float32(mx)+float32(mw)*0.5 {
				g.Debug = (g.Debug - 1 + 16) % 16
			} else {
				g.Debug = (g.Debug + 1) % 16
			}
		}
	case 8: // Back
		if enterPressed || rl.IsKeyPressed(rl.KeyX) || rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			g.Paused = false
		}
	case 9: // Quit
		if enterPressed || rl.IsKeyPressed(rl.KeyX) || rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			rl.CloseWindow()
			os.Exit(0)
		}
	}
}

func (g *Game) updateThreatSlider(mouse rl.Vector2) {
	sliderX, sliderY, sliderW, sliderH := g.threatSliderRect()

	// Keyboard
	if rl.IsKeyPressed(rl.KeyRight) {
		g.ThreatMax = min(g.ThreatMax+0.25, 10.0)
	}
	if rl.IsKeyPressed(rl.KeyLeft) {
		g.ThreatMax = max(g.ThreatMax-0.25, 0.5)
	}

	// Mouse drag on slider
	if rl.IsMouseButtonDown(rl.MouseLeftButton) {
		if mouse.Y >= float32(sliderY) && mouse.Y < float32(sliderY+sliderH) &&
			mouse.X >= float32(sliderX) && mouse.X <= float32(sliderX+sliderW) {
			raw := float64(mouse.X-float32(sliderX)) / float64(sliderW)
			g.ThreatMax = 0.5 + raw*9.5 // 0.5 to 10.0
			g.ThreatMax = max(0.5, min(10.0, g.ThreatMax))
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

// menuItemRect returns x, y, w, h for a menu row
func (g *Game) menuItemRect(index int) (int32, int32, int32, int32) {
	scale := float32(g.WindowSize) / 512.0
	rowH := int32(42 * scale)
	menuX := int32(80 * scale)
	menuW := g.WindowSize - 2*menuX
	baseY := int32(68 * scale)
	y := baseY + int32(index)*rowH
	return menuX, y, menuW, rowH
}

// titleMenuRect returns x, y, w, h for a title menu row
func (g *Game) titleMenuRect(index int) (int32, int32, int32, int32) {
	scale := float32(g.WindowSize) / 512.0
	rowH := int32(50 * scale)
	menuX := int32(140 * scale)
	menuW := g.WindowSize - 2*menuX
	baseY := int32(float32(g.WindowSize)/2 + 40*scale)
	y := baseY + int32(index)*rowH
	return menuX, y, menuW, rowH
}

// volumeSliderRect returns the slider track rect
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

// sfxVolumeSliderRect returns the SFX volume slider track rect
func (g *Game) sfxVolumeSliderRect() (int32, int32, int32, int32) {
	menuX, y, menuW, rowH := g.menuItemRect(3)
	labelW := int32(float32(menuW) * 0.35)
	sliderX := menuX + labelW
	sliderW := menuW - labelW - int32(20*float32(g.WindowSize)/512.0)
	sliderH := int32(float32(rowH) * 0.4)
	sliderY := y + (rowH-sliderH)/2
	return sliderX, sliderY, sliderW, sliderH
}

// UpdateTitle handles title screen
func (g *Game) UpdateTitle() {
	g.Tick++

	mouse := rl.GetMousePosition()

	// Arrow navigation — checked first so keyboard takes priority
	if rl.IsKeyPressed(rl.KeyUp) {
		g.TitleMenuIndex = (g.TitleMenuIndex - 1 + titleMenuCount) % titleMenuCount
	} else if rl.IsKeyPressed(rl.KeyDown) {
		g.TitleMenuIndex = (g.TitleMenuIndex + 1) % titleMenuCount
	} else if g.mouseMoved(mouse) {
		// Mouse hover on title menu — only when mouse moved and within margin
		scale := float32(g.WindowSize) / 512.0
		margin := float32(19 * scale) // ~0.5 cm dead zone
		firstX, firstY, firstW, _ := g.titleMenuRect(0)
		_, lastY, _, lastH := g.titleMenuRect(titleMenuCount - 1)
		if mouse.X >= float32(firstX)-margin && mouse.X <= float32(firstX+firstW)+margin &&
			mouse.Y >= float32(firstY)-margin && mouse.Y <= float32(lastY+lastH)+margin {
			for i := 0; i < titleMenuCount; i++ {
				_, itemY, _, itemH := g.titleMenuRect(i)
				if mouse.Y >= float32(itemY) && mouse.Y < float32(itemY+itemH) {
					if g.TitleMenuIndex != i {
						g.TitleMenuIndex = i
					}
					break
				}
			}
		}
	}

	enterPressed := rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeyX)
	clicked := rl.IsMouseButtonPressed(rl.MouseLeftButton)

	switch g.TitleMenuIndex {
	case 0: // Start
		if enterPressed || clicked {
			if g.FirstLaunch {
				g.FirstLaunch = false
				g.ShowHelp = true
			}
			g.StartGame()
		}
	case 1: // Options
		if enterPressed || clicked {
			g.Paused = true
		}
	case 2: // Credits
		if enterPressed || clicked {
			g.ShowCredits = true
		}
	case 3: // Quit
		if enterPressed || clicked {
			rl.CloseWindow()
			os.Exit(0)
		}
	}
}

// UpdatePlaying handles gameplay
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
		g.CamX += max(-2, min(2, diff))
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

// UpdateGameOver handles game over screen
func (g *Game) UpdateGameOver() {
	if rl.IsKeyPressed(rl.KeyX) || rl.IsKeyPressed(rl.KeyEnter) {
		g.StartGame()
		return
	}
	g.StepSimulation()
}

// StepSimulation advances objects, periodic events, and effects by one tick
func (g *Game) StepSimulation() {
	for _, obj := range g.Objects {
		g.Act(obj)
	}

	// Silently reap thugs more than a screen width behind the player
	if g.Player != nil {
		px := g.Player.X
		for i := len(g.Objects) - 1; i >= 0; i-- {
			obj := g.Objects[i]
			if obj.Thug && px-obj.X > 120 {
				g.ThugCount--
				g.Threat -= obj.Threat
				g.Objects = slices.Delete(g.Objects, i, i+1)
			}
		}
	}

	g.Tick++

	if g.Tick%200 == 0 {
		g.ReapCrumb()
	}
	if g.Tick%402 == 0 && g.CamX < FinalScreen && g.Rng.Float64() > 0.75-float64(g.Level)*0.05 {
		g.SpawnWaif()
	}
	if g.Tick%200 == 0 && g.Rng.Float64() > 0.75 {
		g.Log("respawn", nil)
		g.Respawn()
	}

	g.UpdatePings()
	g.UpdateSparks()
	g.UpdateShake()
}

// UpdateShake updates screen shake
func (g *Game) UpdateShake() {
	if g.ShakeFactor > 0 {
		g.ShakeFactor = max(0, g.ShakeFactor-0.05)
	}
}

// StartGame initializes a new game
func (g *Game) StartGame() {
	won := g.Won

	g.State = StatePlaying
	g.Playing = true
	g.Won = false
	g.Respite = 90
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
		g.ThreatMax += 0.125
		g.Level++
	} else {
		// Game over: reset everything
		g.HeartCount = 4
		g.BombCount = 3
		g.MineCount = 3
		g.Score = 0
		g.ThreatMax = ThreatMaxStart
		g.Level = 0
	}

	g.Seed = time.Now().UnixNano()
	g.Rng = rand.New(rand.NewSource(g.Seed))

	g.Objects = nil
	g.Crumbs = nil
	g.Sparks = nil
	g.Pings = nil

	// Reset path
	for i := range g.Path {
		for j := range g.Path[i] {
			g.Path[i][j] = MaxN
		}
	}

	// Create player
	g.Player = g.Emplace(DefPlayer, 64, 80, func(obj *GameObject) {
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

// Draw renders the game
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

// DrawOptions draws the options overlay (also serves as pause menu during gameplay)
func (g *Game) DrawOptions() {
	// Semi-transparent background
	rl.DrawRectangle(0, 0, g.WindowSize, g.WindowSize, rl.Color{R: 0, G: 0, B: 0, A: 200})

	scale := float32(g.WindowSize) / 512.0
	fontSize := 28 * scale
	smallFont := 22 * scale

	// Title — show "PAUSED" only during gameplay
	if g.State == StatePlaying || g.State == StateGameOver {
		titleX := float32(g.WindowSize)/2 - 50*scale
		titleY := 12 * scale
		rl.DrawTextEx(g.Font, "PAUSED", rl.Vector2{X: titleX, Y: titleY}, 55*scale, 2, ColorOrange)
	} else {
		titleX := float32(g.WindowSize)/2 - 60*scale
		titleY := 12 * scale
		rl.DrawTextEx(g.Font, "OPTIONS", rl.Vector2{X: titleX, Y: titleY}, 55*scale, 2, ColorOrange)
	}

	for i := 0; i < optionsItemCount; i++ {
		menuX, y, menuW, rowH := g.menuItemRect(i)
		selected := i == g.MenuIndex

		// Highlight bar
		if selected {
			rl.DrawRectangle(menuX, y, menuW, rowH, rl.Color{R: 255, G: 163, B: 0, A: 40})
		}

		labelColor := ColorDarkOrange
		valueColor := ColorOrange
		if selected {
			labelColor = ColorOrange
			valueColor = rl.White
		}

		lx := float32(menuX) + 10*scale
		ly := float32(y) + float32(rowH-int32(fontSize))/2

		switch i {
		case 0: // Volume slider
			rl.DrawTextEx(g.Font, "Volume", rl.Vector2{X: lx, Y: ly}, fontSize, 2, labelColor)
			sx, sy, sw, sh := g.volumeSliderRect()
			rl.DrawRectangle(sx, sy, sw, sh, rl.Color{R: 80, G: 80, B: 80, A: 255})
			fillW := int32(float32(sw) * g.MusicVolume)
			rl.DrawRectangle(sx, sy, fillW, sh, valueColor)
			knobX := sx + fillW
			knobH := int32(float32(rowH) * 0.6)
			knobY := y + (rowH-knobH)/2
			rl.DrawRectangle(knobX-int32(2*scale), knobY, int32(4*scale), knobH, rl.White)
			pctText := fmt.Sprintf("%d%%", int(g.MusicVolume*100+0.5))
			pctX := float32(sx+sw) + 8*scale
			rl.DrawTextEx(g.Font, pctText, rl.Vector2{X: pctX, Y: ly}, fontSize, 2, valueColor)

		case 1: // Music checkbox
			rl.DrawTextEx(g.Font, "Music", rl.Vector2{X: lx, Y: ly}, fontSize, 2, labelColor)
			cbSize := int32(fontSize * 0.7)
			cbX := float32(menuX) + float32(menuW)*0.35
			cbY := float32(y) + float32(rowH-cbSize)/2
			rl.DrawRectangleLines(int32(cbX), int32(cbY), cbSize, cbSize, valueColor)
			if g.MusicPlaying {
				pad := int32(3 * scale)
				rl.DrawRectangle(int32(cbX)+pad, int32(cbY)+pad, cbSize-2*pad, cbSize-2*pad, valueColor)
			}
			stateText := "Off"
			if g.MusicPlaying {
				stateText = "On"
			}
			rl.DrawTextEx(g.Font, stateText, rl.Vector2{X: cbX + float32(cbSize) + 10*scale, Y: ly}, fontSize, 2, valueColor)

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
			rl.DrawTextEx(g.Font, "SFX Vol", rl.Vector2{X: lx, Y: ly}, fontSize, 2, labelColor)
			sx, sy, sw, sh := g.sfxVolumeSliderRect()
			rl.DrawRectangle(sx, sy, sw, sh, rl.Color{R: 80, G: 80, B: 80, A: 255})
			fillW := int32(float32(sw) * g.SfxVolume)
			rl.DrawRectangle(sx, sy, fillW, sh, valueColor)
			knobX := sx + fillW
			knobH := int32(float32(rowH) * 0.6)
			knobY := y + (rowH-knobH)/2
			rl.DrawRectangle(knobX-int32(2*scale), knobY, int32(4*scale), knobH, rl.White)
			pctText := fmt.Sprintf("%d%%", int(g.SfxVolume*100+0.5))
			pctX := float32(sx+sw) + 8*scale
			rl.DrawTextEx(g.Font, pctText, rl.Vector2{X: pctX, Y: ly}, fontSize, 2, valueColor)

		case 4: // SFX toggle
			rl.DrawTextEx(g.Font, "SFX", rl.Vector2{X: lx, Y: ly}, fontSize, 2, labelColor)
			cbSize := int32(fontSize * 0.7)
			cbX := float32(menuX) + float32(menuW)*0.35
			cbY := float32(y) + float32(rowH-cbSize)/2
			rl.DrawRectangleLines(int32(cbX), int32(cbY), cbSize, cbSize, valueColor)
			if g.SfxEnabled {
				pad := int32(3 * scale)
				rl.DrawRectangle(int32(cbX)+pad, int32(cbY)+pad, cbSize-2*pad, cbSize-2*pad, valueColor)
			}
			stateText := "Off"
			if g.SfxEnabled {
				stateText = "On"
			}
			rl.DrawTextEx(g.Font, stateText, rl.Vector2{X: cbX + float32(cbSize) + 10*scale, Y: ly}, fontSize, 2, valueColor)

		case 5: // Mode
			rl.DrawTextEx(g.Font, "Mode", rl.Vector2{X: lx, Y: ly}, fontSize, 2, labelColor)
			arrX := float32(menuX) + float32(menuW)*0.35
			curMode := "Normal"
			if g.Mode < len(modeNames) {
				curMode = modeNames[g.Mode]
			}
			rl.DrawTextEx(g.Font, "<", rl.Vector2{X: arrX, Y: ly}, fontSize, 2, valueColor)
			rl.DrawTextEx(g.Font, curMode, rl.Vector2{X: arrX + 20*scale, Y: ly}, fontSize, 2, valueColor)
			endX := float32(menuX+menuW) - 30*scale
			rl.DrawTextEx(g.Font, ">", rl.Vector2{X: endX, Y: ly}, fontSize, 2, valueColor)

		case 6: // Threat slider
			rl.DrawTextEx(g.Font, "Threat", rl.Vector2{X: lx, Y: ly}, fontSize, 2, labelColor)
			sx, sy, sw, sh := g.threatSliderRect()
			rl.DrawRectangle(sx, sy, sw, sh, rl.Color{R: 80, G: 80, B: 80, A: 255})
			frac := float32((g.ThreatMax - 0.5) / 9.5)
			fillW := int32(float32(sw) * frac)
			rl.DrawRectangle(sx, sy, fillW, sh, valueColor)
			knobX := sx + fillW
			knobH := int32(float32(rowH) * 0.6)
			knobY := y + (rowH-knobH)/2
			rl.DrawRectangle(knobX-int32(2*scale), knobY, int32(4*scale), knobH, rl.White)
			threatText := fmt.Sprintf("%.1f", g.ThreatMax)
			threatX := float32(sx+sw) + 8*scale
			rl.DrawTextEx(g.Font, threatText, rl.Vector2{X: threatX, Y: ly}, fontSize, 2, valueColor)

		case 7: // Debug
			rl.DrawTextEx(g.Font, "Debug", rl.Vector2{X: lx, Y: ly}, fontSize, 2, labelColor)
			arrX := float32(menuX) + float32(menuW)*0.35
			rl.DrawTextEx(g.Font, "<", rl.Vector2{X: arrX, Y: ly}, fontSize, 2, valueColor)
			rl.DrawTextEx(g.Font, fmt.Sprintf("%d", g.Debug), rl.Vector2{X: arrX + 20*scale, Y: ly}, fontSize, 2, valueColor)
			endX := float32(menuX+menuW) - 30*scale
			rl.DrawTextEx(g.Font, ">", rl.Vector2{X: endX, Y: ly}, fontSize, 2, valueColor)

		case 8: // Back
			rl.DrawTextEx(g.Font, "Back", rl.Vector2{X: lx, Y: ly}, fontSize, 2, labelColor)

		case 9: // Quit
			rl.DrawTextEx(g.Font, "Quit", rl.Vector2{X: lx, Y: ly}, fontSize, 2, labelColor)
		}
	}
}

// DrawHelp draws the help overlay
func (g *Game) DrawHelp() {
	rl.DrawRectangle(0, 0, g.WindowSize, g.WindowSize, rl.Color{R: 0, G: 0, B: 0, A: 220})
	scale := float32(g.WindowSize) / 512.0
	cx := float32(g.WindowSize) / 2
	cy := float32(g.WindowSize)/2 - 40*scale

	rl.DrawTextEx(g.Font, "HOW TO PLAY", rl.Vector2{X: cx - 120*scale, Y: cy - 80*scale}, 60*scale, 2, ColorOrange)
	rl.DrawTextEx(g.Font, "Arrow keys to move", rl.Vector2{X: cx - 160*scale, Y: cy + 10*scale}, 30*scale, 2, ColorDarkOrange)
	rl.DrawTextEx(g.Font, "Z to dash", rl.Vector2{X: cx - 160*scale, Y: cy + 50*scale}, 30*scale, 2, ColorDarkOrange)
	rl.DrawTextEx(g.Font, "X to throw bomb", rl.Vector2{X: cx - 160*scale, Y: cy + 90*scale}, 30*scale, 2, ColorDarkOrange)
	rl.DrawTextEx(g.Font, "C to place mine", rl.Vector2{X: cx - 160*scale, Y: cy + 130*scale}, 30*scale, 2, ColorDarkOrange)
	rl.DrawTextEx(g.Font, "Press any key", rl.Vector2{X: cx - 120*scale, Y: cy + 200*scale}, 30*scale, 2, ColorOrange)
}

// DrawCredits draws the credits overlay
func (g *Game) DrawCredits() {
	rl.DrawRectangle(0, 0, g.WindowSize, g.WindowSize, rl.Color{R: 0, G: 0, B: 0, A: 220})
	scale := float32(g.WindowSize) / 512.0
	cx := float32(g.WindowSize) / 2
	cy := float32(g.WindowSize)/2 - 40*scale

	rl.DrawTextEx(g.Font, "CREDITS", rl.Vector2{X: cx - 80*scale, Y: cy - 80*scale}, 60*scale, 2, ColorOrange)
	rl.DrawTextEx(g.Font, "Game by DuelingMachine", rl.Vector2{X: cx - 180*scale, Y: cy + 10*scale}, 30*scale, 2, ColorDarkOrange)
	rl.DrawTextEx(g.Font, "Sprites by Hoyb and Q-Bit", rl.Vector2{X: cx - 200*scale, Y: cy + 60*scale}, 30*scale, 2, ColorDarkOrange)
	rl.DrawTextEx(g.Font, "Music by Gruber", rl.Vector2{X: cx - 130*scale, Y: cy + 110*scale}, 30*scale, 2, ColorDarkOrange)
	rl.DrawTextEx(g.Font, "Press any key", rl.Vector2{X: cx - 120*scale, Y: cy + 200*scale}, 30*scale, 2, ColorOrange)
}

// DrawTitle draws title screen text and menu
func (g *Game) DrawTitle() {
	scale := float32(g.WindowSize) / 512.0
	cx := float32(g.WindowSize)/2 + 48
	cy := float32(g.WindowSize)/2 - 48
	rl.DrawTextEx(g.Font, "HUNTED", rl.Vector2{X: cx - 120, Y: cy - 80 + 32}, 80, 2, ColorOrange)

	// Title menu
	titleLabels := []string{"Start", "Options", "Credits", "Quit"}
	for i := 0; i < titleMenuCount; i++ {
		menuX, y, menuW, rowH := g.titleMenuRect(i)
		selected := i == g.TitleMenuIndex

		if selected {
			rl.DrawRectangle(menuX, y, menuW, rowH, rl.Color{R: 255, G: 163, B: 0, A: 40})
		}

		color := ColorDarkOrange
		if selected {
			color = ColorOrange
		}

		lx := float32(menuX) + float32(menuW)/2 - 40*scale
		ly := float32(y) + float32(rowH-int32(40*scale))/2
		rl.DrawTextEx(g.Font, titleLabels[i], rl.Vector2{X: lx, Y: ly}, 40*scale, 2, color)
	}
}

// DrawTiles draws the tile map in world coordinates
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

// DrawSpriteScaled draws a sprite at (x,y) in world coordinates scaled to w x h tiles
// DrawSpriteScaled draws a multi-tile sprite region at (x,y) in world coordinates, w x h tiles
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

// DrawGame draws gameplay in world coordinates (camera handles scaling)
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
			phase := float32(obj.InvulnCount) * 0.15
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
		color := rl.Color{R: 255, G: uint8(200 + g.Rng.Intn(55)), B: 0, A: 255}
		rl.DrawRectangle(int32(s.X), int32(s.Y), 2, 2, color)
	}

	// Draw pings
	for _, p := range g.Pings {
		rl.DrawCircleLines(int32(p.X), int32(p.Y), float32(p.Radius), rl.White)
	}
}

// DrawCrumbs draws breadcrumb trail as orange dots (debug&2)
func (g *Game) DrawCrumbs() {
	for _, c := range g.Crumbs {
		rl.DrawPixel(int32(c.X), int32(c.Y), Pico8Colors[9])
	}
}

// DrawDirection draws a colored pixel showing a thug's movement direction (debug&2)
func (g *Game) DrawDirection(obj *GameObject) {
	if obj.Dot != 0 {
		c := obj.Dot
		if c < 0 || c > 15 {
			c = 7 // white fallback
		}
		rl.DrawPixel(int32(obj.X)+3, int32(obj.Y)+3, Pico8Colors[c])
	}
}

// DrawTargetLine draws a line from thug to its target (debug&2)
// Crumb target: light grey, player target: red, waif target: pink
func (g *Game) DrawTargetLine(obj *GameObject) {
	if !obj.Thug {
		return
	}
	ox, oy := int32(obj.X), int32(obj.Y)
	if obj.TargetCrumb != nil {
		rl.DrawLine(ox, oy, int32(obj.TargetCrumb.X), int32(obj.TargetCrumb.Y), Pico8Colors[6])
	}
	if obj.Target != nil {
		color := Pico8Colors[12] // blue default
		if obj.Target.Player {
			color = Pico8Colors[8] // red for player
		} else if obj.Target.Waif {
			color = Pico8Colors[14] // pink for waif
		}
		rl.DrawLine(ox, oy, int32(obj.Target.X), int32(obj.Target.Y), color)
	}
}

// DrawBoundsRect draws the bounding box of an object (debug&4)
func (g *Game) DrawBoundsRect(obj *GameObject) {
	bx, by, bx2, by2 := obj.Bounds(nil, nil)
	rl.DrawRectangleLines(int32(bx), int32(by), int32(bx2-bx+1), int32(by2-by+1), Pico8Colors[3])
}

// DrawPathGrid draws the pathfinding grid with distance values (debug&8)
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
			rl.DrawRectangleLines(x, y, TileSize, TileSize, Pico8Colors[1])

			pathCol := camCol + i + 1 // 1-based, matches Lua: path[c+i]
			if pathCol >= 1 && pathCol <= 32 && j+1 >= 1 && j+1 <= 20 {
				val := g.Path[pathCol-1][j] // Go is 0-based
				if val < MaxN {
					// Draw small number
					txt := strconv.Itoa(val)
					rl.DrawText(txt, x+1, y+1, 5, Pico8Colors[9])
				}
			}
		}
	}
}

// DrawSprite draws a sprite from the spritesheet in world coordinates
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

// DrawSpriteUI draws a sprite in screen coordinates, scaled to match the camera zoom
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

// DrawExtraGrid draws up to 9 orange pixels in a 3x3 grid at screen coordinates (sx, sy)
func (g *Game) DrawExtraGrid(sx, sy, scale float32, n int) {
	px := scale // one scaled pixel
	for i := 0; i < n; i++ {
		col := float32(i % 3)
		row := float32(i / 3)
		rl.DrawRectangleV(
			rl.Vector2{X: sx + col*2*px, Y: sy + row*2*px},
			rl.Vector2{X: px, Y: px},
			rl.Orange,
		)
	}
}

var (
	ColorOrange     = rl.Color{R: 255, G: 163, B: 0, A: 255} // PICO-8 color 9
	ColorDarkOrange = rl.Color{R: 171, G: 82, B: 54, A: 255} // PICO-8 color 4

	// PICO-8 color palette for debug drawing
	Pico8Colors = [16]rl.Color{
		{R: 0, G: 0, B: 0, A: 255},       // 0: black
		{R: 29, G: 43, B: 83, A: 255},    // 1: dark blue
		{R: 126, G: 37, B: 83, A: 255},   // 2: dark purple
		{R: 0, G: 135, B: 81, A: 255},    // 3: dark green
		{R: 171, G: 82, B: 54, A: 255},   // 4: brown
		{R: 95, G: 87, B: 79, A: 255},    // 5: dark grey
		{R: 194, G: 195, B: 199, A: 255}, // 6: light grey
		{R: 255, G: 241, B: 232, A: 255}, // 7: white
		{R: 255, G: 0, B: 77, A: 255},    // 8: red
		{R: 255, G: 163, B: 0, A: 255},   // 9: orange
		{R: 255, G: 236, B: 39, A: 255},  // 10: yellow
		{R: 0, G: 228, B: 54, A: 255},    // 11: green
		{R: 41, G: 173, B: 255, A: 255},  // 12: blue
		{R: 131, G: 118, B: 156, A: 255}, // 13: indigo
		{R: 255, G: 119, B: 168, A: 255}, // 14: pink
		{R: 255, G: 204, B: 170, A: 255}, // 15: peach
	}
)

// DrawUI draws the user interface in screen coordinates
func (g *Game) DrawUI() {
	scale := float32(g.WindowSize) / VisibleSize
	color := ColorOrange
	if !g.Playing {
		color = ColorDarkOrange
	}

	// Score
	fontSize := float32(TileSize) * scale
	rl.DrawTextEx(g.Font, strconv.Itoa(g.Score), rl.Vector2{X: 2 * scale, Y: 0}, fontSize, 1, color)

	// Hearts: sprites at x=20+i*8 for i=1..3, show heart (170) or heart2 (171)
	for i := 1; i <= 3; i++ {
		x := float32(20 + i*8)
		sprIdx := 171 // heart2 (empty)
		if i < g.HeartCount {
			sprIdx = 170 // heart (full)
		}
		g.DrawSpriteUI(sprIdx, x, -1)
	}
	// Plus sign if hearts > 4
	if g.HeartCount > 4 {
		cx := (float32(20-1+4*8) + 0.5) * scale
		cy := (float32(1) + 0.5) * scale
		rl.DrawCircle(int32(cx), int32(cy), 1.5*scale, rl.White)
	}

	// Bombs: sprite 236 at x=54+i*4 for i=1..min(bombcnt,3)
	cnt := min(g.BombCount, 3)
	for i := 1; i <= cnt; i++ {
		x := float32(54 + i*4)
		g.DrawSpriteUI(236, x, 0)
	}
	// Extra bombs as 3x3 grid of orange pixels
	if g.BombCount > 3 {
		g.DrawExtraGrid(float32(54+2+4*4)*scale, 1*scale, scale, min(g.BombCount-3, 9))
	}

	// Mines: sprite 2 at x=72+i*6 for i=1..min(minecnt,3)
	cnt = min(g.MineCount, 3)
	for i := 1; i <= cnt; i++ {
		x := float32(72 + i*6)
		g.DrawSpriteUI(2, x, 0)
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
			rl.DrawTextEx(g.Font, g.DebugMsg, rl.Vector2{X: 2 * scale, Y: 120 * scale}, smallFont, 1, Pico8Colors[8])
		}
		// Show FPS
		fpsText := fmt.Sprintf("fps:%d", rl.GetFPS())
		rl.DrawTextEx(g.Font, fpsText, rl.Vector2{X: 2 * scale, Y: 9 * scale}, smallFont, 1, color)
	}
}

// DrawGameOver draws game over screen in screen coordinates
func (g *Game) DrawGameOver() {
	centerX := g.WindowSize/2 + 48
	centerY := g.WindowSize/2 - 48

	if g.Win() {
		rl.DrawTextEx(g.Font, "SAFE!", rl.Vector2{X: float32(centerX - 100), Y: float32(centerY - 40)}, 80, 2, rl.Green)
	} else {
		rl.DrawTextEx(g.Font, "GAME OVER", rl.Vector2{X: float32(centerX - 200), Y: float32(centerY - 40)}, 80, 2, rl.Red)
	}

	rl.DrawTextEx(g.Font, fmt.Sprintf("Score: %d", g.Score), rl.Vector2{X: float32(centerX - 120), Y: float32(centerY + 60)}, 40, 2, ColorOrange)
	rl.DrawTextEx(g.Font, fmt.Sprintf("High Score: %d", g.HighScore), rl.Vector2{X: float32(centerX - 160), Y: float32(centerY + 120)}, 40, 2, ColorOrange)
	if g.Win() {
		rl.DrawTextEx(g.Font, "Press X to continue", rl.Vector2{X: float32(centerX - 280), Y: float32(centerY + 200)}, 40, 2, ColorDarkOrange)
	} else {
		rl.DrawTextEx(g.Font, "Press X to play again", rl.Vector2{X: float32(centerX - 280), Y: float32(centerY + 200)}, 40, 2, ColorDarkOrange)
	}
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

	rl.UnloadFont(game.Font)
}
