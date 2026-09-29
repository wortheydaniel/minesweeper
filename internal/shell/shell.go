// Package shell is the only code that touches Ebitengine. It opens the
// window, turns Ebitengine's input into ui.Input, and shows the RGBA frame the
// ui.Model draws. Keep it thin: replacing Ebitengine should mean rewriting
// just this package.
package shell

import (
	"errors"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/wortheydaniel/minesweeper/internal/engine"
	"github.com/wortheydaniel/minesweeper/internal/ui"
)

// Config for Run.
type Config struct {
	Model *ui.Model
	Smoke bool // self-test: click once, run ~30 frames, exit
}

var keyMap = []struct {
	e ebiten.Key
	k ui.Key
}{
	{ebiten.KeyArrowLeft, ui.KeyLeft}, {ebiten.KeyArrowRight, ui.KeyRight},
	{ebiten.KeyArrowUp, ui.KeyUp}, {ebiten.KeyArrowDown, ui.KeyDown},
	{ebiten.KeyHome, ui.KeyHome}, {ebiten.KeyEnd, ui.KeyEnd},
	{ebiten.KeyPageUp, ui.KeyPageUp}, {ebiten.KeyPageDown, ui.KeyPageDown},
	{ebiten.KeySpace, ui.KeySpace}, {ebiten.KeyEnter, ui.KeyEnter},
	{ebiten.KeyNumpadEnter, ui.KeyEnter}, {ebiten.KeyEscape, ui.KeyEscape},
	{ebiten.KeyF, ui.KeyF}, {ebiten.KeyC, ui.KeyC},
	{ebiten.KeyF2, ui.KeyF2}, {ebiten.KeyF10, ui.KeyF10},
	{ebiten.KeyTab, ui.KeyTab}, {ebiten.KeyBackspace, ui.KeyBackspace},
}

// Key auto-repeat, in ticks (60 per second): 0.4 s delay, then 15 per second.
const (
	repeatDelay = 24
	repeatEvery = 4
)

type game struct {
	m     *ui.Model
	smoke bool

	tex          *ebiten.Image
	texW, texH   int
	winW, winH   int
	scale        int
	frames       int
	lastDraw     time.Time
	smokeFailure error
}

// idleFrame is the shortest time between frames while nothing is changing.
// With vsync (normal desktops) frames are already longer than this and it has
// no effect; without vsync (VMs, remote desktops) it stops the loop spinning.
const idleFrame = 25 * time.Millisecond

// Run opens the window and blocks until the game ends.
func Run(cfg Config) error {
	g := &game{m: cfg.Model, smoke: cfg.Smoke}
	w, h := g.m.Size()
	ebiten.SetWindowTitle("Minesweeper")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeDisabled)
	ebiten.SetWindowSize(w*2, h*2)
	g.scale = 2
	if err := ebiten.RunGame(g); err != nil {
		return err
	}
	return g.smokeFailure
}

func (g *game) Layout(int, int) (int, int) { return g.m.Size() }

// autoScale picks a whole-number scale that suits the monitor: roughly one
// step per 540 device-independent pixels of height, shrunk until it fits.
func autoScale(w, h int) int {
	mw, mh := ebiten.Monitor().Size()
	s := mh / 540
	if s < 1 {
		s = 1
	}
	if s > 4 {
		s = 4
	}
	for s > 1 && (w*s > mw*9/10 || h*s > mh*9/10) {
		s--
	}
	return s
}

func (g *game) input() ui.Input {
	x, y := ebiten.CursorPosition()
	in := ui.Input{
		X: x, Y: y,
		Left:    ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft),
		Middle:  ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle),
		Right:   ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight),
		Text:    ebiten.AppendInputChars(nil),
		Focused: ebiten.IsFocused(),
		Now:     time.Now(),
	}
	for _, km := range keyMap {
		d := inpututil.KeyPressDuration(km.e)
		if d == 1 || (d > repeatDelay && (d-repeatDelay)%repeatEvery == 0) {
			in.Keys = append(in.Keys, km.k)
		}
	}
	return in
}

// scripted gives --smoke a fixed input: click the middle of the board.
func (g *game) scripted(in ui.Input) ui.Input {
	switch g.frames {
	case 5, 6, 7:
		in.X, in.Y = g.m.CellCenter(4, 4)
		in.Left = g.frames != 7
		in.Keys, in.Text = nil, nil
	}
	return in
}

func (g *game) Update() error {
	// Keep the window matched to the model's size and scale.
	w, h := g.m.Size()
	s := g.m.ScaleSetting()
	if s == 0 {
		s = autoScale(w, h)
	}
	if w != g.winW || h != g.winH || s != g.scale {
		ebiten.SetWindowSize(w*s, h*s)
		g.winW, g.winH, g.scale = w, h, s
	}

	in := g.input()
	if g.smoke {
		in = g.scripted(in)
	}
	if g.m.Update(in) {
		return ebiten.Termination
	}

	if g.smoke {
		g.frames++
		if g.frames >= 30 {
			if g.m.Phase() == engine.Ready {
				g.smokeFailure = errors.New("smoke test: the scripted click did not start a game")
			}
			return ebiten.Termination
		}
	}
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	img, changed := g.m.Frame()
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if g.tex == nil || g.texW != w || g.texH != h {
		g.tex, g.texW, g.texH = ebiten.NewImage(w, h), w, h
		changed = true
	}
	if changed {
		g.tex.WritePixels(img.Pix)
	} else if wait := idleFrame - time.Since(g.lastDraw); wait > 0 {
		time.Sleep(wait)
	}
	g.lastDraw = time.Now()
	screen.DrawImage(g.tex, nil)
}
