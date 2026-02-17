package main

import (
	"math/rand"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func (g *Game) NewID() int {
	g.NextID++
	return g.NextID
}

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
	if def == g.DefJunk && obj.FrameFunc == nil {
		obj.FrameFunc = func(o *GameObject) {
			o.CurrentFrames = max(6, g.Rng.Intn(240))
		}
	}

	// Start idling
	obj.SetIdle()

	g.Objects = append(g.Objects, obj)
	return obj
}

func (o *GameObject) SetIdle() {
	if o.IsIdle {
		return
	}
	o.IsIdle = true
	o.IsDash = false
	o.ResetPose(o.Idle, o.IdleFrames)
}

func (o *GameObject) SetMoving() {
	if !o.IsIdle && (!o.IsDash || o.Speed != o.BaseSpeed) {
		return
	}
	o.IsIdle = false
	o.IsDash = false
	o.ResetPose(o.Poses, o.Frames)
}

func (o *GameObject) SetDashing() {
	if o.IsDash {
		return
	}
	o.IsDash = true
	o.Speed = o.Speed2
	o.ResetPose(o.Poses2, o.Frames2)
}

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
	g.Emplace(g.DefWaif, x, y, func(obj *GameObject) {
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

func SlowStart(g *Game, p *GameObject) bool {
	p.Delay--
	if p.Delay == 0 {
		p.PreAct = nil
		p.YMin = TopEdge
		p.YMax = BottomEdge - 8
	}
	return false
}

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

func (g *Game) randomFloorTile(rng *rand.Rand) int {
	n := rng.Float64() * 100
	if n < 90 || g.AccentTileCnt == 0 {
		return g.BlackTile
	}
	idx := min(int((101-n)/2), g.AccentTileCnt-1)
	return g.FirstAccentTile + idx
}

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
		if c < px-nopL || c > px+nopR || r < py-nopU || r > py+nopD {
			g.populateTile(c, r, factor)
		}
	}
}

func (g *Game) populateTile(c, r int, factor float64) {
	if g.Threat < g.ThreatMax*factor {
		idx := g.Rng.Intn(len(g.Thugs)-1) + 1
		def := g.Thugs[idx]
		if g.Spawn(def, float64(c*TileSize), float64(r*TileSize), factor) {
			return
		}
	}
	if r > 1 && r < 18 {
		n := g.Rng.Float64() * 100
		if n < g.CratePct {
			g.Emplace(g.DefCrate, float64(c*TileSize), float64(r*TileSize))
		} else if n < g.CratePct+g.RockPct {
			g.TileMap[r][c] = g.RockTile
		} else if n < g.CratePct+g.RockPct+g.JunkPct {
			g.Emplace(g.DefJunk, float64(c*TileSize), float64(r*TileSize))
		}
	}
}

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
