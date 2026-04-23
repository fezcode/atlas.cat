package viewer

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

var (
	eofStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#FF0000")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1)

	lineNumStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")).
			MarginRight(1)

	hexOffsetStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D4AF37")).
			Bold(true).
			MarginRight(1)

	hexByteStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	hexASCIIStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555"))

	searchMatchStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#D4AF37")).
				Foreground(lipgloss.Color("#000000"))

	currentMatchStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#FFFFFF")).
				Foreground(lipgloss.Color("#000000")).
				Bold(true)

	cursorOverlayStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#D4AF37")).
				Foreground(lipgloss.Color("#000000"))
)

type Processor struct {
	Path            string
	ShowLineNumbers bool
	HexMode         bool
	WrapLines       bool
	ViewportWidth   int

	// CursorY/CursorX < 0 disable cursor overlay.
	CursorY int
	CursorX int

	lines []string
}

func NewProcessor(path string, showLines, hexMode, wrap bool) (*Processor, error) {
	p := &Processor{
		Path:            path,
		ShowLineNumbers: showLines,
		HexMode:         hexMode,
		WrapLines:       wrap,
		CursorY:         -1,
		CursorX:         -1,
	}

	if err := p.Reload(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Processor) Reload() error {
	f, err := os.Open(p.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	var newLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		newLines = append(newLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	p.lines = newLines
	return nil
}

func (p *Processor) GetPlain() string {
	return strings.Join(p.lines, "\n")
}

func (p *Processor) LinesCount() int {
	return len(p.lines)
}

func (p *Processor) HighlightAll(searchQuery string, matchIndex int) string {
	content := p.GetPlain()
	if p.HexMode {
		return p.renderHexWithSearch(searchQuery, matchIndex)
	}

	lexer := lexers.Get(p.Path)
	if lexer == nil {
		lexer = lexers.Analyse(content)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get("monokai")
	formatter := formatters.Get("terminal256")

	iterator, _ := lexer.Tokenise(nil, content)
	var buf bytes.Buffer
	formatter.Format(&buf, style, iterator)

	highlighted := buf.String()

	if searchQuery != "" {
		highlighted = p.applySearchHighlight(highlighted, searchQuery, matchIndex)
	}

	lines := strings.Split(highlighted, "\n")
	var finalLines []string
	width := len(fmt.Sprintf("%d", len(lines)))

	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			break
		}
		if i == p.CursorY && p.CursorX >= 0 {
			line = overlayCursor(line, p.CursorX)
		}
		prefix := ""
		if p.ShowLineNumbers {
			prefix = lineNumStyle.Render(fmt.Sprintf("%*d", width, i+1))
		}

		if p.WrapLines && p.ViewportWidth > 0 {
			contentWidth := p.ViewportWidth
			if p.ShowLineNumbers {
				contentWidth -= (width + 1)
			}
			if contentWidth > 0 {
				wrapped := lipgloss.NewStyle().Width(contentWidth).Render(line)
				subLines := strings.Split(wrapped, "\n")
				for j, sl := range subLines {
					if j == 0 {
						finalLines = append(finalLines, prefix+sl)
					} else {
						finalLines = append(finalLines, strings.Repeat(" ", width+1)+sl)
					}
				}
				continue
			}
		}
		finalLines = append(finalLines, prefix+line)
	}

	highlighted = strings.Join(finalLines, "\n")
	if !strings.HasSuffix(highlighted, "\n") {
		highlighted += "\n"
	}
	highlighted += eofStyle.Render("EOF")
	return highlighted
}

// overlayCursor paints a single-cell cursor on an ANSI-styled line at the
// given plain-text column, skipping over SGR escape sequences.
func overlayCursor(line string, col int) string {
	var out strings.Builder
	plainCol := 0
	i := 0
	placed := false

	for i < len(line) {
		if i+1 < len(line) && line[i] == '\x1b' && line[i+1] == '[' {
			end := strings.IndexAny(line[i+2:], "mABCDHJKfhnpsu")
			if end == -1 {
				out.WriteString(line[i:])
				i = len(line)
				continue
			}
			end += i + 2 + 1
			out.WriteString(line[i:end])
			i = end
			continue
		}

		if plainCol == col && !placed {
			ch := string(line[i])
			// Terminate any active SGR, render the cursor cell, then let the
			// next token's SGR re-establish styling for subsequent text.
			out.WriteString("\x1b[0m")
			out.WriteString(cursorOverlayStyle.Render(ch))
			placed = true
			i++
			plainCol++
			continue
		}

		out.WriteByte(line[i])
		i++
		plainCol++
	}

	if !placed && plainCol == col {
		out.WriteString("\x1b[0m")
		out.WriteString(cursorOverlayStyle.Render(" "))
	}

	return out.String()
}

func (p *Processor) applySearchHighlight(highlighted, query string, matchIndex int) string {
	var result strings.Builder
	cursor := 0
	matchCounter := 0

	for {
		start := strings.Index(highlighted[cursor:], "\x1b[")
		if start == -1 {
			res, count := p.highlightPlainPart(highlighted[cursor:], query, matchIndex, matchCounter)
			result.WriteString(res)
			matchCounter = count
			break
		}

		start += cursor
		res, count := p.highlightPlainPart(highlighted[cursor:start], query, matchIndex, matchCounter)
		result.WriteString(res)
		matchCounter = count

		end := strings.IndexAny(highlighted[start:], "mABCDHJKfhnpsu")
		if end == -1 {
			result.WriteString(highlighted[start:])
			break
		}
		end += start + 1
		result.WriteString(highlighted[start:end])
		cursor = end
	}

	return result.String()
}

func (p *Processor) highlightPlainPart(text, query string, targetIdx, currentCount int) (string, int) {
	if query == "" || text == "" {
		return text, currentCount
	}

	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)

	var result strings.Builder
	cursor := 0
	count := currentCount
	for {
		idx := strings.Index(lowerText[cursor:], lowerQuery)
		if idx == -1 {
			result.WriteString(text[cursor:])
			break
		}

		idx += cursor
		result.WriteString(text[cursor:idx])

		matchText := text[idx : idx+len(query)]
		style := searchMatchStyle
		if count == targetIdx {
			style = currentMatchStyle
		}
		result.WriteString(style.Render(matchText))

		count++
		cursor = idx + len(query)
	}
	return result.String(), count
}

func (p *Processor) renderHexWithSearch(query string, matchIndex int) string {
	content := p.GetPlain()
	var sb strings.Builder
	d := hex.Dumper(&sb)
	d.Write([]byte(content))
	d.Close()

	lines := strings.Split(sb.String(), "\n")
	var styledSB strings.Builder
	matchCounter := 0

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "  ", 3)
		if len(parts) >= 1 {
			styledSB.WriteString(hexOffsetStyle.Render(parts[0]))
		}
		if len(parts) >= 2 {
			res, count := p.highlightPlainPart(parts[1], query, matchIndex, matchCounter)
			styledSB.WriteString(hexByteStyle.Render(res))
			matchCounter = count
		}
		if len(parts) >= 3 {
			styledSB.WriteString("  ")
			res, count := p.highlightPlainPart(parts[2], query, matchIndex, matchCounter)
			styledSB.WriteString(hexASCIIStyle.Render("|"+res+"|"))
			matchCounter = count
		}
		styledSB.WriteRune('\n')
	}
	styledSB.WriteString("\n" + eofStyle.Render("EOF"))
	return styledSB.String()
}
