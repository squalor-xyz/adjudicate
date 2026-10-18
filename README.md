<img src="assets/adjudicate-512.png" alt="" width="96">

# adjudicate

adjudicate (CLI `adj`) is a plain-text decision log for owner and agent work.
An agent asks a question with options and a recommendation, the owner decides
in batches, and every later agent reads the decisions instead of asking again.

- One Markdown file per record in a `decisions/` directory, with a JSON
  frontmatter block, so records diff, merge and review in a pull request.
- Sequential IDs; a record is `open`, `decided`, `rejected`, `withdrawn` or
  `superseded`, and keeps its history.
- No server, no database, no network, no LLM calls. Go standard library only.

This is version 0.1.0. [CHANGELOG.md](CHANGELOG.md) lists what each release
changed; [PROJECT.md](PROJECT.md) says what adjudicate is for and what it does.

## Install

Each release on GitHub
(<https://github.com/squalor-xyz/adjudicate/releases>) has an archive per
system: linux, darwin (macOS) and freebsd, each on amd64 and arm64, named
`adj_VERSION_OS_ARCH.tar.gz`, and windows on amd64 and arm64, named
`adj_VERSION_windows_ARCH.zip`, and a `SHA256SUMS` file that lists all of them.
Each archive holds `adj` (`adj.exe` on Windows), `LICENSE`, `NOTICE`,
`README.md` and `CHANGELOG.md`. For 0.1.0 on an Apple silicon Mac:

```sh
curl -LO https://github.com/squalor-xyz/adjudicate/releases/download/v0.1.0/adj_0.1.0_darwin_arm64.tar.gz
curl -LO https://github.com/squalor-xyz/adjudicate/releases/download/v0.1.0/SHA256SUMS
shasum -a 256 --check --ignore-missing SHA256SUMS   # Linux: sha256sum --check --ignore-missing SHA256SUMS
tar -xzf adj_0.1.0_darwin_arm64.tar.gz
mkdir -p ~/.local/bin
install -m 0755 adj_0.1.0_darwin_arm64/adj ~/.local/bin/adj   # or any directory on your PATH
adj version                                                   # v0.1.0
```

On Windows (the zips come with the releases after 0.1.0), in PowerShell, for
amd64; the output of `Get-FileHash` must match the line for the zip in
`SHA256SUMS` (compare ignoring case: `Get-FileHash` prints uppercase hex,
`SHA256SUMS` has lowercase):

```powershell
curl.exe -LO https://github.com/squalor-xyz/adjudicate/releases/download/vVERSION/adj_VERSION_windows_amd64.zip
curl.exe -LO https://github.com/squalor-xyz/adjudicate/releases/download/vVERSION/SHA256SUMS
Get-FileHash -Algorithm SHA256 adj_VERSION_windows_amd64.zip
Select-String windows_amd64 SHA256SUMS
Expand-Archive adj_VERSION_windows_amd64.zip -DestinationPath .
# put adj_VERSION_windows_amd64\adj.exe in a directory on your PATH
```

With Go (the version in `go.mod`, 1.27.2, or later) you can install from the
module instead; `adj version` then prints the module version, `v0.1.0`:

```sh
go install github.com/squalor-xyz/adjudicate/cmd/adj@v0.1.0
```

`go install` puts `adj` in `$(go env GOPATH)/bin`. To build from a checkout,
see below.

## Build from source

You need Go (the version in `go.mod`). From the repo root, source the in-repo
cache setup first. It points Go's build, module and test caches, `GOPATH` and
temp files at `.cache/` in the checkout (`GOCACHE`, `GOMODCACHE`, `GOPATH`,
`GOTMPDIR`, `TMPDIR`) and keeps Go off the network; use bash or zsh:

```sh
. scripts/env.sh
go build -o .cache/bin/adj ./cmd/adj
```

Put `.cache/bin` on your `PATH`, or run `.cache/bin/adj`. To run the tests and
checks:

```sh
go test ./...
go vet ./...
gofmt -l cmd internal    # prints nothing
```

`go test ./...` includes a round-trip test that builds the binary and walks
the whole life of a decision (`cmd/adj/roundtrip_test.go`).

`adj version` and `adj --version` print one line. A release build sets the
version with the linker, as the release workflow does:

```sh
go build -ldflags "-X github.com/squalor-xyz/adjudicate/internal/cli.versionOverride=v0.1.0" -o .cache/bin/adj ./cmd/adj
```

Without that, `adj` prints the main module version that Go records in the
binary. With the Go toolchain this repo uses (1.27.2), a plain `go build` inside
a git checkout records a pseudo-version made from the commit time and revision,
such as `v0.0.0-20261017153413-4d1d8a898308+dirty`, and
`go build -buildvcs=false` records none, so `adj` prints `(devel)`. If the
version is `(devel)` or empty, `adj` adds the VCS revision when the build
information has one.

## Quick start

```sh
adj init                                    # in the repo root
adj ask "Which cache?" --option "mem:In memory" --option "redis:Use Redis" \
    --recommend redis --reasoning "Hosts share it" --key storage.cache
adj inbox                                   # the owner reads the open questions
adj decide 1 redis --why "Several hosts share it" --by jon
adj context                                 # agents read the decisions in force
adj check storage.cache                     # has this been decided?
```

Commands find the repo root from the working directory upwards (the nearest
`.adjudicate.json`). `--root DIR` names it instead, either before the command
(`adj --root DIR list`) or after it (`adj list --root DIR`). Giving it both
before and after the command, or twice before it, is a usage error (exit 2);
twice after the command still takes the last. Before the command `adj` takes
`--root DIR`; `--help` and `-h` print the main usage, and `adj --version` as the
first argument prints the version. Any other flag there is a usage error.
`adj COMMAND --help` prints a command's flags.

## Using it in a repo

Set up the store once, in the root of the repo whose decisions it keeps, and
tell its agents about it:

```sh
adj init                  # writes .adjudicate.json and creates decisions/
adj ai >> AGENTS.md       # the agent instructions; or paste them into CLAUDE.md
git add .adjudicate.json decisions AGENTS.md
```

Commit `.adjudicate.json` and `decisions/`: the records are plain Markdown
files that diff, merge and review like code. `adj ask` takes IDs under a lock
file, `decisions/.adj.lock`; add it to `.gitignore` if you do not want it in
git (`adj` writes no `.gitignore`).

Agents work from the snippet that `adj ai` printed. Before they ask, they run
`adj check QUERY`; they read the decisions in force with `adj context`, and ask
with `adj ask`. Set `ADJ_ASKED_BY` to name the asker. Agents never decide.

The owner clears the inbox in batches:

```sh
export ADJ_DECIDED_BY="Your Name"   # or pass --by on each command
adj inbox                           # the open questions, oldest first
adj decide 3 redis --why "Several hosts share it"
adj decide 4 --text "Neither; use files" --why "No server to run"
adj reject 5 --why "Out of scope"
adj supersede 2 --by 6              # 6 replaces decision 2
adj amend 6 --rationale "Clearer reasons"
```

Run `adj lint` in CI, so a merge that leaves two records with one ID, a broken
record or a dangling link fails the build (`adj renumber` fixes duplicate IDs).
In GitHub Actions:

```yaml
- uses: actions/setup-go@v5
  with:
    go-version: stable
- run: go install github.com/squalor-xyz/adjudicate/cmd/adj@v0.1.0
- run: adj lint
```

## Linking to slicer

Use refs to link a question to a slicer slice, then retrieve its records:

```sh
adj ask "Which cache?" --ref S20 --recommend-text "Use files"
adj list --ref S20 --json
adj context --ref S20
```

`list` includes every status; `context` includes only decisions in force.
Repeat `--ref` to require every reference, for example `--ref S20 --ref C12`.
Matching is exact and case-sensitive: `S20` does not match `S200` or `s20`.
Neither tool depends on the other; refs are opaque strings.

## Exit codes

For every command: 0 success; 1 a failed operation or failed check (no repo,
unknown ID, record in the wrong status); 2 a usage error. Two commands are
special: `adj check` exits 0 for every completed search, including `closed` and
`not found`, so a script reads the result from the output; `adj lint` exits 1
when it finds an error and 0 when it finds none or only warnings.

## Commands

### `adj init [--root DIR]`

Writes `.adjudicate.json` (`{"store": "decisions"}`) if it is missing and
creates the store directory. It acts on DIR or the working directory, never on
a parent, and DIR must already exist. Running it again changes nothing.

### `adj ask TITLE [flags]`

Records an open question and prints its ID. Flags may come before or after
TITLE.

```
--option LABEL:SUMMARY    an answer on offer; repeat for each option
--tradeoffs LABEL:TEXT    the trade-offs of the option LABEL; once per option
--recommend LABEL         the option you recommend
--recommend-text TEXT     a recommendation that is not one of the options
                          (instead of --recommend)
--reasoning TEXT          why you recommend it (needs --recommend or
                          --recommend-text)
--key KEY                 the decision key, such as storage.engine
--tag TAG                 a tag; repeat for several
--ref REF                 a reference; repeat for several (opaque, kept in order)
--context TEXT            why the question is asked
--stdin                   read the context from standard input instead
--by NAME                 who asks; default: $ADJ_ASKED_BY, else left blank
--root DIR                the repo root
```

### `adj decide ID OPTION --why TEXT [flags]`

Answers an open question: OPTION must be one of the record's option labels. To
give a free-text answer instead, use `adj decide ID --text TEXT --why TEXT`.
The record becomes `decided`. Prints its ID. Only the owner decides; this is a
convention, not enforced.

```
--by NAME              who decided; default: $ADJ_DECIDED_BY, else blank
--decided-at DATE      the date the decision took effect, YYYY-MM-DD;
                       default: today
```

`--decided-at` records a decision made earlier: `decided_at` is DATE, while
the history entry keeps the day you ran the command and notes
`backdated to DATE`. A date that is not a valid `YYYY-MM-DD` date, or that is
after today, is a usage error (exit 2). A date before the question was created
is allowed.

### `adj reject ID --why TEXT` and `adj withdraw ID --why TEXT`

Close an open question without an answer. `reject` records that the question
will not be answered; `withdraw` records that the asker no longer needs an
answer. Both print the ID and take `--by NAME` (default `$ADJ_DECIDED_BY`).

### `adj supersede OLD --by NEW`

Replaces a decided record with another decided one. OLD becomes `superseded`
and points to NEW; NEW lists OLD in `supersedes`. OLD stays readable with
`adj show` and leaves `adj context`. Prints OLD's ID.

### `adj amend ID [flags]`

Corrects the text of a record of any status and prints its ID. Give at least
one flag; several may change several fields in one call.

```
--title TEXT              the new title (one line)
--context TEXT            the new context
--stdin                   read the context from standard input instead
--summary LABEL:TEXT      the new summary of the option LABEL; once per option
--tradeoffs LABEL:TEXT    the new trade-offs of the option LABEL; once per option
--rationale TEXT          the new rationale (an open record has none)
--root DIR                the repo root
```

Only text changes: never the status, decision, who decided or asked, dates,
links, key, tags, an option's label or the recommendation. An empty TEXT in
`--summary` or `--tradeoffs` clears that text. Each call appends one `amend`
entry to the history that keeps the old text of every field it changed, and
`adj show` prints it. If nothing would change, it fails and writes nothing.
Changing the title does not rename the record file: the file name keeps the
slug of the title the question was asked with, so it can drift from the title.

Agents amend only the title, context and option texts of an `open` record; only
the owner amends a decided record, its rationale included. This is guidance
(`adj ai` says so), not enforced.

### `adj show ID [--json]`

Prints one record: its fields and history. With `--json` it prints the record
as a JSON object.

### `adj list [flags]`

Lists records in ID order, one line each (ID, status, title).

```
--status S   only records with status S: open, decided, rejected, withdrawn
             or superseded
--tag T      only records with tag T; repeat to require several tags
--ref REF    only records with this exact ref; repeat to require every ref
--key K      only records whose key is K exactly
--json       print {"records": [...]}
```

Every filter given must match. Refs are case-sensitive and matched as typed,
including surrounding whitespace; a blank `--ref` exits 2 before store access.
No match is not an error.

### `adj inbox [[--all] [--json] | -i [--by NAME]] [--root DIR]`

Lists the open questions, oldest first, each with its references, options and
recommendation. An empty inbox prints nothing.

With `--all`, read the user repo list managed by `adj repos` and show each
repo's open questions under its absolute path, in config insertion order.
Records remain oldest first within each repo. Successful empty repos keep
their header; an empty or missing user config prints nothing. A missing repo,
missing `.adjudicate.json`, or unreadable store produces a warning on stderr
naming that repo; the others are still printed, and the command exits 1.
`--all --json` prints `{"repos": [{"root": "...", "records": [...]}], "errors": []}`
(see `docs/json.md`); per-repo errors appear in JSON as well as stderr. The
arrays are always arrays, including when empty. `--all` with `--root` exits 2;
no current repo is needed. Without `--all`, output is unchanged.

With `-i`, the owner walks through this repo's open questions in the same
oldest-first order. Each question shows the usual inbox block, then prompts:

```text
Action [d=option, t=text, r=reject, w=withdraw, s=skip, q=quit, ?=help]:
```

`d` asks for an option label; a blank label returns to the action prompt.
If the record has no options, `d` reports an error pointing to `t` and returns
to the action prompt without asking for a label. `t` asks for free-text decision
text. `r` rejects,
`w` withdraws, `s` skips and `q` quits; `?` prints help. Each write asks for a
non-blank rationale. Unknown labels and blank text or rationale re-prompt the
same field. Decision text and rationale keep their surrounding whitespace;
option labels are trimmed, as with `adj decide`. `--by NAME` sets the owner,
falling back to `$ADJ_DECIDED_BY` when empty or absent. Writes use the same
validation, dates and history as `decide`, `reject` and `withdraw`.

Quit or EOF stops without writing an incomplete answer, including a last line
without a newline. A nonempty walk ends with `N decided, N rejected, N withdrawn,
N skipped, N failed`, counting only completed actions; unvisited questions stay
open. A failed write reports its error on stderr and advances to the next
question; any failed write makes the final exit code 1. An input read error also
exits 1. An empty inbox prints nothing and exits 0. `-i` with `--json`, `--all`
or an argument exits 2, as does `--by` without `-i`. This mode is owner-only;
agents must not run it. It uses ordinary stdin lines, without terminal raw mode.

### `adj check QUERY [--json]`

Asks whether QUERY has been decided, so an agent checks before it asks. QUERY
is one substring, matched ignoring case against a record's title, key, tags,
references and decision text; quote a query that has spaces. Matching records
show their references in text output. The first line of the output is the result:

```
decided     a decided record matches
open        no decided record matches, but an open question does
closed      only rejected, withdrawn or superseded records match
not found   nothing matches
```

Then one line per matching record, with the decision and its rationale. A
superseded match leads to the decided record that replaced it.

### `adj context [flags]`

Prints the decided records as compact context for an agent: per decision its
ID, title, key, the decision and the first line of the rationale. Open,
rejected, withdrawn and superseded records are left out.

```
--tag T          only records with tag T; repeat to require several tags
--ref REF        only records with this exact ref; repeat to require every ref
--key-prefix P   only records whose key starts with P
--format F       md (the default) or json
--max-bytes N    keep the newest decisions that fit in N bytes
```

Every filter given must match. Refs are case-sensitive and matched as typed,
including surrounding whitespace; a blank `--ref` exits 2 before store access.
The same store gives the same bytes. With `--max-bytes N` (N at least 1) the
output is at most N bytes, the omitted line included: the newest decisions
(`decided_at`, latest first, then the higher ID) that fit are kept and still
printed in ID order, the filters are applied first, and the Markdown ends with a
blank line and `N decisions omitted` (`1 decision omitted` for one). With
`--format json` the object gains `omitted`, the number left out, `0` included;
without `--max-bytes` that key is never there. If no decision fits, only the
omitted line is printed (JSON: `"decisions": []`); if not even that fits in N
bytes, nothing is printed and the exit code is 1. A bad N exits 2.

### `adj lint [--json]`

Checks the store for integrity problems and prints one line per finding
(`FILE: record ID: SEVERITY: CHECK: MESSAGE`), then a count. A clean store
prints nothing. The checks are `parse`, `fields`, `id-mismatch`,
`duplicate-id`, `renumber-leftover`, `dangling-link`, `supersede-state`,
`supersede-cycle`, `key-conflict` and `decided-by` (a warning). Exit 1 if there
is an error, else 0.

### `adj renumber [--keep-links]`

Owner-only. Resolves record files that share an ID, which merged branches and
clones can produce (`adj lint` reports them as `duplicate-id`). The record
created first keeps the ID; each other one is written to a new file with the
next free ID, and its old file is removed. Links to a shared ID cannot be told
apart, so renumber stops until you check them and pass `--keep-links`. See
`adj renumber --help`.

### `adj repos add [PATH]`, `adj repos remove PATH`, `adj repos list [--json]`

Manage the per-user list of repos. `add` defaults to the current repo root
(`--root DIR` overrides that default); an explicit PATH is used instead.
Relative paths are resolved against the working directory and stored as cleaned
absolute paths, without resolving symlinks. A repo must hold a regular
`.adjudicate.json` file. Adding a listed path succeeds without rewriting the
file. `remove` needs a PATH and exits 1 without writing if it is not listed;
it can remove a repo that no longer exists. Add and remove print nothing.
`list` prints one stored path per line in insertion order, or
`{"repos": [...]}` with `--json`; it does not check whether repos still exist.
An empty list prints nothing (JSON: `{"repos": []}`). `--root` is accepted but
unused for explicit paths and for list/remove; these need no current repo.

The list lives in `$XDG_CONFIG_HOME/adjudicate/config.json`, falling back to
`$HOME/.config/adjudicate/config.json` when XDG_CONFIG_HOME is unset, empty or relative,
on every OS. If HOME is needed, it must be absolute; an unset, empty or relative HOME
causes exit 1.
A missing file is an empty list. Bad JSON, unknown keys or non-absolute repo
paths exit 1 and leave the file unchanged. Writes create the directory (0755)
and replace the config (0644) atomically using a temp file in the same directory
and rename. This user config is separate from `.adjudicate.json`.

Removing from a missing config fails as unlisted without creating a directory
or lock file.
Add/remove hold `config.json.lock` beside the user config for the complete
read-modify-write operation (flock on Unix, LockFileEx on Windows). They wait
up to 5 seconds, then exit 1 naming the lock file without changing the config.
The empty lock file remains after release; list ignores it. Other platforms
without either locking primitive retain the store lock's no-lock fallback.

### `adj version [--root DIR]`

Prints the version: the release override set with `-ldflags -X`, else what Go
build information reports (see Build from source). The same value is printed by
`adj --version`. It reads no repo and needs no config, and accepts `--root` but
does not use it.

### `adj ai`

Prints a short snippet for your own `AGENTS.md` that tells agents how to check,
ask and read decisions, and what they must leave to the owner. It reads no repo
and takes no argument.

## Output for scripts

`show`, `list`, `inbox`, `check`, `lint` and `repos list` print JSON with `--json`; `context`
does it with `--format json`. The shapes are in [docs/json.md](docs/json.md).
Until 1.0 a shape may change in a minor release; the CHANGELOG names every
change. From 1.0 a release may add keys but never renames, removes or retypes
an existing key.

## Environment

- `XDG_CONFIG_HOME`, else `$HOME/.config`: base directory for the user repo list.
- `ADJ_ASKED_BY`: the default `--by` for `adj ask`.
- `ADJ_DECIDED_BY`: the default `--by` for `decide`, `reject` and `withdraw`.

## Releases

CI (`.github/workflows/ci.yml`) runs `go vet`, `gofmt`, `go test` and `adj
lint` on this repo's own `decisions/` for every push to `main` and every pull
request, and runs the tests on macOS and Windows too. Pushing a tag `vX.Y.Z`
runs `.github/workflows/release.yml`: the same checks, then a build for each
system and architecture above with the version set from the tag, and a GitHub release with the
archives, `SHA256SUMS`, and the `## X.Y.Z` section of
[CHANGELOG.md](CHANGELOG.md) as its notes. The release fails if that section
is missing, so write it before you tag.

## License

GPL-3.0-or-later. See `LICENSE` for the full text and `NOTICE` for the
copyright notice.

The icon, `assets/adjudicate-512.png`, is a Squalor LLC brand asset and is not
covered by the GPL. It is all rights reserved, with a grant to keep it
unmodified in this repository's `assets/` and README.
