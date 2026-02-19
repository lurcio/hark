# Contributing to hark

Thanks for your interest in contributing!

## Getting started

```bash
git clone https://github.com/lurcio/hark.git
cd hark
go build -o hark ./cmd/hark
go test ./...
```

Requires Go 1.25+.

## Development workflow

1. Fork the repo and create a branch from `main`.
2. Make your changes.
3. Run tests and the linter:
   ```bash
   go test -race ./...
   golangci-lint run ./...
   ```
4. Open a pull request against `main`.

## Testing philosophy

Test logic, not UI. Focus on the diff engine, gitignore matching, timeline, config merging, and debouncing. Don't test Bubble Tea rendering, Chroma output, or terminal escape sequences.

## Code style

- Follow standard Go conventions (`gofmt`, `go vet`).
- All CI checks must pass before merge.

## Reporting bugs

Use [GitHub Issues](https://github.com/lurcio/hark/issues). Include your OS, Go version, and steps to reproduce.
