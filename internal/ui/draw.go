package ui

import (
	"image"
	"image/color"
	"strings"
)

type rgba = color.RGBA

func hex(v uint32) rgba { return rgba{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255} }

// palette holds every colour the game draws with.
type palette struct {
	bg, light, shadow, edge rgba // panel face and bevel tones
	cell, grid              rgba // revealed cell face and its grid line
	num                     [9]rgba
	ledOn, ledOff, ledBg    rgba
	text, textDim, err      rgba
	selBg, selText          rgba // menu highlight
	field                   rgba // text-input background
	boom                    rgba // triggered-mine background
}

var classic = &palette{
	bg: hex(0xC0C0C0), light: hex(0xFFFFFF), shadow: hex(0x808080), edge: hex(0x404040),
	cell: hex(0xC0C0C0), grid: hex(0x808080),
	num: [9]rgba{{}, hex(0x0000FF), hex(0x008000), hex(0xFF0000), hex(0x000080),
		hex(0x800000), hex(0x008080), hex(0x000000), hex(0x808080)},
	ledOn: hex(0xFF2010), ledOff: hex(0x480800), ledBg: hex(0x000000),
	text: hex(0x000000), textDim: hex(0x606060), err: hex(0xC00000),
	selBg: hex(0x000080), selText: hex(0xFFFFFF),
	field: hex(0xFFFFFF), boom: hex(0xFF0000),
}

var dark = &palette{
	bg: hex(0x2E3138), light: hex(0x4C515C), shadow: hex(0x1A1C21), edge: hex(0x0E0F12),
	cell: hex(0x23262C), grid: hex(0x1A1C21),
	num: [9]rgba{{}, hex(0x6EA8FF), hex(0x5FD07A), hex(0xFF6B6B), hex(0xA595FF),
		hex(0xFF9F5A), hex(0x4FD1C5), hex(0xE6E6E6), hex(0xA0A0A0)},
	ledOn: hex(0xFF4030), ledOff: hex(0x3A1210), ledBg: hex(0x000000),
	text: hex(0xE6E6E6), textDim: hex(0x9098A5), err: hex(0xFF7070),
	selBg: hex(0x3B6FD4), selText: hex(0xFFFFFF),
	field: hex(0x15171B), boom: hex(0xC02020),
}

func paletteFor(theme string) *palette {
	if theme == "dark" {
		return dark
	}
	return classic
}

// canvas wraps an image with clipped drawing helpers.
type canvas struct{ *image.RGBA }

func (c canvas) px(x, y int, col rgba) {
	if image.Pt(x, y).In(c.Rect) {
		i := c.PixOffset(x, y)
		c.Pix[i], c.Pix[i+1], c.Pix[i+2], c.Pix[i+3] = col.R, col.G, col.B, 255
	}
}

func (c canvas) rect(x, y, w, h int, col rgba) {
	r := image.Rect(x, y, x+w, y+h).Intersect(c.Rect)
	for yy := r.Min.Y; yy < r.Max.Y; yy++ {
		i := c.PixOffset(r.Min.X, yy)
		for xx := r.Min.X; xx < r.Max.X; xx++ {
			c.Pix[i], c.Pix[i+1], c.Pix[i+2], c.Pix[i+3] = col.R, col.G, col.B, 255
			i += 4
		}
	}
}

// bevel draws t-pixel 3D edges inside the box; raised or sunken.
func (c canvas) bevel(x, y, w, h, t int, raised bool, p *palette) {
	hi, lo := p.light, p.shadow
	if !raised {
		hi, lo = lo, hi
	}
	for k := 0; k < t; k++ {
		c.rect(x+k, y+k, w-2*k, 1, hi)     // top
		c.rect(x+k, y+k, 1, h-2*k, hi)     // left
		c.rect(x+k, y+h-1-k, w-2*k, 1, lo) // bottom
		c.rect(x+w-1-k, y+k, 1, h-2*k, lo) // right
	}
}

// ---- pixel font -----------------------------------------------------------

// 5x7 glyphs, rows separated by '/'. Text is upper-cased before drawing.
var glyphSrc = map[rune]string{
	'A':  ".###./#...#/#...#/#####/#...#/#...#/#...#",
	'B':  "####./#...#/#...#/####./#...#/#...#/####.",
	'C':  ".###./#...#/#..../#..../#..../#...#/.###.",
	'D':  "####./#...#/#...#/#...#/#...#/#...#/####.",
	'E':  "#####/#..../#..../####./#..../#..../#####",
	'F':  "#####/#..../#..../####./#..../#..../#....",
	'G':  ".###./#...#/#..../#.###/#...#/#...#/.###.",
	'H':  "#...#/#...#/#...#/#####/#...#/#...#/#...#",
	'I':  ".###./..#../..#../..#../..#../..#../.###.",
	'J':  "..###/...#./...#./...#./...#./#..#./.##..",
	'K':  "#...#/#..#./#.#../##.../#.#../#..#./#...#",
	'L':  "#..../#..../#..../#..../#..../#..../#####",
	'M':  "#...#/##.##/#.#.#/#.#.#/#...#/#...#/#...#",
	'N':  "#...#/##..#/#.#.#/#..##/#...#/#...#/#...#",
	'O':  ".###./#...#/#...#/#...#/#...#/#...#/.###.",
	'P':  "####./#...#/#...#/####./#..../#..../#....",
	'Q':  ".###./#...#/#...#/#...#/#.#.#/#..#./.##.#",
	'R':  "####./#...#/#...#/####./#.#../#..#./#...#",
	'S':  ".####/#..../#..../.###./....#/....#/####.",
	'T':  "#####/..#../..#../..#../..#../..#../..#..",
	'U':  "#...#/#...#/#...#/#...#/#...#/#...#/.###.",
	'V':  "#...#/#...#/#...#/#...#/#...#/.#.#./..#..",
	'W':  "#...#/#...#/#...#/#.#.#/#.#.#/##.##/#...#",
	'X':  "#...#/#...#/.#.#./..#../.#.#./#...#/#...#",
	'Y':  "#...#/#...#/.#.#./..#../..#../..#../..#..",
	'Z':  "#####/....#/...#./..#../.#.../#..../#####",
	'0':  ".###./#...#/#..##/#.#.#/##..#/#...#/.###.",
	'1':  "..#../.##../..#../..#../..#../..#../.###.",
	'2':  ".###./#...#/....#/...#./..#../.#.../#####",
	'3':  "#####/...#./..#../...#./....#/#...#/.###.",
	'4':  "...#./..##./.#.#./#..#./#####/...#./...#.",
	'5':  "#####/#..../####./....#/....#/#...#/.###.",
	'6':  "..##./.#.../#..../####./#...#/#...#/.###.",
	'7':  "#####/....#/...#./..#../.#.../.#.../.#...",
	'8':  ".###./#...#/#...#/.###./#...#/#...#/.###.",
	'9':  ".###./#...#/#...#/.####/....#/...#./.##..",
	'.':  "...../...../...../...../...../...../..#..",
	',':  "...../...../...../...../..#../..#../.#...",
	':':  "...../..#../...../...../..#../...../.....",
	'-':  "...../...../...../.###./...../...../.....",
	'+':  "...../..#../..#../#####/..#../..#../.....",
	'/':  "....#/....#/...#./..#../.#.../#..../#....",
	'(':  "...#./..#../.#.../.#.../.#.../..#../...#.",
	')':  ".#.../..#../...#./...#./...#./..#../.#...",
	'?':  ".###./#...#/....#/...#./..#../...../..#..",
	'!':  "..#../..#../..#../..#../..#../...../..#..",
	'\'': "..#../..#../.#.../...../...../...../.....",
	'*':  "...../..#../#.#.#/.###./#.#.#/..#../.....",
	'%':  "##..#/##..#/...#./..#../.#.../#..##/#..##",
	'=':  "...../...../#####/...../#####/...../.....",
	'_':  "...../...../...../...../...../...../#####",
	'>':  "#..../.#.../..#../...#./..#../.#.../#....",
	'<':  "....#/...#./..#../.#.../..#../...#./....#",
	'#':  ".#.#./#####/.#.#./.#.#./.#.#./#####/.#.#.",
}

var glyphs = func() map[rune][7]uint8 {
	m := map[rune][7]uint8{}
	for r, s := range glyphSrc {
		var rows [7]uint8
		for y, row := range strings.Split(s, "/") {
			for x, ch := range row {
				if ch == '#' {
					rows[y] |= 1 << (4 - x)
				}
			}
		}
		m[r] = rows
	}
	return m
}()

const (
	glyphW  = 5
	advance = 6
	glyphH  = 7
)

func glyphRows(r rune) ([7]uint8, bool) {
	if r >= 'a' && r <= 'z' {
		r -= 'a' - 'A'
	}
	if r == ' ' {
		return [7]uint8{}, true
	}
	g, ok := glyphs[r]
	if !ok {
		g = glyphs['?']
	}
	return g, ok
}

// textW is the pixel width of s.
func textW(s string) int {
	n := len([]rune(s))
	if n == 0 {
		return 0
	}
	return n*advance - 1
}

// text draws s with its top-left at (x, y).
func (c canvas) text(x, y int, s string, col rgba) {
	for _, r := range s {
		g, _ := glyphRows(r)
		for row := 0; row < glyphH; row++ {
			for b := 0; b < glyphW; b++ {
				if g[row]&(1<<(4-b)) != 0 {
					c.px(x+b, y+row, col)
				}
			}
		}
		x += advance
	}
}

// textC draws s horizontally centred on cx.
func (c canvas) textC(cx, y int, s string, col rgba) { c.text(cx-textW(s)/2, y, s, col) }

// bigDigit draws a digit as a bold 6x9 glyph (used for cell numbers).
func (c canvas) bigDigit(x, y int, d int, col rgba) {
	g, _ := glyphRows(rune('0' + d))
	oy := 0
	for row := 0; row < glyphH; row++ {
		reps := 1
		if row == 1 || row == 5 { // stretch two rows to make the glyph taller
			reps = 2
		}
		for k := 0; k < reps; k++ {
			for b := 0; b < glyphW; b++ {
				if g[row]&(1<<(4-b)) != 0 {
					c.px(x+b, y+oy, col)
					c.px(x+b+1, y+oy, col) // bold: double-strike horizontally
				}
			}
			oy++
		}
	}
}
