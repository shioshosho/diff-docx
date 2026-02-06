package progress

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

const (
	defaultBarWidth = 40
	fillChar        = "█"
	emptyChar       = "░"
)

// Bar is a tqdm-like progress bar.
type Bar struct {
	total   int
	current int
	width   int
}

// New creates a new progress bar with the given total steps.
func New(total int) *Bar {
	return &Bar{total: total, width: barWidth()}
}

// Advance increments the progress and renders with the given description.
func (b *Bar) Advance(desc string) {
	b.current++
	b.render(desc)
}

// Done clears the progress bar line.
func (b *Bar) Done() {
	fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", b.width+40))
}

func (b *Bar) render(desc string) {
	pct := float64(b.current) / float64(b.total)
	filled := int(pct * float64(b.width))
	if filled > b.width {
		filled = b.width
	}

	bar := strings.Repeat(fillChar, filled) + strings.Repeat(emptyChar, b.width-filled)
	fmt.Fprintf(os.Stderr, "\r%3.0f%%|%s| %d/%d %s", pct*100, bar, b.current, b.total, desc)
}

func barWidth() int {
	if w, _, err := term.GetSize(int(os.Stderr.Fd())); err == nil && w > 80 {
		return w / 3
	}
	return defaultBarWidth
}
