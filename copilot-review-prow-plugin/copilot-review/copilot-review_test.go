package copilotreview

import (
	"testing"
)

func TestCopilotReviewRegex(t *testing.T) {
	tests := []struct {
		input string
		match bool
	}{
		{"/copilot-review", true},
		{"/copilot-review\n", true},
		{"/copilot-review  ", true},
		{"  /copilot-review", false},
		{"/copilot-review-foo", false},
		{"some text /copilot-review", false},
		{"LGTM\n/copilot-review\nthanks", true},
		{"/Copilot-Review", true},
		{"/copilot-review\n/approve", true},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := CopilotReviewRe.MatchString(tt.input)
			if got != tt.match {
				t.Errorf("CopilotReviewRe.MatchString(%q) = %v, want %v", tt.input, got, tt.match)
			}
		})
	}
}

func TestHelpProvider(t *testing.T) {
	help, err := HelpProvider(nil)
	if err != nil {
		t.Fatalf("HelpProvider returned error: %v", err)
	}
	if help == nil {
		t.Fatal("HelpProvider returned nil")
	}
	if help.Description == "" {
		t.Error("HelpProvider returned empty description")
	}
}

func TestGHToken(t *testing.T) {
	// COPILOT_REVIEW_TOKEN takes precedence
	t.Setenv("COPILOT_REVIEW_TOKEN", "copilot-tok")
	t.Setenv("GH_TOKEN", "gh-tok")
	t.Setenv("GITHUB_TOKEN", "github-tok")
	if got := GHToken(); got != "copilot-tok" {
		t.Errorf("expected copilot-tok, got %s", got)
	}

	// GH_TOKEN is next
	t.Setenv("COPILOT_REVIEW_TOKEN", "")
	if got := GHToken(); got != "gh-tok" {
		t.Errorf("expected gh-tok, got %s", got)
	}

	// GITHUB_TOKEN is fallback
	t.Setenv("GH_TOKEN", "")
	if got := GHToken(); got != "github-tok" {
		t.Errorf("expected github-tok, got %s", got)
	}

	// Empty when nothing set
	t.Setenv("GITHUB_TOKEN", "")
	if got := GHToken(); got != "" {
		t.Errorf("expected empty, got %s", got)
	}
}
