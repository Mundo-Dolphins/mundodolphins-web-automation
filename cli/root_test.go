package cli

import (
	"bytes"
	"testing"
)

func TestNewRegistersSubcommands(t *testing.T) {
	root := New(Options{})
	for _, name := range []string{"rss", "social", "social-autopr", "checker"} {
		if cmd, _, err := root.Find([]string{name}); err != nil || cmd.Name() != name {
			t.Errorf("subcommand %q not registered (err=%v)", name, err)
		}
	}
}

func TestToggleDebug(t *testing.T) {
	tests := []struct {
		name      string
		debugFlag bool
	}{
		{"debug enabled", true},
		{"debug disabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			debug = tt.debugFlag
			toggleDebug(nil, nil)
			// If no panic occurs, the test passes
		})
	}
}

func TestRootCmdFlags(t *testing.T) {
	// Verify the debug flag exists
	flag := New(Options{}).PersistentFlags().Lookup("debug")
	if flag == nil {
		t.Fatal("Expected debug flag to be defined")
	}

	if flag.Shorthand != "d" {
		t.Errorf("Expected debug flag shorthand to be 'd', got '%s'", flag.Shorthand)
	}
}

func TestRootCmdOutput(t *testing.T) {
	// Capture command output
	root := New(Options{})
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	// Verify the command has a description
	if root.Short == "" {
		t.Error("RootCmd should have a short description")
	}

	if root.Long == "" {
		t.Error("RootCmd should have a long description")
	}
}
