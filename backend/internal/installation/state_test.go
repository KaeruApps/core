package installation

import "testing"

func TestParseState(t *testing.T) {
	for _, value := range []string{"required", "configuring", "restoring", "ready"} {
		state, err := ParseState(value)
		if err != nil || string(state) != value {
			t.Fatalf("ParseState(%q) = %q, %v", value, state, err)
		}
	}

	if _, err := ParseState("unknown"); err == nil {
		t.Fatal("ParseState() expected an error for an unknown state")
	}
}
