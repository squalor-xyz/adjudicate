// SPDX-License-Identifier: GPL-3.0-or-later

package adr

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func readFixture(t *testing.T, dir, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Every fixture parses to the title (adr-tools' "N. " prefix stripped),
// status, date, context, decision, justification, consequences and options
// that its text holds.
func TestParseFixtures(t *testing.T) {
	tests := []struct {
		dir, name string
		want      ADR
	}{
		{"nygard", "0001-record-architecture-decisions.md", ADR{
			Format: Nygard, Number: 1, Title: "Record architecture decisions", Status: Accepted,
			Date:         "2016-02-12",
			Context:      "We need to record the architectural decisions made on this project.",
			Decision:     "We will use Architecture Decision Records, as described by Michael Nygard.",
			Consequences: "See Michael Nygard's article, linked above. For a lightweight ADR toolset, see Nat Pryce's adr-tools.",
		}},
		{"nygard", "0002-use-postgresql.md", ADR{
			Format: Nygard, Number: 2, Title: "Use PostgreSQL", Status: Superseded,
			SupersededBy: &Link{File: "0004-use-cockroachdb.md", Number: 4},
			Date:         "2017-03-01",
			Context:      "The service needs a relational store with transactions.",
			Decision:     "We will store all service data in PostgreSQL.",
			Consequences: "We run and patch PostgreSQL ourselves, and a single node limits availability.",
		}},
		{"nygard", "0003-adopt-trunk-based-development.md", ADR{
			Format: Nygard, Number: 3, Title: "Adopt trunk-based development", Status: Proposed,
			Date:         "2018-05-20",
			Context:      "Long-lived branches make merges painful and hide integration problems.",
			Decision:     "We will merge to the main branch at least daily and use feature flags.",
			Consequences: "Unfinished work ships behind flags, which need cleaning up.",
		}},
		{"nygard", "0004-use-cockroachdb.md", ADR{
			// "Supersedes [..]" in the status is a link to an older ADR, not a status.
			Format: Nygard, Number: 4, Title: "Use CockroachDB", Status: Accepted,
			Date:         "2019-07-09",
			Context:      "A single PostgreSQL node cannot give the availability the service now needs.",
			Decision:     "We will store all service data in CockroachDB.",
			Consequences: "Operations learn a new database; the wire protocol stays PostgreSQL's.",
		}},
		{"nygard", "0005-use-mongodb.md", ADR{
			Format: Nygard, Number: 5, Title: "Use MongoDB", Status: Rejected,
			Date:         "2019-08-01",
			Context:      "A document store was suggested for the event data.",
			Decision:     "We will not use MongoDB.",
			Consequences: "Event data stays in the relational store, so it needs a schema.",
		}},
		{"nygard", "0006-keep-subversion.md", ADR{
			Format: Nygard, Number: 6, Title: "Keep Subversion", Status: Deprecated,
			Date:         "2012-01-15",
			Context:      "The code lives in a Subversion repository.",
			Decision:     "We will keep the code in Subversion.",
			Consequences: "No pull requests; reviews happen by mail.",
		}},
		{"nygard", "0007-ship-weekly.md", ADR{
			// No Date line: Date stays empty and the command uses the run date.
			Format: Nygard, Number: 7, Title: "Ship weekly", Status: Accepted,
			Context:      "Releases are rare and large.",
			Decision:     "We will cut a release every Friday.",
			Consequences: "Each release is small; the Friday freeze starts at noon.",
		}},
		{"nygard", "0008-use-semantic-versioning-crlf.md", ADR{
			// CRLF line endings, no trailing "\r" in any field.
			Format: Nygard, Number: 8, Title: "Use semantic versioning", Status: Accepted,
			Date:         "2020-02-29",
			Context:      "Consumers need to tell breaking changes from fixes.\nThey read the version number to do so.",
			Decision:     "We will version releases as MAJOR.MINOR.PATCH.",
			Consequences: "A breaking change always raises MAJOR.",
		}},
		{"madr", "0001-use-markdown-architectural-decision-records.md", ADR{
			Format: MADR, Number: 1, Title: "Use Markdown Architectural Decision Records", Status: Accepted,
			Date:     "2022-11-30",
			Deciders: "Alice Example, Bob Example",
			Context: "We want to record architectural decisions made in this project.\n" +
				"Which format and structure should these records follow?",
			Options: []string{"MADR 3.0.0", "Michael Nygard's template", "Sustainable Architectural Decisions", "Other templates"},
			// "Chosen option" names the decision; the text after "because" is the justification.
			Decision:      "MADR 3.0.0",
			Justification: "it is lean and the decision drivers are met.",
			Consequences: "* Good, because the records are plain text next to the code\n" +
				"* Bad, because the template is long for small decisions",
		}},
		{"madr", "0002-use-plain-junit5-for-testing.md", ADR{
			// MADR 2: "* Status:", "* Deciders:" and "* Date:" bullets, markdown links in options.
			Format: MADR, Number: 2, Title: "Use plain JUnit5 for testing", Status: Accepted,
			Date:          "2019-04-23",
			Deciders:      "Dana Example, Eli Example",
			Context:       "We want to write unit tests in Java. Which test framework should we use?",
			Options:       []string{"JUnit5", "TestNG", "Hamcrest"},
			Decision:      "JUnit5",
			Justification: "it is the standard and the team knows it.",
			Consequences: "Positive Consequences:\n* Tests run in every IDE\n\n" +
				"Negative Consequences:\n* Parametrised tests need an extension",
		}},
		{"madr", "0003-use-spring-boot.md", ADR{
			Format: MADR, Number: 3, Title: "Use Spring Boot", Status: Superseded,
			SupersededBy:  &Link{Number: 5},
			Date:          "2021-01-10",
			Context:       "Which application framework should the services use?",
			Options:       []string{"Spring Boot", "Micronaut"},
			Decision:      "Spring Boot",
			Justification: "the team has years of experience with it.",
		}},
		{"madr", "0004-evaluate-graalvm.md", ADR{
			// A plain "Status:" line and "Date:" line after the title.
			Format: MADR, Number: 4, Title: "Evaluate GraalVM native images", Status: Proposed,
			Date:          "2023-02-14",
			Context:       "Start-up time matters for our functions. Should we build native images?",
			Options:       []string{"Native images with GraalVM", "Keep the JVM"},
			Decision:      "Native images with GraalVM",
			Justification: "start-up drops from seconds to milliseconds.",
		}},
		{"madr", "0005-use-quarkus.md", ADR{
			Format: MADR, Number: 5, Title: "Use Quarkus", Status: Accepted,
			Date:          "2022-09-01",
			Deciders:      "Alice Example",
			Context:       "Which application framework should the services use, now that start-up time matters?",
			Options:       []string{"Quarkus", "Spring Boot"},
			Decision:      "Quarkus",
			Justification: "it starts fast and fits our native-image plan.",
			Consequences:  "* Good, because start-up is fast\n* Bad, because few engineers know it yet",
		}},
		{"madr", "0006-use-xml-configuration.md", ADR{
			Format: MADR, Number: 6, Title: "Use XML configuration", Status: Rejected,
			Date:          "2020-06-01",
			Context:       "How should the services be configured?",
			Options:       []string{"XML files", "YAML files"},
			Decision:      "YAML files",
			Justification: "XML is too verbose for hand editing.",
		}},
		{"madr", "0007-keep-java-8.md", ADR{
			// No "Chosen option": the outcome text is the decision.
			Format: MADR, Number: 7, Title: "Keep Java 8", Status: Deprecated,
			Date:     "2018-03-03",
			Context:  "Which Java version do we run?",
			Decision: "We stay on Java 8 until the vendor ends support.",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.dir+"/"+tt.name, func(t *testing.T) {
			got, err := Parse(tt.name, readFixture(t, tt.dir, tt.name))
			if err != nil {
				t.Fatal(err)
			}
			want := tt.want
			want.File = tt.name
			want.DateText = want.Date // every fixture date is a plain YYYY-MM-DD
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Parse =\n%+v\nwant\n%+v", got, want)
			}
		})
	}
}

func TestParseErrorsNameWhatIsMissing(t *testing.T) {
	tests := []struct {
		name, text, want string
	}{
		{"no title", "## Status\n\nAccepted\n", "title"},
		{"empty file", "", "title"},
		{"no status", "# 1. T\n\nDate: 2020-01-01\n\n## Context\n\nx\n", "status"},
		{"empty status section", "# 1. T\n\n## Status\n\n## Context\n\nx\n", "status"},
		{"unknown status word", "# 1. T\n\n## Status\n\nMaybe later\n\n## Decision\n\nx\n", `unknown status "Maybe"`},
		{"unknown status in front matter", "---\nstatus: draft\n---\n# T\n", `unknown status "draft"`},
		{"front matter not closed", "---\nstatus: accepted\n# T\n", "front matter"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse("0001-t.md", []byte(tt.text))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Parse error = %v, want one that contains %q", err, tt.want)
			}
		})
	}
}

// The status word is the first word, whatever its case, markup or trailing text.
func TestParseStatusWord(t *testing.T) {
	tests := []struct {
		status string
		want   Status
	}{
		{"Accepted", Accepted},
		{"accepted", Accepted},
		{"ACCEPTED", Accepted},
		{"**Accepted**", Accepted},
		{"Accepted on 2020-01-01", Accepted},
		{"accepted.", Accepted},
		{"- Accepted", Accepted},
		{"* Rejected", Rejected},
		{"+ Proposed", Proposed},
		{"Proposed", Proposed},
		{"Rejected", Rejected},
		{"Deprecated", Deprecated},
		{"Superseded", Superseded},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got, err := Parse("0001-t.md", []byte("# 1. T\n\n## Status\n\n"+tt.status+"\n"))
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != tt.want {
				t.Errorf("status = %q, want %q", got.Status, tt.want)
			}
		})
	}
}

func TestParseSupersededByLinks(t *testing.T) {
	tests := []struct {
		name, status string
		want         *Link
	}{
		{"adr-tools link", "Superseded by [4. Use CockroachDB](0004-use-cockroachdb.md)", &Link{File: "0004-use-cockroachdb.md", Number: 4}},
		{"link in a path", "Superseded by [4. X](../adr/0004-x.md#top)", &Link{File: "0004-x.md", Number: 4}},
		{"madr id", "superseded by ADR-0123", &Link{Number: 123}},
		{"madr id with link", "superseded by [ADR-0007](0007-x.md)", &Link{File: "0007-x.md", Number: 7}},
		{"bare number", "Superseded by 12", &Link{Number: 12}},
		{"after another status line", "Accepted\n\nSuperseded by [9. Z](0009-z.md)", &Link{File: "0009-z.md", Number: 9}},
		{"number in the link text when the file has none", "Superseded by [ADR-0009](notes.md)", &Link{File: "notes.md", Number: 9}},
		{"no target", "Superseded", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse("0001-t.md", []byte("# 1. T\n\n## Status\n\n"+tt.status+"\n"))
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != Superseded {
				t.Errorf("status = %q, want superseded", got.Status)
			}
			if !reflect.DeepEqual(got.SupersededBy, tt.want) {
				t.Errorf("SupersededBy = %+v, want %+v", got.SupersededBy, tt.want)
			}
		})
	}
}

func TestParseDates(t *testing.T) {
	tests := []struct {
		name, line, wantDate, wantText string
	}{
		{"plain", "Date: 2020-01-02", "2020-01-02", "2020-01-02"},
		{"lowercase key", "date: 2020-01-02", "2020-01-02", "2020-01-02"},
		{"bullet", "* Date: 2020-01-02", "2020-01-02", "2020-01-02"},
		{"bold key", "**Date:** 2020-01-02", "2020-01-02", "2020-01-02"},
		{"timestamp", "Date: 2020-01-02T10:00:00Z", "2020-01-02", "2020-01-02T10:00:00Z"},
		{"not a date", "Date: last spring", "", "last spring"},
		{"impossible date", "Date: 2020-02-31", "", "2020-02-31"},
		{"none", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse("0001-t.md", []byte("# 1. T\n\n"+tt.line+"\n\n## Status\n\nAccepted\n"))
			if err != nil {
				t.Fatal(err)
			}
			if got.Date != tt.wantDate || got.DateText != tt.wantText {
				t.Errorf("Date = %q, DateText = %q; want %q, %q", got.Date, got.DateText, tt.wantDate, tt.wantText)
			}
		})
	}
}

func TestParseNumber(t *testing.T) {
	tests := []struct {
		name, file, title string
		want              int
	}{
		{"title prefix wins", "0009-x.md", "# 3. T", 3},
		{"file name", "0012-x.md", "# T", 12},
		{"adr prefix in the file name", "adr-0012-x.md", "# T", 12},
		{"upper case prefix", "ADR_7-x.md", "# T", 7},
		{"no number", "decision.md", "# T", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.file, []byte(tt.title+"\n\nStatus: accepted\n"))
			if err != nil {
				t.Fatal(err)
			}
			if got.Number != tt.want {
				t.Errorf("Number = %d, want %d", got.Number, tt.want)
			}
		})
	}
}

// A byte order mark, as some editors on Windows write it, does not hide the title.
func TestParseIgnoresAByteOrderMarkAndOldMacLineEnds(t *testing.T) {
	for name, text := range map[string]string{
		"bom":     "\xef\xbb\xbf# 1. T\n\nStatus: proposed\n",
		"cr only": "# 1. T\r\rStatus: proposed\r",
	} {
		got, err := Parse("0001-t.md", []byte(text))
		if err != nil || got.Title != "T" || got.Status != Proposed {
			t.Errorf("%s: Parse = %+v, %v; want title T, proposed", name, got, err)
		}
	}
}

func TestParseKeepsHeadingsInsideCodeFences(t *testing.T) {
	text := "# 1. T\n\n## Status\n\nAccepted\n\n## Context\n\nBefore.\n\n```\n## Decision\nnot a heading\n```\n\nAfter.\n\n## Decision\n\nDo it.\n"
	got, err := Parse("0001-t.md", []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	if want := "Before.\n\n```\n## Decision\nnot a heading\n```\n\nAfter."; got.Context != want {
		t.Errorf("Context = %q, want %q", got.Context, want)
	}
	if got.Decision != "Do it." {
		t.Errorf("Decision = %q, want %q", got.Decision, "Do it.")
	}
}

func TestParseOptions(t *testing.T) {
	text := "# T\n\nStatus: proposed\n\n## Considered Options\n\n" +
		"* One\n  * a nested bullet\n* Two\n- Three\n+ Four\n1. Five\n* One\n\nSome prose.\n\n## Decision Outcome\n\nLater.\n"
	got, err := Parse("0001-t.md", []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	// Nested bullets and prose are not options; a repeated label is kept once.
	if want := []string{"One", "Two", "Three", "Four", "Five"}; !reflect.DeepEqual(got.Options, want) {
		t.Errorf("Options = %q, want %q", got.Options, want)
	}
}

func TestIsFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"0001-record-architecture-decisions.md", true},
		{"adr-0001-x.md", true},
		{"ADR-12-x.md", true},
		{"0001.md", true},
		{"README.md", false},
		{"adr-template.md", false},
		{"template.md", false},
		{"index.md", false},
		{"0001-x.txt", false},
		{"0001-x.md.bak", false},
		{".0001-x.md", false},
	}
	for _, tt := range tests {
		if got := IsFile(tt.name); got != tt.want {
			t.Errorf("IsFile(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// A chosen option written as a markdown link, or with a link inside its
// quotes, is the link's text; a comma in the URL does not end the label.
func TestParseChosenOptionLinks(t *testing.T) {
	tests := []struct {
		name, outcome, wantDecision, wantWhy string
	}{
		{"bare link", `Chosen option: [Postgres](https://pg.example/a,b), because it is boring.`, "Postgres", "it is boring."},
		{"link in quotes", `Chosen option: "[Postgres](https://pg.example/a,b)", because it is boring.`, "Postgres", "it is boring."},
		{"link in bold", `Chosen option: **[Postgres](https://pg.example)**, because it is boring.`, "Postgres", "it is boring."},
		{"link with no justification", `Chosen option: [Postgres](https://pg.example)`, "Postgres", ""},
		{"plain label with a comma in the text after it", `Chosen option: Postgres, because a, b and c.`, "Postgres", "a, b and c."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse("0001-t.md", []byte("# T\n\nStatus: proposed\n\n## Decision Outcome\n\n"+tt.outcome+"\n"))
			if err != nil {
				t.Fatal(err)
			}
			if got.Decision != tt.wantDecision || got.Justification != tt.wantWhy {
				t.Errorf("Decision = %q, Justification = %q; want %q, %q", got.Decision, got.Justification, tt.wantDecision, tt.wantWhy)
			}
		})
	}
}
