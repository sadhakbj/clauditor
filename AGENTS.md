# Repository Guidelines

## Project Structure & Module Organization
`main.go` registers Cobra commands and defers logic to `cli.go`, `scanner.go`, `dashboard.go`, `tui.go`, and `tui_views.go`. `scanner.go` owns SQLite schema helpers and JSONL parsing, with coverage in `scanner_test.go`. Web assets live in `web/index.html`; screenshots stay under `docs/images/` so they continue to be embedded at build time.

## Build, Test & Development Commands
- `go build -o bin/clauditor .` compiles the binary with embedded assets; run before tagging a release.
- `go run . dashboard --port 8080` rescans transcripts and serves the Vue dashboard for smoke testing.
- `go test ./...` runs unit tests; narrow with `-run TestParseJSONLFile` while working on parsing.
- `go fmt ./...` then `go vet ./...` keeps formatting and static checks aligned with upstream Go tooling.

## Coding Style & Naming Conventions
Always format with `gofmt` (tabs, trailing newline). Exported identifiers use PascalCase, internals camelCase, and command names remain lowercase (`scan`, `today`). Maintain the lightweight `// ── section ──` dividers already in long files and reuse the `color` helpers for CLI output instead of new log abstractions.

## Testing Guidelines
Place tests beside the code under test and name functions `TestXxx` so `go test` discovers them. Mirror the table-driven approach in `scanner_test.go` for new pricing or token scenarios, and rely on `t.TempDir()` for transcript fixtures to avoid touching a real Claude project directory. Document any gaps or skipped cases in the PR body after running `go test ./...`.

## Commit & Pull Request Guidelines
Write imperative, present-tense summaries (`Add dashboard cache filter`), grouping unrelated changes into separate commits. Reference issues with `Fixes #123` where relevant and note what was exercised (`go test ./...`, `go run . tui`). Pull requests should outline the user impact, call out schema or flag changes, and attach screenshots or GIFs when UI output shifts.

## Configuration & Security Notes
Defaults point at `~/.claude/projects` for source transcripts and `~/.claude/usage.db` for storage; override with `--dir` and `--db` during testing or support investigations. Exclude local databases, binaries, and customer data from commits, and request redacted JSONL excerpts rather than entire project folders when debugging external reports.
