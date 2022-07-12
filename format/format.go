package format

import (
	"bytes"
	"io"
	"sync"

	"github.com/dmgk/fallout/cache"
	"github.com/dmgk/fallout/grep"
)

// Formatter flags.
const (
	Fcolor = 1 << iota
	FfilenamesOnly

	Fdefaults = 0
)

// Known color sequences.
var colorMap = map[byte]string{
	'a': "\033[0;30m", // black
	'b': "\033[0;31m", // red
	'c': "\033[0;32m", // green
	'd': "\033[0;33m", // yellow
	'e': "\033[0;34m", // blue
	'f': "\033[0;35m", // magenta
	'g': "\033[0;36m", // cyan
	'h': "\033[0;37m", // white
	'A': "\033[0;90m", // bright black (grey)
	'B': "\033[0;91m", // bright red
	'C': "\033[0;92m", // bright green
	'D': "\033[0;93m", // bright yellow
	'E': "\033[0;94m", // bright blue
	'F': "\033[0;95m", // bright magenta
	'G': "\033[0;96m", // bright cyan
	'H': "\033[0;97m", // bright white
}

const creset = "\033[0m"

// The default color palette.
const DefaultColors = "BCDA"

// Color palette color order.
const (
	cquery = iota
	cmatch
	cpath
	cseparator

	ncolors
)

var colors [ncolors]string

func SetColors(c string) {
	for i, k := range []byte(c) {
		if v, ok := colorMap[k]; ok && i < len(colors) {
			colors[i] = v
		}
	}
}

type Formatter interface {
	Format(entry cache.Entry, matches []*grep.Match) error
}

type textFormatter struct {
	mu sync.Mutex // protects w
	w  io.Writer

	flags   int
	needSep bool
}

func NewText(w io.Writer, flags int) Formatter {
	f := &textFormatter{
		w:     w,
		flags: flags,
	}
	return f
}

func (f *textFormatter) Format(entry cache.Entry, matches []*grep.Match) error {
	buf := bufGet()
	defer bufPut(buf)

	if f.flags&FfilenamesOnly != 0 {
		buf.WriteString(entry.Path())
		buf.WriteByte('\n')
		return f.write(buf)
	}

	if matches != nil {
		if f.flags&Fcolor != 0 {
			buf.WriteString(colors[cpath])
			buf.WriteString(entry.Path())
			buf.WriteString(creset)
		} else {
			buf.WriteString(entry.Path())
		}
		buf.WriteString(":\n")

		for i, m := range matches {
			formatBuf := bufGet()
			defer bufPut(formatBuf)

			if i > 0 {
				if f.flags&Fcolor != 0 {
					formatBuf.WriteString(colors[cseparator])
					formatBuf.WriteString("--------\n")
					formatBuf.WriteString(creset)
				} else {
					formatBuf.WriteString("--------\n")
				}
			}

			if f.flags&Fcolor != 0 {
				if m.ResultSubmatch != nil {
					formatBuf.Write(m.Text[:m.ResultSubmatch[0]])
					formatBuf.WriteString(colors[cmatch])
					formatBuf.Write(m.Text[m.ResultSubmatch[0]:m.ResultSubmatch[1]])
					formatBuf.WriteString(creset)
					formatBuf.Write(m.Text[m.ResultSubmatch[1]:])
				} else {
					formatBuf.Write(m.Text)
				}
			} else {
				formatBuf.Write(m.Text)
			}

			buf.Write(formatBuf.Bytes())
		}

		return f.write(buf)
	}

	return nil
}

func (f *textFormatter) write(buf *bytes.Buffer) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, err := f.w.Write(buf.Bytes())
	return err
}

var bufPool = sync.Pool{
	New: func() interface{} {
		return &bytes.Buffer{}
	},
}

func bufGet() *bytes.Buffer {
	return bufPool.Get().(*bytes.Buffer)
}

func bufPut(buf *bytes.Buffer) {
	buf.Reset()
	bufPool.Put(buf)
}

func init() {
	SetColors(DefaultColors)
}
