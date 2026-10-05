package tui

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestHighlightPreservesText(t *testing.T) {
	name := "/app/Prod/db/prod-host"
	got := highlight(name, []string{"prod", "DB"})
	if plain := ansi.Strip(got); plain != name {
		t.Errorf("highlight changed text: %q", plain)
	}
}

func TestSanitize(t *testing.T) {
	got := sanitize("a\tb\r\nc\x1b[31md\x07")
	if want := "a b\nc[31md"; got != want {
		t.Errorf("sanitize() = %q, want %q", got, want)
	}
}
