// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"slices"
	"strings"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// AskInput is the raw input of adj ask. Options and Tradeoffs are the flag
// values as typed: LABEL:SUMMARY and LABEL:TEXT, split at the first colon.
// Recommend names an option; RecommendText is a recommendation that is not
// one (C167), kept as typed. At most one of them is set.
type AskInput struct {
	Title     string
	Key       string
	Tags      []string
	Context   string
	AskedBy   string
	Options   []string
	Tradeoffs []string
	Recommend string
	Reasoning string

	RecommendText string
}

// splitLabeled splits "LABEL:TEXT" at the first colon, so the text may hold
// colons. The label is trimmed and must not be empty.
func splitLabeled(flagName, v, shape string) (label, text string, err error) {
	label, text, ok := strings.Cut(v, ":")
	if !ok {
		return "", "", usagef("--%s %q: want %s", flagName, v, shape)
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return "", "", usagef("--%s %q: the label is empty", flagName, v)
	}
	return label, strings.TrimSpace(text), nil
}

// build turns the input into a new open record with no ID, or returns a
// *UsageError that says what is wrong. Nothing is written.
func (in AskInput) build(now time.Time) (model.Record, error) {
	if strings.TrimSpace(in.Title) == "" {
		return model.Record{}, usagef("title must not be blank")
	}
	if strings.ContainsAny(in.Title, "\r\n") {
		return model.Record{}, usagef("title must be one line")
	}
	if in.Key != "" && strings.TrimSpace(in.Key) == "" {
		return model.Record{}, usagef("--key must not be blank")
	}
	var opts []model.Option
	index := map[string]int{}
	for _, v := range in.Options {
		label, summary, err := splitLabeled("option", v, "LABEL:SUMMARY")
		if err != nil {
			return model.Record{}, err
		}
		if _, dup := index[label]; dup {
			return model.Record{}, usagef("duplicate option label %q", label)
		}
		index[label] = len(opts)
		opts = append(opts, model.Option{Label: label, Summary: summary})
	}
	seen := map[string]bool{}
	for _, v := range in.Tradeoffs {
		label, text, err := splitLabeled("tradeoffs", v, "LABEL:TEXT")
		if err != nil {
			return model.Record{}, err
		}
		i, ok := index[label]
		if !ok {
			return model.Record{}, usagef("--tradeoffs %q: no option has the label %q", v, label)
		}
		if seen[label] {
			return model.Record{}, usagef("--tradeoffs given twice for option %q", label)
		}
		seen[label] = true
		opts[i].Tradeoffs = text
	}
	var rec *model.Recommendation
	switch {
	case in.Recommend != "" && in.RecommendText != "":
		return model.Record{}, usagef("--recommend and --recommend-text are exclusive")
	case in.RecommendText != "":
		if strings.TrimSpace(in.RecommendText) == "" {
			return model.Record{}, usagef("--recommend-text must not be blank")
		}
		rec = &model.Recommendation{Text: in.RecommendText, Reasoning: in.Reasoning}
	case in.Recommend != "":
		label := strings.TrimSpace(in.Recommend)
		if _, ok := index[label]; !ok {
			return model.Record{}, usagef("--recommend %q is not one of the options", label)
		}
		rec = &model.Recommendation{Label: label, Reasoning: in.Reasoning}
	case in.Reasoning != "":
		return model.Record{}, usagef("--reasoning needs --recommend or --recommend-text")
	}
	var tags []string
	for _, tag := range in.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			return model.Record{}, usagef("tag must not be blank")
		}
		if !slices.Contains(tags, tag) {
			tags = append(tags, tag)
		}
	}
	date := now.Format(time.DateOnly)
	return model.Record{
		Title:          in.Title,
		Status:         model.StatusOpen,
		Key:            in.Key,
		Tags:           tags,
		AskedBy:        in.AskedBy,
		Created:        date,
		Context:        in.Context,
		Options:        opts,
		Recommendation: rec,
		History:        []model.HistoryEntry{{Date: date, Event: model.EventAsk}},
	}, nil
}

// Check reports a *UsageError if the input cannot make a record. Ask runs it
// too; the CLI runs it first so bad arguments are reported before the store
// is looked for.
func (in AskInput) Check() error {
	_, err := in.build(time.Time{})
	return err
}

// Ask creates an open record from in, dated now, in st and returns it with
// its ID. The history holds one entry, ask (C43). Bad input is a *UsageError
// and writes nothing.
func Ask(st *store.Store, in AskInput, now time.Time) (model.Record, error) {
	rec, err := in.build(now)
	if err != nil {
		return model.Record{}, err
	}
	return st.Create(rec)
}
