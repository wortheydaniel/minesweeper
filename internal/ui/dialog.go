package ui

import (
	"fmt"
	"image"
	"strconv"

	"github.com/wortheydaniel/minesweeper/internal/engine"
	"github.com/wortheydaniel/minesweeper/internal/store"
)

// Dialogs are full-window pages drawn below the menu bar, which keeps layout
// trivial and identical at every board size.

type dkind int

const (
	dCustom dkind = iota
	dBest
	dName
	dControls
	dAbout
)

type field struct {
	label, text string
	max         int
	digits      bool
}

type dialog struct {
	kind   dkind
	fields []field
	focus  int
	err    string
	sure   bool // Best Times: reset button has been armed
	// name dialog
	preset string
	rank   int
	ms     int64
}

type btnID int

const (
	bOK btnID = iota
	bCancel
	bReset
)

type button struct {
	label string
	id    btnID
	r     image.Rectangle
}

func newCustomDialog(c engine.Config) *dialog {
	d := &dialog{kind: dCustom, fields: []field{
		{label: "WIDTH", text: strconv.Itoa(c.W), max: 2, digits: true},
		{label: "HEIGHT", text: strconv.Itoa(c.H), max: 2, digits: true},
		{label: "MINES", text: strconv.Itoa(c.Mines), max: 4, digits: true},
	}}
	d.validate()
	return d
}

func newNameDialog(preset string, rank int, ms int64, last string) *dialog {
	return &dialog{kind: dName, preset: preset, rank: rank, ms: ms,
		fields: []field{{label: "NAME", text: last, max: store.MaxName}}}
}

// config parses the custom fields; err is a short message when invalid.
func (d *dialog) config() (engine.Config, string) {
	v := [3]int{}
	for i := range v {
		n, err := strconv.Atoi(d.fields[i].text)
		if err != nil {
			return engine.Config{}, "ENTER NUMBERS"
		}
		v[i] = n
	}
	c := engine.Config{W: v[0], H: v[1], Mines: v[2]}
	switch {
	case c.W < engine.MinW || c.W > engine.MaxW:
		return c, fmt.Sprintf("WIDTH %d-%d", engine.MinW, engine.MaxW)
	case c.H < engine.MinH || c.H > engine.MaxH:
		return c, fmt.Sprintf("HEIGHT %d-%d", engine.MinH, engine.MaxH)
	case c.Mines < 1 || c.Mines > c.W*c.H-9:
		return c, fmt.Sprintf("MINES 1-%d", c.W*c.H-9)
	}
	return c, ""
}

func (d *dialog) validate() {
	if d.kind == dCustom {
		_, d.err = d.config()
	}
}

// ---- geometry -------------------------------------------------------------

func (m *Model) buttons() []button {
	var bs []button
	switch m.dlg.kind {
	case dCustom:
		bs = []button{{label: "OK", id: bOK}, {label: "CANCEL", id: bCancel}}
	case dBest:
		lbl := "RESET"
		if m.dlg.sure {
			lbl = "SURE?"
		}
		bs = []button{{label: lbl, id: bReset}, {label: "OK", id: bOK}}
	default:
		bs = []button{{label: "OK", id: bOK}}
	}
	const bw, bh, sp = 58, 18, 8
	total := len(bs)*bw + (len(bs)-1)*sp
	x := (m.lay.w - total) / 2
	y := m.lay.h - 12 - bh
	for i := range bs {
		bs[i].r = image.Rect(x, y, x+bw, y+bh)
		x += bw + sp
	}
	return bs
}

func (m *Model) fieldRect(i int) image.Rectangle {
	x := m.lay.w/2 - 22
	y := menuH + 44 + i*24
	if m.dlg.kind == dName {
		x, y = m.lay.w/2-58, menuH+84
		return image.Rect(x, y, x+116, y+16)
	}
	return image.Rect(x, y, x+60, y+16)
}

// ---- input ----------------------------------------------------------------

func (m *Model) char(r rune) {
	d := m.dlg
	if d == nil || len(d.fields) == 0 || r < 0x20 || r > 0x7e {
		return
	}
	f := &d.fields[d.focus]
	if f.digits && (r < '0' || r > '9') || len(f.text) >= f.max {
		return
	}
	f.text += string(r)
	d.validate()
}

func (m *Model) dlgKey(k Key) {
	d := m.dlg
	switch k {
	case KeyEscape:
		m.dlgPress(bCancel)
	case KeyEnter:
		m.dlgPress(bOK)
	case KeyBackspace:
		if len(d.fields) > 0 {
			f := &d.fields[d.focus]
			if n := len(f.text); n > 0 {
				f.text = f.text[:n-1]
				d.validate()
			}
		}
	case KeyTab, KeyDown:
		if n := len(d.fields); n > 1 {
			d.focus = (d.focus + 1) % n
		}
	case KeyUp:
		if n := len(d.fields); n > 1 {
			d.focus = (d.focus + n - 1) % n
		}
	}
}

func (m *Model) dlgClick(pt image.Point) {
	for _, b := range m.buttons() {
		if pt.In(b.r) {
			m.dlgPress(b.id)
			return
		}
	}
	for i := range m.dlg.fields {
		if pt.In(m.fieldRect(i)) {
			m.dlg.focus = i
		}
	}
}

func (m *Model) dlgPress(id btnID) {
	d := m.dlg
	switch {
	case id == bReset:
		if !d.sure {
			d.sure = true
			return
		}
		m.st.ResetTimes()
		d.sure = false
	case id == bOK && d.kind == dCustom:
		c, err := d.config()
		if err != "" {
			return
		}
		m.dlg = nil
		m.setBoard(c)
	case id == bOK && d.kind == dName || id == bCancel && d.kind == dName:
		name := d.fields[0].text
		if id == bCancel {
			name = m.st.Data.LastName
		}
		m.st.Rename(d.preset, d.rank, name)
		m.dlg = nil
	default:
		m.dlg = nil
	}
}

// ---- drawing --------------------------------------------------------------

var controlsLines = [][2]string{
	{"LEFT CLICK", "REVEAL"},
	{"RIGHT CLICK", "FLAG"},
	{"MIDDLE CLICK", "CHORD"},
	{"LEFT+RIGHT", "CHORD"},
	{"ARROWS", "MOVE"},
	{"SPACE/ENTER", "REVEAL"},
	{"F", "FLAG"},
	{"C", "CHORD"},
	{"F2", "NEW GAME"},
	{"F10", "MENU"},
	{"ESC", "CLOSE"},
}

func (m *Model) drawDialog(c canvas) {
	p, d, w := m.pal, m.dlg, m.lay.w
	title := map[dkind]string{dCustom: "CUSTOM FIELD", dBest: "BEST TIMES", dName: "NEW BEST TIME!",
		dControls: "CONTROLS", dAbout: "MINESWEEPER"}[d.kind]
	c.textC(w/2, menuH+12, title, p.text)
	c.rect(w/2-textW(title)/2, menuH+22, textW(title), 1, p.shadow)

	switch d.kind {
	case dCustom:
		hints := [3]string{
			fmt.Sprintf("%d-%d", engine.MinW, engine.MaxW),
			fmt.Sprintf("%d-%d", engine.MinH, engine.MaxH),
			"",
		}
		if cfg, err := d.config(); err == "" || cfg.W > 0 {
			hints[2] = fmt.Sprintf("MAX %d", clamp(cfg.W*cfg.H-9, 0, 9999))
		}
		for i, f := range d.fields {
			r := m.fieldRect(i)
			c.text(r.Min.X-textW(f.label)-8, r.Min.Y+4, f.label, p.text)
			c.text(r.Max.X+6, r.Min.Y+4, hints[i], p.textDim)
			m.drawField(c, r, f.text, i == d.focus)
		}
		if d.err != "" {
			c.textC(w/2, menuH+44+3*24+4, d.err, p.err)
		}
	case dName:
		c.textC(w/2, menuH+38, upper(d.preset), p.text)
		c.textC(w/2, menuH+52, fmt.Sprintf("%.2f S - RANK %d", float64(d.ms)/1000, d.rank), p.text)
		c.textC(w/2, menuH+72, "YOUR NAME", p.textDim)
		m.drawField(c, m.fieldRect(0), d.fields[0].text, true)
	case dBest:
		y := menuH + 30
		for _, name := range store.Presets {
			c.text(w/2-84, y, upper(name), p.text)
			y += 10
			rs := m.st.Data.BestTimes[name]
			for i := 0; i < store.MaxRecord; i++ {
				line := "-"
				if i < len(rs) {
					n := rs[i].Name
					if len(n) > 12 {
						n = n[:12]
					}
					line = fmt.Sprintf("%d %-12s %7.2f S", i+1, n, float64(rs[i].MS)/1000)
				}
				c.text(w/2-78, y, line, p.textDim)
				y += 9
			}
			y += 3
		}
	case dControls:
		y := menuH + 34
		for _, l := range controlsLines {
			c.text(w/2-84, y, l[0], p.text)
			c.text(w/2+8, y, l[1], p.textDim)
			y += 12
		}
	case dAbout:
		c.textC(w/2, menuH+40, "A CLASSIC CLONE", p.text)
		if m.opt.Version != "" {
			c.textC(w/2, menuH+56, "VERSION "+m.opt.Version, p.textDim)
		}
		seed := "SEED HIDDEN UNTIL THE GAME ENDS"
		if m.g.Phase() >= engine.Won {
			seed = fmt.Sprintf("SEED %d", m.g.Seed())
		}
		c.textC(w/2, menuH+80, "LAST GAME", p.textDim)
		c.textC(w/2, menuH+92, seed, p.text)
	}

	for _, b := range m.buttons() {
		m.drawButton(c, b)
	}
}

func (m *Model) drawField(c canvas, r image.Rectangle, text string, focused bool) {
	p := m.pal
	c.rect(r.Min.X, r.Min.Y, r.Dx(), r.Dy(), p.field)
	c.bevel(r.Min.X, r.Min.Y, r.Dx(), r.Dy(), 1, false, p)
	c.text(r.Min.X+4, r.Min.Y+4, text, p.text)
	if focused {
		c.rect(r.Min.X+4+textW(text)+2, r.Min.Y+3, 1, 9, p.text)
	}
}

func (m *Model) drawButton(c canvas, b button) {
	p := m.pal
	c.rect(b.r.Min.X, b.r.Min.Y, b.r.Dx(), b.r.Dy(), p.bg)
	c.bevel(b.r.Min.X, b.r.Min.Y, b.r.Dx(), b.r.Dy(), 2, true, p)
	c.textC(b.r.Min.X+b.r.Dx()/2, b.r.Min.Y+6, b.label, p.text)
}
