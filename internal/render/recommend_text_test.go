// SPDX-License-Identifier: GPL-3.0-or-later

package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// A free-text recommendation (C167) shows where the label would: the same
// "Recommendation:" line carries the text, in adj show and adj inbox, followed
// by the reasoning.
func TestRecommendationTextShowsInShowAndInbox(t *testing.T) {
	open := func(text, reasoning string) model.Record {
		return model.Record{ID: 4, Title: "Logging", Status: model.StatusOpen, Created: "2026-03-04",
			Recommendation: &model.Recommendation{Text: text, Reasoning: reasoning}}
	}
	tests := []struct {
		name      string
		rec       model.Record
		wantShow  string // the lines after Created
		wantInbox string // the lines after Created
	}{
		{"text and reasoning", open("Log to stderr", "agents parse it"),
			"Recommendation: Log to stderr\nReasoning: agents parse it\n",
			"  Recommendation: Log to stderr\n  Reasoning: agents parse it\n"},
		{"text only", open("Log to stderr", ""),
			"Recommendation: Log to stderr\n",
			"  Recommendation: Log to stderr\n"},
		{"text with line breaks", open("Log to stderr\nas JSON lines\n\n", ""),
			"Recommendation:\n  Log to stderr\n  as JSON lines\n",
			"  Recommendation:\n    Log to stderr\n    as JSON lines\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var show, inbox bytes.Buffer
			if err := RecordText(&show, tt.rec); err != nil {
				t.Fatal(err)
			}
			if want := "ID: 4\nTitle: Logging\nStatus: open\nCreated: 2026-03-04\n" + tt.wantShow; show.String() != want {
				t.Errorf("RecordText =\n%s\nwant\n%s", show.String(), want)
			}
			if err := InboxText(&inbox, []model.Record{tt.rec}); err != nil {
				t.Fatal(err)
			}
			if want := "4  Logging\n  Created: 2026-03-04\n" + tt.wantInbox; inbox.String() != want {
				t.Errorf("InboxText =\n%s\nwant\n%s", inbox.String(), want)
			}
		})
	}
}

func TestRecommendationTextJSONHasTextAndNoLabel(t *testing.T) {
	r := model.Record{ID: 4, Title: "Logging", Status: model.StatusOpen, Created: "2026-03-04",
		Recommendation: &model.Recommendation{Text: "Log to stderr", Reasoning: "why"}}
	var buf bytes.Buffer
	if err := RecordJSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	want := "  \"recommendation\": {\n    \"text\": \"Log to stderr\",\n    \"reasoning\": \"why\"\n  }"
	if got := buf.String(); !strings.Contains(got, want) || strings.Contains(got, `"label"`) {
		t.Errorf("RecordJSON =\n%s\nwant it to contain\n%s\nand no label key", got, want)
	}
}
