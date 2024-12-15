package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-github/v67/github"
	"github.com/mattn/go-runewidth"
	"github.com/stretchr/testify/assert"
)

// Mock implementations
type mockClient struct {
	response interface{}
	err      error
}

func (m *mockClient) Get(path string, response interface{}) error {
	if m.err != nil {
		return m.err
	}

	// Handle response based on the type
	switch resp := m.response.(type) {
	case *github.User:
		if v, ok := response.(*github.User); ok {
			*v = *resp
		}
	case *github.IssuesSearchResult:
		if v, ok := response.(*github.IssuesSearchResult); ok {
			*v = *resp
		}
	}
	return nil
}

// Helper functions for tests
func createTestIssue(title, url string) *github.Issue {
	return &github.Issue{
		Title:     github.String(title),
		HTMLURL:   github.String(url),
		UpdatedAt: &github.Timestamp{Time: time.Now()},
	}
}

func createTestSearchResult(issues ...*github.Issue) *github.IssuesSearchResult {
	return &github.IssuesSearchResult{Issues: issues}
}

func TestBuildSearchQuery(t *testing.T) {
	tests := []struct {
		name     string
		prType   string
		account  string
		expected string
		wantErr  bool
	}{
		{
			name:     "created PRs query",
			prType:   PRTypeCreated,
			account:  "testuser",
			expected: "is:open+is:pr+archived:false+author:testuser",
			wantErr:  false,
		},
		{
			name:     "requested reviews query",
			prType:   PRTypeRequested,
			account:  "testuser",
			expected: "is:open+is:pr+archived:false+user-review-requested:testuser",
			wantErr:  false,
		},
		{
			name:     "invalid PR type",
			prType:   "invalid",
			account:  "testuser",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &PRChecker{account: tt.account}
			query, err := pc.buildSearchQuery(tt.prType)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, query)
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		length         int
		expectedString string
		expectedWidth  int
	}{
		{
			name:           "short string no truncation",
			input:          "short",
			length:         10,
			expectedString: "short     ",
			expectedWidth:  10,
		},
		{
			name:           "exact length string",
			input:          "exactly10c",
			length:         10,
			expectedString: "exactly10c",
			expectedWidth:  10,
		},
		{
			name:           "long string with truncation",
			input:          "this is a very long string",
			length:         10,
			expectedString: "this is...",
			expectedWidth:  10,
		},
		{
			name:           "unicode string with truncation",
			input:          "こんにちは世界",
			length:         10,
			expectedString: "こんに... ",
			expectedWidth:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.length)
			width := runewidth.StringWidth(result)
			assert.Equal(t, tt.expectedWidth, width, "Display width does not match expected value")
			assert.Equal(t, tt.expectedString, result, "String content does not match expected value")
		})
	}
}

func TestDisplayResults(t *testing.T) {
	tests := []struct {
		name    string
		issues  []*github.Issue
		prType  string
		wantErr bool
	}{
		{
			name:    "empty issues list",
			issues:  nil,
			prType:  PRTypeCreated,
			wantErr: false,
		},
		{
			name: "valid issues list",
			issues: []*github.Issue{
				{
					Title:     github.String("Test PR"),
					HTMLURL:   github.String("https://github.com/test/repo/pull/1"),
					UpdatedAt: &github.Timestamp{Time: time.Now()},
				},
			},
			prType:  PRTypeCreated,
			wantErr: false,
		},
		{
			name: "invalid issue data - missing title",
			issues: []*github.Issue{
				{
					HTMLURL:   github.String("https://github.com/test/repo/pull/1"),
					UpdatedAt: &github.Timestamp{Time: time.Now()},
				},
			},
			prType:  PRTypeCreated,
			wantErr: true,
		},
		{
			name: "invalid issue data - missing url",
			issues: []*github.Issue{
				{
					Title:     github.String("Test PR"),
					UpdatedAt: &github.Timestamp{Time: time.Now()},
				},
			},
			prType:  PRTypeCreated,
			wantErr: true,
		},
		{
			name:    "invalid PR type",
			issues:  nil,
			prType:  "invalid",
			wantErr: true,
		},
		{
			name: "mix of valid and invalid issues",
			issues: []*github.Issue{
				{
					Title:     github.String("Valid PR"),
					HTMLURL:   github.String("https://github.com/test/repo/pull/1"),
					UpdatedAt: &github.Timestamp{Time: time.Now()},
				},
				{
					// Invalid issue with missing title and URL
					UpdatedAt: &github.Timestamp{Time: time.Now()},
				},
			},
			prType:  PRTypeCreated,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &PRChecker{
				account: "testuser",
				display: NewDisplayFormatter(),
			}

			err := pc.displayResults(tt.issues, tt.prType)

			if tt.wantErr {
				assert.Error(t, err, "Expected error for invalid data")
				if tt.prType != "invalid" && len(tt.issues) > 0 {
					assert.Contains(t, err.Error(), "received invalid issue data from GitHub")
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetAccountName(t *testing.T) {
	tests := []struct {
		name     string
		response *github.User
		err      error
		want     string
		wantErr  bool
	}{
		{
			name: "successful response",
			response: &github.User{
				Login: github.String("testuser"),
			},
			want:    "testuser",
			wantErr: false,
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
			client := &mockClient{
				response: tt.response,
				err:      tt.err,
			}

			got, err := getAccountName(client)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSearchIssues(t *testing.T) {
	successResp := &github.IssuesSearchResult{
		Issues: []*github.Issue{
			createTestIssue("PR 1", "url1"),
			createTestIssue("PR 2", "url2"),
		},
	}

	tests := []struct {
		name      string
		prType    string
		mockResp  *github.IssuesSearchResult
		mockErr   error
		wantErr   bool
		wantCount int
	}{
		{
			name:      "successful search",
			prType:    PRTypeCreated,
			mockResp:  successResp,
			wantCount: 2,
		},
		{
			name:    "api error",
			prType:  PRTypeCreated,
			mockErr: fmt.Errorf("api error"),
			wantErr: true,
		},
		{
			name:    "invalid PR type",
			prType:  "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &PRChecker{
				client:  &mockClient{response: tt.mockResp, err: tt.mockErr},
				account: "testuser",
			}

			result, err := pc.searchIssues(tt.prType)
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
	successResponse := createTestSearchResult(createTestIssue("Test PR", "url"))

	tests := []struct {
		name    string
		client  APIClient
		wantErr bool
	}{
		{
			name:    "successful run",
			client:  &mockClient{response: successResponse},
			wantErr: false,
		},
		{
			name:    "api error",
			client:  &mockClient{err: fmt.Errorf("api error")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &PRChecker{
				client:  tt.client,
				account: "testuser",
				display: NewDisplayFormatter(),
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
