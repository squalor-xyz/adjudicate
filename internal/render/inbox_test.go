// SPDX-License-Identifier: GPL-3.0-or-later

package render

import (
	"bytes"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

func TestInboxText(t *testing.T) {
	open := func(id int, title, created string) model.Record {
		return model.Record{ID: id, Title: title, Status: model.StatusOpen, Created: created}
	}
	full := open(7, "Pick a storage engine", "2026-03-04")
	full.Key = "storage.engine"
	full.Tags = []string{"infra"}
	full.AskedBy = "claude-opus"
	full.Context = "why we need to choose"
	full.Options = []model.Option{
		{Label: "a", Summary: "Use A", Tradeoffs: "Younger project"},
		{Label: "b", Summary: "Use B"},
		{Label: "c"},
	}
	full.Recommendation = &model.Recommendation{Label: "a", Reasoning: "A is proven"}
	full.History = []model.HistoryEntry{{Date: "2026-03-04", Event: model.EventAsk}}
	full.Body = "Notes.\n"

	multi := open(8, "Multi-line", "2026-03-05")
	multi.Context = "line one\nline two\n\n"
	multi.Options = []model.Option{
		{Label: "a", Summary: "first\nsecond\n", Tradeoffs: "slow\nfragile\n\n"},
		{Label: "b", Summary: "\n"},
	}
	multi.Recommendation = &model.Recommendation{Label: "a", Reasoning: "line one\nline two\n"}

	tests := []struct {
		name string
		recs []model.Record
		want string
	}{
		{name: "no records prints nothing", want: ""},
		{
			name: "ID, title, created, context, options, tradeoffs and recommendation; nothing else",
			recs: []model.Record{full},
			want: `7  Pick a storage engine
  Created: 2026-03-04
  Context: why we need to choose
  Options:
    a: Use A
      tradeoffs: Younger project
    b: Use B
    c
  Recommendation: a
  Reasoning: A is proven
`,
		},
		{
			name: "a record with no options or recommendation",
			recs: []model.Record{func() model.Record {
				r := open(4, "Logging", "2026-03-04")
				r.Context = "\n\n" // only line breaks: no Context line
				return r
			}()},
			want: "4  Logging\n  Created: 2026-03-04\n",
		},
		{
			name: "a recommendation with no reasoning",
			recs: []model.Record{func() model.Record {
				r := open(5, "Theme", "2026-03-04")
				r.Options = []model.Option{{Label: "dark"}}
				r.Recommendation = &model.Recommendation{Label: "dark"}
				return r
			}()},
			want: "5  Theme\n  Created: 2026-03-04\n  Options:\n    dark\n  Recommendation: dark\n",
		},
		{
			name: "records are separated by one blank line, in the order given",
			recs: []model.Record{open(2, "Second", "2026-03-04"), open(1, "First", "2026-03-05")},
			want: "2  Second\n  Created: 2026-03-04\n\n1  First\n  Created: 2026-03-05\n",
		},
		{
			name: "continuation lines are indented under their entry and trailing line breaks are dropped",
			recs: []model.Record{multi},
			want: `8  Multi-line
  Created: 2026-03-05
  Context:
    line one
    line two
  Options:
    a: first
      second
      tradeoffs: slow
        fragile
    b
  Recommendation: a
  Reasoning:
    line one
    line two
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := InboxText(&buf, tt.recs); err != nil {
				t.Fatal(err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("InboxText =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}
