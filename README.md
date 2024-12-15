# gh myprs

A GitHub CLI extension to check your open pull requests and review requests in a nicely formatted view.

## Features

- 🔍 Lists all your open pull requests and review requests
- 🎨 Color-coded output for better readability
- 📏 Well-formatted columns with proper text truncation
- 🌐 Supports both English and Japanese text
- 🔐 Uses your existing GitHub CLI authentication

## Installation

```bash
gh extension install koh-sh/gh-myprs
```

## Usage

Simply run:

```bash
gh myprs
```

The command will display two sections:
1. Pull requests you've created
2. Pull requests where you're requested as a reviewer

Example output:
```
🔨 Pull Requests Created by koh-sh

Title                                  Updated         URL
--------------------------------------------------------------------------------
chore: update dependency versions      about 3 days... https://github.com/koh-sh/example-repo/pull/123
feat: add new feature                  about 1 week... https://github.com/koh-sh/example-repo/pull/456


👀 Review Requests for koh-sh

Title                                  Updated         URL
--------------------------------------------------------------------------------
docs: improve README                   about 2 days... https://github.com/org/repo/pull/789
fix: resolve bug in core module        about 4 days... https://github.com/org/repo/pull/101
```

## Requirements

- [GitHub CLI](https://cli.github.com/) installed and authenticated

## Authentication

This extension uses your existing GitHub CLI authentication. Make sure you're logged in with:

```bash
gh auth login
```

## License

MIT

## Author

koh-sh
