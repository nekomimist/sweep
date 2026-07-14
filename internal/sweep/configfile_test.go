package sweep

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfigFile writes content to path, creating parent directories as
// needed, and fails the test on error.
func writeConfigFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

// useXDGConfigHome points the default config lookup at a fresh temp dir and
// returns it, so tests never touch the real user config directory.
func useXDGConfigHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

func TestLoadAndMergeConfigProfileAppliedWhenFlagsNotPassed(t *testing.T) {
	xdg := useXDGConfigHome(t)
	writeConfigFile(t, filepath.Join(xdg, "sweep", "config.toml"), `
[default]
dry-run = true
verbose = true
interactive = true
confirm = true
exclude = "\\.git"
age = 7
keep-emacs-backups = 2
directory = "/tmp/from-profile"
`)

	var errOut bytes.Buffer
	config, err := ParseConfig(nil, &errOut)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}

	if !config.DryRun || !config.Verbose || !config.Interactive || !config.Confirm {
		t.Fatalf("boolean flags not applied from profile: %+v", config)
	}
	if config.ExcludePattern != `\.git` {
		t.Fatalf("ExcludePattern = %q, want %q", config.ExcludePattern, `\.git`)
	}
	if config.MinAgeDays != 7 {
		t.Fatalf("MinAgeDays = %d, want 7", config.MinAgeDays)
	}
	if config.KeepEmacsBackups != 2 {
		t.Fatalf("KeepEmacsBackups = %d, want 2", config.KeepEmacsBackups)
	}
	if config.Directory != "/tmp/from-profile" {
		t.Fatalf("Directory = %q, want /tmp/from-profile", config.Directory)
	}
}

func TestLoadAndMergeConfigCLIOverridesProfile(t *testing.T) {
	xdg := useXDGConfigHome(t)
	writeConfigFile(t, filepath.Join(xdg, "sweep", "config.toml"), `
[default]
dry-run = true
confirm = true
age = 7
directory = "/tmp/from-profile"
`)

	var errOut bytes.Buffer
	config, err := ParseConfig([]string{
		"--age", "3",
		"--confirm=false",
		"/tmp/from-cli",
	}, &errOut)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}

	if !config.DryRun {
		t.Fatal("DryRun = false, want true (from profile, not overridden)")
	}
	if config.Confirm {
		t.Fatal("Confirm = true, want false (CLI --confirm=false must override profile confirm=true)")
	}
	if config.MinAgeDays != 3 {
		t.Fatalf("MinAgeDays = %d, want 3 (CLI override)", config.MinAgeDays)
	}
	if config.Directory != "/tmp/from-cli" {
		t.Fatalf("Directory = %q, want /tmp/from-cli (positional arg overrides profile)", config.Directory)
	}
}

func TestLoadAndMergeConfigDefaultProfileMissingFileIsOK(t *testing.T) {
	useXDGConfigHome(t) // empty dir: config.toml does not exist

	var errOut bytes.Buffer
	config, err := ParseConfig(nil, &errOut)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v, want nil", err)
	}
	if config.Directory != defaultDirectory {
		t.Fatalf("Directory = %q, want default %q", config.Directory, defaultDirectory)
	}
}

func TestLoadAndMergeConfigDefaultProfileMissingTableIsOK(t *testing.T) {
	xdg := useXDGConfigHome(t)
	writeConfigFile(t, filepath.Join(xdg, "sweep", "config.toml"), `
[work]
verbose = true
`)

	var errOut bytes.Buffer
	config, err := ParseConfig(nil, &errOut)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v, want nil", err)
	}
	if config.Verbose {
		t.Fatal("Verbose = true, want false: [default] table absent, [work] must not apply")
	}
}

func TestLoadAndMergeConfigExplicitProfileMissingFileErrors(t *testing.T) {
	useXDGConfigHome(t) // config.toml does not exist

	var errOut bytes.Buffer
	_, err := ParseConfig([]string{"--profile", "work"}, &errOut)
	if err == nil {
		t.Fatal("ParseConfig() error = nil, want error for missing config file with explicit --profile")
	}
}

func TestLoadAndMergeConfigExplicitProfileMissingProfileErrors(t *testing.T) {
	xdg := useXDGConfigHome(t)
	writeConfigFile(t, filepath.Join(xdg, "sweep", "config.toml"), `
[default]
verbose = true
`)

	var errOut bytes.Buffer
	_, err := ParseConfig([]string{"--profile", "work"}, &errOut)
	if err == nil || !strings.Contains(err.Error(), "work") {
		t.Fatalf("ParseConfig() error = %v, want error naming missing profile %q", err, "work")
	}
}

func TestLoadAndMergeConfigExplicitDefaultProfileMustExist(t *testing.T) {
	// Unlike the implicit default profile, "-p default" is an explicit
	// selection and must fail the same way any other explicit profile does.
	useXDGConfigHome(t) // config.toml does not exist

	var errOut bytes.Buffer
	_, err := ParseConfig([]string{"--profile", "default"}, &errOut)
	if err == nil {
		t.Fatal("ParseConfig() error = nil, want error: -p default must require the file to exist")
	}
}

func TestLoadAndMergeConfigEmptyProfileNameErrors(t *testing.T) {
	useXDGConfigHome(t)

	var errOut bytes.Buffer
	_, err := ParseConfig([]string{"--profile", ""}, &errOut)
	if err == nil {
		t.Fatal("ParseConfig() error = nil, want error for empty --profile")
	}
}

func TestNoConfigIgnoresExistingFile(t *testing.T) {
	xdg := useXDGConfigHome(t)
	writeConfigFile(t, filepath.Join(xdg, "sweep", "config.toml"), `
[default]
verbose = true
age = 9
`)

	var errOut bytes.Buffer
	config, err := ParseConfig([]string{"--no-config"}, &errOut)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if config.Verbose || config.MinAgeDays != 0 {
		t.Fatalf("config = %+v, want built-in defaults (config file must be ignored)", config)
	}
}

func TestNoConfigCombinedWithProfileOrConfigErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"with --profile", []string{"--no-config", "--profile", "work"}},
		{"with --config", []string{"--no-config", "--config", "/tmp/does-not-matter.toml"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errOut bytes.Buffer
			_, err := ParseConfig(tt.args, &errOut)
			if err == nil {
				t.Fatalf("ParseConfig(%v) error = nil, want contradiction error", tt.args)
			}
		})
	}
}

func TestUnknownKeyErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		args    []string
	}{
		{
			name: "unknown key in selected profile",
			content: `
[default]
verbose = true
bogus = true
`,
			args: nil,
		},
		{
			name: "unknown key in unselected profile",
			content: `
[default]
verbose = true

[work]
bogus = true
`,
			args: nil,
		},
		{
			name: "top-level key outside any table",
			content: `
version = "1.0"

[default]
verbose = true
`,
			args: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.toml")
			writeConfigFile(t, path, tt.content)

			var errOut bytes.Buffer
			_, err := ParseConfig(append([]string{"--config", path}, tt.args...), &errOut)
			if err == nil {
				t.Fatalf("ParseConfig() error = nil, want error for content:\n%s", tt.content)
			}
		})
	}
}

func TestInvalidTOMLSyntaxErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	writeConfigFile(t, path, "[default\nverbose = true\n")

	var errOut bytes.Buffer
	_, err := ParseConfig([]string{"--config", path}, &errOut)
	if err == nil {
		t.Fatal("ParseConfig() error = nil, want TOML syntax error")
	}
}

func TestExplicitConfigMissingFileErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.toml")

	var errOut bytes.Buffer
	_, err := ParseConfig([]string{"--config", path}, &errOut)
	if err == nil {
		t.Fatal("ParseConfig() error = nil, want error for missing --config path")
	}
}

func TestExplicitConfigMissingDefaultTableIsOK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	writeConfigFile(t, path, `
[work]
verbose = true
`)

	var errOut bytes.Buffer
	config, err := ParseConfig([]string{"--config", path}, &errOut)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v, want nil: missing [default] without -p is OK", err)
	}
	if config.Verbose {
		t.Fatal("Verbose = true, want false: [work] must not apply without -p")
	}
}

func TestConfigDirectoryTildeExpansion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	writeConfigFile(t, path, `
[default]
directory = "~/work"
`)

	var errOut bytes.Buffer
	config, err := ParseConfig([]string{"--config", path}, &errOut)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	want := filepath.Join(home, "work")
	if config.Directory != want {
		t.Fatalf("Directory = %q, want %q", config.Directory, want)
	}
}

func TestConfigDirectoryBareTildeExpansion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	writeConfigFile(t, path, `
[default]
directory = "~"
`)

	var errOut bytes.Buffer
	config, err := ParseConfig([]string{"--config", path}, &errOut)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if config.Directory != home {
		t.Fatalf("Directory = %q, want %q", config.Directory, home)
	}
}

func TestValidationFiresOnConfigSourcedAge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	writeConfigFile(t, path, `
[default]
age = -1
`)

	var errOut bytes.Buffer
	_, err := ParseConfig([]string{"--config", path}, &errOut)
	if err == nil || !strings.Contains(err.Error(), "--age") {
		t.Fatalf("ParseConfig() error = %v, want age validation error", err)
	}
}

func TestValidationFiresOnConfigSourcedKeepEmacsBackups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	writeConfigFile(t, path, `
[default]
keep-emacs-backups = -1
`)

	var errOut bytes.Buffer
	_, err := ParseConfig([]string{"--config", path}, &errOut)
	if err == nil || !strings.Contains(err.Error(), "--keep-emacs-backups") {
		t.Fatalf("ParseConfig() error = %v, want keep-emacs-backups validation error", err)
	}
}

func TestNamedProfileIndependentFromDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	writeConfigFile(t, path, `
[default]
verbose = true
age = 5

[work]
confirm = true
`)

	var errOut bytes.Buffer
	config, err := ParseConfig([]string{"--config", path, "--profile", "work"}, &errOut)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if config.Verbose || config.MinAgeDays != 0 {
		t.Fatalf("config = %+v, want [default] values not inherited by [work]", config)
	}
	if !config.Confirm {
		t.Fatal("Confirm = false, want true from [work] profile")
	}
}

func TestVersionSkipsConfigFileEvenIfBroken(t *testing.T) {
	xdg := useXDGConfigHome(t)
	writeConfigFile(t, filepath.Join(xdg, "sweep", "config.toml"), "[default\nthis is not valid TOML")

	var errOut bytes.Buffer
	config, err := ParseConfig([]string{"-v"}, &errOut)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v, want nil: --version must ignore a broken config file", err)
	}
	if !config.ShowVersion {
		t.Fatal("ShowVersion = false, want true")
	}
}

func TestVersionStillValidatesConfigFlagUsage(t *testing.T) {
	useXDGConfigHome(t) // no config.toml present; irrelevant to these cases

	tests := []struct {
		name string
		args []string
	}{
		{"no-config with profile", []string{"-v", "--no-config", "-p", "work"}},
		{"empty profile name", []string{"-v", "-p", ""}},
		{"no-config with config path", []string{"-v", "--no-config", "--config", "x.toml"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errOut bytes.Buffer
			_, err := ParseConfig(tt.args, &errOut)
			if err == nil {
				t.Fatalf("ParseConfig(%v) error = nil, want usage error even with --version", tt.args)
			}
		})
	}
}
