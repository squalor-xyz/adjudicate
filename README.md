# adjudicate

adjudicate (CLI `adj`) is a plain-text decision log for owner and agent work.
An agent asks a question with options and a recommendation, the owner decides
in batches, and every later agent reads the decisions instead of asking again.

- One Markdown file per record in a `decisions/` directory, with a JSON
  frontmatter block, so records diff, merge and review in a pull request.
- Sequential IDs; a record is `open`, `decided`, `rejected`, `withdrawn` or
  `superseded`, and keeps its history.
- No server, no database, no network, no LLM calls. Go standard library only.

## Build

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
twice after the command still takes the last. It is the only flag accepted
before the command, apart from `--help`. `adj COMMAND --help` prints a
command's flags.

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
a parent. Running it again changes nothing.

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

### `adj decide ID OPTION --why TEXT`

Answers an open question: OPTION must be one of the record's option labels. To
give a free-text answer instead, use `adj decide ID --text TEXT --why TEXT`.
The record becomes `decided`. Prints its ID. `--by NAME` says who decided
(default `$ADJ_DECIDED_BY`, else blank). Only the owner decides; this is a
convention, not enforced.

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

### `adj ai`

Prints a short snippet for your own `AGENTS.md` that tells agents how to check,
ask and read decisions, and what they must leave to the owner. It reads no repo
and takes no argument.

## Output for scripts

`show`, `list`, `inbox`, `check` and `lint` print JSON with `--json`; `context`
does it with `--format json`. The shapes are in [docs/json.md](docs/json.md).

## Environment

- `ADJ_ASKED_BY`: the default `--by` for `adj ask`.
- `ADJ_DECIDED_BY`: the default `--by` for `decide`, `reject` and `withdraw`.

## License

GPL-3.0-or-later. See `LICENSE` for the full text and `NOTICE` for the
copyright notice.
