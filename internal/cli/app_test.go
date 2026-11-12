package cli

import (
	"bytes"
	"flag"
	"slices"
	"strings"
	"testing"
)

/**
 * TestParseInterspersed checks that options may follow files, and that
 * "--" ends the options.
 */
func TestParseInterspersed(t *testing.T) {
	for _, tc := range []struct {
		args       []string
		wantOutput string
		wantFiles  []string
	}{
		{[]string{"a.json", "--output", "x", "b.json"}, "x", []string{"a.json", "b.json"}},
		{[]string{"--output", "x", "--", "--not-an-option"}, "x", []string{"--not-an-option"}},
		{[]string{"a.json", "--", "-b.json"}, "", []string{"a.json", "-b.json"}},
	} {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		output := fs.String("output", "", "")
		files, err := parseInterspersed(fs, tc.args)
		if err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		if *output != tc.wantOutput || !slices.Equal(files, tc.wantFiles) {
			t.Errorf("%v: got output %q, files %v", tc.args, *output, files)
		}
	}
}

/**
 * TestUnknownCommand checks that a typo is reported, not ignored.
 */
func TestUnknownCommand(t *testing.T) {
	var stderr bytes.Buffer
	if code := NewApp(&bytes.Buffer{}, &stderr, "test").Run([]string{"doxgen"}); code != ExitError {
		t.Errorf("exit code %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr.String(), `unknown command "doxgen"`) {
		t.Errorf("stderr: %s", stderr.String())
	}
}

/**
 * TestVersion checks the version command prints the App's version.
 */
func TestVersion(t *testing.T) {
	var stdout bytes.Buffer
	NewApp(&stdout, &bytes.Buffer{}, "v1.2.3").Run([]string{"version"})
	if got := strings.TrimSpace(stdout.String()); got != "doxythings v1.2.3" {
		t.Errorf("got %q", got)
	}
}
