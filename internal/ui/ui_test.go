package ui

import (
	"bytes"
	"flag"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wortheydaniel/minesweeper/internal/engine"
	"github.com/wortheydaniel/minesweeper/internal/store"
)

var update = flag.Bool("update", false, "regenerate golden images in testdata/")

var t0 = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

// harness drives a Model the way the shell would.
type harness struct {
	t   *testing.T
	m   *Model
	st  *store.Store
	in  Input
	now time.Time
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	seed := uint64(7)
	st := store.Open(t.TempDir())
	h := &harness{t: t, st: st, now: t0}
	h.m = New(Options{Store: st, Seed: &seed, Version: "test"})
	h.in = Input{Focused: true, X: -1, Y: -1}
	h.tick(func(*Input) {})
	return h
}

func (h *harness) tick(f func(in *Input)) {
	in := h.in
	in.Keys, in.Text, in.Focused, in.Now = nil, nil, true, h.now
	f(&in)
	h.m.Update(in)
	h.in = in
	h.now = h.now.Add(16 * time.Millisecond)
}

func (h *harness) keys(ks ...Key) {
	for _, k := range ks {
		h.tick(func(in *Input) { in.Keys = []Key{k} })
	}
}

func (h *harness) typ(s string) {
	h.tick(func(in *Input) { in.Text = []rune(s) })
}

func (h *harness) moveTo(x, y int) { h.tick(func(in *Input) { in.X, in.Y = x, y }) }

func (h *harness) cellPt(cx, cy int) (int, int) { return h.m.CellCenter(cx, cy) }

func (h *harness) press(l, mid, r bool) {
	h.tick(func(in *Input) { in.Left, in.Middle, in.Right = l, mid, r })
}
func (h *harness) release() { h.press(false, false, false) }

// click moves onto a cell and clicks with the given buttons (l, m, r).
func (h *harness) click(cx, cy int, l, mid, r bool) {
	px, py := h.cellPt(cx, cy)
	h.moveTo(px, py)
	h.press(l, mid, r)
	h.release()
}

func (h *harness) clickAt(x, y int) {
	h.moveTo(x, y)
	h.press(true, false, false)
	h.release()
}

func (h *harness) cell(x, y int) engine.Cell { return h.m.g.Cell(x, y) }

// ---- golden images --------------------------------------------------------

func golden(t *testing.T, name string, img *image.RGBA) {
	t.Helper()
	path := filepath.Join("testdata", name+".png")
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if *update {
		os.MkdirAll("testdata", 0o755)
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("missing golden %s (run: go test ./internal/ui -update): %v", path, err)
	}
	defer f.Close()
	want, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	got := image.NewRGBA(img.Rect)
	copy(got.Pix, img.Pix)
	w := image.NewRGBA(want.Bounds())
	for y := 0; y < want.Bounds().Dy(); y++ {
		for x := 0; x < want.Bounds().Dx(); x++ {
			w.Set(x, y, want.At(x, y))
		}
	}
	if got.Rect != w.Rect || !bytes.Equal(got.Pix, w.Pix) {
		t.Errorf("%s differs from golden (run with -update to review the change)", name)
	}
}

func frame(t *testing.T, m *Model) *image.RGBA {
	t.Helper()
	img, _ := m.Frame()
	return img
}

func (h *harness) playSomething() {
	h.click(4, 4, true, false, false)
	// flag two hidden cells and put a question-free flag on a corner
	h.click(0, 0, false, false, true)
	h.click(8, 8, false, false, true)
}

func TestGoldenScenes(t *testing.T) {
	t.Run("fresh", func(t *testing.T) {
		h := newHarness(t)
		golden(t, "fresh_beginner", frame(t, h.m))
	})
	t.Run("playing", func(t *testing.T) {
		h := newHarness(t)
		h.playSomething()
		h.now = h.now.Add(83 * time.Second)
		h.tick(func(*Input) {})
		golden(t, "playing_beginner", frame(t, h.m))
	})
	t.Run("lost", func(t *testing.T) {
		h := newHarness(t)
		h.playSomething()
		mx, my := findMine(h.m)
		h.click(mx, my, true, false, false)
		golden(t, "lost_beginner", frame(t, h.m))
	})
	t.Run("won", func(t *testing.T) {
		h := newHarness(t)
		h.click(4, 4, true, false, false)
		h.now = h.now.Add(31 * time.Second)
		revealAllSafe(h.m)
		h.m.dlg = nil
		h.m.dirty = true
		golden(t, "won_beginner", frame(t, h.m))
	})
	t.Run("expert dark", func(t *testing.T) {
		h := newHarness(t)
		h.m.activate(aExpert)
		h.m.activate(aTheme)
		h.click(15, 8, true, false, false)
		h.click(1, 1, false, false, true)
		golden(t, "playing_expert_dark", frame(t, h.m))
	})
	t.Run("menu", func(t *testing.T) {
		h := newHarness(t)
		h.keys(KeyF10, KeyDown, KeyDown, KeyDown)
		golden(t, "menu_game", frame(t, h.m))
	})
	t.Run("custom dialog", func(t *testing.T) {
		h := newHarness(t)
		h.m.activate(aCustom)
		golden(t, "dialog_custom", frame(t, h.m))
	})
	t.Run("best times", func(t *testing.T) {
		h := newHarness(t)
		h.st.Data.LastName = "Ada"
		h.st.AddRecord("beginner", 12340, t0)
		h.st.AddRecord("beginner", 9870, t0)
		h.st.AddRecord("expert", 101500, t0)
		h.m.activate(aBest)
		golden(t, "dialog_best", frame(t, h.m))
	})
	t.Run("controls", func(t *testing.T) {
		h := newHarness(t)
		h.m.activate(aControls)
		golden(t, "dialog_controls", frame(t, h.m))
	})
}

func findMine(m *Model) (int, int) {
	c := m.g.Config()
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if m.g.Cell(x, y).Kind == engine.Hidden && isMine(m, x, y) {
				return x, y
			}
		}
	}
	panic("no mine")
}

// isMine peeks by revealing on a copy of the seed layout: simulate with a
// fresh game using the same seed and first click.
func isMine(m *Model, x, y int) bool {
	c := m.g.Config()
	probe, _ := engine.New(c, m.g.Seed())
	probe.Reveal(4, 4, t0)
	probe.Reveal(x, y, t0)
	return probe.Phase() == engine.Lost
}

func revealAllSafe(m *Model) {
	c := m.g.Config()
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if !isMine(m, x, y) {
				m.act(func() error { return m.g.Reveal(x, y, m.now) })
			}
		}
	}
}

func reopen(h *harness) *store.Store {
	h.st.Save()
	return store.Open(h.st.Dir())
}
