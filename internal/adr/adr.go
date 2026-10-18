// SPDX-License-Identifier: GPL-3.0-or-later

// Package adr parses one architecture decision record (ADR) file, in the
// format Michael Nygard's template gives it (as adr-tools writes it) or in
// MADR (Markdown Architectural Decision Records), into a neutral struct. The
// functions are pure: they read no files and know nothing about adjudicate's
// records; internal/commands maps an ADR to a record.
//
// The parser is lenient about layout and strict about the two parts an import
// cannot do without: the title and a status it knows. YAML front matter is
// read by hand, only for its flat "key: value" lines.
package adr

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Format is the template an ADR follows, as far as its layout tells.
type Format string

const (
	// Nygard: "# N. Title", a Date line, then Status, Context, Decision and
	// Consequences sections (adr-tools).
	Nygard Format = "nygard"
	// MADR: optional YAML front matter or a Status line, the title, then
	// Context and Problem Statement, Considered Options and Decision Outcome.
	// An ADR with front matter, a Considered Options section or a Decision
	// Outcome section is MADR.
	MADR Format = "madr"
)

// Status is the status word of an ADR, lower case.
type Status string

const (
	Accepted   Status = "accepted"
	Proposed   Status = "proposed"
	Rejected   Status = "rejected"
	Deprecated Status = "deprecated"
	Superseded Status = "superseded"
)

// Link names another ADR: by the file name in a markdown link, by number, or
// both. At least one is set.
type Link struct {
	File   string // base name of the linked file, "" if the link names none
	Number int    // the ADR number, 0 if the link names none
}

// ADR is one parsed ADR. A field the ADR does not have is empty.
type ADR struct {
	File   string // the file name given to Parse
	Format Format
	// Number is the ADR's number: the "N." in an adr-tools title, else the
	// leading digits of the file name (after an optional "adr-"). 0 means
	// none; an ADR numbered 0000 counts as unnumbered.
	Number int
	Title  string
	Status Status
	// SupersededBy is the ADR named by "Superseded by ..." in the status; nil
	// if the status is not superseded or names none. "Supersedes ..." lines
	// are not read: the older ADR carries the link.
	SupersededBy *Link
	// Date is the date line as YYYY-MM-DD, "" if there is none or it is not a
	// date; DateText is the line's text as written, so a caller can tell the
	// two apart.
	Date, DateText string
	// Deciders is the "Deciders" or "decision-makers" field, as written.
	Deciders string
	Context  string
	// Options are the top-level bullets of Considered Options, with markdown
	// links reduced to their text and repeated labels kept once.
	Options []string
	// Decision is the Decision section (Nygard), or in MADR the label after
	// "Chosen option", else the Decision Outcome text.
	Decision string
	// Justification is the text after "because" in MADR's "Chosen option"
	// sentence.
	Justification string
	// Consequences is the Consequences section, or in MADR the Consequences
	// subsections of Decision Outcome.
	Consequences string
}

// IsFile reports whether a file name looks like an ADR: it ends in .md and
// starts with a number, with an optional "adr-" or "adr_" before it
// (0007-use-postgres.md, adr-0007-use-postgres.md). README.md, MADR's
// adr-template.md and other notes in an ADR folder are not ADRs.
func IsFile(name string) bool { return fileName.MatchString(name) }

var (
	fileName     = regexp.MustCompile(`(?i)^(?:adr[-_]?)?[0-9]+(?:[-_.].*)?\.md$`)
	fileNumRe    = regexp.MustCompile(`(?i)^(?:adr[-_]?)?([0-9]+)`)
	titleNumRe   = regexp.MustCompile(`^([0-9]+)\.\s+(.*)$`)
	headingRe    = regexp.MustCompile(`^(#{1,6})[ \t]+(.*?)[ \t]*$`)
	commentRe    = regexp.MustCompile(`\s*<!--.*?-->\s*`)
	metaRe       = regexp.MustCompile(`^\s*(?:[*+-]\s+)?(?:\*\*|__)?([A-Za-z][A-Za-z -]*?)\s*(?:\*\*|__)?\s*:\s*(?:\*\*|__)?\s*(.*)$`)
	fmKeyRe      = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_-]*)[ \t]*:[ \t]*(.*)$`)
	dateRe       = regexp.MustCompile(`^([0-9]{4}-[0-9]{2}-[0-9]{2})(?:$|[^0-9])`)
	listMarkerRe = regexp.MustCompile(`^[-+*][ \t]+`)
	bulletRe     = regexp.MustCompile(`^(?:[*+-]|[0-9]+[.)])[ \t]+(.*)$`)
	leadLinkRe   = regexp.MustCompile(`^\[([^\]]*)\]\([^)]*\)`)
	mdLinkRe     = regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)
	idRe         = regexp.MustCompile(`(?i)(?:adr[-_ ]?)?([0-9]+)`)
	mdFileRe     = regexp.MustCompile(`[^\s()<>\[\]]+\.md`)
	chosenRe     = regexp.MustCompile(`(?is)^chosen option\s*:?\s*(.*)$`)
	becauseRe    = regexp.MustCompile(`(?i)\bbecause\b`)
)

// heading is an ATX heading of the body.
type heading struct {
	level int
	name  string // lower case, trimmed, "optional" comments removed
	text  string // as written, comments removed
	line  int    // index in the body lines
}

// section is a heading with the lines under it.
type section struct {
	heading
	all      []string  // up to the next heading of the same or a higher level
	direct   []string  // up to the next heading of any level
	children []section // the headings of a lower level inside all
}

// Parse parses the ADR file name with the given content. Line endings may be
// LF or CRLF, and a byte order mark is ignored. The error is for an ADR with
// no title, no status or a status word it does not know; it names the file.
func Parse(name string, data []byte) (ADR, error) {
	a := ADR{File: name}
	text := strings.TrimPrefix(string(data), "\xef\xbb\xbf")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")

	front := map[string]string{}
	hasFront := false
	if strings.TrimRight(lines[0], " \t") == "---" {
		end := -1
		for i := 1; i < len(lines); i++ {
			if l := strings.TrimRight(lines[i], " \t"); l == "---" || l == "..." {
				end = i
				break
			}
		}
		if end < 0 {
			return ADR{}, fmt.Errorf("%s: front matter is not closed by a --- line", name)
		}
		for _, l := range lines[1:end] {
			if m := fmKeyRe.FindStringSubmatch(l); m != nil {
				front[strings.ToLower(m[1])] = unquote(strings.TrimSpace(m[2]))
			}
		}
		hasFront = true
		lines = lines[end+1:]
	}

	heads := headings(lines)
	titleAt := -1
	for i, h := range heads {
		if h.level == 1 {
			titleAt = i
			break
		}
	}
	if titleAt < 0 {
		return ADR{}, fmt.Errorf("%s: no title: want a line that starts with \"# \"", name)
	}
	title := heads[titleAt].text
	a.Number = FileNumber(name)
	if m := titleNumRe.FindStringSubmatch(title); m != nil {
		title = m[2]
		if n, _ := strconv.Atoi(m[1]); n > 0 {
			a.Number = n
		}
	}
	if strings.TrimSpace(title) == "" {
		return ADR{}, fmt.Errorf("%s: the title is empty", name)
	}
	a.Title = strings.TrimSpace(title)

	// The lines between the title and the next heading hold "Date:" and
	// "Status:" lines.
	meta := map[string]string{}
	end := len(lines)
	if titleAt+1 < len(heads) {
		end = heads[titleAt+1].line
	}
	for _, l := range lines[heads[titleAt].line+1 : end] {
		if m := metaRe.FindStringSubmatch(l); m != nil {
			k := strings.ToLower(strings.TrimSpace(m[1]))
			if _, seen := meta[k]; !seen {
				meta[k] = strings.TrimSpace(m[2])
			}
		}
	}

	secs := sections(lines, heads[titleAt+1:])
	find := func(match func(string) bool) *section {
		for i := range secs {
			if secs[i].level == 2 && match(secs[i].name) {
				return &secs[i]
			}
		}
		return nil
	}
	named := func(names ...string) *section {
		return find(func(n string) bool {
			for _, want := range names {
				if n == want {
					return true
				}
			}
			return false
		})
	}
	outcome := find(func(n string) bool { return strings.HasPrefix(n, "decision outcome") })
	options := named("considered options")
	if hasFront || outcome != nil || options != nil {
		a.Format = MADR
	} else {
		a.Format = Nygard
	}

	// Status: front matter, a "Status:" line, or the Status section.
	var statusLines []string
	switch {
	case front["status"] != "":
		statusLines = []string{front["status"]}
	case meta["status"] != "":
		statusLines = []string{meta["status"]}
	default:
		if s := named("status"); s != nil {
			statusLines = nonBlank(s.all)
		}
	}
	var err error
	if a.Status, a.SupersededBy, err = parseStatus(statusLines); err != nil {
		return ADR{}, fmt.Errorf("%s: %w", name, err)
	}

	a.DateText = firstNonEmpty(front["date"], meta["date"])
	if m := dateRe.FindStringSubmatch(a.DateText); m != nil {
		if _, err := time.Parse(time.DateOnly, m[1]); err == nil {
			a.Date = m[1]
		}
	}
	a.Deciders = firstNonEmpty(front["decision-makers"], front["deciders"], meta["decision-makers"], meta["decision makers"], meta["deciders"])

	if s := find(func(n string) bool { return n == "context" || strings.HasPrefix(n, "context and ") }); s != nil {
		a.Context = text2(s.all)
	}
	if options != nil {
		a.Options = parseOptions(options.direct)
	}

	decision := named("decision")
	switch {
	case outcome != nil:
		a.Decision, a.Justification = chosenOption(text2(outcome.direct))
	case decision != nil:
		a.Decision, a.Justification = chosenOption(text2(decision.all))
	}
	if s := named("consequences"); s != nil {
		a.Consequences = text2(s.all)
	} else if outcome != nil {
		a.Consequences = outcomeConsequences(outcome.children)
	}
	return a, nil
}

// headings returns the ATX headings of lines, skipping those inside fenced
// code blocks.
func headings(lines []string) []heading {
	var out []heading
	fence := ""
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if f := fenceOf(t); f != "" {
			switch {
			case fence == "":
				fence = f
			case f == fence:
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		m := headingRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		txt := strings.TrimSpace(strings.TrimRight(commentRe.ReplaceAllString(m[2], " "), "# \t"))
		out = append(out, heading{
			level: len(m[1]),
			name:  strings.Join(strings.Fields(strings.TrimSuffix(strings.ToLower(txt), ":")), " "),
			text:  txt,
			line:  i,
		})
	}
	return out
}

// fenceOf returns "```" or "~~~" if the trimmed line opens or closes a fenced
// code block, else "".
func fenceOf(t string) string {
	switch {
	case strings.HasPrefix(t, "```"):
		return "```"
	case strings.HasPrefix(t, "~~~"):
		return "~~~"
	}
	return ""
}

// sections returns a section for each heading in heads, which must belong to
// lines and be in order.
func sections(lines []string, heads []heading) []section {
	out := make([]section, len(heads))
	for i, h := range heads {
		direct, all := len(lines), len(lines)
		if i+1 < len(heads) {
			direct = heads[i+1].line
		}
		for j := i + 1; j < len(heads); j++ {
			if heads[j].level <= h.level {
				all = heads[j].line
				break
			}
		}
		out[i] = section{heading: h, direct: lines[h.line+1 : direct], all: lines[h.line+1 : all]}
	}
	for i, h := range heads {
		for j := i + 1; j < len(heads) && heads[j].level > h.level; j++ {
			out[i].children = append(out[i].children, out[j])
		}
	}
	return out
}

// text2 joins lines and trims the blank space around them.
func text2(lines []string) string { return strings.TrimSpace(strings.Join(lines, "\n")) }

func nonBlank(lines []string) []string {
	var out []string
	for _, l := range lines {
		if l = commentRe.ReplaceAllString(l, " "); strings.TrimSpace(l) != "" {
			out = append(out, strings.TrimSpace(l))
		}
	}
	return out
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// unquote removes one pair of matching quotes around s.
func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// FileNumber is the number at the start of a file name, after an optional
// "adr-", 0 if there is none.
func FileNumber(name string) int {
	if m := fileNumRe.FindStringSubmatch(name); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

// statuses maps a status word to its Status.
var statuses = map[string]Status{
	"accepted":   Accepted,
	"proposed":   Proposed,
	"rejected":   Rejected,
	"deprecated": Deprecated,
	"superseded": Superseded,
}

// parseStatus reads the lines of a status. A line "Superseded by ..." makes
// the ADR superseded and names its replacement; otherwise the first line with
// a known first word gives the status. Other lines (adr-tools' "Supersedes
// [...]", "Amended by [...]") are ignored.
func parseStatus(lines []string) (Status, *Link, error) {
	if len(lines) == 0 {
		return "", nil, errors.New("no status: want a Status section or a Status line")
	}
	var (
		status Status
		link   *Link
		unseen string
	)
	for _, l := range lines {
		clean := strings.Trim(listMarkerRe.ReplaceAllString(strings.TrimSpace(l), ""), "*_` \t")
		low := strings.ToLower(clean)
		if _, ok := strings.CutPrefix(low, "superseded by"); ok {
			status = Superseded
			link = parseLink(clean[len("superseded by"):])
			continue
		}
		word := firstWord(clean)
		if s, ok := statuses[strings.ToLower(word)]; ok {
			if status == "" {
				status = s
			}
			continue
		}
		if unseen == "" {
			unseen = word
		}
	}
	if status == "" {
		if unseen == "" {
			return "", nil, errors.New("no status: the status is empty")
		}
		return "", nil, fmt.Errorf("unknown status %q: want accepted, proposed, rejected, deprecated or superseded", unseen)
	}
	return status, link, nil
}

// firstWord is the first run of letters and digits in s.
func firstWord(s string) string {
	s = strings.TrimLeft(s, "*_`[( \t")
	end := 0
	for end < len(s) {
		c := s[end]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' {
			end++
			continue
		}
		break
	}
	return s[:end]
}

// parseLink reads what follows "Superseded by": a markdown link, an ADR
// number or a file name. It returns nil if the text names none.
func parseLink(rest string) *Link {
	rest = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(rest), ":"))
	var l Link
	if m := mdLinkRe.FindStringSubmatch(rest); m != nil {
		target := m[2]
		if i := strings.IndexAny(target, "#?"); i >= 0 {
			target = target[:i]
		}
		target = strings.TrimSpace(target)
		if strings.HasSuffix(strings.ToLower(target), ".md") {
			l.File = path.Base(target)
		}
		if n := leadingID(m[1]); n > 0 {
			l.Number = n
		} else if l.File != "" {
			l.Number = FileNumber(l.File)
		}
	} else {
		if f := mdFileRe.FindString(rest); f != "" {
			l.File = path.Base(f)
		}
		if m := idRe.FindStringSubmatch(rest); m != nil {
			l.Number, _ = strconv.Atoi(m[1])
		}
	}
	if l.File == "" && l.Number == 0 {
		return nil
	}
	return &l
}

// leadingID is the number at the start of link text such as "4. Use X" or
// "ADR-0007", 0 if there is none.
func leadingID(s string) int {
	m := idRe.FindStringSubmatchIndex(strings.TrimSpace(s))
	if m == nil || m[0] != 0 {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(s)[m[2]:m[3]])
	return n
}

// parseOptions returns the top-level bullets of a Considered Options section.
func parseOptions(lines []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, l := range lines {
		m := bulletRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		label := strings.TrimSpace(mdLinkRe.ReplaceAllString(commentRe.ReplaceAllString(m[1], " "), "$1"))
		if label == "" || seen[label] {
			continue
		}
		seen[label] = true
		out = append(out, label)
	}
	return out
}

// chosenOption reads MADR's outcome sentence, `Chosen option: "X", because Y`.
// If text does not start that way the decision is the whole text.
func chosenOption(text string) (decision, justification string) {
	m := chosenRe.FindStringSubmatch(text)
	if m == nil {
		return text, ""
	}
	rest := strings.TrimSpace(m[1])
	label, after := splitQuoted(rest)
	if label == "" {
		return text, ""
	}
	if loc := becauseRe.FindStringIndex(after); loc != nil {
		justification = strings.TrimSpace(strings.TrimLeft(after[loc[1]:], " \t,:"))
	}
	return label, justification
}

// splitQuoted takes the label at the start of s, a markdown link (its text is
// the label), or in quotes, backticks or bold, or else up to the first comma
// or " because", and returns it with the rest of s. Links inside the label are
// reduced to their text.
func splitQuoted(s string) (label, rest string) {
	if m := leadLinkRe.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1]), s[len(m[0]):]
	}
	closers := map[string]string{`"`: `"`, "\u201c": "\u201d", "'": "'", "`": "`", "**": "**", "*": "*"}
	for _, open := range []string{"**", `"`, "\u201c", "'", "`", "*"} {
		if after, ok := strings.CutPrefix(s, open); ok {
			if i := strings.Index(after, closers[open]); i >= 0 {
				return strings.TrimSpace(mdLinkRe.ReplaceAllString(after[:i], "$1")), after[i+len(closers[open]):]
			}
		}
	}
	end := len(s)
	if i := strings.Index(s, ","); i >= 0 {
		end = i
	}
	if loc := becauseRe.FindStringIndex(s); loc != nil && loc[0] < end {
		end = loc[0]
	}
	return strings.TrimSpace(s[:end]), s[end:]
}

// outcomeConsequences joins the Consequences subsections of Decision Outcome:
// a subsection called Consequences gives its text alone, any other (Positive
// Consequences) its text under its heading.
func outcomeConsequences(children []section) string {
	var parts []string
	for _, c := range children {
		if !strings.Contains(c.name, "consequences") {
			continue
		}
		body := text2(c.all)
		if body == "" {
			continue
		}
		if c.name == "consequences" {
			parts = append(parts, body)
		} else {
			parts = append(parts, c.text+":\n"+body)
		}
	}
	return strings.Join(parts, "\n\n")
}
