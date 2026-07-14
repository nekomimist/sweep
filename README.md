# sweep

`sweep` is a small command-line tool that recursively removes common editor
backup files from a directory tree. It removes regular files whose names end in
`~` or `.bak`.

## Usage

```sh
sweep [flags] [directory]
```

If `directory` is omitted, `sweep` scans the current directory.

### Flags

```text
  -n, --dry-run          print filename but not delete
  -v, --version          show version
  -x, --exclude string   exclude path regexp (default "[\\\\/]\\.elmo[\\\\/]")
  -V, --verbose          verbose output
  -i, --interactive      ask before deleting each file
  -c, --confirm          ask before starting deletion
  -a, --age int          minimum age in days before deletion (0 = delete immediately)
      --keep-emacs-backups int
                         number of newest Emacs numbered backup generations to keep per original file (0 = disabled)
  -p, --profile string   profile name to load from the config file (default "default")
      --config string    path to the config file (default "$XDG_CONFIG_HOME/sweep/config.toml")
      --no-config        ignore config files entirely
  -h, --help             show this help message
```

Examples:

```sh
sweep --dry-run --verbose
sweep --age 7 --confirm ~/work
sweep --age 7 --keep-emacs-backups 2 ~/work
sweep --exclude '[\\/](\\.git|\\.elmo)[\\/]' .
```

Emacs numbered backups such as `file.txt.~1~` and `file.txt.~2~` are grouped by
their original path when `--keep-emacs-backups` is set. The newest generations
by backup number are kept even if they are older than `--age`; non-numbered
backups such as `file.txt~` and `.bak` files continue to use the normal age
rule.

## Configuration file

`sweep` can read defaults from a TOML config file instead of, or in addition
to, command-line flags.

- Default path: `$XDG_CONFIG_HOME/sweep/config.toml` (on Linux, typically
  `~/.config/sweep/config.toml`). Use `--config PATH` to load a file from
  somewhere else, or `--no-config` to ignore config files entirely.
- Each top-level TOML table is a named profile. Use `-p, --profile NAME` to
  select one; if `--profile` is not given, the profile named `default` is
  used.
- Profile keys mirror the long flag names: `dry-run`, `verbose`,
  `interactive`, `confirm`, `exclude`, `age`, `keep-emacs-backups`, and
  `directory` (a `~/` or `~` prefix in `directory` is expanded to the user's
  home directory). Keys outside of any table, or unknown keys in any
  profile, are errors.

Example `~/.config/sweep/config.toml`:

```toml
[default]
age = 7
exclude = '[\\/]\.elmo[\\/]'

[work]
confirm = true
keep-emacs-backups = 2
directory = "~/work"
```

Run `sweep --profile work` to use the `[work]` profile above.

Profiles are independent: a selected profile never inherits values from
`[default]` or any other profile.

Precedence is: built-in defaults < selected profile values < flags explicitly
given on the command line. A flag passed on the command line always wins,
even something like `--confirm=false` overriding `confirm = true` in a
profile; the positional directory argument likewise overrides a profile's
`directory`.

Selection errors:

- Without `--profile`, a missing config file or a config file without a
  `[default]` table is not an error; `sweep` falls back to built-in defaults.
- With `--profile NAME` (including `--profile default`), the config file must
  exist and must contain a `[NAME]` table, or `sweep` exits with an error. An
  empty `--profile ""` is also an error.
- With `--config PATH`, the file must exist and parse successfully, or
  `sweep` exits with an error (a missing `[default]` table in it is still
  fine unless `--profile` was also given).
- `--no-config` combined with `--profile` or `--config` is an error, since
  the combination is contradictory.

## Exit Codes

- `0`: completed successfully, printed help/version, or the user cancelled with
  `--confirm`
- `1`: runtime failure, such as walk or remove errors
- `2`: usage or validation error, such as an invalid regexp, negative age,
  negative Emacs backup retention, too many directory arguments, or a config
  file error (missing file, invalid TOML, unknown key, or missing profile)

## Development

```sh
go test ./...
go vet ./...
go test -cover ./...
go build -o /tmp/sweep-check .
```

The code keeps a root `main` package for `go install
github.com/nekomimist/sweep@...` compatibility. The testable core logic lives
under `internal/sweep`, and `cmd/sweep` provides an equivalent explicit command
entrypoint for repository builds.
