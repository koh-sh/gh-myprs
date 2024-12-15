package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/text"
	"github.com/fatih/color"
	"github.com/google/go-github/v67/github"
	"github.com/mattn/go-runewidth"
)

// GitHub API configuration
const (
	githubAPIVersion   = "2022-11-28"
	githubAcceptHeader = "application/vnd.github+json"
)

// Pull request categories
const (
	categoryCreated  = "created"   // PRs created by the user
	categoryReviewer = "requested" // PRs where user is requested as reviewer
)

// Display configuration
const (
	maxTitleLength  = 33 // Maximum length for PR title display
	maxUpdateLength = 17 // Maximum length for "updated at" timestamp
	columnPadding   = 2  // Space between columns
	displayWidth    = 80 // Total width of display
)

// Status icons
const (
	iconCreated  = "🔨" // Icon for PRs created by user
	iconReviewer = "👀" // Icon for PRs requiring review
)

// AsyncPRResult represents the result of an asynchronous PR fetch operation
type AsyncPRResult struct {
	Issues   []*github.Issue
	Category string
	Error    error
}

// GitHubClient defines the interface for GitHub API operations
type GitHubClient interface {
	Get(ctx context.Context, path string, response interface{}) error
}

// githubRESTClient implements GitHubClient using REST API
type githubRESTClient struct {
	client *api.RESTClient
}

func (c *githubRESTClient) Get(ctx context.Context, path string, response interface{}) error {
	return c.client.Get(path, response)
}

// PRChecker manages GitHub pull request operations and display
type PRChecker struct {
	client    GitHubClient
	username  string
	formatter *DisplayFormatter
}

// DisplayFormatter handles the formatting of PR information
type DisplayFormatter struct {
	headerStyle *color.Color
	titleStyle  *color.Color
	urlStyle    *color.Color
	timeStyle   *color.Color
}

// NewDisplayFormatter creates a DisplayFormatter with predefined styles
func NewDisplayFormatter() *DisplayFormatter {
	return &DisplayFormatter{
		headerStyle: color.New(color.FgGreen, color.Bold),
		titleStyle:  color.New(color.FgCyan),
		urlStyle:    color.New(color.FgBlue, color.Underline),
		timeStyle:   color.New(color.FgYellow),
	}
}

// NewPRChecker initializes a new PRChecker instance
func NewPRChecker() (*PRChecker, error) {
	client, err := initializeGitHubClient()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GitHub client: %w", err)
	}

	username, err := fetchGitHubUsername(client)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch GitHub username: %w", err)
	}

	return &PRChecker{
		client:    client,
		username:  username,
		formatter: NewDisplayFormatter(),
	}, nil
}

// Run executes the main PR checking logic with concurrent requests
func (pc *PRChecker) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results := make(chan AsyncPRResult, 2)
	var wg sync.WaitGroup

	for _, category := range []string{categoryCreated, categoryReviewer} {
		wg.Add(1)
		go func(cat string) {
			defer wg.Done()
			issues, err := pc.fetchPullRequests(ctx, cat)
			var issuesList []*github.Issue
			if issues != nil {
				issuesList = issues.Issues
			}
			results <- AsyncPRResult{
				Issues:   issuesList,
				Category: cat,
				Error:    err,
			}
		}(category)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		if result.Error != nil {
			return fmt.Errorf("error fetching %s PRs: %w", result.Category, result.Error)
		}
		if err := pc.displayPullRequests(result.Issues, result.Category); err != nil {
			return err
		}
	}

	return nil
}

func initializeGitHubClient() (GitHubClient, error) {
	opts := api.ClientOptions{
		Headers: map[string]string{
			"Accept":               githubAcceptHeader,
			"X-GitHub-Api-Version": githubAPIVersion,
		},
	}

	client, err := api.NewRESTClient(opts)
	if err != nil {
		return nil, err
	}

	return &githubRESTClient{client: client}, nil
}

func fetchGitHubUsername(client GitHubClient) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var user github.User
	if err := client.Get(ctx, "user", &user); err != nil {
		return "", fmt.Errorf("failed to fetch user info: %w", err)
	}

	if user.Login == nil {
		return "", fmt.Errorf("received empty username from GitHub")
	}
	return *user.Login, nil
}

func (pc *PRChecker) fetchPullRequests(ctx context.Context, category string) (*github.IssuesSearchResult, error) {
	query, err := pc.buildSearchQuery(category)
	if err != nil {
		return nil, err
	}

	var response github.IssuesSearchResult
	if err := pc.client.Get(ctx, "search/issues?q="+query, &response); err != nil {
		return nil, fmt.Errorf("failed to fetch pull requests: %w", err)
	}

	return &response, nil
}

func (pc *PRChecker) buildSearchQuery(category string) (string, error) {
	baseQuery := "is:open+is:pr+archived:false"

	switch category {
	case categoryCreated:
		return fmt.Sprintf("%s+author:%s", baseQuery, pc.username), nil
	case categoryReviewer:
		return fmt.Sprintf("%s+user-review-requested:%s", baseQuery, pc.username), nil
	default:
		return "", fmt.Errorf("unsupported PR category: %s", category)
	}
}

func (pc *PRChecker) displayPullRequests(issues []*github.Issue, category string) error {
	if err := pc.displaySectionHeader(category); err != nil {
		return err
	}

	if len(issues) == 0 {
		color.Yellow("No pull requests found\n\n")
		return nil
	}

	pc.displayTableHeader()

	if err := pc.displayIssues(issues); err != nil {
		return err
	}

	fmt.Println()
	return nil
}

func (pc *PRChecker) displaySectionHeader(category string) error {
	headerStyle := color.New(color.FgHiMagenta, color.Bold)
	var icon, description string

	switch category {
	case categoryCreated:
		icon, description = iconCreated, "Pull Requests Created by"
	case categoryReviewer:
		icon, description = iconReviewer, "Review Requests for"
	default:
		return fmt.Errorf("unsupported PR category: %s", category)
	}

	headerStyle.Printf("\n%s %s %s\n\n", icon, description, pc.username)
	return nil
}

func (pc *PRChecker) displayTableHeader() {
	padding := strings.Repeat(" ", columnPadding)

	pc.formatter.headerStyle.Printf("Title%s", strings.Repeat(" ", maxTitleLength-len("Title")))
	pc.formatter.headerStyle.Printf("%sUpdated%s", padding, strings.Repeat(" ", maxUpdateLength-len("Updated")))
	pc.formatter.headerStyle.Printf("%sURL\n", padding)
	fmt.Println(color.HiBlackString(strings.Repeat("-", displayWidth)))
}

func (pc *PRChecker) displayIssues(issues []*github.Issue) error {
	currentTime := time.Now()
	padding := strings.Repeat(" ", columnPadding)

	for _, issue := range issues {
		if issue.Title == nil || issue.HTMLURL == nil {
			return fmt.Errorf("received invalid issue data from GitHub")
		}

		title := truncateString(*issue.Title, maxTitleLength)
		updated := truncateString(text.RelativeTimeAgo(currentTime, issue.UpdatedAt.Time), maxUpdateLength)

		pc.formatter.titleStyle.Printf("%s", title)
		pc.formatter.timeStyle.Printf("%s%s", padding, updated)
		pc.formatter.urlStyle.Printf("%s%s\n", padding, *issue.HTMLURL)
	}
	return nil
}

func truncateString(s string, maxLength int) string {
	width := runewidth.StringWidth(s)

	if width <= maxLength {
		return s + strings.Repeat(" ", maxLength-width)
	}

	width = 0
	var truncated []rune
	for _, r := range s {
		w := runewidth.RuneWidth(r)
		if width+w+3 > maxLength {
			break
		}
		width += w
		truncated = append(truncated, r)
	}

	result := string(truncated) + "..."
	resultWidth := runewidth.StringWidth(result)
	return result + strings.Repeat(" ", maxLength-resultWidth)
}

func main() {
	checker, err := NewPRChecker()
	if err != nil {
		log.Fatal(err)
	}

	if err := checker.Run(); err != nil {
		log.Fatal(err)
	}
}
