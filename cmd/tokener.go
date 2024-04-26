package cmd

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/kballard/go-shellquote"
)

func ShellJoin(words ...string) string {
	return shellquote.Join(words...)
}

func ShellSplit(input string) (words []string, err error) {
	return shellquote.Split(input)
}

func ShellQuote(words []string) (result []string) {
	for _, word := range words {
		// Bail out early for "simple" strings
		if word != "" && !strings.ContainsAny(word, "\\'\"`${[|&;<>()~*?! \t\n") {
			result = append(result, word)
		} else {
			var buf bytes.Buffer
			buf.WriteString("'")
			for i := 0; i < len(word); i++ {
				b := word[i]
				if b == '\'' {
					// Replace literal ' with a close ', a \', and a open '
					buf.WriteString("'\\''")
				} else {
					buf.WriteByte(b)
				}
			}
			buf.WriteString("'")
			result = append(result, buf.String())
		}
	}
	return
}

type Tokener interface {
	Len() int
	Copy() Tokener
	Append(s ...string) Tokener
	Prepend(s ...string) Tokener
	Content() []string
	IsEmpty() bool
	Index(pos int) string
	ShellCmd() string
	String() string
	WriteTo(dest io.Writer) (n int, err error)
}

type Tokens []string

func (t Tokens) Len() int {
	return len(t)
}

func (t Tokens) Copy() Tokener {
	result := make([]string, len(t))
	_ = copy(result, t)
	return Tokens(result)
}

func (t Tokens) Append(s ...string) Tokener {
	return append(t, s...)
}

func (t Tokens) Prepend(s ...string) Tokener {
	return append(Tokens(s), t...)
}

func (t Tokens) Content() []string {
	return ([]string)(t)
}

func (t Tokens) IsEmpty() bool {
	return len(t) == 0
}

func (t Tokens) Index(pos int) (result string) {
	if pos >= 0 && pos < len(t) {
		result = t[pos]
		return
	}
	inform("warning", fmt.Errorf("%w: not a valid index: %d", ErrInvalid, pos).Error())
	return
}

func (t Tokens) ShellCmd() string {
	return strings.TrimSpace(strings.Join(t, ` `))
}

func (t Tokens) String() string {
	return `[` + t.ShellCmd() + `]`
}

func (t Tokens) WriteTo(dest io.Writer) (n int, err error) {
	n, err = fmt.Fprintf(dest, t.ShellCmd()+"\n")
	return
}
