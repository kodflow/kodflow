package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Proof holds numbers measured on the code itself rather than on popularity:
// what was written, how much of it is tested, how the pipelines are guarded.
type Proof struct {
	Repos         int
	Lines         map[string]int // non-blank lines of hand-written code per language
	GoCode        int            // non-blank lines in non-test Go files
	GoTest        int            // non-blank lines in _test.go files
	GoTests       int            // Test, Benchmark and Fuzz functions
	ReposWithCI   int
	Workflows     int
	ActionUses    int // third-party action references in workflows
	ActionsPinned int // of which pinned to a full commit SHA
	ShellScripts  int
	ShellStrict   int // of which start with `set -e` in some form
}

// langByExt maps extensions to the languages worth counting. Config and
// markup (YAML, JSON, Markdown) are left out: they would inflate the total
// without saying anything about the code.
var langByExt = map[string]string{
	".go": "Go", ".sh": "Shell", ".bash": "Shell", ".ts": "TypeScript", ".tsx": "TypeScript",
	".js": "JavaScript", ".mjs": "JavaScript", ".py": "Python", ".rs": "Rust", ".tf": "Terraform", ".hcl": "Terraform",
}

// skipDirs are directories whose content was not written by the owner.
var skipDirs = map[string]bool{
	".git": true, "vendor": true, "node_modules": true, "third_party": true, "testdata": true, "dist": true,
}

var (
	reGoTest   = regexp.MustCompile(`(?m)^func (Test|Benchmark|Fuzz)\w*\(`)
	reUses     = regexp.MustCompile(`(?m)^\s*-?\s*uses:\s*["']?([^\s"'#]+)`)
	rePinned   = regexp.MustCompile(`@[0-9a-f]{40}$`)
	reSetE     = regexp.MustCompile(`(?m)^\s*set\s+-[a-z]*e`)
	reGenerate = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)
)

// measureRepos shallow-clones every repository into a temporary directory
// and measures it. Only the default branch at its tip is read.
func measureRepos(ctx context.Context, login string, repos []string) (*Proof, error) {
	tmp, err := os.MkdirTemp("", "profile-proof-")
	if err != nil {
		return nil, fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	p := &Proof{Lines: map[string]int{}}
	for _, r := range repos {
		dir := filepath.Join(tmp, r)
		url := fmt.Sprintf("https://github.com/%s/%s.git", login, r)
		cmd := exec.CommandContext(ctx, "git", "clone", "--quiet", "--depth", "1", url, dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("clone %s: %w: %s", r, err, bytes.TrimSpace(out))
		}
		if err := p.measure(dir); err != nil {
			return nil, fmt.Errorf("measure %s: %w", r, err)
		}
		p.Repos++
	}
	return p, nil
}

// measure walks one checkout and adds its numbers to p.
func (p *Proof) measure(root string) error {
	workflows := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if dir := filepath.ToSlash(filepath.Dir(rel)); dir == ".github/workflows" &&
			(strings.HasSuffix(rel, ".yml") || strings.HasSuffix(rel, ".yaml")) {
			workflows++
			return p.measureWorkflow(path)
		}
		lang, ok := langByExt[filepath.Ext(path)]
		if !ok || !d.Type().IsRegular() {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if lang == "Go" && isGenerated(src) {
			return nil
		}
		n := nonBlank(src)
		p.Lines[lang] += n
		switch {
		case lang == "Go" && strings.HasSuffix(path, "_test.go"):
			p.GoTest += n
			p.GoTests += len(reGoTest.FindAll(src, -1))
		case lang == "Go":
			p.GoCode += n
		case lang == "Shell":
			p.ShellScripts++
			if reSetE.Match(src) {
				p.ShellStrict++
			}
		}
		return nil
	})
	if workflows > 0 {
		p.ReposWithCI++
		p.Workflows += workflows
	}
	return err
}

// measureWorkflow counts third-party actions and how many are pinned to a
// commit SHA: a tag can be moved under your feet, a SHA cannot.
func (p *Proof) measureWorkflow(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, m := range reUses.FindAllSubmatch(src, -1) {
		ref := string(m[1])
		if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "docker://") || !strings.Contains(ref, "@") {
			continue
		}
		p.ActionUses++
		if rePinned.MatchString(ref) {
			p.ActionsPinned++
		}
	}
	return nil
}

// isGenerated follows the Go convention for generated files: the marker
// line must appear before the package clause.
func isGenerated(src []byte) bool {
	sc := bufio.NewScanner(bytes.NewReader(src))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "package ") {
			return false
		}
		if reGenerate.MatchString(line) {
			return true
		}
	}
	return false
}

func nonBlank(src []byte) int {
	n := 0
	for _, l := range bytes.Split(src, []byte("\n")) {
		if len(bytes.TrimSpace(l)) > 0 {
			n++
		}
	}
	return n
}

func percent(part, whole int) int {
	if whole == 0 {
		return 0
	}
	return (100*part + whole/2) / whole
}

// kilo shortens a line count the way people read it: 56157 -> 56k.
func kilo(n int) string {
	switch {
	case n >= 10000:
		return fmt.Sprintf("%dk", (n+500)/1000)
	case n >= 1000:
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	default:
		return fmt.Sprint(n)
	}
}

// renderProof draws the by-the-numbers card: five measured figures, then the
// written lines per language. Every figure is recomputed daily from the code.
func renderProof(pr *Proof, t Theme) string {
	const h, tileY, tileH, gap = 250, 88, 92, 12
	type tile struct{ big, label, sub string }
	ratio := "—"
	if pr.GoCode > 0 {
		ratio = fmt.Sprintf("%.1f×", float64(pr.GoTest)/float64(pr.GoCode))
	}
	tiles := []tile{
		{kilo(pr.GoCode), "Go lines", "tests excluded"},
		{thousands(pr.GoTests), "Go tests", "incl. fuzz & bench"},
		{ratio, "test : code", "lines of Go"},
		{fmt.Sprintf("%d/%d", pr.ReposWithCI, pr.Repos), "repos with CI", fmt.Sprintf("%d workflows", pr.Workflows)},
		{fmt.Sprintf("%d%%", percent(pr.ActionsPinned, pr.ActionUses)), "actions pinned", "to a commit SHA"},
	}
	tileW := (float64(width-2*pad) - float64(gap*(len(tiles)-1))) / float64(len(tiles))

	var b strings.Builder
	svgOpen(&b, t, width, h, "By the numbers", `.in{animation:fade .5s ease-out both}@keyframes fade{from{opacity:0;transform:translateY(4px)}}`)
	text(&b, pad, 44, 16, t.Fg, "700", "by the numbers")
	text(&b, pad, 66, 12, t.Muted, "", fmt.Sprintf("measured daily on %d public repositories · forks and generated code excluded", pr.Repos))
	for i, tl := range tiles {
		x := float64(pad) + float64(i)*(tileW+gap)
		fmt.Fprintf(&b, `<g class="in" style="animation-delay:%dms">`, i*90)
		fmt.Fprintf(&b, `<rect x="%.1f" y="%d" width="%.1f" height="%d" rx="8" fill="%s" stroke="%s"/>`, x, tileY, tileW, tileH, t.Levels[0], t.Border)
		text(&b, x+14, tileY+40, 26, t.Accent, "700", tl.big)
		text(&b, x+14, tileY+62, 12, t.Fg, "", tl.label)
		text(&b, x+14, tileY+78, 11, t.Muted, "", tl.sub)
		b.WriteString(`</g>`)
	}

	// Languages by lines, largest first, JS and TS merged: split, both look
	// smaller than the front-end work they add up to.
	lines := map[string]int{}
	for l, n := range pr.Lines {
		if l == "JavaScript" || l == "TypeScript" {
			l = "JS/TS"
		}
		lines[l] += n
	}
	var parts []string
	for _, ls := range topLanguages(lines, len(lines)) {
		parts = append(parts, fmt.Sprintf("%s %s", ls.Name, kilo(lines[ls.Name])))
	}
	fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="12"><tspan fill="%s" font-weight="700">lines written, tests included  </tspan><tspan fill="%s">%s</tspan></text>`,
		pad, tileY+tileH+40, t.Accent, t.Muted, esc(strings.Join(parts, " · ")))
	b.WriteString(`</svg>`)
	return b.String()
}
