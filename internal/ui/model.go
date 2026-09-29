// Package ui is the whole game front-end: state, input handling and drawing.
// It is deliberately independent of any windowing library. A shell feeds it
// Input snapshots and shows the *image.RGBA frame it produces, so everything
// here can be tested without a display.
package ui

import (
	"image"
	"math/rand/v2"
	"time"

	"github.com/wortheydaniel/minesweeper/internal/engine"
	"github.com/wortheydaniel/minesweeper/internal/store"
)

// Key is a keyboard key the game reacts to.
type Key int

const (
	KeyLeft Key = iota + 1
	KeyRight
	KeyUp
	KeyDown
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeySpace
	KeyEnter
	KeyEscape
	KeyF
	KeyC
	KeyF2
	KeyF10
	KeyTab
	KeyBackspace
)

// Input is everything the model needs to know about the outside world for one
// tick. Pointer coordinates are in logical pixels (X, Y may lie outside the
// frame). Keys are presses this tick, including auto-repeat.
type Input struct {
	X, Y                int
	Left, Middle, Right bool
	Keys                []Key
	Text                []rune
	Focused             bool
	Now                 time.Time
}

// Options configure a Model.
type Options struct {
	Store   *store.Store
	Seed    *uint64 // fixed base seed for reproducible games; nil = random
	Scale   int     // 1-4 forces the window scale; 0 = use the saved setting
	Version string
}

// gesture tracks a mouse press from first button down to last button up.
type gesture struct {
	active, face bool
	l, m, r      bool
	cx, cy       int
	in           bool // pointer is over a board cell
}

func (g gesture) chord() bool { return g.m || (g.l && g.r) }

// Model is the game UI state machine.
type Model struct {
	opt    Options
	st     *store.Store
	cfg    engine.Config
	preset string
	g      *engine.Game
	gameNo uint64

	lay layout
	pal *palette

	fx, fy int // keyboard focus cell
	fshow  bool
	ges    gesture
	prev   Input
	now    time.Time

	menu, msel int // open menu (-1 none) and highlighted item
	dlg        *dialog

	img   *image.RGBA
	dirty bool
	secs  int
	quit  bool
}

// New creates a model showing a fresh game of the last-used board size.
func New(opt Options) *Model {
	m := &Model{opt: opt, st: opt.Store, menu: -1, msel: -1, dirty: true, secs: -1}
	b := m.st.Data.LastBoard
	m.cfg = engine.Config{W: b.W, H: b.H, Mines: b.Mines}
	if m.cfg.Validate() != nil {
		m.cfg = engine.Beginner
	}
	m.pal = paletteFor(m.st.Data.Settings.Theme)
	m.newGame()
	return m
}

func presetName(c engine.Config) string {
	switch c {
	case engine.Beginner:
		return "beginner"
	case engine.Intermediate:
		return "intermediate"
	case engine.Expert:
		return "expert"
	}
	return "custom"
}

func splitmix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

func (m *Model) newGame() {
	var seed uint64
	if m.opt.Seed != nil {
		seed = splitmix(*m.opt.Seed + m.gameNo)
	} else {
		seed = rand.Uint64()
	}
	m.gameNo++
	m.g, _ = engine.New(m.cfg, seed) // cfg is always validated before it gets here
	m.preset = presetName(m.cfg)
	if l := computeLayout(m.cfg.W, m.cfg.H); l != m.lay {
		m.lay, m.img = l, nil
	}
	m.fx, m.fy, m.fshow = m.cfg.W/2, m.cfg.H/2, false
	m.ges = gesture{}
	m.secs = -1
	m.dirty = true
}

func (m *Model) setBoard(c engine.Config) {
	m.cfg = c
	m.st.Data.LastBoard = store.Board{W: c.W, H: c.H, Mines: c.Mines}
	m.st.Save()
	m.newGame()
}

// ---- accessors for the shell ----------------------------------------------

// Size is the logical frame size in pixels.
func (m *Model) Size() (w, h int) { return m.lay.w, m.lay.h }

// ScaleSetting is 0 for automatic, or a forced scale of 1-4.
func (m *Model) ScaleSetting() int {
	if m.opt.Scale > 0 {
		return m.opt.Scale
	}
	return m.st.Data.Settings.Scale
}

// Phase reports the current game's phase.
func (m *Model) Phase() engine.Phase { return m.g.Phase() }

// CellCenter is the pixel at the middle of board cell (x, y).
func (m *Model) CellCenter(x, y int) (int, int) {
	ox, oy := m.lay.cellOrigin(x, y)
	return ox + cellSize/2, oy + cellSize/2
}

// Frame returns the current picture and whether it changed since last call.
func (m *Model) Frame() (*image.RGBA, bool) {
	if !m.dirty && m.img != nil {
		return m.img, false
	}
	m.draw()
	m.dirty = false
	return m.img, true
}

// ---- update ---------------------------------------------------------------

// Update advances the model by one tick. It returns true when the game should
// exit.
func (m *Model) Update(in Input) bool {
	m.now = in.Now
	if !in.Focused && (m.ges.active || m.ges.face) { // lost focus mid-click: cancel
		m.ges = gesture{}
		m.dirty = true
	}
	for _, k := range in.Keys {
		m.key(k)
		m.dirty = true
	}
	for _, r := range in.Text {
		m.char(r)
		m.dirty = true
	}
	m.mouse(in, m.prev)
	if s := m.displaySecs(); s != m.secs {
		m.secs, m.dirty = s, true
	}
	m.prev = in
	return m.quit
}

func (m *Model) displaySecs() int {
	s := int(m.g.Elapsed(m.now) / time.Second)
	if s > 999 {
		s = 999
	}
	return s
}

// ---- keyboard -------------------------------------------------------------

func (m *Model) key(k Key) {
	switch {
	case m.dlg != nil:
		m.dlgKey(k)
	case m.menu >= 0:
		m.menuKey(k)
	default:
		m.gameKey(k)
	}
}

func (m *Model) gameKey(k Key) {
	w, h := m.cfg.W, m.cfg.H
	move := func(dx, dy int) {
		m.fx = clamp(m.fx+dx, 0, w-1)
		m.fy = clamp(m.fy+dy, 0, h-1)
		m.fshow = true
	}
	switch k {
	case KeyLeft:
		move(-1, 0)
	case KeyRight:
		move(1, 0)
	case KeyUp:
		move(0, -1)
	case KeyDown:
		move(0, 1)
	case KeyHome:
		move(-w, 0)
	case KeyEnd:
		move(w, 0)
	case KeyPageUp:
		move(0, -h)
	case KeyPageDown:
		move(0, h)
	case KeySpace, KeyEnter:
		m.fshow = true
		m.primary(m.fx, m.fy)
	case KeyF:
		m.fshow = true
		m.flag(m.fx, m.fy)
	case KeyC:
		m.fshow = true
		m.chord(m.fx, m.fy)
	case KeyF2:
		m.newGame()
	case KeyF10:
		m.openMenu(0)
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---- game actions ---------------------------------------------------------

func (m *Model) act(f func() error) {
	before := m.g.Phase()
	_ = f() // ErrOver after the end of a game is expected and harmless
	if before < engine.Won && m.g.Phase() == engine.Won {
		m.onWin()
	}
	m.dirty = true
}

// primary is a left click or Space: reveal, or chord on a number if enabled.
func (m *Model) primary(x, y int) {
	if c := m.g.Cell(x, y); c.Kind == engine.Number && m.st.Data.Settings.ClickNumberChords {
		m.chord(x, y)
		return
	}
	m.act(func() error { return m.g.Reveal(x, y, m.now) })
}

func (m *Model) flag(x, y int) {
	m.act(func() error { return m.g.ToggleFlag(x, y, m.st.Data.Settings.QuestionMarks) })
}

func (m *Model) chord(x, y int) {
	m.act(func() error { return m.g.Chord(x, y, m.now) })
}

func (m *Model) onWin() {
	if m.preset == "custom" {
		return
	}
	ms := m.g.Elapsed(m.now).Milliseconds()
	if rank := m.st.AddRecord(m.preset, ms, m.now); rank > 0 {
		m.dlg = newNameDialog(m.preset, rank, ms, m.st.Data.LastName)
	}
}

// ---- mouse ----------------------------------------------------------------

func (m *Model) mouse(in, p Input) {
	moved := in.X != p.X || in.Y != p.Y
	leftPress := in.Left && !p.Left
	anyDown := in.Left || in.Middle || in.Right
	wasDown := p.Left || p.Middle || p.Right
	pt := image.Pt(in.X, in.Y)

	switch {
	case m.dlg != nil:
		if leftPress {
			m.dlgClick(pt)
			m.dirty = true
		}
		return
	case m.menu >= 0:
		m.menuMouse(pt, moved, leftPress)
		return
	}

	if leftPress && in.Y >= 0 && in.Y < menuH {
		if i := m.menuBarHit(in.X); i >= 0 {
			m.openMenu(i)
		}
		return
	}

	press := leftPress || (in.Middle && !p.Middle) || (in.Right && !p.Right)
	if press && !m.ges.active && !m.ges.face {
		if leftPress && pt.In(m.lay.face) {
			m.ges = gesture{face: true}
			m.dirty = true
		} else if _, _, ok := m.lay.cellAt(in.X, in.Y); ok {
			m.ges = gesture{active: true}
			m.dirty = true
		}
	}
	if !m.ges.active && !m.ges.face {
		return
	}
	g := &m.ges
	g.l, g.m, g.r = g.l || in.Left, g.m || in.Middle, g.r || in.Right
	x, y, ok := m.lay.cellAt(in.X, in.Y)
	if moved || x != g.cx || y != g.cy || ok != g.in {
		m.dirty = true
	}
	g.cx, g.cy, g.in = x, y, ok
	if anyDown || !wasDown {
		return
	}
	// all buttons released: apply the gesture
	done := *g
	m.ges = gesture{}
	m.dirty = true
	switch {
	case done.face:
		if pt.In(m.lay.face) {
			m.newGame()
		}
	case done.in:
		m.fx, m.fy, m.fshow = done.cx, done.cy, false
		switch {
		case done.chord():
			m.chord(done.cx, done.cy)
		case done.l:
			m.primary(done.cx, done.cy)
		case done.r:
			m.flag(done.cx, done.cy)
		}
	}
}
