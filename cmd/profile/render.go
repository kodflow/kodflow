package main

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"
)

// width matches the README column on a desktop screen; GitHub scales the
// image down on narrow screens, so nothing is laid out for a second size.
const width = 840

const pad = 32

// renderHeader draws the name banner: a shell prompt, the login with a
// blinking cursor, the tagline, and a small activity-like grid for texture.
func renderHeader(p *Profile, t Theme) string {
	const h = 140
	var b strings.Builder
	css := `.cur{animation:blink 1.1s steps(1) infinite}@keyframes blink{50%{opacity:0}}` +
		`.sq{animation:rise .5s ease-out both}@keyframes rise{from{opacity:0}}`
	svgOpen(&b, t, width, h, p.Login+" — "+p.Tagline, css)
	text(&b, pad, 44, 14, t.Muted, "", "~ $ whoami")
	text(&b, pad, 92, 40, t.Fg, "700", p.Login)
	cx := pad + float64(len([]rune(p.Login)))*40*charW + 6
	fmt.Fprintf(&b, `<rect class="cur" x="%.1f" y="62" width="20" height="34" fill="%s"/>`, cx, t.Accent)
	text(&b, pad, 120, 15, t.Muted, "", p.Tagline)

	// The grid is seeded by the login so it is stable between runs: a banner
	// that changes every day would make the daily commit noisy for nothing.
	const cols, rows, cell, gap = 20, 7, 10, 3
	x0 := width - pad - cols*(cell+gap) + gap
	hs := fnv.New32a()
	hs.Write([]byte(p.Login))
	seed := hs.Sum32()
	for c := 0; c < cols; c++ {
		for r := 0; r < rows; r++ {
			seed = seed*1664525 + 1013904223
			lvl := int(seed>>28) % (maxLevel + 1)
			// Brighter towards the right: the grid reads as "more lately".
			if c < cols/3 && lvl == maxLevel {
				lvl = 1
			}
			fmt.Fprintf(&b, `<rect class="sq" style="animation-delay:%dms" x="%d" y="%d" width="%d" height="%d" rx="2" fill="%s"/>`,
				c*40, x0+c*(cell+gap), 24+r*(cell+gap), cell, cell, t.Levels[lvl])
		}
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// logo is the pixel mark shown neofetch-style on the card. It is drawn with
// rectangles, not box-drawing characters: those glyphs fall back to fonts of
// different widths on each system and the letter came apart.
var logo = []string{
	"XX...XX",
	"XX..XX.",
	"XX.XX..",
	"XXXX...",
	"XX.XX..",
	"XX..XX.",
	"XX...XX",
}

const logoCell, logoGap = 11, 2

// renderCard draws the neofetch-like summary: who, what, and live GitHub
// numbers. Stats may be nil (offline run); the rows then show a dash.
func renderCard(p *Profile, s *Stats, t Theme, now time.Time) string {
	const size, lh, keyW = 14, 22, 15
	vx := 180.0 + keyW*size*charW
	maxChars := int((width - pad - vx) / (size * charW))

	type row struct{ k, v string }
	rows := []row{
		{"Role", p.Role},
		{"Uptime", uptime(p.SinceYear, now)},
		{"Focus", strings.Join(p.Focus, " · ")},
		{"Languages", strings.Join(p.Stack["languages"], " · ")},
		{"Infra", strings.Join(p.Stack["infrastructure"], " · ")},
		{"Data", strings.Join(p.Stack["data"], " · ")},
		{"", ""},
		{"Public repos", dash(s, func(s *Stats) int { return s.Repos })},
		{"Stars", dash(s, func(s *Stats) int { return s.Stars })},
		{"Contrib. 1y", dash(s, func(s *Stats) int { return s.Contributions })},
	}
	lines := 0
	for _, r := range rows {
		lines += max(1, len(wrap(r.v, maxChars)))
	}
	hasBar := s != nil && len(s.Languages) > 0
	h := 76 + lines*lh + 20
	if hasBar {
		h += 64
	}

	var b strings.Builder
	svgOpen(&b, t, width, h, p.Login+" at a glance", `.in{animation:fade .5s ease-out both}@keyframes fade{from{opacity:0}}`)
	// Shadow first, then the letter, offset like a terminal ANSI-shadow font.
	for layer, fill := range []string{t.Levels[1], t.Accent} {
		off := 3 * (1 - layer)
		for r, line := range logo {
			for c, ch := range line {
				if ch == 'X' {
					fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" rx="1.5" fill="%s"/>`,
						pad+c*(logoCell+logoGap)+off, 34+r*(logoCell+logoGap)+off, logoCell, logoCell, fill)
				}
			}
		}
	}
	for i, c := range t.Levels[1:] {
		fmt.Fprintf(&b, `<rect x="%d" y="%d" width="26" height="12" rx="2" fill="%s"/>`, pad+i*31, 34+len(logo)*(logoCell+logoGap)+16, c)
	}

	fmt.Fprintf(&b, `<text x="180" y="44" font-size="16" font-weight="700"><tspan fill="%s">%s</tspan><tspan fill="%s">@</tspan><tspan fill="%s">github</tspan></text>`,
		t.Accent, esc(p.Login), t.Fg, t.Accent)
	fmt.Fprintf(&b, `<line x1="180" y1="56" x2="%d" y2="56" stroke="%s"/>`, width-pad, t.Border)

	y := 84
	for i, r := range rows {
		if r.k == "" {
			y += lh
			continue
		}
		fmt.Fprintf(&b, `<g class="in" style="animation-delay:%dms">`, i*60)
		text(&b, 180, float64(y), size, t.Accent, "700", r.k)
		for j, l := range wrap(r.v, maxChars) {
			if j > 0 {
				y += lh
			}
			text(&b, vx, float64(y), size, t.Fg, "", l)
		}
		b.WriteString(`</g>`)
		y += lh
	}

	if hasBar {
		y += 4
		barW := float64(width - pad - 180)
		x := 180.0
		fmt.Fprintf(&b, `<clipPath id="bar"><rect x="180" y="%d" width="%.1f" height="10" rx="5"/></clipPath><g clip-path="url(#bar)">`, y, barW)
		for i, l := range s.Languages {
			w := barW * l.Percent / 100
			fmt.Fprintf(&b, `<rect x="%.1f" y="%d" width="%.1f" height="10" fill="%s"/>`, x, y, w, t.Langs[i%len(t.Langs)])
			x += w
		}
		b.WriteString(`</g>`)
		lx := 180.0
		for i, l := range s.Languages {
			label := fmt.Sprintf("%s %.1f%%", l.Name, l.Percent)
			fmt.Fprintf(&b, `<rect x="%.1f" y="%d" width="10" height="10" rx="2" fill="%s"/>`, lx, y+24, t.Langs[i%len(t.Langs)])
			text(&b, lx+16, float64(y+33), 12, t.Muted, "", label)
			lx += 16 + float64(len([]rune(label)))*12*charW + 12
		}
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// uptime rounds the career length down to five years: an exact start year
// is one more detail that could be matched against a named CV.
func uptime(since int, now time.Time) string {
	if since <= 0 {
		return "—"
	}
	y := now.Year() - since
	return fmt.Sprintf("%d+ years in production", y/5*5)
}

func dash(s *Stats, f func(*Stats) int) string {
	if s == nil || f(s) < 0 {
		return "—"
	}
	return thousands(f(s))
}

// thousands formats n with comma separators, as GitHub does.
func thousands(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// renderTimeline draws technologies against coarse eras. Eras rather than
// years keep the chart from mirroring a dated CV line by line.
func renderTimeline(p *Profile, t Theme) string {
	const rowH, headH, nameW = 24, 30, 250
	fams := p.families()
	h := 112 + len(p.Timeline)*rowH + len(fams)*headH + 44
	colX := float64(pad + nameW)
	colW := (float64(width-pad) - colX) / float64(len(p.Eras))

	var b strings.Builder
	css := `.c{animation:grow .7s cubic-bezier(.2,.7,.2,1) both;transform-box:fill-box;transform-origin:left}` +
		`@keyframes grow{from{transform:scaleX(0)}}`
	svgOpen(&b, t, width, h, "Stack over time", css)
	text(&b, pad, 44, 16, t.Fg, "700", "stack over time")
	text(&b, pad, 66, 12, t.Muted, "", "coarse eras on purpose · brighter cell = more of the working time")
	for i, e := range p.Eras {
		text(&b, colX+float64(i)*colW+4, 100, 12, t.Muted, "", e)
	}

	y := 112
	for _, f := range fams {
		y += headH
		text(&b, pad, float64(y-8), 11, t.Accent, "700", strings.ToUpper(f))
		for _, r := range p.Timeline {
			if r.Family != f {
				continue
			}
			text(&b, pad, float64(y+rowH-8), 13, t.Fg, "", r.Name)
			for i, l := range r.Levels {
				fmt.Fprintf(&b, `<rect class="c" style="animation-delay:%dms" x="%.1f" y="%d" width="%.1f" height="14" rx="3" fill="%s"/>`,
					i*140, colX+float64(i)*colW+2, y+rowH-20, colW-4, t.Levels[l])
			}
			y += rowH
		}
	}

	y += 30
	lx := float64(width - pad - 4*22 - 80)
	text(&b, lx, float64(y), 11, t.Muted, "", "less")
	for i, c := range t.Levels {
		fmt.Fprintf(&b, `<rect x="%.1f" y="%d" width="18" height="10" rx="2" fill="%s" stroke="%s"/>`, lx+36+float64(i)*22, y-9, c, t.Border)
	}
	text(&b, lx+36+4*22+4, float64(y), 11, t.Muted, "", "more")
	b.WriteString(`</svg>`)
	return b.String()
}
