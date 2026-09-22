package main

import (
	"bufio"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func testProfile(t *testing.T) *Profile {
	t.Helper()
	p, err := loadProfile("../../profile.json")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func sampleStats() *Stats {
	return &Stats{Repos: 13, Stars: 19, Contributions: 1234, Languages: []LangShare{
		{"Go", 52.3}, {"Shell", 30.1}, {"TypeScript", 9.6}, {"Python", 4}, {"HCL", 2}, {"Other", 2},
	}}
}

// Every SVG must parse as XML: GitHub shows a broken image, not an error.
func TestSVGsAreWellFormed(t *testing.T) {
	p := testProfile(t)
	now := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	for _, th := range themes {
		for name, svg := range map[string]string{
			"header":       renderHeader(p, th),
			"card":         renderCard(p, sampleStats(), th, now),
			"card-offline": renderCard(p, nil, th, now),
			"timeline":     renderTimeline(p, th),
		} {
			d := xml.NewDecoder(strings.NewReader(svg))
			for {
				_, err := d.Token()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatalf("%s-%s: %v", name, th.Name, err)
				}
			}
		}
	}
}

func TestValidateRejectsWrongLevelCount(t *testing.T) {
	p := &Profile{Login: "x", Eras: []string{"a", "b"}, Timeline: []TimelineRow{{Name: "Go", Levels: []int{1}}}}
	if err := p.validate(); err == nil {
		t.Fatal("expected an error for a row shorter than the eras")
	}
	p.Timeline[0].Levels = []int{1, maxLevel + 1}
	if err := p.validate(); err == nil {
		t.Fatal("expected an error for a level above the maximum")
	}
}

func TestREADMEListsEveryProject(t *testing.T) {
	p := testProfile(t)
	r := renderREADME(p, "https://example.test")
	for _, pr := range p.Projects {
		if !strings.Contains(r, "/"+p.Login+"/"+pr.Repo+")") {
			t.Errorf("README misses %s", pr.Repo)
		}
	}
}

func TestUptimeRoundsDownToFiveYears(t *testing.T) {
	now := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	if got := uptime(2010, now); got != "15+ years in production" {
		t.Fatalf("got %q", got)
	}
}

func TestThousands(t *testing.T) {
	for in, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 1234567: "1,234,567"} {
		if got := thousands(in); got != want {
			t.Errorf("thousands(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestTopLanguagesFoldsTheRest(t *testing.T) {
	got := topLanguages(map[string]int{"Go": 60, "Shell": 30, "C": 6, "Perl": 4}, 2)
	if len(got) != 3 || got[0].Name != "Go" || got[2].Name != "Other" || got[2].Percent != 10 {
		t.Fatalf("got %+v", got)
	}
}

func TestWrap(t *testing.T) {
	got := wrap("aa bb cc dd", 5)
	if strings.Join(got, "|") != "aa bb|cc dd" {
		t.Fatalf("got %q", got)
	}
}

// TestNothingIdentifying fails when a denied term reaches the public output.
// The deny list lives outside the repository (PROFILE_DENYLIST): committing
// it would publish the very names it protects.
func TestNothingIdentifying(t *testing.T) {
	path := os.Getenv("PROFILE_DENYLIST")
	if path == "" {
		t.Skip("PROFILE_DENYLIST not set")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var terms []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" && !strings.HasPrefix(l, "#") {
			terms = append(terms, strings.ToLower(l))
		}
	}
	if len(terms) == 0 {
		t.Fatal("deny list is empty")
	}

	p := testProfile(t)
	raw, err := os.ReadFile("../../profile.json")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	public := strings.ToLower(strings.Join([]string{
		string(raw), renderREADME(p, ""), renderPreview(p),
		renderHeader(p, themes[0]), renderCard(p, sampleStats(), themes[0], now), renderTimeline(p, themes[0]),
	}, "\n"))
	for _, term := range terms {
		if strings.Contains(public, term) {
			t.Errorf("public output contains a denied term (line %q of the deny list)", term)
		}
	}
}
