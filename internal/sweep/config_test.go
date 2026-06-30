package sweep

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestParseConfigDefaults(t *testing.T) {
	var errOut bytes.Buffer
	config, err := ParseConfig(nil, &errOut)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}

	if config.Directory != "." {
		t.Fatalf("Directory = %q, want %q", config.Directory, ".")
	}
	if config.ExcludePattern != defaultExcludePattern {
		t.Fatalf("ExcludePattern = %q, want %q", config.ExcludePattern, defaultExcludePattern)
	}
	if config.MinAge != 0 {
		t.Fatalf("MinAge = %s, want 0", config.MinAge)
	}
}

func TestParseConfigFlags(t *testing.T) {
	var errOut bytes.Buffer
	config, err := ParseConfig([]string{
		"--dry-run",
		"--verbose",
		"--interactive",
		"--confirm",
		"--exclude", `\.git`,
		"--age", "7",
		"/tmp/work",
	}, &errOut)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}

	if !config.DryRun || !config.Verbose || !config.Interactive || !config.Confirm {
		t.Fatalf("boolean flags were not parsed: %+v", config)
	}
	if config.Directory != "/tmp/work" {
		t.Fatalf("Directory = %q, want /tmp/work", config.Directory)
	}
	if config.MinAge != 7*24*time.Hour {
		t.Fatalf("MinAge = %s, want 168h", config.MinAge)
	}
	if !config.ExcludeRegexp.MatchString("/tmp/.git/config") {
		t.Fatal("ExcludeRegexp did not match expected path")
	}
}

func TestParseConfigRejectsNegativeAge(t *testing.T) {
	var errOut bytes.Buffer
	_, err := ParseConfig([]string{"--age", "-1"}, &errOut)
	if err == nil || !strings.Contains(err.Error(), "--age") {
		t.Fatalf("ParseConfig() error = %v, want age error", err)
	}
}

func TestParseConfigRejectsInvalidRegexp(t *testing.T) {
	var errOut bytes.Buffer
	_, err := ParseConfig([]string{"--exclude", "["}, &errOut)
	if err == nil || !strings.Contains(err.Error(), "illegal regexp") {
		t.Fatalf("ParseConfig() error = %v, want regexp error", err)
	}
}

func TestParseConfigRejectsTooManyArgs(t *testing.T) {
	var errOut bytes.Buffer
	_, err := ParseConfig([]string{"one", "two"}, &errOut)
	if err == nil || !strings.Contains(err.Error(), "at most one") {
		t.Fatalf("ParseConfig() error = %v, want argument count error", err)
	}
}

func TestParseConfigHelp(t *testing.T) {
	var errOut bytes.Buffer
	config, err := ParseConfig([]string{"--help"}, &errOut)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if !config.ShowHelp {
		t.Fatal("ShowHelp = false, want true")
	}
	if !strings.Contains(errOut.String(), "--dry-run") {
		t.Fatalf("help output = %q, want flag list", errOut.String())
	}
}
