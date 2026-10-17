// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// The CLI checks its flags first; the command refuses the same input on its
// own, so a caller of Ask cannot make a record with a label and text (C167).
func TestAskInputRecommendText(t *testing.T) {
	now := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)
	rec, err := AskInput{Title: "T", RecommendText: " free text ", Reasoning: "why"}.build(now)
	if err != nil {
		t.Fatal(err)
	}
	if want := (&model.Recommendation{Text: " free text ", Reasoning: "why"}); !reflect.DeepEqual(rec.Recommendation, want) {
		t.Errorf("recommendation = %+v, want %+v (text as typed, no label)", rec.Recommendation, want)
	}

	for _, tt := range []struct {
		name    string
		in      AskInput
		wantErr string
	}{
		{"label and text", AskInput{Title: "T", Options: []string{"a:A"}, Recommend: "a", RecommendText: "x"}, "exclusive"},
		{"blank text", AskInput{Title: "T", RecommendText: " \n"}, "--recommend-text must not be blank"},
		{"reasoning alone", AskInput{Title: "T", Reasoning: "why"}, "--recommend-text"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.Check()
			var ue *UsageError
			if !errors.As(err, &ue) || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Check() = %v, want a *UsageError containing %q", err, tt.wantErr)
			}
		})
	}
}
