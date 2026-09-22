package main

import (
	"fmt"
	"strings"
)

// Theme is one colour scheme. Every SVG is rendered once per theme and the
// README picks the right one with <picture> and prefers-color-scheme.
type Theme struct {
	Name   string
	Bg     string
	Border string
	Fg     string
	Muted  string
	Accent string
	Levels [maxLevel + 1]string // timeline cells, from unused to brightest
	Langs  []string             // language bar segments, largest first
}

// The palette is amber on neutral grey on purpose: it must not echo any other
// site the owner runs, or the colours alone would link the two identities.
var themes = []Theme{
	{
		Name: "dark", Bg: "#0d1117", Border: "#30363d", Fg: "#e6edf3", Muted: "#8b949e", Accent: "#f0a830",
		Levels: [maxLevel + 1]string{"#161b22", "#4d3510", "#9a6418", "#f0a830"},
		Langs:  []string{"#f0a830", "#c8782a", "#8f5a26", "#8b949e", "#57606a", "#30363d"},
	},
	{
		Name: "light", Bg: "#ffffff", Border: "#d0d7de", Fg: "#1f2328", Muted: "#656d76", Accent: "#b35c00",
		Levels: [maxLevel + 1]string{"#f6f8fa", "#f5d9a8", "#e0a04a", "#b35c00"},
		Langs:  []string{"#b35c00", "#d9822b", "#ebb56a", "#656d76", "#afb8c1", "#d0d7de"},
	},
}

// fontStack uses fonts already on the visitor's machine: GitHub serves README
// images through a proxy that blocks web fonts inside SVG.
const fontStack = `ui-monospace,SFMono-Regular,Menlo,Consolas,'Liberation Mono',monospace`

// charW approximates a monospace glyph width as a fraction of the font size;
// layout relies on it because an <img> SVG cannot measure its own text.
const charW = 0.6

// svgOpen starts a document with the shared style block. Motion is opt-out
// through prefers-reduced-motion, which GitHub's image proxy passes through.
func svgOpen(b *strings.Builder, t Theme, w, h int, title, extraCSS string) {
	fmt.Fprintf(b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-label="%s">`, w, h, w, h, esc(title))
	fmt.Fprintf(b, `<title>%s</title><style>text{font-family:%s}%s`+
		`@media (prefers-reduced-motion:reduce){*{animation:none!important}}</style>`, esc(title), fontStack, extraCSS)
	fmt.Fprintf(b, `<rect x="0.5" y="0.5" width="%d" height="%d" rx="10" fill="%s" stroke="%s"/>`, w-1, h-1, t.Bg, t.Border)
}

func text(b *strings.Builder, x, y float64, size int, fill, weight, s string) {
	fmt.Fprintf(b, `<text x="%.1f" y="%.1f" font-size="%d" fill="%s"%s>%s</text>`, x, y, size, fill, weightAttr(weight), esc(s))
}

func weightAttr(w string) string {
	if w == "" {
		return ""
	}
	return ` font-weight="` + w + `"`
}

var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")

func esc(s string) string { return escaper.Replace(s) }

// wrap splits s on spaces into lines of at most width runes.
func wrap(s string, width int) []string {
	var lines []string
	cur := ""
	for _, w := range strings.Fields(s) {
		switch {
		case cur == "":
			cur = w
		case len([]rune(cur))+1+len([]rune(w)) <= width:
			cur += " " + w
		default:
			lines = append(lines, cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}
