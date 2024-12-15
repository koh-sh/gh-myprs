package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-github/v67/github"
	"github.com/mattn/go-runewidth"
	"github.com/stretchr/testify/assert"
)

type MockGitHubClient struct {
	response interface{}
	err      error
}

func (m *MockGitHubClient) Get(ctx context.Context, path string, response interface{}) error {
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if m.err != nil {
		return m.err
	}

	switch v := response.(type) {
	case *github.User:
		if r, ok := m.response.(*github.User); ok && r != nil {
			*v = *r
		} else {
			*v = github.User{} // Return empty struct on error
		}
	case *github.IssuesSearchResult:
		if r, ok := m.response.(*github.IssuesSearchResult); ok && r != nil {
			*v = *r
		} else {
			*v = github.IssuesSearchResult{Issues: []*github.Issue{}} // Return empty result on error
		}
	}
	return nil
}

func createTestPR(title, url string) *github.Issue {
	return &github.Issue{
		Title:     github.String(title),
		HTMLURL:   github.String(url),
		UpdatedAt: &github.Timestamp{Time: time.Now()},
	}
}

func createTestPRList(prs ...*github.Issue) *github.IssuesSearchResult {
	return &github.IssuesSearchResult{Issues: prs}
}

func TestBuildSearchQuery(t *testing.T) {
	tests := []struct {
		name     string
		category string
		username string
		want     string
		wantErr  bool
	}{
		{
			name:     "created PRs query",
			category: categoryCreated,
			username: "testuser",
			want:     "is:open+is:pr+archived:false+author:testuser",
		},
		{
			name:     "review requests query",
			category: categoryReviewer,
			username: "testuser",
			want:     "is:open+is:pr+archived:false+user-review-requested:testuser",
		},
		{
			name:     "invalid category",
			category: "invalid",
			username: "testuser",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &PRChecker{username: tt.username}
			query, err := pc.buildSearchQuery(tt.category)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, query)
		})
	}
}

func TestFetchGitHubUsername(t *testing.T) {
	tests := []struct {
		name     string
		response *github.User
		err      error
		want     string
		wantErr  bool
	}{
		{
			name:     "successful response",
			response: &github.User{Login: github.String("testuser")},
			want:     "testuser",
		},
		{
			name:     "empty response",
			response: &github.User{},
			wantErr:  true,
		},
		{
			name:    "api error",
			err:     fmt.Errorf("api error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &MockGitHubClient{
				response: tt.response,
				err:      tt.err,
			}

			got, err := fetchGitHubUsername(client)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFetchPullRequests(t *testing.T) {
	successResp := createTestPRList(
		createTestPR("PR 1", "url1"),
		createTestPR("PR 2", "url2"),
	)

	tests := []struct {
		name      string
		category  string
		response  *github.IssuesSearchResult
		err       error
		wantErr   bool
		wantCount int
	}{
		{
			name:      "successful fetch",
			category:  categoryCreated,
			response:  successResp,
			wantCount: 2,
		},
		{
			name:     "api error",
			category: categoryCreated,
			err:      fmt.Errorf("api error"),
			wantErr:  true,
		},
		{
			name:     "invalid category",
			category: "invalid",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &MockGitHubClient{
				response: tt.response,
				err:      tt.err,
			}
			pc := &PRChecker{
				client:   client,
				username: "testuser",
			}

			ctx := context.Background()
			result, err := pc.fetchPullRequests(ctx, tt.category)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.wantCount, len(result.Issues))
		})
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name    string
		client  *MockGitHubClient
		wantErr bool
	}{
		{
			name: "successful run",
			client: &MockGitHubClient{
				response: createTestPRList(createTestPR("Test PR", "url")),
			},
		},
		{
			name: "api error",
			client: &MockGitHubClient{
				err: fmt.Errorf("api error"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &PRChecker{
				client:    tt.client,
				username:  "testuser",
				formatter: NewDisplayFormatter(),
			}

			err := pc.Run()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxLength int
		want      string
		wantWidth int
	}{
		{
			name:      "no truncation needed",
			input:     "short",
			maxLength: 10,
			want:      "short     ",
			wantWidth: 10,
		},
		{
			name:      "exact length",
			input:     "exactly10c",
			maxLength: 10,
			want:      "exactly10c",
			wantWidth: 10,
		},
		{
			name:      "requires truncation",
			input:     "this is a very long string",
			maxLength: 10,
			want:      "this is...",
			wantWidth: 10,
		},
		{
			name:      "unicode string truncation",
			input:     "こんにちは世界",
			maxLength: 10,
			want:      "こんに... ",
			wantWidth: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLength)
			width := runewidth.StringWidth(result)
			assert.Equal(t, tt.wantWidth, width)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestDisplayPullRequests(t *testing.T) {
	tests := []struct {
		name     string
		prs      []*github.Issue
		category string
		wantErr  bool
	}{
		{
			name:     "empty PR list",
			prs:      []*github.Issue{},
			category: categoryCreated,
		},
		{
			name: "invalid category",
			prs: []*github.Issue{
				createTestPR("Test PR", "url"),
			},
			category: "invalid",
			wantErr:  true,
		},
		{
			name: "missing PR title",
			prs: []*github.Issue{
				{
					HTMLURL:   github.String("url"),
					UpdatedAt: &github.Timestamp{Time: time.Now()},
				},
			},
			category: categoryCreated,
			wantErr:  true,
		},
		{
			name: "missing PR URL",
			prs: []*github.Issue{
				{
					Title:     github.String("Test PR"),
					UpdatedAt: &github.Timestamp{Time: time.Now()},
				},
			},
			category: categoryCreated,
			wantErr:  true,
		},
		{
			name: "valid PRs",
			prs: []*github.Issue{
				createTestPR("Test PR 1", "url1"),
				createTestPR("Test PR 2", "url2"),
			},
			category: categoryReviewer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &PRChecker{
				username:  "testuser",
				formatter: NewDisplayFormatter(),
			}

			err := pc.displayPullRequests(tt.prs, tt.category)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
