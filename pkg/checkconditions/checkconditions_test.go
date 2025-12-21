package checkconditions

import (
	"testing"
)

func TestConditionMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     ConditionMode
		wantMode ConditionMode
	}{
		{
			name:     "only-old mode",
			mode:     ModeOnlyOld,
			wantMode: ModeOnlyOld,
		},
		{
			name:     "only-new mode",
			mode:     ModeOnlyNew,
			wantMode: ModeOnlyNew,
		},
		{
			name:     "old-compare-new mode",
			mode:     ModeOldCompareNew,
			wantMode: ModeOldCompareNew,
		},
		{
			name:     "new-compare-old mode",
			mode:     ModeNewCompareOld,
			wantMode: ModeNewCompareOld,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mode != tt.wantMode {
				t.Errorf("mode = %v, want %v", tt.mode, tt.wantMode)
			}
		})
	}
}

func TestConditionModeConstants(t *testing.T) {
	// Verify the constant values are as expected
	if ModeOnlyOld != "only-old" {
		t.Errorf("ModeOnlyOld = %q, want %q", ModeOnlyOld, "only-old")
	}
	if ModeOnlyNew != "only-new" {
		t.Errorf("ModeOnlyNew = %q, want %q", ModeOnlyNew, "only-new")
	}
	if ModeOldCompareNew != "old-compare-new" {
		t.Errorf("ModeOldCompareNew = %q, want %q", ModeOldCompareNew, "old-compare-new")
	}
	if ModeNewCompareOld != "new-compare-old" {
		t.Errorf("ModeNewCompareOld = %q, want %q", ModeNewCompareOld, "new-compare-old")
	}
}
