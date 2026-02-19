// Package ui provides TUI components for hark built on Bubble Tea and Lipgloss.
package ui

import "github.com/charmbracelet/lipgloss"

// ChromaPalette maps a theme name to its corresponding Chroma style name and
// provides Lipgloss colours for UI chrome that complement the Chroma theme.
type ChromaPalette struct {
	Name       string // display name (e.g. "Dracula")
	ChromaName string // Chroma style identifier (e.g. "dracula")

	// Lipgloss colours for UI chrome
	StatusBarBg  lipgloss.Color
	StatusBarFg  lipgloss.Color
	TimelineBg   lipgloss.Color
	TimelineFg   lipgloss.Color
	AccentColor  lipgloss.Color
	AddedBg      lipgloss.Color
	RemovedBg    lipgloss.Color
	ContextFg    lipgloss.Color
	HeaderFg     lipgloss.Color
	FileListBg   lipgloss.Color
	FileListFg   lipgloss.Color
	SelectedBg   lipgloss.Color
	UnseenBadge  lipgloss.Color
	HelpBg       lipgloss.Color
	HelpFg       lipgloss.Color
	HelpHeaderFg lipgloss.Color
}

// BundledThemes lists all 10 bundled themes in order.
var BundledThemes = []ChromaPalette{
	{
		Name: "Dracula", ChromaName: "dracula",
		StatusBarBg: lipgloss.Color("#44475a"), StatusBarFg: lipgloss.Color("#f8f8f2"),
		TimelineBg: lipgloss.Color("#44475a"), TimelineFg: lipgloss.Color("#f8f8f2"),
		AccentColor: lipgloss.Color("#bd93f9"), AddedBg: lipgloss.Color("#2a4d2a"),
		RemovedBg: lipgloss.Color("#4d2a2a"), ContextFg: lipgloss.Color("#6272a4"),
		HeaderFg: lipgloss.Color("#bd93f9"), FileListBg: lipgloss.Color("#282a36"),
		FileListFg: lipgloss.Color("#f8f8f2"), SelectedBg: lipgloss.Color("#44475a"),
		UnseenBadge: lipgloss.Color("#50fa7b"), HelpBg: lipgloss.Color("#282a36"),
		HelpFg: lipgloss.Color("#f8f8f2"), HelpHeaderFg: lipgloss.Color("#bd93f9"),
	},
	{
		Name: "Catppuccin Mocha", ChromaName: "catppuccin-mocha",
		StatusBarBg: lipgloss.Color("#313244"), StatusBarFg: lipgloss.Color("#cdd6f4"),
		TimelineBg: lipgloss.Color("#313244"), TimelineFg: lipgloss.Color("#cdd6f4"),
		AccentColor: lipgloss.Color("#cba6f7"), AddedBg: lipgloss.Color("#1e3a2c"),
		RemovedBg: lipgloss.Color("#3a1e1e"), ContextFg: lipgloss.Color("#6c7086"),
		HeaderFg: lipgloss.Color("#cba6f7"), FileListBg: lipgloss.Color("#1e1e2e"),
		FileListFg: lipgloss.Color("#cdd6f4"), SelectedBg: lipgloss.Color("#313244"),
		UnseenBadge: lipgloss.Color("#a6e3a1"), HelpBg: lipgloss.Color("#1e1e2e"),
		HelpFg: lipgloss.Color("#cdd6f4"), HelpHeaderFg: lipgloss.Color("#cba6f7"),
	},
	{
		Name: "Catppuccin Latte", ChromaName: "catppuccin-latte",
		StatusBarBg: lipgloss.Color("#ccd0da"), StatusBarFg: lipgloss.Color("#4c4f69"),
		TimelineBg: lipgloss.Color("#ccd0da"), TimelineFg: lipgloss.Color("#4c4f69"),
		AccentColor: lipgloss.Color("#8839ef"), AddedBg: lipgloss.Color("#d4f0d4"),
		RemovedBg: lipgloss.Color("#f0d4d4"), ContextFg: lipgloss.Color("#9ca0b0"),
		HeaderFg: lipgloss.Color("#8839ef"), FileListBg: lipgloss.Color("#eff1f5"),
		FileListFg: lipgloss.Color("#4c4f69"), SelectedBg: lipgloss.Color("#ccd0da"),
		UnseenBadge: lipgloss.Color("#40a02b"), HelpBg: lipgloss.Color("#eff1f5"),
		HelpFg: lipgloss.Color("#4c4f69"), HelpHeaderFg: lipgloss.Color("#8839ef"),
	},
	{
		Name: "Monokai", ChromaName: "monokai",
		StatusBarBg: lipgloss.Color("#3e3d32"), StatusBarFg: lipgloss.Color("#f8f8f2"),
		TimelineBg: lipgloss.Color("#3e3d32"), TimelineFg: lipgloss.Color("#f8f8f2"),
		AccentColor: lipgloss.Color("#f92672"), AddedBg: lipgloss.Color("#2a4d2a"),
		RemovedBg: lipgloss.Color("#4d2a2a"), ContextFg: lipgloss.Color("#75715e"),
		HeaderFg: lipgloss.Color("#66d9ef"), FileListBg: lipgloss.Color("#272822"),
		FileListFg: lipgloss.Color("#f8f8f2"), SelectedBg: lipgloss.Color("#3e3d32"),
		UnseenBadge: lipgloss.Color("#a6e22e"), HelpBg: lipgloss.Color("#272822"),
		HelpFg: lipgloss.Color("#f8f8f2"), HelpHeaderFg: lipgloss.Color("#66d9ef"),
	},
	{
		Name: "Nord", ChromaName: "nord",
		StatusBarBg: lipgloss.Color("#3b4252"), StatusBarFg: lipgloss.Color("#eceff4"),
		TimelineBg: lipgloss.Color("#3b4252"), TimelineFg: lipgloss.Color("#eceff4"),
		AccentColor: lipgloss.Color("#88c0d0"), AddedBg: lipgloss.Color("#2e3b2e"),
		RemovedBg: lipgloss.Color("#3b2e2e"), ContextFg: lipgloss.Color("#4c566a"),
		HeaderFg: lipgloss.Color("#81a1c1"), FileListBg: lipgloss.Color("#2e3440"),
		FileListFg: lipgloss.Color("#eceff4"), SelectedBg: lipgloss.Color("#3b4252"),
		UnseenBadge: lipgloss.Color("#a3be8c"), HelpBg: lipgloss.Color("#2e3440"),
		HelpFg: lipgloss.Color("#eceff4"), HelpHeaderFg: lipgloss.Color("#81a1c1"),
	},
	{
		Name: "Solarized Dark", ChromaName: "solarized-dark256",
		StatusBarBg: lipgloss.Color("#073642"), StatusBarFg: lipgloss.Color("#839496"),
		TimelineBg: lipgloss.Color("#073642"), TimelineFg: lipgloss.Color("#839496"),
		AccentColor: lipgloss.Color("#268bd2"), AddedBg: lipgloss.Color("#1a3a1a"),
		RemovedBg: lipgloss.Color("#3a1a1a"), ContextFg: lipgloss.Color("#586e75"),
		HeaderFg: lipgloss.Color("#268bd2"), FileListBg: lipgloss.Color("#002b36"),
		FileListFg: lipgloss.Color("#839496"), SelectedBg: lipgloss.Color("#073642"),
		UnseenBadge: lipgloss.Color("#859900"), HelpBg: lipgloss.Color("#002b36"),
		HelpFg: lipgloss.Color("#839496"), HelpHeaderFg: lipgloss.Color("#268bd2"),
	},
	{
		Name: "Solarized Light", ChromaName: "solarized-light",
		StatusBarBg: lipgloss.Color("#eee8d5"), StatusBarFg: lipgloss.Color("#657b83"),
		TimelineBg: lipgloss.Color("#eee8d5"), TimelineFg: lipgloss.Color("#657b83"),
		AccentColor: lipgloss.Color("#268bd2"), AddedBg: lipgloss.Color("#d4f0d4"),
		RemovedBg: lipgloss.Color("#f0d4d4"), ContextFg: lipgloss.Color("#93a1a1"),
		HeaderFg: lipgloss.Color("#268bd2"), FileListBg: lipgloss.Color("#fdf6e3"),
		FileListFg: lipgloss.Color("#657b83"), SelectedBg: lipgloss.Color("#eee8d5"),
		UnseenBadge: lipgloss.Color("#859900"), HelpBg: lipgloss.Color("#fdf6e3"),
		HelpFg: lipgloss.Color("#657b83"), HelpHeaderFg: lipgloss.Color("#268bd2"),
	},
	{
		Name: "One Dark", ChromaName: "onedark",
		StatusBarBg: lipgloss.Color("#3e4451"), StatusBarFg: lipgloss.Color("#abb2bf"),
		TimelineBg: lipgloss.Color("#3e4451"), TimelineFg: lipgloss.Color("#abb2bf"),
		AccentColor: lipgloss.Color("#61afef"), AddedBg: lipgloss.Color("#2a4d2a"),
		RemovedBg: lipgloss.Color("#4d2a2a"), ContextFg: lipgloss.Color("#5c6370"),
		HeaderFg: lipgloss.Color("#61afef"), FileListBg: lipgloss.Color("#282c34"),
		FileListFg: lipgloss.Color("#abb2bf"), SelectedBg: lipgloss.Color("#3e4451"),
		UnseenBadge: lipgloss.Color("#98c379"), HelpBg: lipgloss.Color("#282c34"),
		HelpFg: lipgloss.Color("#abb2bf"), HelpHeaderFg: lipgloss.Color("#61afef"),
	},
	{
		Name: "GitHub Dark", ChromaName: "github-dark",
		StatusBarBg: lipgloss.Color("#161b22"), StatusBarFg: lipgloss.Color("#c9d1d9"),
		TimelineBg: lipgloss.Color("#161b22"), TimelineFg: lipgloss.Color("#c9d1d9"),
		AccentColor: lipgloss.Color("#58a6ff"), AddedBg: lipgloss.Color("#12261e"),
		RemovedBg: lipgloss.Color("#261212"), ContextFg: lipgloss.Color("#8b949e"),
		HeaderFg: lipgloss.Color("#58a6ff"), FileListBg: lipgloss.Color("#0d1117"),
		FileListFg: lipgloss.Color("#c9d1d9"), SelectedBg: lipgloss.Color("#161b22"),
		UnseenBadge: lipgloss.Color("#3fb950"), HelpBg: lipgloss.Color("#0d1117"),
		HelpFg: lipgloss.Color("#c9d1d9"), HelpHeaderFg: lipgloss.Color("#58a6ff"),
	},
	{
		Name: "GitHub Light", ChromaName: "github",
		StatusBarBg: lipgloss.Color("#d0d7de"), StatusBarFg: lipgloss.Color("#24292f"),
		TimelineBg: lipgloss.Color("#d0d7de"), TimelineFg: lipgloss.Color("#24292f"),
		AccentColor: lipgloss.Color("#0969da"), AddedBg: lipgloss.Color("#dafbe1"),
		RemovedBg: lipgloss.Color("#ffebe9"), ContextFg: lipgloss.Color("#656d76"),
		HeaderFg: lipgloss.Color("#0969da"), FileListBg: lipgloss.Color("#ffffff"),
		FileListFg: lipgloss.Color("#24292f"), SelectedBg: lipgloss.Color("#d0d7de"),
		UnseenBadge: lipgloss.Color("#1a7f37"), HelpBg: lipgloss.Color("#ffffff"),
		HelpFg: lipgloss.Color("#24292f"), HelpHeaderFg: lipgloss.Color("#0969da"),
	},
}

// themeIndex maps Chroma style names to BundledThemes indices for fast lookup.
var themeIndex map[string]int

func init() {
	themeIndex = make(map[string]int, len(BundledThemes))
	for i, t := range BundledThemes {
		themeIndex[t.ChromaName] = i
	}
}

// ThemeByName returns the ChromaPalette for the given Chroma style name.
// Falls back to the first bundled theme (Dracula) if not found.
func ThemeByName(chromaName string) ChromaPalette {
	if i, ok := themeIndex[chromaName]; ok {
		return BundledThemes[i]
	}
	return BundledThemes[0]
}

// CycleTheme returns the next theme's Chroma style name given the current one.
func CycleTheme(currentChromaName string) string {
	i, ok := themeIndex[currentChromaName]
	if !ok {
		return BundledThemes[0].ChromaName
	}
	next := (i + 1) % len(BundledThemes)
	return BundledThemes[next].ChromaName
}
