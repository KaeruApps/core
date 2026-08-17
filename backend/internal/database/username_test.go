package database

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSuffixedUsername(t *testing.T) {
	if got := suffixedUsername("frog", 0); got != "frog" {
		t.Fatalf("suffixedUsername(frog, 0) = %q", got)
	}
	if got := suffixedUsername("frog", 3); got != "frog3" {
		t.Fatalf("suffixedUsername(frog, 3) = %q", got)
	}

	longName := strings.Repeat("🐸", maxUsernameLength)
	got := suffixedUsername(longName, 12)
	if utf8.RuneCountInString(got) != maxUsernameLength || !strings.HasSuffix(got, "12") {
		t.Fatalf("suffixed long username has %d characters and suffix %q", utf8.RuneCountInString(got), got[len(got)-2:])
	}
}
