package sweep

import (
	"fmt"
	"os"
	"testing"
)

// TestMain isolates the whole package's test run from the developer's real
// config file. Without this, a real ~/.config/sweep/config.toml containing a
// [default] profile would be picked up by any test that calls
// ParseConfig/Run without an explicit --config or --no-config, breaking
// hermeticity.
//
// t.Setenv is unavailable here since TestMain does not receive a *testing.T,
// so XDG_CONFIG_HOME is overridden with os.Setenv/os.Unsetenv directly and
// restored after m.Run(). Individual tests in configfile_test.go still call
// t.Setenv("XDG_CONFIG_HOME", ...) to point at their own fixtures; that
// continues to work since t.Setenv saves and restores whatever value is in
// place when the test starts (here, our temp dir).
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "sweep-test-config")
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: MkdirTemp: %v\n", err)
		os.Exit(1)
	}

	prevValue, hadPrev := os.LookupEnv("XDG_CONFIG_HOME")
	if err := os.Setenv("XDG_CONFIG_HOME", dir); err != nil {
		os.RemoveAll(dir)
		fmt.Fprintf(os.Stderr, "TestMain: Setenv: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	if hadPrev {
		os.Setenv("XDG_CONFIG_HOME", prevValue)
	} else {
		os.Unsetenv("XDG_CONFIG_HOME")
	}
	os.RemoveAll(dir)

	os.Exit(code)
}
