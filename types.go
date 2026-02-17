// Hunted - Go port of PICO-8 game
// Types, constants, and data structures
package main

import (
	"math/rand"

	rl "github.com/gen2brain/raylib-go/raylib"
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
	MusicEnabled bool // user toggle for music on/off
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
	SongRepeat   bool    // repeat current song instead of cycling
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
	State         GameState
	Playing       bool
	Won           bool
	Tick          int
	Score         int
	HighScore     int
	HeartCount    int
	BombCount     int
	MineCount     int
	CamX, CamY    float64
	ShakeFactor   float64
	Seed          int64
	Rng           *rand.Rand
	Debug         int
	DebugMsg      string
	Mode          int
	ThreatMax     float64
	ThreatMaxBase float64 // user's chosen base, saved to options
	Threat        float64
	ThugCount     int
	WaifCount     int
	Respite       int
	Level         int
	Populating    bool // suppress individual spawn sounds during bulk populate

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
	DefPlayer   *SpriteDef
	DefWaif     *SpriteDef
	DefBomb     *SpriteDef
	DefMine     *SpriteDef
	DefCrate    *SpriteDef
	DefJunk     *SpriteDef
	DefHeart    *SpriteDef
	Defs        map[string]*SpriteDef // name→def for save/load
	Thugs       []*SpriteDef
	StartHearts int
	StartBombs  int
	StartMines  int
	Assets      AssetsConfig
	SoundPaths  SoundsConfig
	GameConfig  // tunable params

	Audio      // music and sound effects
	Render     // graphics resources
	TileConfig // tile map data
	MenuState  // pause menu and overlays
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
	SaveDir       string `yaml:"save_dir"`
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

	DefaultMusicVolume float32 `yaml:"music_volume"`
	DefaultSfxVolume   float32 `yaml:"sfx_volume"`
	DefaultSongIndex   int     `yaml:"song_index"`
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

// SavedOptions is the set of user-adjustable settings persisted to options.yaml.
// Field layout mirrors config.yaml sections for readability.
type SavedOptions struct {
	Options struct {
		MusicVolume  float32 `yaml:"music_volume"`
		MusicEnabled bool    `yaml:"music_enabled"`
		SfxVolume    float32 `yaml:"sfx_volume"`
		SfxEnabled   bool    `yaml:"sfx_enabled"`
		Mode         int     `yaml:"mode"`
		ThreatMax    float64 `yaml:"threat_max"`
		Debug        int     `yaml:"debug"`
		HighScore    int     `yaml:"high_score"`
		SongIndex    int     `yaml:"song_index"`
		SongRepeat   bool    `yaml:"song_repeat"`
	} `yaml:"options"`
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

func cond[T any](b bool, t, f T) T {
	if b {
		return t
	}
	return f
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
