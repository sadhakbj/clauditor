# claude-usage

A CLI tool to track and visualize your Claude Code token usage locally. Parses the JSONL transcript files that Claude Code writes to disk and stores them in a local SQLite database — no API key needed.

Works for Claude Code **Pro and Max** subscribers.

## How it works

Claude Code stores conversation transcripts as JSONL files under `~/.claude/projects/`. `claude-usage` reads those files, extracts token counts and model info from each turn, and writes them to `~/.claude/usage.db`. From there you can print summaries or spin up a local web dashboard.

## Requirements

- Go 1.21+
- Claude Code installed and used at least once

## Installation

```sh
git clone https://github.com/sadhakbj/claude-usage
cd claude-usage
go build -o claude-usage .
```

Optionally move the binary somewhere on your `$PATH`:

```sh
mv claude-usage /usr/local/bin/
```

## Usage

```
claude-usage [command] [flags]
```

### Commands

| Command | Description |
|---------|-------------|
| `scan` | Parse JSONL transcripts and write to the database |
| `today` | Print today's usage broken down by model |
| `stats` | Print all-time statistics (by model, top projects, daily averages) |
| `dashboard` | Run `scan` then start a local web dashboard |

### Global flags

| Flag | Default | Description |
|------|---------|-------------|
| `--db` | `~/.claude/usage.db` | Path to the SQLite database |
| `--dir` | `~/.claude/projects` | Path to Claude projects directory |

### Dashboard flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8080` | Port for the dashboard server |
| `--no-browser` | `false` | Don't open the browser automatically |

### Examples

```sh
# First scan, then check today
claude-usage scan
claude-usage today

# All-time stats
claude-usage stats

# Web dashboard on a custom port, no auto-open
claude-usage dashboard --port 9090 --no-browser

# Use a non-default projects directory or database
claude-usage scan --dir ~/work/.claude/projects --db ~/work/usage.db
```

## Dashboard

The dashboard runs at `http://localhost:8080` (or your chosen port) and shows:

- Daily token usage over time (chart)
- Breakdown by model (chart)
- Top projects by token volume (chart)
- Recent sessions table
- Cost by model table

Use the filter controls to narrow down by model or date range (7d / 30d / 90d / all).

## Cost estimates

Costs are estimated using Anthropic API pricing (April 2026 rates) and are approximations — your actual Claude Code subscription cost may differ.

| Model | Input | Output | Cache write | Cache read |
|-------|-------|--------|-------------|------------|
| Opus 4.x | $6.15/M | $30.75/M | $7.69/M | $0.61/M |
| Sonnet 4.x | $3.69/M | $18.45/M | $4.61/M | $0.37/M |
| Haiku 4.x | $1.23/M | $6.15/M | $1.54/M | $0.12/M |

## Tech

- Pure Go, single binary
- SQLite via [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) (no CGo)
- CLI via [Cobra](https://github.com/spf13/cobra)
- Dashboard frontend: vanilla JS + [Chart.js](https://www.chartjs.org/) (embedded in binary)
