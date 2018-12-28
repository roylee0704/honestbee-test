package external

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type IssuesSearchResult struct {
	TotalCount int `json:"total_count"`
	Items      []*Issue
}

type Issue struct {
	Number    int
	HTMLURL   string `json:"html_url"`
	Title     string
	User      *User
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	Login string
}

const issuesURL = "https://api.github.com/search/issues"

func SearchIssues(terms []string, timeout time.Duration) (*IssuesSearchResult, error) {
	q := url.QueryEscape(strings.Join(terms, " "))
	client := http.Client{
		Timeout: timeout,
	}
	resp, err := client.Get(issuesURL + "?q=" + q)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("search query failed: %s", resp.Status)
	}

	var result IssuesSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		resp.Body.Close()
		return nil, err
	}
	resp.Body.Close()

	return &result, nil
}
