// Command profile renders the kodflow GitHub profile: three SVGs per colour
// scheme and the README that shows them. Everything comes from profile.json
// and the public GitHub API, so the page can be rebuilt anywhere, any day.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "profile:", err)
		os.Exit(1)
	}
}

func run() error {
	profilePath := flag.String("profile", "profile.json", "public profile data")
	out := flag.String("out", "dist", "directory receiving the SVGs")
	readme := flag.String("readme", "", "also write the README to this path")
	base := flag.String("base", "", "URL or path the README uses for the SVGs (default: the repo's output branch)")
	offline := flag.Bool("offline", false, "skip the GitHub API; live numbers show a dash")
	preview := flag.Bool("preview", false, "also write <out>/preview.html, both themes side by side")
	proof := flag.Bool("proof", true, "clone the public repos and render the by-the-numbers card")
	flag.Parse()

	p, err := loadProfile(*profilePath)
	if err != nil {
		return err
	}

	var stats *Stats
	if !*offline {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			token = os.Getenv("GH_TOKEN")
		}
		// A failed API call must not blank the page: the previous SVGs stay
		// published, so fail the run and let the next schedule retry.
		if stats, err = newGitHub(token).fetchStats(ctx, p.Login); err != nil {
			return fmt.Errorf("github stats: %w", err)
		}
	}

	var pr *Proof
	if stats != nil && *proof {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if pr, err = measureRepos(ctx, p.Login, stats.RepoNames); err != nil {
			return fmt.Errorf("proof: %w", err)
		}
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", *out, err)
	}
	now := time.Now()
	for _, t := range themes {
		files := map[string]string{
			"header":   renderHeader(p, t),
			"card":     renderCard(p, stats, t, now),
			"timeline": renderTimeline(p, t),
		}
		if pr != nil {
			files["proof"] = renderProof(pr, t)
		}
		for name, svg := range files {
			if err := writeFile(filepath.Join(*out, name+"-"+t.Name+".svg"), svg); err != nil {
				return err
			}
		}
	}

	if *readme != "" {
		b := *base
		if b == "" {
			b = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/output", p.Login, p.Login)
		}
		if err := writeFile(*readme, renderREADME(p, b, *proof)); err != nil {
			return err
		}
	}
	if *preview {
		if err := writeFile(filepath.Join(*out, "preview.html"), renderPreview(p, pr != nil)); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
