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
`adj_VERSION_OS_ARCH.tar.gz`, and a `SHA256SUMS` file. Each archive holds `adj`,
`LICENSE`, `NOTICE`, `README.md` and `CHANGELOG.md`. For 0.1.0 on an Apple
silicon Mac:

```sh
curl -LO https://github.com/squalor-xyz/adjudicate/releases/download/v0.1.0/adj_0.1.0_darwin_arm64.tar.gz
curl -LO https://github.com/squalor-xyz/adjudicate/releases/download/v0.1.0/SHA256SUMS
shasum -a 256 --check --ignore-missing SHA256SUMS   # Linux: sha256sum --check --ignore-missing SHA256SUMS
tar -xzf adj_0.1.0_darwin_arm64.tar.gz
mkdir -p ~/.local/bin
install -m 0755 adj_0.1.0_darwin_arm64/adj ~/.local/bin/adj   # or any directory on your PATH
adj version                                                   # v0.1.0
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
--key K      only records whose key is K exactly
--json       print {"records": [...]}
```

Every filter given must match. No match is not an error.

### `adj inbox [--json]`

Lists the open questions, oldest first, each with its references, options and
recommendation. An empty inbox prints nothing.

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
--key-prefix P   only records whose key starts with P
--format F       md (the default) or json
```

The same store gives the same bytes.

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

`show`, `list`, `inbox`, `check` and `lint` print JSON with `--json`; `context`
does it with `--format json`. The shapes are in [docs/json.md](docs/json.md).
Until 1.0 a shape may change in a minor release; the CHANGELOG names every
change. From 1.0 a release may add keys but never renames, removes or retypes
an existing key.

## Environment

- `ADJ_ASKED_BY`: the default `--by` for `adj ask`.
- `ADJ_DECIDED_BY`: the default `--by` for `decide`, `reject` and `withdraw`.

## Releases

CI (`.github/workflows/ci.yml`) runs `go vet`, `gofmt`, `go test` and `adj
lint` on this repo's own `decisions/` for every push to `main` and every pull
request, and runs the tests on macOS too. Pushing a tag `vX.Y.Z` runs
`.github/workflows/release.yml`: the same checks, then a build for the six
systems above with the version set from the tag, and a GitHub release with the
archives, `SHA256SUMS`, and the `## X.Y.Z` section of
[CHANGELOG.md](CHANGELOG.md) as its notes. The release fails if that section
is missing, so write it before you tag.

## License

GPL-3.0-or-later. See `LICENSE` for the full text and `NOTICE` for the
copyright notice.

The icon, `assets/adjudicate-512.png`, is a Squalor LLC brand asset and is not
covered by the GPL. It is all rights reserved, with a grant to keep it
unmodified in this repository's `assets/` and README.
