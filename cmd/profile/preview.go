package main

import (
	"fmt"
	"strings"
)

// renderPreview writes a local page that shows the profile in both themes,
// framed roughly like GitHub, so a change can be judged before it is pushed.
func renderPreview(p *Profile, withProof bool) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><meta charset="utf-8"><title>Profile preview</title>
<style>
body{margin:0;font:14px/1.5 -apple-system,"Segoe UI",Helvetica,Arial,sans-serif;display:grid;grid-template-columns:1fr 1fr}
section{padding:24px}.dark{background:#0d1117;color:#e6edf3}.light{background:#fff;color:#1f2328}
.frame{max-width:880px;margin:auto;border:1px solid;border-radius:6px;padding:16px 24px;border-color:#30363d}.light .frame{border-color:#d0d7de}
img{width:100%;display:block;margin:0 0 16px}table{border-collapse:collapse;width:100%;margin:0 0 16px}
td,th{border:1px solid #30363d;padding:6px 13px;text-align:left}.light td,.light th{border-color:#d0d7de}
a{color:#4493f8;font-weight:600;text-decoration:none}.light a{color:#0969da}h3{margin:24px 0 16px}sub{color:#8b949e}
@media (max-width:1200px){body{grid-template-columns:1fr}}
</style>`)
	for _, t := range themes {
		fmt.Fprintf(&b, `<section class="%s"><div class="frame">`, t.Name)
		img := func(name, alt string) {
			fmt.Fprintf(&b, `<img alt="%s" src="%s-%s.svg">`, esc(alt), name, t.Name)
		}
		img("header", "kodflow")
		img("card", "At a glance")
		if withProof {
			img("proof", "By the numbers")
		}
		if len(p.Projects) > 0 {
			b.WriteString(`<h3>Selected work</h3><table><tr><th>Project</th><th>What it does</th></tr>`)
			for _, pr := range p.Projects {
				fmt.Fprintf(&b, `<tr><td><a href="https://github.com/%s/%s">%s</a></td><td>%s</td></tr>`, p.Login, pr.Repo, esc(pr.Repo), esc(pr.What))
			}
			b.WriteString(`</table>`)
		}
		img("timeline", "Stack over time")
		b.WriteString(`<sub>Rendered every day by a small Go program in this repository, from the public GitHub API only. No third-party stat service, no tracking pixel.</sub></div></section>`)
	}
	return b.String()
}
