# lazyjsonl

A terminal UI for inspecting, filtering, and managing JSONL files — like lazygit, but for structured data.

Open a file or a folder, browse records in a column table, filter in real time with a real query language, inspect full entries, sort, delete, and export — all without leaving the terminal. Built on [jsonldb](https://github.com/xZhad/jsonldb) for the data layer and [bubbletea](https://github.com/charmbracelet/bubbletea) for the TUI.

lazyjsonl adds **zero query/storage logic** — filtering, schema discovery, sorting, paging, and deletes all route through jsonldb's public API.

---

## Install

Homebrew:

```sh
brew install xZhad/tap/lazyjsonl
```

Or with Go (requires Go 1.26+):

```sh
go install github.com/xZhad/lazyjsonl@latest
```

---

## Usage

```sh
lazyjsonl                     # current directory (same as `lazyjsonl .`)
lazyjsonl data.jsonl          # open a single file
lazyjsonl ./logs              # open a directory (browse every *.jsonl; TUI only)
lazyjsonl .                   # current directory (TUI only)
```

Folders are a TUI feature: the CLI (flags or piped output) reads exactly one `.jsonl` file and says so if given a folder. A path starting with `-` must be written `./-name`.

When stdout is a terminal, lazyjsonl launches the interactive TUI. When piped — or when any of `--filter`, `--count`, `--out` or `--output` is given — it runs non-interactively. `lazyjsonl --help` (or `-h`) prints the usage; any other unknown `-`/`--` argument is rejected with a pointer to it.

### CLI mode (pipe-friendly)

```sh
# filter and print matching records as JSONL
lazyjsonl data.jsonl --filter "completed=true" --output json

# count matches
lazyjsonl data.jsonl --filter "topic~=pomo" --count

# export a filtered subset to a new file
lazyjsonl data.jsonl --filter "completed=true" --out done.jsonl
```

| Flag | Meaning |
|------|---------|
| `--filter <dsl>` | filter expression (see below) |
| `--count` | print the number of matches instead of records |
| `--out <file>` | write matching records to a file (atomic) instead of stdout; its directory must already exist |
| `--output <fmt>` | `json` or `jsonl` (JSON lines either way, the default); anything else is an error, use `--out FILE` to write a file |
| `-h`, `--help` | show the usage and exit |

`--count` cannot be combined with `--out` (it prints a number, not records); the combination is rejected before anything is opened.

---

## Key bindings (TUI)

| Key | Action |
|-----|--------|
| `j` / `k` / `↑` `↓` | move row cursor |
| `h` / `l` / `←` `→` | previous / next page |
| `H` / `L` | move column cursor (for sort) |
| `J` / `K` | next / previous file |
| `tab` | switch focus between the file list and the table |
| `enter` | full-record detail view |
| `/` | open the filter bar |
| `s` | sort by the column under the cursor (toggles asc/desc) |
| `c` | toggle showing all columns |
| `d` | delete the current row (with confirmation) |
| `e` | export the current view to `<name>.export.jsonl` |
| `y` | yank the current record as JSON to the clipboard |
| `r` | reload the file from disk |
| `g` / `G` | jump to first / last page |
| `?` | toggle the help overlay |
| `q` / `ctrl+c` | quit |

Columns are inferred from the data (by field presence) via jsonldb's schema discovery.

---

## Filter syntax

The filter bar (and `--filter`) accept jsonldb's query DSL:

```
completed=true                       # exact match (type-coerced)
topic~=jsonl                         # substring, case-insensitive
notes                                # key exists
completed=true topic~=pomo           # AND (space-separated)
status=active |= status=paused       # OR
a=1 (b=2 |= c=3)                     # grouping
!completed                           # NOT
duration>=1500 started>=2026-06-01   # numeric + time comparison
```

Operators: `=` `!=` `>` `>=` `<` `<=` `~=` (contains) `^=` (prefix) `$=` (suffix) `=~` (regex), plus bare-key existence, `!` (NOT), `()` (grouping), and `|=` (OR). A malformed filter is shown inline in the filter bar — the TUI never crashes on a bad query.

---

## Notes

- Files are loaded in full (JSONL files are small by design); no streaming.
- Deletes target a record by its line in the file and rewrite atomically.
- Clipboard yank currently uses `pbcopy` (macOS); on other platforms `y` reports that the clipboard is unavailable.
- Columns whose own name contains a dot (`{"a.b": 1}`): lazyjsonl displays them and feeds them to stats, histogram, scatter and time series, but sorting, grouping, group measures / bar-chart values and diving into objects under such a column need jsonldb support (its sort, query DSL and aggregates always split names on dots), so `s` keeps jsonldb's order, and `f`/`F`/`a`/dive refuse with a status message. If a record has both a top-level `a.b` and a nested `a` -> `b`, the table shows the top-level value while sort and filter (jsonldb) use the nested one.

---

## License

MIT
