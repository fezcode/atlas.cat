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

	// Style for search matches
	searchMatchStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#D4AF37")).
				Foreground(lipgloss.Color("#000000"))

	currentMatchStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#FFFFFF")).
				Foreground(lipgloss.Color("#000000")).
				Bold(true)
)

type Processor struct {
	Path            string
	ShowLineNumbers bool
	HexMode         bool
	WrapLines       bool
	ViewportWidth   int

	lines []string // Plain text lines
}

func NewProcessor(path string, showLines, hexMode, wrap bool) (*Processor, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	p := &Processor{
		Path:            path,
		ShowLineNumbers: showLines,
		HexMode:         hexMode,
		WrapLines:       wrap,
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		p.lines = append(p.lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *Processor) GetPlain() string {
	return strings.Join(p.lines, "\n")
}

func (p *Processor) HighlightAll(searchQuery string, matchIndex int, matches []int) string {
	content := p.GetPlain()
	if p.HexMode {
		return p.renderHexWithSearch(searchQuery, matchIndex, matches)
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
	
	// Since sophisticated ANSI-aware search highlighting is complex, 
	// we'll use a simpler approach: 
	// If search query is present, we wrap matches in the final string 
	// BUT we only do it if the match doesn't contain ANSI codes.
	// For a senior developer tool, we'll implement a basic but working version.
	if searchQuery != "" {
		highlighted = p.applySearchHighlight(highlighted, searchQuery, matchIndex, matches)
	}

	lines := strings.Split(highlighted, "\n")
	var finalLines []string
	width := len(fmt.Sprintf("%d", len(lines)))

	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			break
		}

		formattedLine := line
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
				wrapped := lipgloss.NewStyle().Width(contentWidth).Render(formattedLine)
				subLines := strings.Split(wrapped, "\n")
				for j, sl := range subLines {
					if j == 0 {
						finalLines = append(finalLines, prefix+sl)
					} else {
						indent := strings.Repeat(" ", width+1)
						finalLines = append(finalLines, indent+sl)
					}
				}
				continue
			}
		}

		finalLines = append(finalLines, prefix+formattedLine)
	}

	highlighted = strings.Join(finalLines, "\n")

	if !strings.HasSuffix(highlighted, "\n") {
		highlighted += "\n"
	}
	highlighted += eofStyle.Render("EOF")
	return highlighted
}

func (p *Processor) applySearchHighlight(highlighted, query string, matchIndex int, matches []int) string {
	// This is a simplified search highlight that works for non-overlapping ANSI cases.
	// In a real-world scenario, we'd use the Tokenizer injection method.
	// For now, let's use a placeholder-based replacement to avoid breaking existing ANSI.
	
	// We'll highlight the CURRENT match with a different style if possible.
	// To keep it simple and bug-free, we'll just use strings.Replace
	// but we must be careful not to replace inside ANSI codes.
	
	// We'll use a regex-free way to find parts of the string that aren't ANSI codes.
	var result strings.Builder
	cursor := 0
	for {
		// Find next ANSI escape
		start := strings.Index(highlighted[cursor:], "\x1b[")
		if start == -1 {
			// No more ANSI, process the rest
			result.WriteString(p.highlightPlainPart(highlighted[cursor:], query))
			break
		}
		
		start += cursor
		// Process plain text before ANSI
		result.WriteString(p.highlightPlainPart(highlighted[cursor:start], query))
		
		// Find end of ANSI escape
		end := strings.IndexAny(highlighted[start:], "mABCDHJKfhnpsu") // Common ANSI terminators
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

func (p *Processor) highlightPlainPart(text, query string) string {
	if query == "" || text == "" {
		return text
	}
	
	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)
	
	var result strings.Builder
	cursor := 0
	for {
		idx := strings.Index(lowerText[cursor:], lowerQuery)
		if idx == -1 {
			result.WriteString(text[cursor:])
			break
		}
		
		idx += cursor
		result.WriteString(text[cursor:idx])
		
		// Wrap match in style
		matchText := text[idx : idx+len(query)]
		result.WriteString(searchMatchStyle.Render(matchText))
		
		cursor = idx + len(query)
	}
	return result.String()
}

func (p *Processor) renderHexWithSearch(query string, matchIndex int, matches []int) string {
	content := p.GetPlain()
	var sb strings.Builder
	d := hex.Dumper(&sb)
	d.Write([]byte(content))
	d.Close()

	lines := strings.Split(sb.String(), "\n")
	var styledSB strings.Builder
	
	for _, line := range lines {
		if line == "" {
			continue
		}
		
		parts := strings.SplitN(line, "  ", 3)
		if len(parts) >= 1 {
			styledSB.WriteString(hexOffsetStyle.Render(parts[0]))
		}
		if len(parts) >= 2 {
			bytesPart := parts[1]
			if query != "" {
				bytesPart = p.highlightPlainPart(bytesPart, query)
			}
			styledSB.WriteString(hexByteStyle.Render(bytesPart))
		}
		if len(parts) >= 3 {
			styledSB.WriteString("  ")
			asciiPart := parts[2]
			if query != "" {
				asciiPart = p.highlightPlainPart(asciiPart, query)
			}
			styledSB.WriteString(hexASCIIStyle.Render("|" + asciiPart + "|"))
		}
		styledSB.WriteRune('\n')
	}
	styledSB.WriteString("\n")
	styledSB.WriteString(eofStyle.Render("EOF"))
	return styledSB.String()
}
