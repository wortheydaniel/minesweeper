package ui

import (
	"github.com/wortheydaniel/minesweeper/internal/engine"
)

// ---- LED counters ---------------------------------------------------------

const (
	ledDigitW = 13
	ledDigitH = 23
	ledW      = 3*ledDigitW + 2 + 2 // three digits, 1px gaps, 1px frame each side
	ledH      = ledDigitH + 2
)

// segment bits: a top, b top-right, c bottom-right, d bottom, e bottom-left, f top-left, g middle
const (
	sa = 1 << iota
	sb
	sc
	sd
	se
	sf
	sg
)

var ledSegs = [...]uint8{
	sa | sb | sc | sd | se | sf,      // 0
	sb | sc,                          // 1
	sa | sb | sg | se | sd,           // 2
	sa | sb | sg | sc | sd,           // 3
	sf | sg | sb | sc,                // 4
	sa | sf | sg | sc | sd,           // 5
	sa | sf | sg | se | sc | sd,      // 6
	sa | sb | sc,                     // 7
	sa | sb | sc | sd | se | sf | sg, // 8
	sa | sb | sc | sd | sf | sg,      // 9
}

func (c canvas) ledDigit(x, y int, segs uint8, p *palette) {
	seg := func(bit uint8, sx, sy, w, h int) {
		col := p.ledOff
		if segs&bit != 0 {
			col = p.ledOn
		}
		c.rect(x+sx, y+sy, w, h, col)
	}
	seg(sa, 2, 1, 9, 2)
	seg(sg, 2, 10, 9, 3)
	seg(sd, 2, 20, 9, 2)
	seg(sf, 1, 3, 2, 7)
	seg(sb, 10, 3, 2, 7)
	seg(se, 1, 13, 2, 7)
	seg(sc, 10, 13, 2, 7)
}

// led draws a 3-digit counter (value clamped to -99..999) at (x, y).
func (c canvas) led(x, y, v int, p *palette) {
	c.rect(x, y, ledW, ledH, p.ledBg)
	c.bevel(x, y, ledW, ledH, 1, false, p)
	if v < -99 {
		v = -99
	}
	if v > 999 {
		v = 999
	}
	var s [3]uint8
	if v < 0 {
		a := -v
		s = [3]uint8{sg, ledSegs[a/10], ledSegs[a%10]}
	} else {
		s = [3]uint8{ledSegs[v/100], ledSegs[v/10%10], ledSegs[v%10]}
	}
	for i, seg := range s {
		c.ledDigit(x+2+i*(ledDigitW+1), y+1, seg, p)
	}
}

// ---- face -----------------------------------------------------------------

type face int

const (
	faceSmile face = iota
	faceOh
	faceDead
	faceCool
)

const faceSize = 26

func (c canvas) face(x, y int, f face, pressed bool, p *palette) {
	black := hex(0x000000)
	yellow := hex(0xFFE000)
	c.rect(x, y, faceSize, faceSize, p.shadow) // 1px outline
	c.rect(x+1, y+1, faceSize-2, faceSize-2, p.bg)
	if pressed {
		c.bevel(x+1, y+1, faceSize-2, faceSize-2, 1, false, p)
	} else {
		c.bevel(x+1, y+1, faceSize-2, faceSize-2, 2, true, p)
	}
	cx, cy := x+13, y+13
	if pressed {
		cx, cy = cx+1, cy+1
	}
	// head: filled circle with a black rim
	for dy := -8; dy <= 8; dy++ {
		for dx := -8; dx <= 8; dx++ {
			d := dx*dx + dy*dy
			switch {
			case d <= 49+7:
				c.px(cx+dx, cy+dy, yellow)
			case d <= 64+8:
				c.px(cx+dx, cy+dy, black)
			}
		}
	}
	smile := func() {
		for _, o := range [][2]int{{-4, 2}, {4, 2}, {-3, 3}, {3, 3}, {-2, 4}, {-1, 4}, {0, 4}, {1, 4}, {2, 4}} {
			c.px(cx+o[0], cy+o[1], black)
		}
	}
	switch f {
	case faceSmile:
		c.rect(cx-4, cy-3, 2, 2, black)
		c.rect(cx+3, cy-3, 2, 2, black)
		smile()
	case faceOh:
		c.rect(cx-4, cy-3, 2, 2, black)
		c.rect(cx+3, cy-3, 2, 2, black)
		c.rect(cx-1, cy+2, 3, 4, black)
	case faceDead:
		for _, ex := range []int{-3, 4} {
			for _, o := range [][2]int{{-1, -1}, {1, -1}, {0, 0}, {-1, 1}, {1, 1}} {
				c.px(cx+ex+o[0], cy-2+o[1], black)
			}
		}
		for _, o := range [][2]int{{-2, 3}, {-1, 3}, {0, 3}, {1, 3}, {2, 3}, {-3, 4}, {3, 4}} {
			c.px(cx+o[0], cy+o[1], black)
		}
	case faceCool:
		c.rect(cx-6, cy-4, 6, 3, black)
		c.rect(cx+1, cy-4, 6, 3, black)
		c.rect(cx-1, cy-4, 2, 1, black)
		smile()
	}
}

// ---- cell contents --------------------------------------------------------

const cellSize = 16

func (c canvas) flag(x, y int) {
	black, red := hex(0x000000), hex(0xFF0000)
	for i, w := range []int{1, 3, 5, 3, 1} {
		c.rect(x+8-w, y+3+i, w, 1, red)
	}
	c.rect(x+8, y+3, 1, 8, black)
	c.rect(x+6, y+10, 5, 1, black)
	c.rect(x+4, y+11, 9, 2, black)
}

func (c canvas) mine(x, y int) {
	black, white := hex(0x000000), hex(0xFFFFFF)
	cx, cy := x+8, y+8
	c.rect(cx-6, cy, 13, 1, black)
	c.rect(cx, cy-6, 1, 13, black)
	for _, o := range [][2]int{{-4, -4}, {4, -4}, {-4, 4}, {4, 4}, {-5, -5}, {5, -5}, {-5, 5}, {5, 5}} {
		c.px(cx+o[0], cy+o[1], black)
	}
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			if dx*dx+dy*dy <= 18 {
				c.px(cx+dx, cy+dy, black)
			}
		}
	}
	c.rect(cx-2, cy-2, 2, 2, white)
}

func (c canvas) cross(x, y int) {
	red := hex(0xFF0000)
	for i := 2; i <= 13; i++ {
		c.px(x+i, y+i, red)
		c.px(x+i+1, y+i, red)
		c.px(x+15-i, y+i, red)
		c.px(x+14-i, y+i, red)
	}
}

// cell draws one board cell at (x, y). pressed shows a hidden cell as if
// it were being clicked.
func (c canvas) cell(x, y int, cl engine.Cell, pressed bool, p *palette) {
	flat := func(bg rgba) {
		c.rect(x, y, cellSize, cellSize, bg)
		c.rect(x, y, cellSize, 1, p.grid)
		c.rect(x, y, 1, cellSize, p.grid)
	}
	switch cl.Kind {
	case engine.Hidden, engine.Flag, engine.Question:
		if pressed && cl.Kind != engine.Flag {
			flat(p.cell)
			if cl.Kind == engine.Question {
				c.text(x+6, y+4, "?", p.text)
			}
			return
		}
		c.rect(x, y, cellSize, cellSize, p.bg)
		c.bevel(x, y, cellSize, cellSize, 2, true, p)
		switch cl.Kind {
		case engine.Flag:
			c.flag(x, y)
		case engine.Question:
			c.text(x+6, y+4, "?", p.text)
		}
	case engine.Number:
		flat(p.cell)
		if cl.N > 0 {
			c.bigDigit(x+5, y+3, int(cl.N), p.num[cl.N])
		}
	case engine.Mine:
		flat(p.cell)
		c.mine(x, y)
	case engine.Triggered:
		flat(p.boom)
		c.mine(x, y)
	case engine.WrongFlag:
		flat(p.cell)
		c.mine(x, y)
		c.cross(x, y)
	}
}
