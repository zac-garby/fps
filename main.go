package main

import (
	"fmt"
	"math"
	"runtime"
	"strings"

	"github.com/veandco/go-sdl2/img"

	"github.com/veandco/go-sdl2/sdl"
)

const (
	focalLength float64 = 0.8
)

var (
	textures map[string]*sdl.Texture = make(map[string]*sdl.Texture)
)

// A Tile is used to store the type of a particular tile in the level.
type Tile = int

const (
	empty Tile = 0
	wall1 Tile = 1
	wall2 Tile = 2
)

// A Map stores the set of tiles which make up a game level.
type Map = [][]Tile

// An Entity represents a sprite to be drawn in the world.
type Entity struct {
	X, Y, Width float64
	Texture     *sdl.Texture
}

func render(renderer *sdl.Renderer, level Map, entities []*Entity, xs, ys, angle float64) {
	w, h, err := renderer.GetOutputSize()
	if err != nil {
		panic(err)
	}

	var (
		width    = float64(w)
		height   = float64(h)
		gap      = 16.0
		depthMap = make([]float64, w)
	)

	for sweep := 0.0; sweep < 1; sweep += gap / width {
		var (
			screenX         = int32(sweep * width)
			r               = (2*sweep - 1) / (2 * focalLength)
			tempxd          = r / math.Sqrt(r*r+1)
			tempyd          = 1 / math.Sqrt(r*r+1)
			cos             = math.Cos(angle)
			sin             = math.Sin(angle)
			xd              = tempxd*cos - tempyd*sin
			yd              = tempyd*cos + tempxd*sin
			closestDistance = math.Inf(1)
			closestMu       = math.NaN()
			closestTile     Tile
		)

		for y := 0; y < len(level); y++ {
			row := level[y]
			for x := 0; x < len(row); x++ {
				if row[x] == empty {
					continue
				}

				dist, mu, hit := rayBox(xs, ys, xd, yd, float64(x), float64(y))
				if hit && dist < closestDistance {
					closestDistance = dist
					closestMu = mu
					closestTile = row[x]
				}
			}
		}

		depthMap[screenX] = closestDistance

		if !math.IsInf(closestDistance, 1) {
			sliceHeight := int32(
				math.Round(height/(closestDistance*tempyd)/gap) * gap,
			)

			dest := &sdl.Rect{
				X: int32(screenX),
				Y: int32(height/2) - sliceHeight/2,
				W: int32(gap),
				H: sliceHeight,
			}

			tex := textureFor(closestTile)

			_, _, tw, th, _ := tex.Query()

			src := &sdl.Rect{
				X: int32(closestMu * float64(tw)),
				Y: 1,
				W: 1,
				H: th,
			}

			var (
				torchMul   = math.Pow(4*sweep*(1-sweep), 1.4)
				brightness = uint8(math.Min(255*torchMul/math.Pow(closestDistance, 1.5), 255))
			)
			if closestDistance < 1 {
				brightness = 255
			}

			tex.SetColorMod(brightness, brightness, brightness)
			renderer.Copy(tex, src, dest)
		}
	}
}

func rayLine(xs, ys, xd, yd, xc1, yc1, xc2, yc2 float64) (float64, float64, bool) {
	lambda := (xs*yc1 + xc1*yc2 + xc2*ys - xs*yc2 - xc1*ys - xc2*yc1) / (xd*yc1 + xc2*yd - xd*yc2 - xc1*yd)
	if lambda <= 0 {
		return 0, 0, false
	}

	mu := (xd*yc1 + xs*yd - xc1*yd - xd*ys) / (xd*yc2 + xc1*yd - xd*yc1 - xc2*yd)
	if mu < 0 || mu > 1 {
		return 0, 0, false
	}

	dist := lambda / math.Sqrt(xd*xd+yd*yd)

	return dist, mu, true
}

func rayBox(xs, ys, xd, yd, cx, cy float64) (float64, float64, bool) {
	sides := [][4]float64{
		{cx, cy, cx + 1, cy},
		{cx, cy, cx, cy + 1},
		{cx, cy - 1, cx + 1, cy - 1},
		{cx - 1, cy, cx - 1, cy + 1},
	}

	var (
		closest   = 0.0
		closestMu = 0.0
		didHit    = false
	)

	for _, side := range sides {
		dist, mu, hit := rayLine(xs, ys, xd, yd, side[0], side[1], side[2], side[3])
		if !hit {
			continue
		}

		if !didHit || dist < closest {
			closest = dist
			closestMu = mu
			didHit = true
		}
	}

	return closest, closestMu, didHit
}

func mapFromString(str string) Map {
	var (
		lines = strings.Split(str, "\n")
		m     = make(Map, len(lines))
	)

	for j, line := range strings.Split(str, "\n") {
		m[j] = make([]Tile, len(line))

		for i, char := range line {
			var t Tile
			switch char {
			case '1':
				t = wall1
			case '2':
				t = wall2
			default:
				t = empty
			}

			m[j][i] = t
		}
	}

	return m
}

func main() {
	runtime.LockOSThread()

	level := mapFromString(`
 111111111111111111111111111111111
 1                               1
 1                               2
 1                               1
 1      11111111111111111111111111
 1                               1
 1                               1
 1                               1
 1                               1
 1                               1
 1                               1
 1                               1
 1                               1
 1                               1
 1                               1
 1                               1
 1                               1
 111111111111111111111111111111111`,
	)

	entities := []*Entity{}

	var (
		x, y     = 2.5, 2.0
		angle    = 0.0
		toTurn   = 0.0
		bobTimer = 0.0
	)

	var version sdl.Version
	sdl.GetVersion(&version)
	fmt.Printf("running SDL %d.%d.%d", version.Major, version.Minor, version.Patch)

	if err := sdl.Init(sdl.INIT_EVERYTHING); err != nil {
		panic(err)
	}
	defer sdl.Quit()

	img.Init(img.INIT_PNG)
	defer img.Quit()

	window, renderer, err := sdl.CreateWindowAndRenderer(512, 512, sdl.WINDOW_SHOWN)
	if err != nil {
		panic(err)
	}
	defer window.Destroy()
	defer renderer.Destroy()

	window.SetTitle("fps")

	loadTextures(renderer)

	running := true
	for running {
		for evt := sdl.PollEvent(); evt != nil; evt = sdl.PollEvent() {
			switch s := evt.(type) {
			case *sdl.QuitEvent:
				running = false
				break

			case *sdl.KeyboardEvent:
				if s.State == sdl.PRESSED && s.Keysym.Scancode == sdl.SCANCODE_Q {
					toTurn = math.Pi
				}
			}
		}

		if toTurn > 0.005 {
			angle += 0.013
			toTurn -= 0.013
		}

		keys := sdl.GetKeyboardState()
		if keys[sdl.SCANCODE_LEFT] == 1 {
			angle += 0.002
		}

		if keys[sdl.SCANCODE_RIGHT] == 1 {
			angle -= 0.002
		}

		moving := false

		speed := 0.002
		if keys[sdl.SCANCODE_LSHIFT] == 1 {
			speed = 0.005
			if keys[sdl.SCANCODE_UP] == 1 ||
				keys[sdl.SCANCODE_W] == 1 ||
				keys[sdl.SCANCODE_DOWN] == 1 ||
				keys[sdl.SCANCODE_S] == 1 ||
				keys[sdl.SCANCODE_A] == 1 ||
				keys[sdl.SCANCODE_D] == 1 {
				bobTimer += 0.01
			}
		}

		if keys[sdl.SCANCODE_UP] == 1 || keys[sdl.SCANCODE_W] == 1 {
			y -= speed * math.Cos(angle)
			x += speed * math.Sin(angle)
			moving = true
		}
		if keys[sdl.SCANCODE_DOWN] == 1 || keys[sdl.SCANCODE_S] == 1 {
			y += speed * math.Cos(angle)
			x -= speed * math.Sin(angle)
			moving = true
		}

		if keys[sdl.SCANCODE_D] == 1 {
			y += speed * math.Cos(angle+math.Pi/2)
			x -= speed * math.Sin(angle+math.Pi/2)
			moving = true
		}
		if keys[sdl.SCANCODE_A] == 1 {
			y -= speed * math.Cos(angle+math.Pi/2)
			x += speed * math.Sin(angle+math.Pi/2)
			moving = true
		}

		if moving {
			bobTimer += 0.016
		}

		renderer.SetDrawColor(0, 0, 0, 255)
		renderer.Clear()

		render(renderer, level, entities, x, y, angle)

		var (
			boby = int32(math.Abs(math.Sin(bobTimer*0.3) * 32))
			bobx = int32(math.Cos(bobTimer*0.3) * 20)
		)

		renderer.Copy(textures["shotgun"], nil, &sdl.Rect{X: 512 - 384 + bobx, Y: 512 - 384 + boby, W: 384, H: 384})

		renderer.Present()
	}
}

func loadTextures(renderer *sdl.Renderer) {
	toLoad := []string{
		"shotgun",
		"wall",
		"wall-2",
	}

	for _, name := range toLoad {
		tex, err := img.LoadTexture(renderer, fmt.Sprintf("assets/%s.png", name))
		if err != nil {
			panic(err)
		}

		textures[name] = tex
	}
}

func textureFor(t Tile) *sdl.Texture {
	switch t {
	case wall1:
		return textures["wall"]
	case wall2:
		return textures["wall-2"]
	default:
		panic(fmt.Sprintf("undefined tile when getting texture: %d", t))
	}
}
