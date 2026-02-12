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
)

// Direction flags
const (
	DirLeft  = 1
	DirRight = 2
	DirUp    = 4
	DirDown  = 8
)

// Kind identifies an object's type as a bit flag
type Kind uint8

const (
	Player Kind = 1 << iota
	Waif
	Thug
	Crate
	Heart
	Friendly = Player | Waif
)

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
	Kind         Kind
	YMin         int
	YMax         int
	XMaxLimit    int
	InvulnFrames int
}

// Anim holds animation state for a game object
type Anim struct {
	Idle          []int
	IdleFrames    int
	Poses         []int
	Frames        int
	Poses2        []int
	Frames2       int
	Speed2        float64
	CurrentPoses  []int
	CurrentFrames int
	PoseIndex     int
	FrameCount    int
	CurrentSprite int
	IsIdle        bool
	IsDash        bool
	FrameFunc     func(*GameObject) // called on animation frame reset, like Lua's framesfn
}

// AI holds pathfinding and targeting state for a game object
type AI struct {
	Target      *GameObject
	TargetCrumb *Crumb
	JitterX     float64
	JitterY     float64
	JitterTime  int
	PathC       int
	PathR       int
	Dot         int
	Delay       int
	Spotted     bool // waif: spotted sound has played
	Scored      bool
	Controller  func(*Game, *GameObject) int
	PostMove    func(*Game, *GameObject)
	PreAct      func(*Game, *GameObject) bool
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
	Kind      Kind
	Threat    float64
	YMin      int
	YMax      int
	XMaxLimit int
	Diag      bool
	Armed     bool

	// Invulnerability
	InvulnFrames int
	InvulnCount  int
	IsInvuln     bool
	PalIndex     int  // for waif recoloring
	PalSwap      bool // draw with palette swap shader

	Anim // animation state
	AI   // pathfinding, targeting, and control
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
	Color  rl.Color
}

// Audio holds music and sound effect state
type Audio struct {
	Songs        []string
	SongIndex    int
	Music        rl.Music
	MusicPlaying bool
	MusicVolume  float32
	SndExplosion []rl.Sound
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

// Render holds graphics resources and display state
type Render struct {
	Spritesheet       rl.Texture2D
	SpriteRects       []rl.Rectangle
	InvulnShader      rl.Shader
	BrightnessLoc     int32
	PaletteSwapShader rl.Shader
	PaletteIndexLoc   int32
	Camera            rl.Camera2D
	WindowSize        int32 // Square window dimension
	Font              rl.Font
}

// TileConfig holds tile map data and parameters
type TileConfig struct {
	TileMap         [][]int
	EndzoneTile     int
	FirstWallTile   int
	WallTileCnt     int
	FirstAccentTile int
	AccentTileCnt   int
	AccentPct       int
	BlackTile       int
	RockTile        int
}

// MenuState holds pause menu and overlay state
type MenuState struct {
	Paused         bool
	MenuIndex      int        // options menu selected item
	ShowHelp       bool       // help overlay visible
	ShowCredits    bool       // credits overlay visible
	FirstLaunch    bool       // show help on first game start
	TitleMenuIndex int        // 0=start, 1=options, 2=credits, 3=quit
	LastMousePos   rl.Vector2 // previous frame mouse position for detecting movement
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

	// Configuration from config.yaml
	Defs           map[string]*SpriteDef
	Thugs          []*SpriteDef
	ThreatMaxStart float64
	StartHearts    int
	StartBombs     int
	StartMines     int
	Assets         AssetsConfig
	SoundPaths     SoundsConfig
	GameConfig     // tunable params

	Audio      // music and sound effects
	Render     // graphics resources
	TileConfig // tile map data
	MenuState  // pause menu and overlays
}

// NewGame creates a new game instance
func NewGame() *Game {
	g := &Game{
		State:     StateTitle,
		Mode:      1,
		Debug:     0,
		CamY:      16,
		Path:      make([][]int, 32),
		Audio:     Audio{MusicVolume: 0.5, SfxVolume: 1.0, SfxEnabled: true},
		MenuState: MenuState{FirstLaunch: true},
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

// LoadResources loads game resources using paths from config
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

// InitMusic initializes the audio device, loads sounds, scans for MP3 files and starts playing
func (g *Game) InitMusic() {
	rl.InitAudioDevice()
	g.LoadSounds()

	entries, err := os.ReadDir(g.Assets.MusicDir)
	if err != nil {
		fmt.Printf("No music directory: %v\n", err)
		return
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".mp3") {
			g.Songs = append(g.Songs, filepath.Join(g.Assets.MusicDir, e.Name()))
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

// LoadSounds loads all sound effects using paths from config
func (g *Game) LoadSounds() {
	if _, err := os.Stat(g.Assets.SoundsDir); os.IsNotExist(err) {
		fmt.Println("No sounds directory")
		return
	}
	load := func(path string, name string) rl.Sound {
		if path == "" {
			fmt.Printf("Warning: no path for sound %s\n", name)
			return rl.Sound{}
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Printf("Warning: missing sound file %s\n", path)
			return rl.Sound{}
		}
		return rl.LoadSound(path)
	}
	g.SndExplosion = make([]rl.Sound, len(g.SoundPaths.Explosion))
	for i, path := range g.SoundPaths.Explosion {
		g.SndExplosion[i] = load(path, fmt.Sprintf("explosion%d", i))
	}
	g.SndCrunch = load(g.SoundPaths.Crunch, "crunch")
	g.SndPopulate = load(g.SoundPaths.Populate, "populate")
	g.SndSafe = load(g.SoundPaths.Safe, "safe")
	g.SndSpawn = load(g.SoundPaths.Spawn, "spawn")
	g.SndSpawnWaif = load(g.SoundPaths.SpawnWaif, "spawn_waif")
	g.SndThrow = load(g.SoundPaths.Throw, "throw")
	g.SndSpotted = load(g.SoundPaths.Spotted, "spotted")
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
	if g.SoundsLoaded && g.Rng != nil && len(g.SndExplosion) > 0 {
		g.PlaySfx(g.SndExplosion[g.Rng.Intn(len(g.SndExplosion))], "explosion")
	}
}

// MapData represents the YAML structure for the tile map
// ConfigData is the YAML config file format
type ConfigData struct {
	Assets  AssetsConfig              `yaml:"assets"`
	Config  GameConfig                `yaml:"config"`
	Tiles   TilesConfig               `yaml:"tiles"`
	Sprites map[string]*SpriteDefYAML `yaml:"sprites"`
	Thugs   []string                  `yaml:"thugs"`
	Sounds  SoundsConfig              `yaml:"sounds"`
}

// AssetsConfig holds file paths for game assets
type AssetsConfig struct {
	Spritesheet   string `yaml:"spritesheet"`
	Font          string `yaml:"font"`
	InvulnShader  string `yaml:"invuln_shader"`
	PaletteShader string `yaml:"palette_shader"`
	SoundsDir     string `yaml:"sounds_dir"`
	MusicDir      string `yaml:"music_dir"`
}

// GameConfig holds tunable game parameters
type GameConfig struct {
	ThreatMaxStart float64 `yaml:"threat_max_start"`
	HeartCount     int     `yaml:"heart_count"`
	BombCount      int     `yaml:"bomb_count"`
	MineCount      int     `yaml:"mine_count"`

	ThreatMaxInc float64 `yaml:"threat_max_inc"`
	ThreatStep   float64 `yaml:"threat_step"`
	ThreatCapHi  float64 `yaml:"threat_cap_hi"`
	ThreatCapLo  float64 `yaml:"threat_cap_lo"`
	ThreatSpeedX float64 `yaml:"threat_speed_x"`

	RespawnF     int     `yaml:"respawn_f"`
	RespawnPct   float64 `yaml:"respawn_pct"`
	WaifSpawnF   int     `yaml:"waif_spawn_f"`
	WaifSpawnPct float64 `yaml:"waif_spawn_pct"`
	WaifLevelPct float64 `yaml:"waif_level_pct"`

	ThugScoreX    int `yaml:"thug_score_x"`
	WaifPenaltyX  int `yaml:"waif_penalty_x"`
	WaifSafeScore int `yaml:"waif_safe_score"`

	SpeedDecayPxf float64 `yaml:"speed_decay_pxf"`
	RelaxedSpeedX float64 `yaml:"relaxed_speed_x"`
	CameraMaxPxf  float64 `yaml:"camera_max_pxf"`

	WaifDelayF   int     `yaml:"waif_delay_f"`
	WaifFollowPx float64 `yaml:"waif_follow_px"`
	WaifJitterPx int     `yaml:"waif_jitter_px"`
	WaifJitterF  int     `yaml:"waif_jitter_f"`

	RespiteF   int     `yaml:"respite_f"`
	ShakeDecay float64 `yaml:"shake_decay"`
	PingMaxPx  float64 `yaml:"ping_max_px"`
	ThugReapPx float64 `yaml:"thug_reap_px"`

	CratePct float64 `yaml:"crate_pct"`
	RockPct  float64 `yaml:"rock_pct"`
	JunkPct  float64 `yaml:"junk_pct"`
}

// TilesConfig holds tile map layout and parameters
type TilesConfig struct {
	BlackTile       int    `yaml:"black_tile"`
	RockTile        int    `yaml:"rock_tile"`
	EndzoneTile     int    `yaml:"endzone_tile"`
	FirstWallTile   int    `yaml:"first_wall_tile"`
	WallTileCnt     int    `yaml:"wall_tile_cnt"`
	FirstAccentTile int    `yaml:"first_accent_tile"`
	AccentTileCnt   int    `yaml:"accent_tile_cnt"`
	AccentPct       int    `yaml:"accent_pct"`
	Map             string `yaml:"map"`
}

// SoundsConfig holds file paths for sound effects
type SoundsConfig struct {
	Explosion []string `yaml:"explosion"`
	Crunch    string   `yaml:"crunch"`
	Populate  string   `yaml:"populate"`
	Safe      string   `yaml:"safe"`
	Spawn     string   `yaml:"spawn"`
	SpawnWaif string   `yaml:"spawn_waif"`
	Throw     string   `yaml:"throw"`
	Spotted   string   `yaml:"spotted"`
}

// SpriteDefYAML is the YAML representation of a sprite definition
type SpriteDefYAML struct {
	Name         string  `yaml:"name,omitempty"`
	Idle         []int   `yaml:"idle"`
	IdleFrames   int     `yaml:"idle_frames,omitempty"`
	Poses        []int   `yaml:"poses,omitempty"`
	Frames       int     `yaml:"frames,omitempty"`
	Poses2       []int   `yaml:"poses2,omitempty"`
	Frames2      int     `yaml:"frames2,omitempty"`
	Speed        float64 `yaml:"speed,omitempty"`
	Speed2       float64 `yaml:"speed2,omitempty"`
	Width        int     `yaml:"width,omitempty"`
	Height       int     `yaml:"height,omitempty"`
	XOff         int     `yaml:"x_off,omitempty"`
	YOff         int     `yaml:"y_off,omitempty"`
	Avoid        bool    `yaml:"avoid,omitempty"`
	Ignore       bool    `yaml:"ignore,omitempty"`
	Decay        bool    `yaml:"decay,omitempty"`
	Bounce       bool    `yaml:"bounce,omitempty"`
	Final        string  `yaml:"final,omitempty"`
	Linger       bool    `yaml:"linger,omitempty"`
	Kind         string  `yaml:"kind,omitempty"`
	YMin         int     `yaml:"y_min,omitempty"`
	YMax         int     `yaml:"y_max,omitempty"`
	XMaxLimit    int     `yaml:"x_max_limit,omitempty"`
	InvulnFrames int     `yaml:"invuln_frames,omitempty"`
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
	name := y.Name
	if name == "" {
		name = key
	}
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
		Width:        y.Width,
		Height:       y.Height,
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

// LoadConfig loads all configuration from config.yaml: assets, sprites, thugs, tiles
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
	g.ThreatMaxStart = cfg.Config.ThreatMaxStart
	g.ThreatMax = cfg.Config.ThreatMaxStart
	g.StartHearts = cfg.Config.HeartCount
	g.StartBombs = cfg.Config.BombCount
	g.StartMines = cfg.Config.MineCount

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

	// Parse sprite definitions
	g.Defs = make(map[string]*SpriteDef, len(cfg.Sprites))
	for key, ydef := range cfg.Sprites {
		g.Defs[key] = ydef.toSpriteDef(key)
	}

	// Resolve Final references for sprites
	for key, def := range g.Defs {
		if f := cfg.Sprites[key].Final; f != "" {
			def.Final = g.Defs[f]
		}
	}

	// Build thug list from sprite references
	g.Thugs = make([]*SpriteDef, len(cfg.Thugs))
	for i, name := range cfg.Thugs {
		g.Thugs[i] = g.Defs[name]
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
		tile == g.RockTile
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
		Kind:         def.Kind,
		YMin:         def.YMin,
		YMax:         def.YMax,
		XMaxLimit:    def.XMaxLimit,
		LastDir:      DirRight,
		InvulnFrames: def.InvulnFrames,
		Anim: Anim{
			Idle:       def.Idle,
			IdleFrames: def.IdleFrames,
			Poses:      def.Poses,
			Frames:     def.Frames,
			Poses2:     def.Poses2,
			Frames2:    def.Frames2,
			Speed2:     def.Speed2,
		},
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
	if def == g.Defs["junk"] && obj.FrameFunc == nil {
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
		g.Emplace(g.Defs["mine"], p.X, p.Y, func(obj *GameObject) {
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
		g.Emplace(g.Defs["bomb"], p.X, p.Y, func(obj *GameObject) {
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
		if p.Final != g.Defs["junk"] {
			g.PlaySfx(g.SndCrunch, "crunch")
		}
		return true
	}
	return false
}

// Reap removes an object from the game
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
	if q.Kind == Crate && p.Kind&Friendly != 0 {
		g.OpenCrate(q)
	} else if p.Kind&Friendly != 0 && q.Kind&Friendly != 0 {
		if p.Kind != Player {
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
	return g.walk(p, dir, false)
}

func (g *Game) walk(p *GameObject, dir int, slide bool) bool {
	for attempt := 0; ; attempt++ {
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
				// Snap away from movement direction
				snapX, snapY := math.Floor, math.Floor
				if x < 0 {
					snapX = math.Ceil
				}
				if y < 0 {
					snapY = math.Ceil
				}
				p.X = snapX(p.X)
				p.Y = snapY(p.Y)
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
			if attempt > 0 {
				// Already bounced once — stop to avoid infinite loop
				return false
			}
			lr := dir & (DirLeft | DirRight)
			ud := dir & (DirUp | DirDown)
			if !p.Diag {
				switch lr {
				case DirLeft:
					lr = DirRight
				case DirRight:
					lr = DirLeft
				}
				switch ud {
				case DirUp:
					ud = DirDown
				case DirDown:
					ud = DirUp
				}
			} else if g.Obstacle(p, 0, 0, &lx, &ny) {
				switch ud {
				case DirUp:
					ud = DirDown
				case DirDown:
					ud = DirUp
				}
			} else if g.Obstacle(p, 0, 0, &nx, &ly) {
				switch lr {
				case DirLeft:
					lr = DirRight
				case DirRight:
					lr = DirLeft
				}
			}
			dir = lr | ud
			p.Dir = dir
			continue // retry with bounced direction
		}

		// Slide around corners (disabled on recursive slide calls)
		if !slide {
			if y != 0 && g.Obstacle(p, 0, 0, &lx, &ny) {
				testX := lx + 3
				if x == 0 && !g.Obstacle(p, 0, 0, &testX, &ny) {
					return g.walk(p, DirRight, true)
				}
				testX = lx - 3
				if x == 0 && !g.Obstacle(p, 0, 0, &testX, &ny) {
					return g.walk(p, DirLeft, true)
				}
				ny = ly
			}
			if x != 0 && g.Obstacle(p, 0, 0, &nx, &ly) {
				testY := ly + 3
				if y == 0 && !g.Obstacle(p, 0, 0, &nx, &testY) {
					return g.walk(p, DirDown, true)
				}
				testY = ly - 3
				if y == 0 && !g.Obstacle(p, 0, 0, &nx, &testY) {
					return g.walk(p, DirUp, true)
				}
				nx = lx
			}
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
		p.Speed = max(0, p.Speed-g.SpeedDecayPxf)
	} else {
		base := p.BaseSpeed
		if p.Kind == Thug && g.Mode == 2 {
			base *= g.RelaxedSpeedX
		}
		p.Speed = max(base, p.Speed-g.SpeedDecayPxf)

		switch dir & (DirLeft | DirRight) {
		case DirLeft:
			p.Flip = true
		case DirRight:
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
func (g *Game) MakePing(x, y float64, color rl.Color) {
	g.Pings = append(g.Pings, Ping{X: x, Y: y, Radius: 0, Color: color})
}

// UpdatePings updates ping effects
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
		if (q.Kind&Friendly != 0 && q.X < FinishLine) || (q.Kind == Thug && !g.Playing) {
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
		if t != nil && t.Kind == Waif && !t.Spotted {
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
	if px >= FinishLine || g.Won {
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
			p.JitterX = float64(g.Rng.Intn(g.WaifJitterPx*2+1) - g.WaifJitterPx)
			p.JitterY = float64(g.Rng.Intn(g.WaifJitterPx*2+1) - g.WaifJitterPx)
			p.JitterTime = g.WaifJitterF
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
	if p.X > FinishLine && !p.Scored && (g.Playing || g.Won) {
		g.Score += g.WaifSafeScore
		p.Scored = true
		p.Ignore = true
		g.PlaySfx(g.SndSafe, "safe")
	} else if p.X > XMax {
		g.Reap(p)
	} else if g.Player != nil && Distance(p, g.Player) < g.WaifFollowPx && g.Player.Speed > 0 {
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
	s := (def.Speed - (def.Speed / (g.ThreatMax * g.ThreatSpeedX)))

	if g.Threat+t > g.ThreatMax*factor {
		return false
	}

	g.Emplace(def, x, y, func(obj *GameObject) {
		obj.Speed = s
		obj.BaseSpeed = s
		obj.Threat = t
		obj.XMaxLimit = FinishLine - obj.Width - obj.XOff
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

	g.MakePing(x, y, rl.White)

	palIndex := g.Rng.Intn(3)
	g.Emplace(g.Defs["waif"], x, y, func(obj *GameObject) {
		obj.PalIndex = palIndex
		obj.PalSwap = true
		obj.JitterTime = 0
		obj.Controller = FollowPath
		obj.PostMove = WaifPostMove
		obj.Delay = g.WaifDelayF
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
	idx := g.Rng.Intn(len(g.Thugs))
	def := g.Thugs[idx]

	x := g.CamX + float64(g.Rng.Intn(120-def.Width))
	var y float64
	if g.Playing {
		if g.Player != nil && g.Player.Y < 76 {
			y = float64(YMax - def.Height)
		}
	} else if g.Rng.Intn(2) == 1 {
		y = float64(YMax - def.Height)
	}

	if g.Spawn(def, x, y, 1) {
		g.MakePing(x, y, rl.Red)
	}
}

// randomFloorTile returns a black or accent tile index using rng
func (g *Game) randomFloorTile(rng *rand.Rand) int {
	n := rng.Float64() * 100
	if n < 90 || g.AccentTileCnt == 0 {
		return g.BlackTile
	}
	idx := min(int((101-n)/2), g.AccentTileCnt-1)
	return g.FirstAccentTile + idx
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

		g.TileMap[r][c] = g.randomFloorTile(g.Rng)

		// Only spawn objects outside nopop zone
		if c < px-nopL || c > px+nopR || r < py-nopU || r > py+nopD {
			added := false
			if g.Threat < g.ThreatMax*factor {
				idx := g.Rng.Intn(len(g.Thugs)-1) + 1 // skip mech (index 0) due to 16x16
				def := g.Thugs[idx]
				added = g.Spawn(def, float64(c*TileSize), float64(r*TileSize), factor)
			}
			if !added && r > 1 && r < 18 {
				n := g.Rng.Float64() * 100
				if n < g.CratePct {
					g.Emplace(g.Defs["crate"], float64(c*TileSize), float64(r*TileSize))
				} else if n < g.CratePct+g.RockPct {
					g.TileMap[r][c] = g.RockTile
				} else if n < g.CratePct+g.RockPct+g.JunkPct {
					g.Emplace(g.Defs["junk"], float64(c*TileSize), float64(r*TileSize))
				}
			}
		}
	}
}

// PopulateTitleBackground fills empty tiles with black/accent tiles for the title screen
func (g *Game) PopulateTitleBackground() {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for r := range g.TileMap {
		for c := range g.TileMap[r] {
			if g.TileMap[r][c] == 0 {
				g.TileMap[r][c] = g.randomFloorTile(rng)
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
			g.ThreatMax = g.ThreatMaxStart
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
		g.ThreatMax = min(g.ThreatMax+g.ThreatStep, g.ThreatCapHi)
	}
	if rl.IsKeyPressed(rl.KeyLeft) {
		g.ThreatMax = max(g.ThreatMax-g.ThreatStep, g.ThreatCapLo)
	}

	// Mouse drag on slider
	if rl.IsMouseButtonDown(rl.MouseLeftButton) {
		if mouse.Y >= float32(sliderY) && mouse.Y < float32(sliderY+sliderH) &&
			mouse.X >= float32(sliderX) && mouse.X <= float32(sliderX+sliderW) {
			raw := float64(mouse.X-float32(sliderX)) / float64(sliderW)
			g.ThreatMax = g.ThreatCapLo + raw*(g.ThreatCapHi-g.ThreatCapLo)
			g.ThreatMax = max(g.ThreatCapLo, min(g.ThreatCapHi, g.ThreatMax))
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

// UpdateShake updates screen shake
func (g *Game) UpdateShake() {
	if g.ShakeFactor > 0 {
		g.ShakeFactor = max(0, g.ShakeFactor-g.ShakeDecay)
	}
}

// StartGame initializes a new game
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
		g.ThreatMax = g.ThreatMaxStart
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
	g.Player = g.Emplace(g.Defs["player"], 64, 80, func(obj *GameObject) {
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
	rl.DrawRectangle(0, 0, g.WindowSize, g.WindowSize, OverlayDim)

	scale := float32(g.WindowSize) / 512.0
	fontSize := 28 * scale
	smallFont := 22 * scale

	// Title — show "PAUSED" only during gameplay
	if g.State == StatePlaying || g.State == StateGameOver {
		titleX := float32(g.WindowSize)/2 - 50*scale
		titleY := 12 * scale
		rl.DrawTextEx(g.Font, "PAUSED", rl.Vector2{X: titleX, Y: titleY}, 55*scale, 2, PicoOrange)
	} else {
		titleX := float32(g.WindowSize)/2 - 60*scale
		titleY := 12 * scale
		rl.DrawTextEx(g.Font, "OPTIONS", rl.Vector2{X: titleX, Y: titleY}, 55*scale, 2, PicoOrange)
	}

	for i := 0; i < optionsItemCount; i++ {
		menuX, y, menuW, rowH := g.menuItemRect(i)
		selected := i == g.MenuIndex

		// Highlight bar
		if selected {
			rl.DrawRectangle(menuX, y, menuW, rowH, HighlightOrange)
		}

		labelColor := PicoBrown
		valueColor := PicoOrange
		if selected {
			labelColor = PicoOrange
			valueColor = rl.White
		}

		lx := float32(menuX) + 10*scale
		ly := float32(y) + float32(rowH-int32(fontSize))/2

		switch i {
		case 0: // Volume slider
			rl.DrawTextEx(g.Font, "Volume", rl.Vector2{X: lx, Y: ly}, fontSize, 2, labelColor)
			sx, sy, sw, sh := g.volumeSliderRect()
			rl.DrawRectangle(sx, sy, sw, sh, SliderTrack)
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
			rl.DrawRectangle(sx, sy, sw, sh, SliderTrack)
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
			rl.DrawRectangle(sx, sy, sw, sh, SliderTrack)
			frac := float32((g.ThreatMax - g.ThreatCapLo) / (g.ThreatCapHi - g.ThreatCapLo))
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

// DrawCredits draws the credits overlay
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

// DrawTitle draws title screen text and menu
func (g *Game) DrawTitle() {
	scale := float32(g.WindowSize) / 512.0
	cx := float32(g.WindowSize)/2 + 48
	cy := float32(g.WindowSize)/2 - 48
	rl.DrawTextEx(g.Font, "HUNTED", rl.Vector2{X: cx - 120, Y: cy - 80 + 32}, 80, 2, PicoOrange)

	// Title menu
	titleLabels := []string{"Start", "Options", "Credits", "Quit"}
	for i := 0; i < titleMenuCount; i++ {
		menuX, y, menuW, rowH := g.titleMenuRect(i)
		selected := i == g.TitleMenuIndex

		if selected {
			rl.DrawRectangle(menuX, y, menuW, rowH, HighlightOrange)
		}

		color := PicoBrown
		if selected {
			color = PicoOrange
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
		// Randomized orange-yellow for spark shimmer
		color := rl.Color{R: 255, G: uint8(200 + g.Rng.Intn(55)), B: 0, A: 255}
		rl.DrawRectangle(int32(s.X), int32(s.Y), 2, 2, color)
	}

	// Draw pings
	for _, p := range g.Pings {
		rl.DrawCircleLines(int32(p.X), int32(p.Y), float32(p.Radius), p.Color)
	}
}

// DrawCrumbs draws breadcrumb trail as orange dots (debug&2)
func (g *Game) DrawCrumbs() {
	for _, c := range g.Crumbs {
		rl.DrawPixel(int32(c.X), int32(c.Y), PicoOrange)
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

// DrawBoundsRect draws the bounding box of an object (debug&4)
func (g *Game) DrawBoundsRect(obj *GameObject) {
	bx, by, bx2, by2 := obj.Bounds(nil, nil)
	rl.DrawRectangleLines(int32(bx), int32(by), int32(bx2-bx+1), int32(by2-by+1), PicoDarkGreen)
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
			PicoOrange,
		)
	}
}

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

// DrawUI draws the user interface in screen coordinates
func (g *Game) DrawUI() {
	scale := float32(g.WindowSize) / VisibleSize
	color := PicoOrange
	if !g.Playing {
		color = PicoBrown
	}

	// Score
	fontSize := float32(TileSize) * scale
	rl.DrawTextEx(g.Font, strconv.Itoa(g.Score), rl.Vector2{X: 2 * scale, Y: 0}, fontSize, 1, color)

	// Hearts
	heartSprite := g.Defs["heart"].Idle[0]
	for i := 1; i <= 3; i++ {
		x := float32(20 + i*8)
		sprIdx := heartSprite + 1 // empty heart
		if i < g.HeartCount {
			sprIdx = heartSprite
		}
		g.DrawSpriteUI(sprIdx, x, -1)
	}
	// Plus sign if hearts > 4
	if g.HeartCount > 4 {
		cx := (float32(20-1+4*8) + 0.5) * scale
		cy := (float32(1) + 0.5) * scale
		rl.DrawCircle(int32(cx), int32(cy), 1.5*scale, rl.White)
	}

	// Bombs
	bombSprite := g.Defs["bomb"].Idle[0]
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
	mineSprite := g.Defs["mine"].Idle[0]
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

// DrawGameOver draws game over screen in screen coordinates
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
