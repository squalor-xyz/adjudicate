// SPDX-License-Identifier: GPL-3.0-or-later

package render

import (
	"bytes"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

func strp(s string) *string { return &s }

// An amend entry shows the old text under its line, each as "was FIELD: text";
// multi-line text goes under the label, indented; an old empty text is shown
// as (empty); trailing line breaks are not shown.
func TestRecordTextShowsTheOldTextOfAnAmend(t *testing.T) {
	r := model.Record{ID: 1, Title: "New", Status: model.StatusOpen, Created: "2026-03-04",
		History: []model.HistoryEntry{
			{Date: "2026-03-04", Event: model.EventAsk},
			{Date: "2026-10-17", Event: model.EventAmend, Note: "title, context, summary a, tradeoffs b, rationale", Was: &model.Was{
				Title:     strp("Old"),
				Context:   strp("one\ntwo\n\n"),
				Rationale: strp("because"),
				Options: []model.OptionWas{
					{Label: "a", Summary: strp("Use A")},
					{Label: "b", Tradeoffs: strp("")},
				},
			}},
			{Date: "2026-10-18", Event: model.EventDecide},
		}}
	var buf bytes.Buffer
	if err := RecordText(&buf, r); err != nil {
		t.Fatal(err)
	}
	want := `ID: 1
Title: New
Status: open
Created: 2026-03-04
History:
  2026-03-04 ask
  2026-10-17 amend: title, context, summary a, tradeoffs b, rationale
    was title: Old
    was context:
      one
      two
    was summary a: Use A
    was tradeoffs b: (empty)
    was rationale: because
  2026-10-18 decide
`
	if got := buf.String(); got != want {
		t.Errorf("RecordText =\n%s\nwant\n%s", got, want)
	}
	if got := *r.History[1].Was.Context; got != "one\ntwo\n\n" {
		t.Errorf("RecordText changed the record: context %q", got)
	}
}
