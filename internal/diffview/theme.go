package diffview

import (
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// InitializeTheme configures diff colors from one of: auto, dark, light.
func InitializeTheme(mode string) {
	theme := strings.ToLower(strings.TrimSpace(mode))
	if theme == "" {
		theme = "auto"
	}

	dark := true
	switch theme {
	case "dark":
		dark = true
	case "light":
		dark = false
	default:
		dark = detectDarkBackground()
	}

	if dark {
		applyDarkTheme()
		return
	}
	applyLightTheme()
}

func detectDarkBackground() bool {
	if dark, ok := darkFromColorFGBG(os.Getenv("COLORFGBG")); ok {
		return dark
	}
	return termenv.HasDarkBackground()
}

func darkFromColorFGBG(value string) (bool, bool) {
	parts := strings.Split(strings.TrimSpace(value), ";")
	for i := len(parts) - 1; i >= 0; i-- {
		idx, err := strconv.Atoi(strings.TrimSpace(parts[i]))
		if err != nil {
			continue
		}
		return isDarkColorIndex(idx), true
	}
	return false, false
}

func isDarkColorIndex(idx int) bool {
	if idx < 0 {
		return true
	}
	if idx <= 6 {
		return true
	}
	if idx <= 15 {
		return false
	}
	if idx >= 16 && idx <= 231 {
		cube := idx - 16
		r := cube / 36
		g := (cube / 6) % 6
		b := cube % 6
		return rgbLuma(cubeChannelValue(r), cubeChannelValue(g), cubeChannelValue(b)) < 128
	}
	if idx >= 232 && idx <= 255 {
		gray := 8 + (idx-232)*10
		return gray < 128
	}
	return idx < 128
}

func cubeChannelValue(v int) int {
	if v <= 0 {
		return 0
	}
	return 55 + v*40
}

func rgbLuma(r, g, b int) int {
	return (299*r + 587*g + 114*b) / 1000
}

func applyDarkTheme() {
	addBaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("78")).Background(lipgloss.Color("#1a2620"))
	deleteBaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Background(lipgloss.Color("#2a1f21"))
	changeOldBaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("210")).Background(lipgloss.Color("#252022"))
	changeNewBaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("121")).Background(lipgloss.Color("#1f2523"))
	contextBaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	hunkBaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("111")).Bold(true)

	addWordStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("121")).Background(lipgloss.Color("22")).Bold(true)
	deleteWordStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("210")).Background(lipgloss.Color("52")).Bold(true)
	cursorRowBg = lipgloss.Color("236")

	cursorGutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("45")).Bold(true)
	commentGutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("220")).Bold(true)
	cursorCommentGutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("201")).Bold(true)
	addGutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("121")).Background(lipgloss.Color("22")).Bold(true)
	deleteGutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("210")).Background(lipgloss.Color("52")).Bold(true)
	changeOldGutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("210")).Background(lipgloss.Color("53")).Bold(true)
	changeNewGutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("121")).Background(lipgloss.Color("23")).Bold(true)
	addMetaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("121")).Background(lipgloss.Color("22")).Bold(true)
	deleteMetaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("210")).Background(lipgloss.Color("52")).Bold(true)
	changeOldMetaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("210")).Background(lipgloss.Color("53")).Bold(true)
	changeNewMetaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("121")).Background(lipgloss.Color("23")).Bold(true)

	commentInlineTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Background(lipgloss.Color("236"))

	syntaxKeywordColor = lipgloss.Color("141")
	syntaxStringColor = lipgloss.Color("186")
	syntaxCommentColor = lipgloss.Color("244")
	syntaxTypeColor = lipgloss.Color("117")
	syntaxFunctionColor = lipgloss.Color("221")
	syntaxNumberColor = lipgloss.Color("215")
	syntaxOperatorColor = lipgloss.Color("204")
	syntaxPreprocessorColor = lipgloss.Color("178")
}

func applyLightTheme() {
	addBaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("22")).Background(lipgloss.Color("194"))
	deleteBaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("88")).Background(lipgloss.Color("224"))
	changeOldBaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("130")).Background(lipgloss.Color("223"))
	changeNewBaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("28")).Background(lipgloss.Color("193"))
	contextBaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("236"))
	hunkBaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("25")).Bold(true)

	addWordStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("22")).Background(lipgloss.Color("121")).Bold(true)
	deleteWordStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("88")).Background(lipgloss.Color("217")).Bold(true)
	cursorRowBg = lipgloss.Color("254")

	cursorGutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(lipgloss.Color("25")).Bold(true)
	commentGutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("220")).Bold(true)
	cursorCommentGutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(lipgloss.Color("161")).Bold(true)
	addGutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("22")).Background(lipgloss.Color("121")).Bold(true)
	deleteGutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("88")).Background(lipgloss.Color("217")).Bold(true)
	changeOldGutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("130")).Background(lipgloss.Color("223")).Bold(true)
	changeNewGutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("28")).Background(lipgloss.Color("193")).Bold(true)
	addMetaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("22")).Background(lipgloss.Color("121")).Bold(true)
	deleteMetaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("88")).Background(lipgloss.Color("217")).Bold(true)
	changeOldMetaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("130")).Background(lipgloss.Color("223")).Bold(true)
	changeNewMetaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("28")).Background(lipgloss.Color("193")).Bold(true)

	commentInlineTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Background(lipgloss.Color("253"))

	syntaxKeywordColor = lipgloss.Color("55")
	syntaxStringColor = lipgloss.Color("94")
	syntaxCommentColor = lipgloss.Color("244")
	syntaxTypeColor = lipgloss.Color("24")
	syntaxFunctionColor = lipgloss.Color("130")
	syntaxNumberColor = lipgloss.Color("88")
	syntaxOperatorColor = lipgloss.Color("161")
	syntaxPreprocessorColor = lipgloss.Color("89")
}
