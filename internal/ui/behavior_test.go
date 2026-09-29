package ui

import (
	"testing"
	"time"

	"github.com/wortheydaniel/minesweeper/internal/engine"
)

// startGame makes the first click in the middle so a region is open.
func (h *harness) startGame() { h.click(4, 4, true, false, false) }

func TestLeftClickRevealsOnRelease(t *testing.T) {
	h := newHarness(t)
	px, py := h.cellPt(4, 4)
	h.moveTo(px, py)
	h.press(true, false, false)
	if h.m.g.Phase() != engine.Ready {
		t.Fatal("reveal must wait for the button release")
	}
	h.release()
	if h.m.g.Phase() == engine.Ready || h.cell(4, 4).Kind != engine.Number {
		t.Fatal("release should reveal")
	}
}

func TestRightClickFlagsAndFlagBlocksReveal(t *testing.T) {
	h := newHarness(t)
	h.click(0, 0, false, false, true)
	if h.cell(0, 0).Kind != engine.Flag || h.m.g.MinesLeft() != 9 {
		t.Fatal("right click should flag")
	}
	h.click(0, 0, true, false, false)
	if h.cell(0, 0).Kind != engine.Flag || h.m.g.Phase() != engine.Ready {
		t.Fatal("left click must not reveal a flagged cell")
	}
}

func TestReleaseOutsideBoardCancels(t *testing.T) {
	h := newHarness(t)
	px, py := h.cellPt(4, 4)
	h.moveTo(px, py)
	h.press(true, false, false)
	h.moveTo(1, h.m.lay.h-1) // off the board
	h.release()
	if h.m.g.Phase() != engine.Ready {
		t.Fatal("releasing outside the board must cancel")
	}
}

func TestFocusLossCancelsGesture(t *testing.T) {
	h := newHarness(t)
	px, py := h.cellPt(4, 4)
	h.moveTo(px, py)
	h.press(true, false, false)
	h.tick(func(in *Input) { in.Focused = false; in.Left = true })
	h.tick(func(in *Input) { in.Left = false })
	if h.m.g.Phase() != engine.Ready {
		t.Fatal("losing focus mid-click must cancel it")
	}
	if h.m.ges.active {
		t.Fatal("gesture left stuck")
	}
}

// chordSetup returns coordinates of a revealed number whose mines can be
// flagged, using the fixed seed of the harness.
func (h *harness) chordSetup() (nx, ny int, mines [][2]int) {
	h.startGame()
	c := h.m.g.Config()
	probe, _ := engine.New(c, h.m.g.Seed())
	probe.Reveal(4, 4, t0)
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if h.cell(x, y).Kind != engine.Number || h.cell(x, y).N == 0 {
				continue
			}
			var ms [][2]int
			hiddenSafe := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					ax, ay := x+dx, y+dy
					if (dx == 0 && dy == 0) || ax < 0 || ay < 0 || ax >= c.W || ay >= c.H {
						continue
					}
					if h.cell(ax, ay).Kind == engine.Number {
						continue
					}
					p2, _ := engine.New(c, h.m.g.Seed())
					p2.Reveal(4, 4, t0)
					p2.Reveal(ax, ay, t0)
					if p2.Phase() == engine.Lost {
						ms = append(ms, [2]int{ax, ay})
					} else {
						hiddenSafe++
					}
				}
			}
			if len(ms) == int(h.cell(x, y).N) && hiddenSafe > 0 {
				return x, y, ms
			}
		}
	}
	h.t.Fatal("no chord position")
	return
}

func TestChordGestures(t *testing.T) {
	cases := []struct {
		name string
		do   func(h *harness, x, y int)
	}{
		{"middle button", func(h *harness, x, y int) { h.click(x, y, false, true, false) }},
		{"left+right together", func(h *harness, x, y int) {
			px, py := h.cellPt(x, y)
			h.moveTo(px, py)
			h.press(true, false, false)
			h.press(true, false, true)
			h.press(false, false, true) // release left first
			h.release()
		}},
		{"right then left", func(h *harness, x, y int) {
			px, py := h.cellPt(x, y)
			h.moveTo(px, py)
			h.press(false, false, true)
			h.press(true, false, true)
			h.release()
		}},
		{"click on number (setting on)", func(h *harness, x, y int) { h.click(x, y, true, false, false) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			x, y, mines := h.chordSetup()
			for _, mn := range mines {
				h.click(mn[0], mn[1], false, false, true)
			}
			left := h.m.g.MinesLeft()
			before := countHidden(h)
			c.do(h, x, y)
			if countHidden(h) >= before {
				t.Fatalf("chord should reveal neighbours (hidden %d -> %d)", before, countHidden(h))
			}
			if h.m.g.Phase() == engine.Lost || h.m.g.MinesLeft() != left {
				t.Fatal("correct chord must not lose")
			}
		})
	}
}

func TestClickToChordCanBeDisabled(t *testing.T) {
	h := newHarness(t)
	x, y, mines := h.chordSetup()
	for _, mn := range mines {
		h.click(mn[0], mn[1], false, false, true)
	}
	h.m.activate(aChord)
	if h.st.Data.Settings.ClickNumberChords {
		t.Fatal("setting should be off")
	}
	before := countHidden(h)
	h.click(x, y, true, false, false)
	if countHidden(h) != before {
		t.Fatal("with the setting off a left click on a number must do nothing")
	}
	h.click(x, y, false, true, false) // middle still chords
	if countHidden(h) >= before {
		t.Fatal("middle click should still chord")
	}
}

func countHidden(h *harness) int {
	n := 0
	c := h.m.g.Config()
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if k := h.cell(x, y).Kind; k == engine.Hidden || k == engine.Question {
				n++
			}
		}
	}
	return n
}

func TestFaceClickStartsNewGame(t *testing.T) {
	h := newHarness(t)
	h.startGame()
	seed := h.m.g.Seed()
	f := h.m.lay.face
	h.clickAt(f.Min.X+5, f.Min.Y+5)
	if h.m.g.Phase() != engine.Ready || h.m.g.Seed() == seed {
		t.Fatal("clicking the face should start a fresh game")
	}
}

func TestKeyboardOnlyPlay(t *testing.T) {
	h := newHarness(t)
	if h.m.fx != 4 || h.m.fy != 4 {
		t.Fatalf("focus starts at the centre, got %d,%d", h.m.fx, h.m.fy)
	}
	h.keys(KeyLeft, KeyLeft, KeyUp) // (2,3)
	h.keys(KeyF)
	if h.cell(2, 3).Kind != engine.Flag {
		t.Fatal("F should flag the focus cell")
	}
	h.keys(KeyHome, KeyPageUp)
	if h.m.fx != 0 || h.m.fy != 0 {
		t.Fatalf("Home/PgUp should reach the corner, got %d,%d", h.m.fx, h.m.fy)
	}
	h.keys(KeyLeft, KeyUp) // clamped
	if h.m.fx != 0 || h.m.fy != 0 {
		t.Fatal("focus must stay on the board")
	}
	h.keys(KeyEnd, KeyPageDown)
	if h.m.fx != 8 || h.m.fy != 8 {
		t.Fatal("End/PgDn should reach the far corner")
	}
	h.keys(KeySpace)
	if h.m.g.Phase() == engine.Ready {
		t.Fatal("Space should reveal")
	}
	seed := h.m.g.Seed()
	h.keys(KeyF2)
	if h.m.g.Seed() == seed || h.m.g.Phase() != engine.Ready {
		t.Fatal("F2 should start a new game")
	}
}

func TestQuestionMarks(t *testing.T) {
	h := newHarness(t)
	h.m.activate(aMarks)
	h.click(0, 0, false, false, true)
	h.click(0, 0, false, false, true)
	if h.cell(0, 0).Kind != engine.Question {
		t.Fatal("second right click should give a question mark when enabled")
	}
	h.click(0, 0, false, false, true)
	if h.cell(0, 0).Kind != engine.Hidden {
		t.Fatal("third right click should clear")
	}
}

func TestTimerRedrawsEachSecond(t *testing.T) {
	h := newHarness(t)
	h.startGame()
	h.m.Frame()
	h.tick(func(*Input) {})
	if _, changed := h.m.Frame(); changed {
		t.Fatal("an idle tick within the same second must not repaint")
	}
	h.now = h.now.Add(1100 * time.Millisecond)
	h.tick(func(*Input) {})
	if _, changed := h.m.Frame(); !changed {
		t.Fatal("the timer tick should repaint")
	}
	if h.m.displaySecs() != 1 {
		t.Fatalf("secs %d", h.m.displaySecs())
	}
	h.now = h.now.Add(2000 * time.Second)
	h.tick(func(*Input) {})
	if h.m.displaySecs() != 999 {
		t.Fatal("timer display must cap at 999")
	}
}

func TestMenuKeyboardAndMouse(t *testing.T) {
	h := newHarness(t)
	h.keys(KeyF10, KeyDown, KeyDown, KeyDown, KeyDown, KeyEnter) // NEW, BEGINNER, INTERMEDIATE, EXPERT -> selects EXPERT
	if h.m.cfg != engine.Expert {
		t.Fatalf("cfg %+v, want Expert", h.m.cfg)
	}
	w, _ := h.m.Size()
	if w <= 200 {
		t.Fatal("window should have grown for Expert")
	}
	if h.st.Data.LastBoard.W != 30 {
		t.Fatal("last board should be saved")
	}
	// mouse: open the Game menu, pick BEGINNER
	h.clickAt(10, 6)
	if h.m.menu != 0 {
		t.Fatal("clicking GAME should open the menu")
	}
	_, rs := h.m.dropdown()
	r := rs[2]
	h.clickAt(r.Min.X+10, r.Min.Y+4)
	if h.m.cfg != engine.Beginner || h.m.menu != -1 {
		t.Fatalf("menu click failed: %+v menu=%d", h.m.cfg, h.m.menu)
	}
	// Esc closes without acting
	h.keys(KeyF10, KeyEscape)
	if h.m.menu != -1 {
		t.Fatal("Esc should close the menu")
	}
	// click outside closes
	h.clickAt(10, 6)
	h.clickAt(h.m.lay.w-3, h.m.lay.h-3)
	if h.m.menu != -1 {
		t.Fatal("clicking away should close the menu")
	}
}

func TestMenuBlocksBoardClicks(t *testing.T) {
	h := newHarness(t)
	h.keys(KeyF10)
	h.click(4, 4, true, false, false)
	if h.m.g.Phase() != engine.Ready {
		t.Fatal("a click while a menu is open must only close the menu")
	}
}

func TestCustomDialog(t *testing.T) {
	h := newHarness(t)
	h.m.activate(aCustom)
	if h.m.dlg == nil || h.m.dlg.err != "" {
		t.Fatal("custom dialog should open valid")
	}
	// width field: replace 9 with 12
	h.keys(KeyBackspace)
	h.typ("12")
	h.keys(KeyTab, KeyBackspace)
	h.typ("11")
	h.keys(KeyTab, KeyBackspace, KeyBackspace)
	h.typ("999")
	if h.m.dlg.err == "" {
		t.Fatal("999 mines on 12x11 must be rejected")
	}
	h.keys(KeyEnter)
	if h.m.dlg == nil {
		t.Fatal("invalid input must keep the dialog open")
	}
	h.keys(KeyBackspace, KeyBackspace, KeyBackspace)
	h.typ("20")
	h.typ("ab!") // non-digits ignored
	if h.m.dlg.fields[2].text != "20" {
		t.Fatalf("mines field %q", h.m.dlg.fields[2].text)
	}
	h.keys(KeyEnter)
	if h.m.dlg != nil || h.m.cfg != (engine.Config{W: 12, H: 11, Mines: 20}) {
		t.Fatalf("custom board not applied: %+v", h.m.cfg)
	}
	if h.m.preset != "custom" {
		t.Fatal("preset should be custom")
	}
	// Esc cancels
	h.m.activate(aCustom)
	h.keys(KeyEscape)
	if h.m.dlg != nil || h.m.cfg.W != 12 {
		t.Fatal("Esc should cancel without change")
	}
}

func TestWinRecordsTimeAndNameDialog(t *testing.T) {
	h := newHarness(t)
	h.startGame()
	h.now = h.now.Add(42 * time.Second)
	h.tick(func(*Input) {})
	revealAllSafeAt(h)
	if h.m.g.Phase() != engine.Won {
		t.Fatal("expected a win")
	}
	if h.m.dlg == nil || h.m.dlg.kind != dName || h.m.dlg.rank != 1 {
		t.Fatal("a top-5 win should open the name dialog")
	}
	rs := h.st.Data.BestTimes["beginner"]
	if len(rs) != 1 || rs[0].MS < 41000 || rs[0].MS > 43000 {
		t.Fatalf("record not saved immediately: %+v", rs)
	}
	for range h.m.dlg.fields[0].text { // clear "Anonymous"
		h.keys(KeyBackspace)
	}
	h.typ("Ada")
	h.keys(KeyEnter)
	if h.m.dlg != nil || h.st.Data.BestTimes["beginner"][0].Name != "Ada" || h.st.Data.LastName != "Ada" {
		t.Fatal("name not saved")
	}
	// Esc on the name dialog keeps the default name
	h2 := newHarness(t)
	h2.startGame()
	revealAllSafeAt(h2)
	h2.keys(KeyEscape)
	if h2.st.Data.BestTimes["beginner"][0].Name != "Anonymous" {
		t.Fatal("Esc should keep the default name")
	}
}

func revealAllSafeAt(h *harness) {
	c := h.m.g.Config()
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if !isMine(h.m, x, y) && h.cell(x, y).Kind != engine.Number {
				h.click(x, y, true, false, false)
			}
		}
	}
}

func TestCustomWinDoesNotRecord(t *testing.T) {
	h := newHarness(t)
	h.m.setBoard(engine.Config{W: 9, H: 9, Mines: 5})
	h.startGame()
	revealAllSafeAt(h)
	if h.m.g.Phase() != engine.Won || h.m.dlg != nil {
		t.Fatal("custom wins must not open the name dialog")
	}
	if len(h.st.Data.BestTimes["custom"]) != 0 {
		t.Fatal("custom boards are never recorded")
	}
}

func TestBestTimesReset(t *testing.T) {
	h := newHarness(t)
	h.st.AddRecord("beginner", 5000, t0)
	h.m.activate(aBest)
	rs := h.m.buttons()
	reset := rs[0]
	h.clickAt(reset.r.Min.X+3, reset.r.Min.Y+3)
	if len(h.st.Data.BestTimes["beginner"]) != 1 || !h.m.dlg.sure {
		t.Fatal("the first click only arms the reset")
	}
	h.clickAt(reset.r.Min.X+3, reset.r.Min.Y+3)
	if len(h.st.Data.BestTimes["beginner"]) != 0 {
		t.Fatal("the second click should reset")
	}
	h.keys(KeyEnter)
	if h.m.dlg != nil {
		t.Fatal("Enter should close")
	}
}

func TestThemeScaleAndExitSettings(t *testing.T) {
	h := newHarness(t)
	h.m.activate(aTheme)
	if h.st.Data.Settings.Theme != "dark" || h.m.pal != dark {
		t.Fatal("theme should switch to dark")
	}
	for i := 0; i < 5; i++ {
		h.m.activate(aScale)
	}
	if h.st.Data.Settings.Scale != 0 || h.m.ScaleSetting() != 0 {
		t.Fatal("scale should cycle back to auto")
	}
	h.m.activate(aScale)
	if h.m.ScaleSetting() != 1 {
		t.Fatal("scale should step to 1x")
	}
	h.m.opt.Scale = 3
	if h.m.ScaleSetting() != 3 {
		t.Fatal("the flag override wins")
	}
	if h.m.Update(Input{Focused: true, Now: h.now}) {
		t.Fatal("not quitting yet")
	}
	h.m.activate(aExit)
	if !h.m.Update(Input{Focused: true, Now: h.now}) {
		t.Fatal("Exit should quit")
	}
}

func TestSettingsSurviveRestart(t *testing.T) {
	h := newHarness(t)
	h.m.activate(aExpert)
	h.m.activate(aTheme)
	h.m.activate(aMarks)
	st2 := reopen(h)
	m2 := New(Options{Store: st2})
	if m2.cfg != engine.Expert || m2.pal != dark || !st2.Data.Settings.QuestionMarks {
		t.Fatal("settings and last board should persist")
	}
}

func TestFrameOnlyRepaintsWhenDirty(t *testing.T) {
	h := newHarness(t)
	if _, ch := h.m.Frame(); !ch {
		t.Fatal("first frame must draw")
	}
	h.tick(func(*Input) {})
	if _, ch := h.m.Frame(); ch {
		t.Fatal("idle tick must not repaint")
	}
	px, py := h.cellPt(1, 1)
	h.moveTo(px, py) // hover only: no repaint
	if _, ch := h.m.Frame(); ch {
		t.Fatal("plain hover must not repaint")
	}
}

func TestLayoutHitTesting(t *testing.T) {
	for _, c := range []engine.Config{engine.Beginner, engine.Intermediate, engine.Expert, {W: 50, H: 30, Mines: 100}} {
		l := computeLayout(c.W, c.H)
		for _, p := range [][2]int{{0, 0}, {c.W - 1, 0}, {0, c.H - 1}, {c.W - 1, c.H - 1}, {c.W / 2, c.H / 2}} {
			ox, oy := l.cellOrigin(p[0], p[1])
			for _, d := range [][2]int{{0, 0}, {cellSize - 1, cellSize - 1}, {8, 8}} {
				x, y, ok := l.cellAt(ox+d[0], oy+d[1])
				if !ok || x != p[0] || y != p[1] {
					t.Fatalf("%+v cell %v offset %v -> %d,%d,%v", c, p, d, x, y, ok)
				}
			}
			if _, _, ok := l.cellAt(ox-boardBev-1, oy); p[0] == 0 && ok {
				t.Fatal("frame must not hit-test as a cell")
			}
		}
		if l.w < minW {
			t.Fatalf("frame narrower than the dialogs need: %d", l.w)
		}
		if !l.mines.In(l.header) && l.mines.X < l.header.Min.X {
			t.Fatal("counter outside header")
		}
	}
}
