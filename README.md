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
  -h, --help             show this help message
```

Examples:

```sh
sweep --dry-run --verbose
sweep --age 7 --confirm ~/work
sweep --exclude '[\\/](\\.git|\\.elmo)[\\/]' .
```

## Exit Codes

- `0`: completed successfully, printed help/version, or the user cancelled with
  `--confirm`
- `1`: runtime failure, such as walk or remove errors
- `2`: usage or validation error, such as an invalid regexp, negative age, or
  too many directory arguments

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
