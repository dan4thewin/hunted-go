package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

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

	if g.MusicEnabled {
		g.PlaySong(g.SongIndex)
	}
}

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
	g.Music.Looping = false
	rl.SetMusicVolume(g.Music, g.MusicVolume)
	rl.PlayMusicStream(g.Music)
	g.MusicPlaying = true
	fmt.Printf("Now playing: %s\n", filepath.Base(g.Songs[g.SongIndex]))
}

func (g *Game) NextSong() {
	if len(g.Songs) == 0 {
		return
	}
	g.PlaySong(g.SongIndex + 1)
}

func (g *Game) StopMusic() {
	if g.MusicPlaying {
		rl.StopMusicStream(g.Music)
		rl.UnloadMusicStream(g.Music)
		g.MusicPlaying = false
		fmt.Println("Music stopped")
	}
}

func (g *Game) UpdateMusic() {
	if !g.MusicPlaying {
		return
	}
	rl.UpdateMusicStream(g.Music)
	// Auto-advance or repeat when current song finishes
	if !rl.IsMusicStreamPlaying(g.Music) {
		if g.SongRepeat {
			g.PlaySong(g.SongIndex)
		} else {
			g.NextSong()
		}
	}
}

func (g *Game) CleanupMusic() {
	if g.MusicPlaying {
		rl.StopMusicStream(g.Music)
		rl.UnloadMusicStream(g.Music)
	}
	g.UnloadSounds()
	rl.CloseAudioDevice()
}

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

func (g *Game) PlaySfx(s rl.Sound, name string) {
	if g.SoundsLoaded && g.SfxEnabled {
		rl.SetSoundVolume(s, g.SfxVolume)
		rl.PlaySound(s)
		g.Log("sfx:"+name, nil)
	}
}

func (g *Game) PlayExplosion() {
	if g.SoundsLoaded && g.Rng != nil && len(g.SndExplosion) > 0 {
		g.PlaySfx(g.SndExplosion[g.Rng.Intn(len(g.SndExplosion))], "explosion")
	}
}
