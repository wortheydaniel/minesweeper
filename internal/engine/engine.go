// Package engine implements the Minesweeper rules. It performs no I/O, never
// reads the clock (callers pass "now") and has no package-level randomness.
package engine

import (
	"errors"
	"math/bits"
	"math/rand/v2"
	"time"
)

// Board size limits for custom games.
const (
	MinW, MaxW = 9, 50
	MinH, MaxH = 9, 30
)

var (
	ErrConfig = errors.New("engine: invalid board configuration")
	ErrRange  = errors.New("engine: coordinates out of range")
	ErrOver   = errors.New("engine: game is over")
)

// Config describes a board.
type Config struct{ W, H, Mines int }

// Standard difficulty presets.
var (
	Beginner     = Config{9, 9, 10}
	Intermediate = Config{16, 16, 40}
	Expert       = Config{30, 16, 99}
)

// Validate checks the limits. Mines <= W*H-9 guarantees a safe 3x3 first click.
func (c Config) Validate() error {
	if c.W < MinW || c.W > MaxW || c.H < MinH || c.H > MaxH ||
		c.Mines < 1 || c.Mines > c.W*c.H-9 {
		return ErrConfig
	}
	return nil
}

// Phase is the game lifecycle state.
type Phase uint8

const (
	Ready Phase = iota
	Playing
	Won
	Lost
)

type mark uint8

const (
	markNone mark = iota
	markFlag
	markQuestion
)

// Kind is what a cell looks like to the player.
type Kind uint8

const (
	Hidden Kind = iota
	Flag
	Question
	Number    // revealed; Cell.N is the adjacent mine count (0-8)
	Mine      // shown after a loss
	Triggered // the mine(s) that ended the game
	WrongFlag // shown after a loss: flag on a non-mine
)

// Cell is the player-visible state of one square. While a game is running it
// never reveals where mines are.
type Cell struct {
	Kind Kind
	N    uint8
}

// Game is one round of Minesweeper.
type Game struct {
	cfg        Config
	seed       uint64
	mine, open []bool
	trig       []bool
	adj        []uint8
	mark       []mark
	phase      Phase
	flags      int
	left       int // safe cells still to reveal
	start, end time.Time
}

// New creates a game. Mines are placed on the first reveal using seed.
func New(cfg Config, seed uint64) (*Game, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	n := cfg.W * cfg.H
	return &Game{
		cfg: cfg, seed: seed,
		mine: make([]bool, n), open: make([]bool, n), trig: make([]bool, n),
		adj: make([]uint8, n), mark: make([]mark, n),
		left: n - cfg.Mines,
	}, nil
}

func (g *Game) Config() Config { return g.cfg }
func (g *Game) Phase() Phase   { return g.phase }
func (g *Game) Seed() uint64   { return g.seed }

// MinesLeft is mines minus flags placed (may be negative).
func (g *Game) MinesLeft() int { return g.cfg.Mines - g.flags }

// Elapsed is the play time: zero before the first reveal, frozen at the end.
func (g *Game) Elapsed(now time.Time) time.Duration {
	switch {
	case g.phase == Ready:
		return 0
	case g.phase >= Won:
		return g.end.Sub(g.start)
	}
	return now.Sub(g.start)
}

func (g *Game) idx(x, y int) (int, error) {
	if x < 0 || y < 0 || x >= g.cfg.W || y >= g.cfg.H {
		return 0, ErrRange
	}
	return y*g.cfg.W + x, nil
}

// Cell returns what the player sees at (x, y); out-of-range yields Hidden.
func (g *Game) Cell(x, y int) Cell {
	i, err := g.idx(x, y)
	if err != nil {
		return Cell{}
	}
	if g.open[i] {
		return Cell{Number, g.adj[i]}
	}
	if g.phase == Lost {
		switch {
		case g.trig[i]:
			return Cell{Kind: Triggered}
		case g.mine[i] && g.mark[i] != markFlag:
			return Cell{Kind: Mine}
		case !g.mine[i] && g.mark[i] == markFlag:
			return Cell{Kind: WrongFlag}
		}
	}
	switch g.mark[i] {
	case markFlag:
		return Cell{Kind: Flag}
	case markQuestion:
		return Cell{Kind: Question}
	}
	return Cell{}
}

// Reveal opens a cell. Flagged and already-open cells are a no-op.
func (g *Game) Reveal(x, y int, now time.Time) error {
	i, err := g.idx(x, y)
	if err != nil {
		return err
	}
	if g.phase >= Won {
		return ErrOver
	}
	if g.open[i] || g.mark[i] == markFlag {
		return nil
	}
	if g.phase == Ready {
		g.place(x, y)
		g.phase, g.start = Playing, now
	}
	if g.mine[i] {
		g.trig[i] = true
		g.finish(Lost, now)
		return nil
	}
	g.flood(i)
	g.checkWin(now)
	return nil
}

// ToggleFlag cycles hidden -> flag -> (question ->) hidden.
func (g *Game) ToggleFlag(x, y int, useQuestion bool) error {
	i, err := g.idx(x, y)
	if err != nil {
		return err
	}
	if g.phase >= Won {
		return ErrOver
	}
	if g.open[i] {
		return nil
	}
	switch g.mark[i] {
	case markNone:
		g.mark[i] = markFlag
		g.flags++
	case markFlag:
		g.flags--
		g.mark[i] = markNone
		if useQuestion {
			g.mark[i] = markQuestion
		}
	default:
		g.mark[i] = markNone
	}
	return nil
}

// Chord opens every unflagged neighbour of a revealed number whose adjacent
// flag count equals the number. A wrong flag therefore loses the game.
func (g *Game) Chord(x, y int, now time.Time) error {
	i, err := g.idx(x, y)
	if err != nil {
		return err
	}
	if g.phase >= Won {
		return ErrOver
	}
	if !g.open[i] || g.adj[i] == 0 {
		return nil
	}
	var nb [8]int
	ns := g.neighbours(i, nb[:0])
	flags := 0
	for _, j := range ns {
		if g.mark[j] == markFlag {
			flags++
		}
	}
	if flags != int(g.adj[i]) {
		return nil
	}
	hit := false
	for _, j := range ns {
		if g.open[j] || g.mark[j] == markFlag {
			continue
		}
		if g.mine[j] {
			g.trig[j], hit = true, true
		} else {
			g.flood(j)
		}
	}
	if hit {
		g.finish(Lost, now)
	} else {
		g.checkWin(now)
	}
	return nil
}

func (g *Game) finish(p Phase, now time.Time) { g.phase, g.end = p, now }

func (g *Game) checkWin(now time.Time) {
	if g.left > 0 {
		return
	}
	for i, m := range g.mine {
		if m {
			g.mark[i] = markFlag
		}
	}
	g.flags = g.cfg.Mines
	g.finish(Won, now)
}

// neighbours appends the in-bounds 8-neighbours of cell i to buf.
func (g *Game) neighbours(i int, buf []int) []int {
	w, h := g.cfg.W, g.cfg.H
	x, y := i%w, i/w
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			nx, ny := x+dx, y+dy
			if (dx != 0 || dy != 0) && nx >= 0 && ny >= 0 && nx < w && ny < h {
				buf = append(buf, ny*w+nx)
			}
		}
	}
	return buf
}

// flood opens cell i and, for zeros, everything connected to it (iterative).
func (g *Game) flood(i int) {
	stack := []int{i}
	var nb [8]int
	for len(stack) > 0 {
		j := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if g.open[j] || g.mark[j] == markFlag {
			continue
		}
		g.open[j] = true
		g.left--
		g.mark[j] = markNone
		if g.adj[j] == 0 {
			for _, k := range g.neighbours(j, nb[:0]) {
				if !g.open[k] {
					stack = append(stack, k)
				}
			}
		}
	}
}

// place lays out the mines, keeping the 3x3 block around (fx, fy) clear.
func (g *Game) place(fx, fy int) {
	w, h := g.cfg.W, g.cfg.H
	cand := make([]int, 0, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if abs(x-fx) > 1 || abs(y-fy) > 1 {
				cand = append(cand, y*w+x)
			}
		}
	}
	src := rand.NewPCG(g.seed, g.seed^0x9e3779b97f4a7c15)
	for k := 0; k < g.cfg.Mines; k++ {
		j := k + int(bounded(src, uint64(len(cand)-k)))
		cand[k], cand[j] = cand[j], cand[k]
		g.mine[cand[k]] = true
	}
	var nb [8]int
	for i := range g.adj {
		for _, j := range g.neighbours(i, nb[:0]) {
			if g.mine[j] {
				g.adj[i]++
			}
		}
	}
}

// bounded returns an unbiased value in [0, n) (Lemire's method). It is written
// out here, rather than using rand.IntN, so layouts for a given seed cannot
// change when Go's library does.
func bounded(src *rand.PCG, n uint64) uint64 {
	hi, lo := bits.Mul64(src.Uint64(), n)
	if lo < n {
		t := -n % n
		for lo < t {
			hi, lo = bits.Mul64(src.Uint64(), n)
		}
	}
	return hi
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
