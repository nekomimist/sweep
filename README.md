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

## Exit Codes

- `0`: completed successfully, printed help/version, or the user cancelled with
  `--confirm`
- `1`: runtime failure, such as walk or remove errors
- `2`: usage or validation error, such as an invalid regexp, negative age,
  negative Emacs backup retention, or too many directory arguments

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
