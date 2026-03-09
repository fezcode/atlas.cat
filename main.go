package main

import (
	"flag"
	"fmt"
	"os"

	"atlas.cat/internal/ui"
	"atlas.cat/internal/viewer"
	tea "github.com/charmbracelet/bubbletea"
)

var Version = "dev"

func main() {
	showVersion := flag.Bool("v", false, "Show version")
	showVersionLong := flag.Bool("version", false, "Show version")
	noInteractive := flag.Bool("n", false, "Non-interactive mode (direct output)")
	showLineNumbers := flag.Bool("l", false, "Show line numbers")
	hexMode := flag.Bool("H", false, "Hex mode")
	wrapLines := flag.Bool("w", false, "Wrap long lines")
	
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Atlas Cat - A beautiful terminal text viewer.\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n  atlas.cat [flags] <file>\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	
	flag.Parse()

	if *showVersion || *showVersionLong {
		fmt.Printf("atlas.cat v%s\n", Version)
		return
	}

	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	filePath := args[0]
	// We no longer read everything here to save RAM
	p, err := viewer.NewProcessor(filePath, *showLineNumbers, *hexMode, *wrapLines)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing: %v\n", err)
		os.Exit(1)
	}

	if *noInteractive {
		fmt.Print(p.HighlightAll("", -1))
		return
	}

	// Interactive TUI Mode
	pModel := ui.NewModel(p)
	prog := tea.NewProgram(pModel, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
