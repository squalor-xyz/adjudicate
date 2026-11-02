// SPDX-License-Identifier: GPL-3.0-or-later

package contract

import (
	"errors"
	"strings"
	"testing"
)

func TestStringNamesTheContractAndItsVersion(t *testing.T) {
	if got, want := String(), "adj-json v1"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if Version != 1 {
		t.Errorf("Version = %d: this test names v1, so a bump must update it with the fixture of shapes (internal/cli/contract_shapes_test.go)", Version)
	}
}

func TestCheckRefusesOnlyAPinThatDiffers(t *testing.T) {
	two, one, zero := 2, Version, 0
	tests := []struct {
		name    string
		pin     *int
		wantErr bool
	}{
		{"no pin", nil, false},
		{"pin equal to the running version", &one, false},
		{"pin to a later version", &two, true},
		{"pin to a non-positive version is not this version either", &zero, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Check(tt.pin)
			if tt.wantErr != (err != nil) {
				t.Fatalf("Check = %v, want error: %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrMismatch) {
				t.Errorf("error %v does not match ErrMismatch", err)
			}
		})
	}
}

func TestCheckMessageNamesBothVersionsAndTheWayOut(t *testing.T) {
	two := 2
	err := Check(&two)
	if err == nil {
		t.Fatal("Check(2) = nil")
	}
	msg := err.Error()
	for _, want := range []string{"adj-json v2", "adj-json v1", "ask the owner to run `adj contract pin 1`", "or to install an adj that speaks adj-json v2"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q does not contain %q", msg, want)
		}
	}
}
