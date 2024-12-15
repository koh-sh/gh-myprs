package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/text"
	"github.com/fatih/color"
	"github.com/google/go-github/v67/github"
	"github.com/mattn/go-runewidth"
)

// Constants for GitHub API
const (
	githubAPIVersion   = "2022-11-28"
	githubAcceptHeader = "application/vnd.github+json"
)

// Constants for PR types
const (
	PRTypeCreated   = "created"
	PRTypeRequested = "requested"
)

// Constants for display formatting
const (
	titleWidth    = 33
	updatedWidth  = 17
	columnSpacing = 2
	lineWidth     = 80
)

// Display related constants
const (
	createdIcon   = "🔨"
	requestedIcon = "👀"
)

// Define APIClient Interface
type APIClient interface {
	Get(path string, response interface{}) error
}

// PRChecker handles GitHub PR-related operations
type PRChecker struct {
	client  APIClient
	account string
	display *DisplayFormatter
}

// DisplayFormatter handles the formatting and display of PR information
type DisplayFormatter struct {
	headerFmt *color.Color
	titleFmt  *color.Color
	urlFmt    *color.Color
	timeFmt   *color.Color
}

// NewDisplayFormatter creates a new DisplayFormatter with predefined styles
func NewDisplayFormatter() *DisplayFormatter {
	return &DisplayFormatter{
		headerFmt: color.New(color.FgGreen, color.Bold),
		titleFmt:  color.New(color.FgCyan),
		urlFmt:    color.New(color.FgBlue, color.Underline),
		timeFmt:   color.New(color.FgYellow),
	}
}

// NewPRChecker creates a new PRChecker instance
func NewPRChecker() (*PRChecker, error) {
	client, err := createAPIClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	account, err := getAccountName(client)
	if err != nil {
		return nil, fmt.Errorf("failed to get account name: %w", err)
	}

	return &PRChecker{
		client:  client,
		account: account,
		display: NewDisplayFormatter(),
	}, nil
}

// Run executes the main PR checking logic
func (pc *PRChecker) Run() error {
	prTypes := []string{PRTypeCreated, PRTypeRequested}

	for _, prType := range prTypes {
		if err := pc.processAndDisplayPRs(prType); err != nil {
			return fmt.Errorf("error processing %s PRs: %w", prType, err)
		}
	}
	return nil
}

func createAPIClient() (APIClient, error) {
	opts := api.ClientOptions{
		Headers: map[string]string{
			"Accept":               githubAcceptHeader,
			"X-GitHub-Api-Version": githubAPIVersion,
		},
	}
	return api.NewRESTClient(opts)
}

func getAccountName(client APIClient) (string, error) {
	var user github.User
	if err := client.Get("user", &user); err != nil {
		return "", fmt.Errorf("failed to get user info: %w", err)
	}

	if user.Login == nil {
		return "", fmt.Errorf("received empty login name from GitHub")
	}
	return *user.Login, nil
}

func (pc *PRChecker) processAndDisplayPRs(prType string) error {
	issues, err := pc.searchIssues(prType)
	if err != nil {
		return err
	}
	return pc.displayResults(issues.Issues, prType)
}

func (pc *PRChecker) searchIssues(prType string) (*github.IssuesSearchResult, error) {
	query, err := pc.buildSearchQuery(prType)
	if err != nil {
		return nil, err
	}

	var response github.IssuesSearchResult
	if err := pc.client.Get("search/issues?q="+query, &response); err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}

	return &response, nil
}

func (pc *PRChecker) buildSearchQuery(prType string) (string, error) {
	baseQuery := "is:open+is:pr+archived:false"

	switch prType {
	case PRTypeCreated:
		return fmt.Sprintf("%s+author:%s", baseQuery, pc.account), nil
	case PRTypeRequested:
		return fmt.Sprintf("%s+user-review-requested:%s", baseQuery, pc.account), nil
	default:
		return "", fmt.Errorf("unsupported PR type: %s", prType)
	}
}

func (pc *PRChecker) displayResults(issues []*github.Issue, prType string) error {
	if err := pc.printSectionHeader(prType); err != nil {
		return err
	}

	if len(issues) == 0 {
		color.Yellow("No pull requests found\n\n")
		return nil
	}

	pc.printTableHeader()

	if err := pc.printIssues(issues); err != nil {
		return err
	}

	fmt.Println()
	return nil
}

func (pc *PRChecker) printSectionHeader(prType string) error {
	headerStyle := color.New(color.FgHiMagenta, color.Bold)
	var icon, text string

	switch prType {
	case PRTypeCreated:
		icon, text = createdIcon, "Pull Requests Created by"
	case PRTypeRequested:
		icon, text = requestedIcon, "Review Requests for"
	default:
		return fmt.Errorf("unsupported PR type: %s", prType)
	}

	headerStyle.Printf("\n%s %s %s\n\n", icon, text, pc.account)
	return nil
}

func (pc *PRChecker) printTableHeader() {
	spacing := strings.Repeat(" ", columnSpacing)

	pc.display.headerFmt.Printf("Title%s", strings.Repeat(" ", titleWidth-len("Title")))
	pc.display.headerFmt.Printf("%sUpdated%s", spacing, strings.Repeat(" ", updatedWidth-len("Updated")))
	pc.display.headerFmt.Printf("%sURL\n", spacing)
	fmt.Println(color.HiBlackString(strings.Repeat("-", lineWidth)))
}

func (pc *PRChecker) printIssues(issues []*github.Issue) error {
	now := time.Now()
	spacing := strings.Repeat(" ", columnSpacing)

	for _, issue := range issues {
		if issue.Title == nil || issue.HTMLURL == nil {
			return fmt.Errorf("received invalid issue data from GitHub")
		}

		title := truncateString(*issue.Title, titleWidth)
		updated := truncateString(text.RelativeTimeAgo(now, issue.UpdatedAt.Time), updatedWidth)

		pc.display.titleFmt.Printf("%s", title)
		pc.display.timeFmt.Printf("%s%s", spacing, updated)
		pc.display.urlFmt.Printf("%s%s\n", spacing, *issue.HTMLURL)
	}
	return nil
}

func truncateString(s string, length int) string {
	width := runewidth.StringWidth(s)

	if width <= length {
		return s + strings.Repeat(" ", length-width)
	}

	width = 0
	var truncated []rune
	for _, r := range s {
		w := runewidth.RuneWidth(r)
		if width+w+3 > length {
			break
		}
		width += w
		truncated = append(truncated, r)
	}

	result := string(truncated) + "..."
	resultWidth := runewidth.StringWidth(result)
	return result + strings.Repeat(" ", length-resultWidth)
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
