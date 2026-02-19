package diff

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// Highlighter provides syntax highlighting for diff content.
type Highlighter struct {
	style     *chroma.Style
	themeName string
}

// NewHighlighter creates a Highlighter with the given Chroma theme name.
// Falls back to "dracula" if the theme is not found.
func NewHighlighter(theme string) *Highlighter {
	s := styles.Get(theme)
	if s == nil {
		s = styles.Get("dracula")
	}
	name := theme
	if s != nil && s.Name != "" {
		name = s.Name
	}
	return &Highlighter{style: s, themeName: name}
}

// SetTheme changes the highlighting theme. No-op if theme is not found.
func (h *Highlighter) SetTheme(theme string) {
	s := styles.Get(theme)
	if s != nil {
		h.style = s
		h.themeName = theme
	}
}

// Theme returns the current theme name.
func (h *Highlighter) Theme() string {
	return h.themeName
}

// Style returns the underlying Chroma style for direct UI integration.
func (h *Highlighter) Style() *chroma.Style {
	return h.style
}

// DetectLanguage returns the Chroma language name for a given filename,
// or empty string if the language cannot be determined.
func DetectLanguage(filename string) string {
	lexer := lexers.Match(filename)
	if lexer == nil {
		return ""
	}
	config := lexer.Config()
	if config == nil {
		return ""
	}
	return config.Name
}

// Highlight returns content with ANSI terminal256 syntax highlighting applied.
// The filename is used to detect the programming language.
func (h *Highlighter) Highlight(filename, content string) (string, error) {
	lexer := getLexer(filename, content)
	lexer = chroma.Coalesce(lexer)

	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	iterator, err := lexer.Tokenise(nil, content)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := formatter.Format(&buf, h.style, iterator); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// Token represents a syntax-highlighted segment of text.
type Token struct {
	Type  chroma.TokenType
	Value string
}

// Tokenize returns syntax tokens grouped by line for the given content.
// This is useful for UI rendering where tokens need to be combined with
// diff-level styling (e.g., green background for added lines with syntax
// colors on the text).
func (h *Highlighter) Tokenize(filename, content string) ([][]Token, error) {
	lexer := getLexer(filename, content)
	lexer = chroma.Coalesce(lexer)

	iterator, err := lexer.Tokenise(nil, content)
	if err != nil {
		return nil, err
	}

	var lines [][]Token
	var currentLine []Token

	tokens := iterator.Tokens()
	for _, tok := range tokens {
		if tok.Type == chroma.EOFType {
			break
		}
		parts := strings.Split(tok.Value, "\n")
		for i, part := range parts {
			if i > 0 {
				lines = append(lines, currentLine)
				currentLine = nil
			}
			if part != "" {
				currentLine = append(currentLine, Token{
					Type:  tok.Type,
					Value: part,
				})
			}
		}
	}
	if len(currentLine) > 0 {
		lines = append(lines, currentLine)
	}

	return lines, nil
}

// getLexer returns the best lexer for the given filename and content.
func getLexer(filename, content string) chroma.Lexer {
	// Try matching by filename first
	lexer := lexers.Match(filename)
	if lexer != nil {
		return lexer
	}

	// Try matching by extension
	ext := filepath.Ext(filename)
	if ext != "" {
		lexer = lexers.Match("file" + ext)
		if lexer != nil {
			return lexer
		}
	}

	// Try content analysis
	lexer = lexers.Analyse(content)
	if lexer != nil {
		return lexer
	}

	return lexers.Fallback
}
