package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
)

// Stats are the live numbers shown on the card. They come from the public
// GitHub API only, so they say nothing a visitor could not already see.
type Stats struct {
	Repos         int
	Stars         int
	Contributions int // last 12 months; -1 when no token allows the GraphQL call
	Languages     []LangShare
	RepoNames     []string // own repositories, forks excluded
}

// LangShare is one language's share of the bytes in the owner's own repos.
type LangShare struct {
	Name    string
	Percent float64
}

const apiBase = "https://api.github.com"

// github is a minimal client: three endpoints do not justify a dependency.
type github struct {
	http  *http.Client
	token string
}

func newGitHub(token string) *github {
	return &github{http: &http.Client{Timeout: 20 * time.Second}, token: token}
}

func (g *github) do(ctx context.Context, method, url string, body any, out any) error {
	var rd *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rd)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	resp, err := g.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s %s: %s", method, url, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}
	return nil
}

type repo struct {
	Name  string `json:"name"`
	Fork  bool   `json:"fork"`
	Stars int    `json:"stargazers_count"`
}

// fetchStats gathers repo, star, language and contribution counts for login.
// Forks are excluded everywhere: their code and stars belong to someone else.
func (g *github) fetchStats(ctx context.Context, login string) (*Stats, error) {
	var repos []repo
	url := fmt.Sprintf("%s/users/%s/repos?per_page=100&type=owner", apiBase, login)
	if err := g.do(ctx, http.MethodGet, url, nil, &repos); err != nil {
		return nil, err
	}
	s := &Stats{Contributions: -1}
	bytesBy := map[string]int{}
	for _, r := range repos {
		if r.Fork {
			continue
		}
		s.Repos++
		s.Stars += r.Stars
		s.RepoNames = append(s.RepoNames, r.Name)
		var langs map[string]int
		if err := g.do(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/languages", apiBase, login, r.Name), nil, &langs); err != nil {
			return nil, err
		}
		for l, n := range langs {
			bytesBy[l] += n
		}
	}
	s.Languages = topLanguages(bytesBy, 5)
	if g.token != "" {
		n, err := g.contributions(ctx, login)
		if err != nil {
			return nil, err
		}
		s.Contributions = n
	}
	return s, nil
}

// contributions returns the public contribution total of the last year. The
// GraphQL API refuses anonymous calls, hence the token requirement.
func (g *github) contributions(ctx context.Context, login string) (int, error) {
	q := map[string]any{
		"query":     `query($l:String!){user(login:$l){contributionsCollection{contributionCalendar{totalContributions}}}}`,
		"variables": map[string]string{"l": login},
	}
	var out struct {
		Data struct {
			User struct {
				ContributionsCollection struct {
					ContributionCalendar struct {
						TotalContributions int `json:"totalContributions"`
					} `json:"contributionCalendar"`
				} `json:"contributionsCollection"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := g.do(ctx, http.MethodPost, apiBase+"/graphql", q, &out); err != nil {
		return 0, err
	}
	return out.Data.User.ContributionsCollection.ContributionCalendar.TotalContributions, nil
}

// topLanguages keeps the n largest languages by bytes and folds the rest into
// "Other" so the bar always sums to 100 %.
func topLanguages(bytesBy map[string]int, n int) []LangShare {
	total := 0
	type kv struct {
		k string
		v int
	}
	var all []kv
	for k, v := range bytesBy {
		total += v
		all = append(all, kv{k, v})
	}
	if total == 0 {
		return nil
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].v != all[j].v {
			return all[i].v > all[j].v
		}
		return all[i].k < all[j].k
	})
	var out []LangShare
	rest := 0
	for i, e := range all {
		if i < n {
			out = append(out, LangShare{e.k, 100 * float64(e.v) / float64(total)})
		} else {
			rest += e.v
		}
	}
	if rest > 0 {
		out = append(out, LangShare{"Other", 100 * float64(rest) / float64(total)})
	}
	return out
}
