package ui

import (
	"fmt"
	"image"

	"github.com/wortheydaniel/minesweeper/internal/engine"
)

type act int

const (
	aNone act = iota
	aNew
	aBeginner
	aIntermediate
	aExpert
	aCustom
	aMarks
	aChord
	aTheme
	aScale
	aBest
	aExit
	aControls
	aAbout
)

type item struct {
	label, key string
	act        act
	check, sep bool
}

var menuTitles = []string{"GAME", "HELP"}

const (
	itemH  = 12
	sepH   = 6
	gutter = 14 // room for the check mark
)

func (m *Model) menuItems() []item {
	if m.menu == 1 {
		return []item{{label: "CONTROLS...", act: aControls}, {label: "ABOUT", act: aAbout}}
	}
	s := m.st.Data.Settings
	scale := "AUTO"
	if s.Scale > 0 {
		scale = fmt.Sprintf("%dX", s.Scale)
	}
	return []item{
		{label: "NEW", key: "F2", act: aNew},
		{sep: true},
		{label: "BEGINNER", act: aBeginner, check: m.preset == "beginner"},
		{label: "INTERMEDIATE", act: aIntermediate, check: m.preset == "intermediate"},
		{label: "EXPERT", act: aExpert, check: m.preset == "expert"},
		{label: "CUSTOM...", act: aCustom, check: m.preset == "custom"},
		{sep: true},
		{label: "MARKS (?)", act: aMarks, check: s.QuestionMarks},
		{label: "CLICK NUMBER TO CHORD", act: aChord, check: s.ClickNumberChords},
		{label: "THEME: " + upper(s.Theme), act: aTheme},
		{label: "SCALE: " + scale, act: aScale},
		{sep: true},
		{label: "BEST TIMES...", act: aBest},
		{sep: true},
		{label: "EXIT", act: aExit},
	}
}

func upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}

// menuBar returns the clickable title rectangles.
func menuBar() []image.Rectangle {
	var rs []image.Rectangle
	x := 2
	for _, t := range menuTitles {
		w := textW(t) + 12
		rs = append(rs, image.Rect(x, 0, x+w, menuH))
		x += w
	}
	return rs
}

func (m *Model) menuBarHit(x int) int {
	for i, r := range menuBar() {
		if x >= r.Min.X && x < r.Max.X {
			return i
		}
	}
	return -1
}

// dropdown returns the panel rectangle and each item's rectangle.
func (m *Model) dropdown() (image.Rectangle, []image.Rectangle) {
	items := m.menuItems()
	w := 0
	for _, it := range items {
		iw := textW(it.label)
		if it.key != "" {
			iw += 16 + textW(it.key)
		}
		if iw > w {
			w = iw
		}
	}
	w += gutter + 10
	x0 := menuBar()[m.menu].Min.X
	y := menuH + 2
	rs := make([]image.Rectangle, len(items))
	for i, it := range items {
		h := itemH
		if it.sep {
			h = sepH
		}
		rs[i] = image.Rect(x0+2, y, x0+w-2, y+h)
		y += h
	}
	return image.Rect(x0, menuH, x0+w, y+2), rs
}

func (m *Model) openMenu(i int) {
	m.menu, m.msel = i, -1
	m.ges = gesture{}
	m.dirty = true
}

func (m *Model) closeMenu() { m.menu, m.msel, m.dirty = -1, -1, true }

func (m *Model) menuKey(k Key) {
	items := m.menuItems()
	step := func(d int) {
		n := len(items)
		i := m.msel
		if i < 0 && d < 0 { // nothing selected: Up starts from the bottom
			i = 0
		}
		for c := 0; c < n; c++ {
			i = (i + d + n) % n
			if !items[i].sep {
				m.msel = i
				return
			}
		}
	}
	switch k {
	case KeyDown:
		step(1)
	case KeyUp:
		step(-1)
	case KeyLeft, KeyRight:
		d := 1
		if k == KeyLeft {
			d = len(menuTitles) - 1
		}
		m.openMenu((m.menu + d) % len(menuTitles))
		m.msel = 0
	case KeyEnter, KeySpace:
		if m.msel >= 0 {
			m.activate(items[m.msel].act)
		}
	case KeyEscape, KeyF10:
		m.closeMenu()
	}
}

func (m *Model) menuMouse(pt image.Point, moved, leftPress bool) {
	panel, rs := m.dropdown()
	hover := -1
	for i, r := range rs {
		if pt.In(r) && !m.menuItems()[i].sep {
			hover = i
		}
	}
	if moved {
		if pt.In(panel) && hover != m.msel {
			m.msel, m.dirty = hover, true
		}
		if pt.Y >= 0 && pt.Y < menuH {
			if i := m.menuBarHit(pt.X); i >= 0 && i != m.menu {
				m.openMenu(i)
			}
		}
	}
	if !leftPress {
		return
	}
	switch {
	case pt.In(panel):
		if hover >= 0 {
			m.activate(m.menuItems()[hover].act)
		}
	case pt.Y >= 0 && pt.Y < menuH && m.menuBarHit(pt.X) == m.menu:
		m.closeMenu()
	default:
		m.closeMenu()
	}
}

func (m *Model) activate(a act) {
	m.closeMenu()
	s := &m.st.Data.Settings
	switch a {
	case aNew:
		m.newGame()
	case aBeginner:
		m.setBoard(engine.Beginner)
	case aIntermediate:
		m.setBoard(engine.Intermediate)
	case aExpert:
		m.setBoard(engine.Expert)
	case aCustom:
		m.dlg = newCustomDialog(m.cfg)
	case aMarks:
		s.QuestionMarks = !s.QuestionMarks
		m.st.Save()
	case aChord:
		s.ClickNumberChords = !s.ClickNumberChords
		m.st.Save()
	case aTheme:
		if s.Theme == "dark" {
			s.Theme = "classic"
		} else {
			s.Theme = "dark"
		}
		m.pal = paletteFor(s.Theme)
		m.st.Save()
	case aScale:
		s.Scale = (s.Scale + 1) % 5
		m.st.Save()
	case aBest:
		m.dlg = &dialog{kind: dBest}
	case aControls:
		m.dlg = &dialog{kind: dControls}
	case aAbout:
		m.dlg = &dialog{kind: dAbout}
	case aExit:
		m.quit = true
	}
	m.dirty = true
}

// ---- drawing --------------------------------------------------------------

func (m *Model) drawMenuBar(c canvas) {
	p := m.pal
	c.rect(0, menuH-1, m.lay.w, 1, p.shadow)
	for i, r := range menuBar() {
		col := p.text
		if m.menu == i {
			c.rect(r.Min.X, r.Min.Y, r.Dx(), r.Dy()-1, p.selBg)
			col = p.selText
		}
		c.text(r.Min.X+6, 4, menuTitles[i], col)
	}
	if n := m.st.Notice; n != "" {
		c.text(m.lay.w-textW(n)-6, 4, n, p.err)
	}
}

func (m *Model) drawDropdown(c canvas) {
	p := m.pal
	panel, rs := m.dropdown()
	c.rect(panel.Min.X, panel.Min.Y, panel.Dx(), panel.Dy(), p.bg)
	c.bevel(panel.Min.X, panel.Min.Y, panel.Dx(), panel.Dy(), 1, true, p)
	c.rect(panel.Min.X, panel.Max.Y-1, panel.Dx(), 1, p.edge)
	c.rect(panel.Max.X-1, panel.Min.Y, 1, panel.Dy(), p.edge)
	for i, it := range m.menuItems() {
		r := rs[i]
		if it.sep {
			c.rect(r.Min.X+2, r.Min.Y+2, r.Dx()-4, 1, p.shadow)
			c.rect(r.Min.X+2, r.Min.Y+3, r.Dx()-4, 1, p.light)
			continue
		}
		col := p.text
		if i == m.msel {
			c.rect(r.Min.X, r.Min.Y, r.Dx(), r.Dy(), p.selBg)
			col = p.selText
		}
		if it.check {
			c.tick(r.Min.X+3, r.Min.Y+3, col)
		}
		c.text(r.Min.X+gutter, r.Min.Y+2, it.label, col)
		if it.key != "" {
			c.text(r.Max.X-textW(it.key)-4, r.Min.Y+2, it.key, col)
		}
	}
}

// tick draws a small check mark.
func (c canvas) tick(x, y int, col rgba) {
	for _, o := range [][2]int{{0, 3}, {1, 4}, {2, 5}, {3, 4}, {4, 3}, {5, 2}, {6, 1}, {7, 0},
		{0, 2}, {1, 3}, {2, 4}, {3, 3}, {4, 2}, {5, 1}, {6, 0}} {
		c.px(x+o[0], y+o[1], col)
	}
}
