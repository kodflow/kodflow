package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Profile is the public projection of the owner's background. It is the only
// input the renderer reads: nothing here may name an employer, a client, a
// product under NDA or a date finer than an era, because the page is public
// and meant to stay anonymous.
type Profile struct {
	Login     string              `json:"login"`
	Tagline   string              `json:"tagline"`
	Role      string              `json:"role"`
	SinceYear int                 `json:"since_year"`
	Focus     []string            `json:"focus"`
	Stack     map[string][]string `json:"stack"`
	Eras      []string            `json:"eras"`
	Timeline  []TimelineRow       `json:"timeline"`
	Projects  []Project           `json:"projects"`
}

// TimelineRow is one technology line of the stack-over-time chart. Levels
// holds one intensity per era, from 0 (unused) to 3 (most of the time).
type TimelineRow struct {
	Family string `json:"family"`
	Name   string `json:"name"`
	Levels []int  `json:"levels"`
}

// Project is a public repository worth showing, with a one-line pitch kept
// here rather than read from GitHub so the wording stays consistent.
type Project struct {
	Repo string `json:"repo"`
	What string `json:"what"`
}

// maxLevel is the brightest intensity a timeline cell can take.
const maxLevel = 3

// loadProfile reads and validates the profile file at path.
func loadProfile(path string) (*Profile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read profile: %w", err)
	}
	var p Profile
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("parse profile: %w", err)
	}
	if err := p.validate(); err != nil {
		return nil, fmt.Errorf("invalid profile: %w", err)
	}
	return &p, nil
}

// validate rejects a profile the renderer would draw wrongly rather than
// letting a malformed row shift every cell after it.
func (p *Profile) validate() error {
	if p.Login == "" {
		return errors.New("login is empty")
	}
	if len(p.Eras) == 0 {
		return errors.New("eras is empty")
	}
	for _, r := range p.Timeline {
		if len(r.Levels) != len(p.Eras) {
			return fmt.Errorf("timeline %q has %d levels for %d eras", r.Name, len(r.Levels), len(p.Eras))
		}
		for _, l := range r.Levels {
			if l < 0 || l > maxLevel {
				return fmt.Errorf("timeline %q level %d outside 0..%d", r.Name, l, maxLevel)
			}
		}
	}
	return nil
}

// families returns the timeline families in first-seen order, so the chart
// follows the order the profile author chose.
func (p *Profile) families() []string {
	var out []string
	seen := map[string]bool{}
	for _, r := range p.Timeline {
		if !seen[r.Family] {
			seen[r.Family] = true
			out = append(out, r.Family)
		}
	}
	return out
}
