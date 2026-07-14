package sweep

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/spf13/pflag"
)

// profileFile mirrors the keys a TOML profile table may contain. Pointer
// fields let us distinguish "key absent from the file" from "key present
// with the zero value", so precedence against CLI flags can be resolved
// correctly.
type profileFile struct {
	DryRun           *bool   `toml:"dry-run"`
	Verbose          *bool   `toml:"verbose"`
	Interactive      *bool   `toml:"interactive"`
	Confirm          *bool   `toml:"confirm"`
	Exclude          *string `toml:"exclude"`
	Age              *int    `toml:"age"`
	KeepEmacsBackups *int    `toml:"keep-emacs-backups"`
	Directory        *string `toml:"directory"`
}

// configFile is the top-level shape of the config file: a set of named
// profiles, each a TOML table. Any top-level key that is not a table fails
// to decode into profileFile and is reported as an error.
type configFile map[string]profileFile

// defaultConfigPath returns the default sweep config file location.
func defaultConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("determine default config path: %w", err)
	}
	return filepath.Join(dir, "sweep", "config.toml"), nil
}

// loadConfigFile parses the TOML file at path and rejects unknown keys
// anywhere in the document, in any profile or at the top level.
func loadConfigFile(path string) (configFile, error) {
	var doc configFile
	meta, err := toml.DecodeFile(path, &doc)
	if err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", path, err)
	}

	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, key := range undecoded {
			keys[i] = key.String()
		}
		return nil, fmt.Errorf("config file %s: unknown key(s): %s", path, strings.Join(keys, ", "))
	}

	return doc, nil
}

// expandHomeDir expands a leading "~" or "~/" in path using the user's home
// directory. Values that do not start with "~" are returned unchanged.
func expandHomeDir(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("expand ~ in config directory: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[len("~/"):]), nil
}

// mergeProfile applies profile values onto config, but only for flags the
// user did not explicitly pass on the command line. Directory is a special
// case: it is not bound to a flag, so it is always applied here; a
// positional directory argument overrides it later in ParseConfig.
func mergeProfile(config *Config, flags *pflag.FlagSet, profile profileFile) error {
	if profile.DryRun != nil && !flags.Changed("dry-run") {
		config.DryRun = *profile.DryRun
	}
	if profile.Verbose != nil && !flags.Changed("verbose") {
		config.Verbose = *profile.Verbose
	}
	if profile.Interactive != nil && !flags.Changed("interactive") {
		config.Interactive = *profile.Interactive
	}
	if profile.Confirm != nil && !flags.Changed("confirm") {
		config.Confirm = *profile.Confirm
	}
	if profile.Exclude != nil && !flags.Changed("exclude") {
		config.ExcludePattern = *profile.Exclude
	}
	if profile.Age != nil && !flags.Changed("age") {
		config.MinAgeDays = *profile.Age
	}
	if profile.KeepEmacsBackups != nil && !flags.Changed("keep-emacs-backups") {
		config.KeepEmacsBackups = *profile.KeepEmacsBackups
	}
	if profile.Directory != nil {
		dir, err := expandHomeDir(*profile.Directory)
		if err != nil {
			return err
		}
		config.Directory = dir
	}

	return nil
}

// validateConfigFlags checks --profile/--config/--no-config for usage errors
// that depend only on the parsed flags, never on filesystem access:
//   - --no-config combined with --profile or --config is a contradiction.
//   - --profile given with an empty name is invalid.
//
// This runs unconditionally in ParseConfig, including when --version is
// set, since these are CLI usage errors rather than config file I/O.
func validateConfigFlags(config *Config, flags *pflag.FlagSet) error {
	profileGiven := flags.Changed("profile")
	configGiven := flags.Changed("config")

	if config.noConfig {
		if profileGiven || configGiven {
			return fmt.Errorf("--no-config cannot be combined with --profile or --config")
		}
		return nil
	}

	if profileGiven && config.profileName == "" {
		return fmt.Errorf("--profile must not be empty")
	}

	return nil
}

// loadAndMergeConfig resolves the config file path and profile selection
// implied by --profile/--config/--no-config, then merges the selected
// profile's values onto config. It assumes validateConfigFlags has already
// been called and returned nil.
//
// Selection rules:
//   - --no-config: config files are ignored entirely.
//   - --profile not given: the profile name is "default". A missing config
//     file, or a config file without a [default] table, is not an error.
//   - --profile given: the config file must exist and must contain that
//     profile, or it is an error.
//   - --config given: that path must exist and parse, or it is an error,
//     regardless of whether --profile was also given.
func loadAndMergeConfig(config *Config, flags *pflag.FlagSet) error {
	if config.noConfig {
		return nil
	}

	profileGiven := flags.Changed("profile")
	configGiven := flags.Changed("config")

	profileName := "default"
	if profileGiven {
		profileName = config.profileName
	}

	path := config.configPath
	if !configGiven {
		defaultPath, err := defaultConfigPath()
		if err != nil {
			return err
		}
		path = defaultPath
	}

	mustExist := profileGiven || configGiven

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			if mustExist {
				return fmt.Errorf("config file %s does not exist", path)
			}
			return nil
		}
		return fmt.Errorf("cannot access config file %s: %w", path, err)
	}

	doc, err := loadConfigFile(path)
	if err != nil {
		return err
	}

	profile, ok := doc[profileName]
	if !ok {
		if profileGiven {
			return fmt.Errorf("config file %s: profile %q not found", path, profileName)
		}
		return nil
	}

	return mergeProfile(config, flags, profile)
}
