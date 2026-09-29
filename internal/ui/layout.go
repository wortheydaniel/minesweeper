package ui

import "image"

// All metrics are in logical pixels; the shell scales the finished frame.
const (
	menuH    = 16
	pad      = 8
	headerH  = 38
	gap      = 8
	boardBev = 3
	minW     = 200 // wide enough for the dialogs
)

type layout struct {
	w, h         int
	cols, rows   int
	header       image.Rectangle
	board        image.Rectangle // the cells area (inside the frame)
	frame        image.Rectangle
	face         image.Rectangle
	mines, timer image.Point // top-left of each LED
}

func computeLayout(cols, rows int) layout {
	frameW := cols*cellSize + 2*boardBev
	frameH := rows*cellSize + 2*boardBev
	w := frameW + 2*pad
	if w < minW {
		w = minW
	}
	fx := (w - frameW) / 2
	hy := menuH + pad
	fy := hy + headerH + gap
	l := layout{w: w, h: fy + frameH + pad, cols: cols, rows: rows}
	l.header = image.Rect(fx, hy, fx+frameW, hy+headerH)
	l.frame = image.Rect(fx, fy, fx+frameW, fy+frameH)
	l.board = image.Rect(fx+boardBev, fy+boardBev, fx+boardBev+cols*cellSize, fy+boardBev+rows*cellSize)
	ly := hy + (headerH-ledH)/2
	l.mines = image.Pt(fx+8, ly)
	l.timer = image.Pt(fx+frameW-8-ledW, ly)
	fcx := fx + frameW/2
	l.face = image.Rect(fcx-faceSize/2, hy+(headerH-faceSize)/2, fcx-faceSize/2+faceSize, hy+(headerH-faceSize)/2+faceSize)
	return l
}

// cellAt maps a pixel to board coordinates.
func (l layout) cellAt(px, py int) (x, y int, ok bool) {
	if !image.Pt(px, py).In(l.board) {
		return 0, 0, false
	}
	return (px - l.board.Min.X) / cellSize, (py - l.board.Min.Y) / cellSize, true
}

func (l layout) cellOrigin(x, y int) (int, int) {
	return l.board.Min.X + x*cellSize, l.board.Min.Y + y*cellSize
}
