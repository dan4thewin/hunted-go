package main

import (
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// optionsPath returns the full path to the options file
func (g *Game) optionsPath() string {
	return filepath.Join(g.Assets.SaveDir, "options.yaml")
}

// savePath returns the full path to the save game file
func (g *Game) savePath() string {
	return filepath.Join(g.Assets.SaveDir, "savegame.csv")
}

func (g *Game) SaveOptions() {
	var s SavedOptions
	s.Options.MusicVolume = g.MusicVolume
	s.Options.MusicEnabled = g.MusicEnabled
	s.Options.SfxVolume = g.SfxVolume
	s.Options.SfxEnabled = g.SfxEnabled
	s.Options.Mode = g.Mode
	s.Options.ThreatMax = g.ThreatMaxBase
	s.Options.Debug = g.Debug
	s.Options.HighScore = g.HighScore
	s.Options.SongIndex = g.SongIndex
	s.Options.SongRepeat = g.SongRepeat

	data, err := yaml.Marshal(&s)
	if err != nil {
		fmt.Printf("save options: %v\n", err)
		return
	}
	if err := os.MkdirAll(g.Assets.SaveDir, 0755); err != nil {
		fmt.Printf("save options: %v\n", err)
		return
	}
	if err := os.WriteFile(g.optionsPath(), data, 0644); err != nil {
		fmt.Printf("save options: %v\n", err)
	}
}

func (g *Game) LoadOptions() {
	data, err := os.ReadFile(g.optionsPath())
	if err != nil {
		return // no saved options yet
	}
	var s SavedOptions
	if err := yaml.Unmarshal(data, &s); err != nil {
		fmt.Printf("load options: %v\n", err)
		return
	}
	g.MusicVolume = s.Options.MusicVolume
	g.MusicEnabled = s.Options.MusicEnabled
	g.SfxVolume = s.Options.SfxVolume
	g.SfxEnabled = s.Options.SfxEnabled
	g.Mode = s.Options.Mode
	if s.Options.ThreatMax > 0 {
		g.ThreatMaxBase = s.Options.ThreatMax
		g.ThreatMax = s.Options.ThreatMax
	}
	g.Debug = s.Options.Debug
	g.HighScore = s.Options.HighScore
	g.SongIndex = s.Options.SongIndex
	g.SongRepeat = s.Options.SongRepeat
}

func (g *Game) HasSaveGame() bool {
	_, err := os.Stat(g.savePath())
	return err == nil
}

func (g *Game) SaveGame() {
	if err := os.MkdirAll(g.Assets.SaveDir, 0755); err != nil {
		fmt.Printf("save game: %v\n", err)
		return
	}
	f, err := os.Create(g.savePath())
	if err != nil {
		fmt.Printf("save game: %v\n", err)
		return
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	// Header + game state row
	w.Write([]string{
		"score", "hearts", "bombs", "mines", "threat_max", "threat",
		"level", "cam_x", "cam_y", "tick", "seed", "thug_count", "waif_count",
	})
	w.Write([]string{
		strconv.Itoa(g.Score),
		strconv.Itoa(g.HeartCount),
		strconv.Itoa(g.BombCount),
		strconv.Itoa(g.MineCount),
		strconv.FormatFloat(g.ThreatMax, 'f', -1, 64),
		strconv.FormatFloat(g.Threat, 'f', -1, 64),
		strconv.Itoa(g.Level),
		strconv.FormatFloat(g.CamX, 'f', -1, 64),
		strconv.FormatFloat(g.CamY, 'f', -1, 64),
		strconv.Itoa(g.Tick),
		strconv.FormatInt(g.Seed, 10),
		strconv.Itoa(g.ThugCount),
		strconv.Itoa(g.WaifCount),
	})

	// Object header
	w.Write([]string{
		"name", "x", "y", "speed", "base_speed", "dir", "last_dir", "flip",
		"armed", "ignore", "threat", "invuln_count", "is_invuln",
		"pal_index", "pal_swap", "pose_index", "frame_count",
		"current_sprite", "is_idle", "current_frames",
		"delay", "spotted", "scored",
		"jitter_x", "jitter_y", "jitter_time",
	})

	for _, obj := range g.Objects {
		// Only save player, thugs, and waifs
		if obj.Kind != Player && obj.Kind != Thug && obj.Kind != Waif {
			continue
		}
		w.Write([]string{
			obj.Name,
			strconv.FormatFloat(obj.X, 'f', -1, 64),
			strconv.FormatFloat(obj.Y, 'f', -1, 64),
			strconv.FormatFloat(obj.Speed, 'f', -1, 64),
			strconv.FormatFloat(obj.BaseSpeed, 'f', -1, 64),
			strconv.Itoa(obj.Dir),
			strconv.Itoa(obj.LastDir),
			strconv.FormatBool(obj.Flip),
			strconv.FormatBool(obj.Armed),
			strconv.FormatBool(obj.Ignore),
			strconv.FormatFloat(obj.Threat, 'f', -1, 64),
			strconv.FormatBool(obj.IsInvuln),
			strconv.Itoa(obj.PalIndex),
			strconv.FormatBool(obj.PalSwap),
			strconv.Itoa(obj.PoseIndex),
			strconv.Itoa(obj.FrameCount),
			strconv.Itoa(obj.CurrentSprite),
			strconv.FormatBool(obj.IsIdle),
			strconv.Itoa(obj.CurrentFrames),
			strconv.Itoa(obj.Delay),
			strconv.FormatBool(obj.Spotted),
			strconv.FormatBool(obj.Scored),
			strconv.FormatFloat(obj.JitterX, 'f', -1, 64),
			strconv.FormatFloat(obj.JitterY, 'f', -1, 64),
			strconv.Itoa(obj.JitterTime),
		})
	}
	fmt.Println("Game saved")
}

func (g *Game) LoadGame() error {
	f, err := os.Open(g.savePath())
	if err != nil {
		return err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // game state and object rows have different field counts
	rows, err := r.ReadAll()
	if err != nil {
		return fmt.Errorf("load game: %v", err)
	}
	if len(rows) < 4 {
		return fmt.Errorf("load game: file too short")
	}

	// Row 0 = game state header, Row 1 = game state values
	gs := rows[1]
	g.Score, _ = strconv.Atoi(gs[0])
	g.HeartCount, _ = strconv.Atoi(gs[1])
	g.BombCount, _ = strconv.Atoi(gs[2])
	g.MineCount, _ = strconv.Atoi(gs[3])
	g.ThreatMax, _ = strconv.ParseFloat(gs[4], 64)
	g.Threat, _ = strconv.ParseFloat(gs[5], 64)
	g.Level, _ = strconv.Atoi(gs[6])
	g.CamX, _ = strconv.ParseFloat(gs[7], 64)
	g.CamY, _ = strconv.ParseFloat(gs[8], 64)
	g.Tick, _ = strconv.Atoi(gs[9])
	g.Seed, _ = strconv.ParseInt(gs[10], 10, 64)
	g.ThugCount, _ = strconv.Atoi(gs[11])
	g.WaifCount, _ = strconv.Atoi(gs[12])
	g.Rng = rand.New(rand.NewSource(g.Seed))

	// Set up playing state
	g.State = StatePlaying
	g.Playing = true
	g.Won = false
	g.Respite = g.RespiteF
	g.ShakeFactor = 0
	g.Objects = nil
	g.Crumbs = nil
	g.Sparks = nil
	g.Pings = nil

	// Reset tiles back to base map
	for r := range g.TileMap {
		for c := range g.TileMap[r] {
			tile := g.TileMap[r][c]
			if tile != 0 && tile != g.EndzoneTile &&
				!(tile >= g.FirstWallTile && tile < g.FirstWallTile+g.WallTileCnt) {
				g.TileMap[r][c] = 0
			}
		}
	}

	// Reset path
	g.resetPathGrid()

	// Row 2 = object header, Row 3+ = objects
	for _, row := range rows[3:] {
		g.loadObject(row)
	}

	if g.Player != nil {
		g.UpdatePath(g.Player)
	}

	os.Remove(g.savePath())
	fmt.Println("Game loaded")
	return nil
}

func (g *Game) loadObject(row []string) {
	if len(row) < 26 {
		return
	}
	name := row[0]
	def := g.Defs[name]
	if def == nil {
		fmt.Printf("load game: unknown sprite %q, skipping\n", name)
		return
	}

	x, _ := strconv.ParseFloat(row[1], 64)
	y, _ := strconv.ParseFloat(row[2], 64)
	speed, _ := strconv.ParseFloat(row[3], 64)
	baseSpeed, _ := strconv.ParseFloat(row[4], 64)
	dir, _ := strconv.Atoi(row[5])
	lastDir, _ := strconv.Atoi(row[6])
	flip, _ := strconv.ParseBool(row[7])
	armed, _ := strconv.ParseBool(row[8])
	ignore, _ := strconv.ParseBool(row[9])
	threat, _ := strconv.ParseFloat(row[10], 64)
	isInvuln, _ := strconv.ParseBool(row[11])
	palIndex, _ := strconv.Atoi(row[12])
	palSwap, _ := strconv.ParseBool(row[13])
	poseIndex, _ := strconv.Atoi(row[14])
	frameCount, _ := strconv.Atoi(row[15])
	currentSprite, _ := strconv.Atoi(row[16])
	isIdle, _ := strconv.ParseBool(row[17])
	currentFrames, _ := strconv.Atoi(row[18])
	delay, _ := strconv.Atoi(row[19])
	spotted, _ := strconv.ParseBool(row[20])
	scored, _ := strconv.ParseBool(row[21])
	jitterX, _ := strconv.ParseFloat(row[22], 64)
	jitterY, _ := strconv.ParseFloat(row[23], 64)
	jitterTime, _ := strconv.Atoi(row[24])

	obj := g.Emplace(def, x, y)
	obj.Speed = speed
	obj.BaseSpeed = baseSpeed
	obj.Dir = dir
	obj.LastDir = lastDir
	obj.Flip = flip
	obj.Armed = armed
	obj.Ignore = ignore
	obj.Threat = threat
	obj.IsInvuln = isInvuln
	obj.PalIndex = palIndex
	obj.PalSwap = palSwap
	obj.PoseIndex = poseIndex
	obj.FrameCount = frameCount
	obj.CurrentSprite = currentSprite
	obj.IsIdle = isIdle
	obj.CurrentFrames = currentFrames
	obj.Delay = delay
	obj.Spotted = spotted
	obj.Scored = scored
	obj.JitterX = jitterX
	obj.JitterY = jitterY
	obj.JitterTime = jitterTime

	// Re-attach controllers based on Kind
	switch obj.Kind {
	case Player:
		obj.Controller = PlayerInput
		obj.PostMove = PlayerPostMove
		obj.PathC = -1
		obj.PathR = -1
		g.Player = obj
		if obj.IsInvuln {
			obj.PreAct = InvulnPreAct
		}
	case Thug:
		obj.Controller = Follow
		obj.XMaxLimit = FinishLine - obj.Width - obj.XOff
	case Waif:
		obj.Controller = FollowPath
		obj.PostMove = WaifPostMove
		if obj.Delay > 0 {
			obj.PreAct = SlowStart
		}
	}
}
