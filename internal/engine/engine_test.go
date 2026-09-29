package engine

import (
	"hash/fnv"
	"testing"
	"time"
)

var t0 = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func mustNew(t *testing.T, c Config, seed uint64) *Game {
	t.Helper()
	g, err := New(c, seed)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestConfigBounds(t *testing.T) {
	bad := []Config{{8, 9, 10}, {9, 8, 10}, {51, 9, 10}, {9, 31, 10}, {9, 9, 0}, {9, 9, 73}}
	for _, c := range bad {
		if c.Validate() == nil {
			t.Errorf("%+v should be invalid", c)
		}
	}
	for _, c := range []Config{Beginner, Intermediate, Expert, {9, 9, 72}, {50, 30, 1491}} {
		if err := c.Validate(); err != nil {
			t.Errorf("%+v should be valid: %v", c, err)
		}
	}
}

// First click: never a mine, always opens a region (a zero), at every position
// and even at maximum density.
func TestFirstClickAlwaysSafeAndOpens(t *testing.T) {
	configs := []Config{Beginner, {9, 9, 72}, {12, 10, 111}, {30, 16, 99}}
	for _, c := range configs {
		for seed := uint64(0); seed < 40; seed++ {
			for _, p := range [][2]int{{0, 0}, {c.W - 1, 0}, {0, c.H - 1}, {c.W - 1, c.H - 1}, {c.W / 2, c.H / 2}, {3, 0}} {
				g := mustNew(t, c, seed)
				if err := g.Reveal(p[0], p[1], t0); err != nil {
					t.Fatal(err)
				}
				if g.Phase() == Lost {
					t.Fatalf("%+v seed %d click %v: lost on first click", c, seed, p)
				}
				if cell := g.Cell(p[0], p[1]); cell.Kind != Number || cell.N != 0 {
					t.Fatalf("%+v seed %d click %v: first cell %+v, want an opened zero", c, seed, p, cell)
				}
			}
		}
	}
}

func TestPlacementCountsAndAdjacency(t *testing.T) {
	for seed := uint64(1); seed <= 50; seed++ {
		g := mustNew(t, Intermediate, seed)
		g.Reveal(5, 5, t0)
		n := 0
		for _, m := range g.mine {
			if m {
				n++
			}
		}
		if n != Intermediate.Mines {
			t.Fatalf("seed %d: %d mines, want %d", seed, n, Intermediate.Mines)
		}
		for y := 0; y < g.cfg.H; y++ {
			for x := 0; x < g.cfg.W; x++ {
				want := 0
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						nx, ny := x+dx, y+dy
						if (dx != 0 || dy != 0) && nx >= 0 && ny >= 0 && nx < g.cfg.W && ny < g.cfg.H && g.mine[ny*g.cfg.W+nx] {
							want++
						}
					}
				}
				if int(g.adj[y*g.cfg.W+x]) != want {
					t.Fatalf("seed %d (%d,%d): adj %d, want %d", seed, x, y, g.adj[y*g.cfg.W+x], want)
				}
			}
		}
	}
}

func layoutHash(g *Game) uint64 {
	h := fnv.New64a()
	for _, m := range g.mine {
		if m {
			h.Write([]byte{1})
		} else {
			h.Write([]byte{0})
		}
	}
	return h.Sum64()
}

// Golden: layouts for a seed must never change (SPEC G-5). Update the values
// only as a deliberate, announced break of reproducible boards.
func TestLayoutIsStable(t *testing.T) {
	cases := []struct {
		seed uint64
		want uint64
	}{{1, 0x29f756178f7cf182}, {42, 0xaee50b0f55b00320}, {20260929, 0x11156055738281a}}
	for _, c := range cases {
		g := mustNew(t, Expert, c.seed)
		g.Reveal(15, 8, t0)
		if got := layoutHash(g); got != c.want {
			t.Errorf("seed %d: layout hash %#x, want %#x", c.seed, got, c.want)
		}
		// determinism within a run
		g2 := mustNew(t, Expert, c.seed)
		g2.Reveal(15, 8, t0)
		if layoutHash(g2) != layoutHash(g) {
			t.Errorf("seed %d: not deterministic", c.seed)
		}
	}
}

// Reference flood fill (recursive, obviously correct) for small boards.
func refFlood(g *Game, x, y int, seen map[int]bool) {
	w, h := g.cfg.W, g.cfg.H
	if x < 0 || y < 0 || x >= w || y >= h || seen[y*w+x] || g.mine[y*w+x] {
		return
	}
	seen[y*w+x] = true
	if g.adj[y*w+x] != 0 {
		return
	}
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			refFlood(g, x+dx, y+dy, seen)
		}
	}
}

func TestFloodMatchesReference(t *testing.T) {
	for seed := uint64(1); seed <= 60; seed++ {
		g := mustNew(t, Beginner, seed)
		g.Reveal(4, 4, t0)
		ref := map[int]bool{}
		refFlood(g, 4, 4, ref)
		for i := range g.open {
			if g.open[i] != ref[i] {
				t.Fatalf("seed %d cell %d: open=%v ref=%v", seed, i, g.open[i], ref[i])
			}
		}
	}
}

func TestFloodLargeSparseBoardDoesNotRecurse(t *testing.T) {
	g := mustNew(t, Config{50, 30, 1}, 7)
	if err := g.Reveal(25, 15, t0); err != nil {
		t.Fatal(err)
	}
	if g.Phase() != Won { // 1 mine on 1500 cells: the fill opens everything else
		t.Fatalf("phase %v, want Won", g.Phase())
	}
}

func TestFlagCycleAndCounter(t *testing.T) {
	g := mustNew(t, Beginner, 1)
	g.ToggleFlag(0, 0, false)
	if g.Cell(0, 0).Kind != Flag || g.MinesLeft() != 9 {
		t.Fatal("flag not placed")
	}
	g.ToggleFlag(0, 0, false)
	if g.Cell(0, 0).Kind != Hidden || g.MinesLeft() != 10 {
		t.Fatal("flag not removed")
	}
	g.ToggleFlag(0, 0, true)
	g.ToggleFlag(0, 0, true)
	if g.Cell(0, 0).Kind != Question || g.MinesLeft() != 10 {
		t.Fatalf("want question mark not counted, got %+v left %d", g.Cell(0, 0), g.MinesLeft())
	}
	g.ToggleFlag(0, 0, true)
	if g.Cell(0, 0).Kind != Hidden {
		t.Fatal("question mark should clear")
	}
	for i := 0; i < 12; i++ { // counter may go negative
		g.ToggleFlag(i%9, i/9, false)
	}
	if g.MinesLeft() != -2 {
		t.Fatalf("MinesLeft %d, want -2", g.MinesLeft())
	}
	if g.Phase() != Ready {
		t.Fatal("flagging must not start the game")
	}
}

func TestFlaggedCellCannotBeRevealed(t *testing.T) {
	g := mustNew(t, Beginner, 1)
	g.ToggleFlag(4, 4, false)
	g.Reveal(4, 4, t0)
	if g.Phase() != Ready || g.Cell(4, 4).Kind != Flag {
		t.Fatal("flagged cell must ignore reveal")
	}
}

// findMine returns some mine and some safe cell that is still hidden.
func find(g *Game, mine bool) (int, int) {
	for i := range g.mine {
		if g.mine[i] == mine && !g.open[i] {
			return i % g.cfg.W, i / g.cfg.W
		}
	}
	panic("none")
}

func TestLoseShowsEverything(t *testing.T) {
	g := mustNew(t, Beginner, 3)
	g.Reveal(4, 4, t0)
	mx, my := find(g, true)
	fx, fy := find(g, false) // wrong flag on a safe cell
	g.ToggleFlag(fx, fy, false)
	g.Reveal(mx, my, t0.Add(5*time.Second))
	if g.Phase() != Lost {
		t.Fatal("should be lost")
	}
	if g.Cell(mx, my).Kind != Triggered {
		t.Fatal("triggered mine not marked")
	}
	if g.Cell(fx, fy).Kind != WrongFlag {
		t.Fatal("wrong flag not shown")
	}
	mines := 0
	for y := 0; y < 9; y++ {
		for x := 0; x < 9; x++ {
			if k := g.Cell(x, y).Kind; k == Mine || k == Triggered {
				mines++
			}
		}
	}
	if mines != 10 {
		t.Fatalf("%d mines shown, want 10", mines)
	}
	if err := g.Reveal(0, 0, t0); err != ErrOver {
		t.Fatalf("want ErrOver, got %v", err)
	}
	if g.ToggleFlag(0, 0, false) != ErrOver || g.Chord(0, 0, t0) != ErrOver {
		t.Fatal("actions after the end must be rejected")
	}
}

func TestWinAutoFlagsAndStopsTimer(t *testing.T) {
	g := mustNew(t, Beginner, 5)
	g.Reveal(4, 4, t0)
	now := t0.Add(42 * time.Second)
	for i := range g.mine {
		if !g.mine[i] && !g.open[i] {
			g.Reveal(i%9, i/9, now)
		}
	}
	if g.Phase() != Won {
		t.Fatalf("phase %v, want Won", g.Phase())
	}
	if g.MinesLeft() != 0 {
		t.Fatalf("MinesLeft %d, want 0", g.MinesLeft())
	}
	for i, m := range g.mine {
		if m && g.Cell(i%9, i/9).Kind != Flag {
			t.Fatal("mines should be auto-flagged on win")
		}
	}
	if e := g.Elapsed(now.Add(time.Hour)); e != 42*time.Second {
		t.Fatalf("elapsed %v, want 42s frozen", e)
	}
}

func TestTimerStartsOnFirstReveal(t *testing.T) {
	g := mustNew(t, Beginner, 2)
	if g.Elapsed(t0.Add(time.Hour)) != 0 {
		t.Fatal("timer must not run before the first reveal")
	}
	g.ToggleFlag(0, 0, false)
	if g.Elapsed(t0.Add(time.Hour)) != 0 {
		t.Fatal("flagging must not start the timer")
	}
	g.Reveal(4, 4, t0)
	if e := g.Elapsed(t0.Add(7 * time.Second)); e != 7*time.Second {
		t.Fatalf("elapsed %v, want 7s", e)
	}
}

// Chord scenarios on a hand-built position.
func chordFixture(t *testing.T) (*Game, int, int) {
	t.Helper()
	for seed := uint64(1); seed < 500; seed++ {
		g := mustNew(t, Beginner, seed)
		g.Reveal(4, 4, t0)
		for i := range g.open {
			x, y := i%9, i/9
			if g.open[i] && g.adj[i] == 1 {
				// need exactly one hidden mine neighbour and at least one hidden safe neighbour
				var ns [8]int
				hiddenSafe, mines := 0, 0
				for _, j := range g.neighbours(i, ns[:0]) {
					if g.mine[j] {
						mines++
					} else if !g.open[j] {
						hiddenSafe++
					}
				}
				if mines == 1 && hiddenSafe >= 1 {
					return g, x, y
				}
			}
		}
	}
	t.Fatal("no chord fixture found")
	return nil, 0, 0
}

func neighbourMine(g *Game, x, y int) (int, int) {
	var ns [8]int
	for _, j := range g.neighbours(y*9+x, ns[:0]) {
		if g.mine[j] {
			return j % 9, j / 9
		}
	}
	panic("no mine")
}

func TestChordWithoutEnoughFlagsIsNoOp(t *testing.T) {
	g, x, y := chordFixture(t)
	before := g.left
	g.Chord(x, y, t0)
	if g.left != before || g.Phase() != Playing {
		t.Fatal("chord with 0 flags must do nothing")
	}
}

func TestChordCorrectFlagOpensNeighbours(t *testing.T) {
	g, x, y := chordFixture(t)
	mx, my := neighbourMine(g, x, y)
	g.ToggleFlag(mx, my, false)
	before := g.left
	g.Chord(x, y, t0)
	if g.left >= before {
		t.Fatal("chord should open the remaining neighbours")
	}
	if g.Phase() == Lost {
		t.Fatal("correct chord must not lose")
	}
}

func TestChordWrongFlagLoses(t *testing.T) {
	g, x, y := chordFixture(t)
	mx, my := neighbourMine(g, x, y)
	var ns [8]int
	for _, j := range g.neighbours(y*9+x, ns[:0]) { // flag a safe neighbour instead
		if !g.mine[j] && !g.open[j] {
			g.ToggleFlag(j%9, j/9, false)
			break
		}
	}
	g.Chord(x, y, t0)
	if g.Phase() != Lost || g.Cell(mx, my).Kind != Triggered {
		t.Fatal("chord with a wrong flag must lose and mark the mine")
	}
}

func TestChordTooManyFlagsAndOnZeroOrHidden(t *testing.T) {
	g, x, y := chordFixture(t)
	var ns [8]int
	for _, j := range g.neighbours(y*9+x, ns[:0]) {
		if !g.open[j] {
			g.ToggleFlag(j%9, j/9, false) // flag every hidden neighbour: more than the number
		}
	}
	before := g.left
	g.Chord(x, y, t0)
	if g.left != before {
		t.Fatal("too many flags: chord must do nothing")
	}
	g.Chord(4, 4, t0) // a zero
	for i := range g.open {
		if !g.open[i] && g.mark[i] == markNone {
			g.Chord(i%9, i/9, t0) // hidden cell
			break
		}
	}
	if g.left != before {
		t.Fatal("chord on zero/hidden must do nothing")
	}
}

func TestRangeErrors(t *testing.T) {
	g := mustNew(t, Beginner, 1)
	if g.Reveal(-1, 0, t0) != ErrRange || g.Reveal(0, 9, t0) != ErrRange ||
		g.ToggleFlag(9, 0, false) != ErrRange || g.Chord(0, -1, t0) != ErrRange {
		t.Fatal("out-of-range must return ErrRange")
	}
}

// While the game runs, the visible board must never reveal a mine.
func TestNoMineLeakWhilePlaying(t *testing.T) {
	g := mustNew(t, Beginner, 9)
	g.Reveal(4, 4, t0)
	for y := 0; y < 9; y++ {
		for x := 0; x < 9; x++ {
			switch g.Cell(x, y).Kind {
			case Mine, Triggered, WrongFlag:
				t.Fatalf("leak at (%d,%d) while playing", x, y)
			}
		}
	}
}
