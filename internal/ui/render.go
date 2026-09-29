package ui

import (
	"image"

	"github.com/wortheydaniel/minesweeper/internal/engine"
)

func (m *Model) draw() {
	l := m.lay
	if m.img == nil || m.img.Rect.Dx() != l.w || m.img.Rect.Dy() != l.h {
		m.img = image.NewRGBA(image.Rect(0, 0, l.w, l.h))
	}
	c := canvas{m.img}
	c.rect(0, 0, l.w, l.h, m.pal.bg)
	m.drawMenuBar(c)
	if m.dlg != nil {
		m.drawDialog(c)
	} else {
		m.drawHeader(c)
		m.drawBoard(c)
	}
	if m.menu >= 0 {
		m.drawDropdown(c)
	}
}

func (m *Model) drawHeader(c canvas) {
	p, l := m.pal, m.lay
	c.bevel(l.header.Min.X, l.header.Min.Y, l.header.Dx(), l.header.Dy(), 2, false, p)
	c.led(l.mines.X, l.mines.Y, m.g.MinesLeft(), p)
	c.led(l.timer.X, l.timer.Y, m.displaySecs(), p)

	f := faceSmile
	switch {
	case m.g.Phase() == engine.Lost:
		f = faceDead
	case m.g.Phase() == engine.Won:
		f = faceCool
	case m.ges.active && m.ges.in && (m.ges.l || m.ges.m):
		f = faceOh
	}
	pressed := m.ges.face && image.Pt(m.prev.X, m.prev.Y).In(l.face)
	c.face(l.face.Min.X, l.face.Min.Y, f, pressed, p)
}

func (m *Model) drawBoard(c canvas) {
	p, l := m.pal, m.lay
	c.bevel(l.frame.Min.X, l.frame.Min.Y, l.frame.Dx(), l.frame.Dy(), boardBev, false, p)

	// Which cells look pressed under the current mouse gesture?
	px0, px1, py0, py1 := -1, -1, -1, -1 // inclusive range; empty when -1
	if g := m.ges; g.active && g.in {
		switch {
		case g.chord():
			px0, px1, py0, py1 = g.cx-1, g.cx+1, g.cy-1, g.cy+1
		case g.l:
			px0, px1, py0, py1 = g.cx, g.cx, g.cy, g.cy
		}
	}
	for y := 0; y < l.rows; y++ {
		for x := 0; x < l.cols; x++ {
			ox, oy := l.cellOrigin(x, y)
			pressed := x >= px0 && x <= px1 && y >= py0 && y <= py1
			c.cell(ox, oy, m.g.Cell(x, y), pressed, p)
		}
	}
	if m.fshow {
		ox, oy := l.cellOrigin(m.fx, m.fy)
		for i := 3; i < 13; i += 2 {
			c.px(ox+i, oy+3, p.edge)
			c.px(ox+i, oy+12, p.edge)
			c.px(ox+3, oy+i, p.edge)
			c.px(ox+12, oy+i, p.edge)
		}
	}
}
