package termui

import "io"

// NewWithWidth is for tests that need a width other than the detected one; width is clamped the same way.
func NewWithWidth(w io.Writer, width int) *Printer {
	return &Printer{
		w:     w,
		width: clampWidth(width),
	}
}
