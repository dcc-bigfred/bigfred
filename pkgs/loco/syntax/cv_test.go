package syntax

import (
	"reflect"
	"testing"
)

func TestParseCVString(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  []CVEntry
		separator string
		wantErr   bool
	}{
		{
			name:  "single line separator",
			input: "cv1=17, cv2=5, cv6=7",
			expected: []CVEntry{
				{Number: 1, Value: 17},
				{Number: 2, Value: 5},
				{Number: 6, Value: 7},
			},
			separator: ",",
		},
		{
			name:  "single line separator, with inline comment",
			input: "cv1=17, #cv2=5, cv6=7",
			expected: []CVEntry{
				{Number: 1, Value: 17},
				{Number: 6, Value: 7},
			},
			separator: ",",
		},
		{
			name:  "by small letters",
			input: "cv1=2",
			expected: []CVEntry{
				{Number: 1, Value: 2},
			},
			separator: "",
		},
		{
			name:  "single cv entry",
			input: "CV1=2",
			expected: []CVEntry{
				{Number: 1, Value: 2},
			},
			separator: "",
		},
		{
			name:  "multiple cv entries",
			input: "CV1=2\nCV2=3",
			expected: []CVEntry{
				{Number: 1, Value: 2},
				{Number: 2, Value: 3},
			},
			separator: "",
		},
		{
			name:  "ignore comments",
			input: "CV1=2\n# this is a comment\nCV2=3",
			expected: []CVEntry{
				{Number: 1, Value: 2},
				{Number: 2, Value: 3},
			},
			separator: "",
		},
		{
			name:  "ignore empty lines",
			input: "CV1=2\n\nCV2=3\n\n",
			expected: []CVEntry{
				{Number: 1, Value: 2},
				{Number: 2, Value: 3},
			},
			separator: "",
		},
		{
			name:  "ignore inline comments",
			input: "CV1=2 # comment\nCV2=3",
			expected: []CVEntry{
				{Number: 1, Value: 2},
				{Number: 2, Value: 3},
			},
			separator: "",
		},
		{
			name:  "handle whitespace",
			input: "  CV1 = 2  \n  CV2 = 3  ",
			expected: []CVEntry{
				{Number: 1, Value: 2},
				{Number: 2, Value: 3},
			},
			separator: "",
		},
		{
			name:  "handle duplicate cv numbers - last value wins",
			input: "CV1=2\nCV1=3",
			expected: []CVEntry{
				{Number: 1, Value: 3},
			},
			separator: "",
		},
		{
			name:  "cv without value",
			input: "CV1",
			expected: []CVEntry{
				{Number: 1, Value: 0},
			},
			separator: "",
		},
		{
			name:  "mixed cv entries with and without values",
			input: "CV1=2\nCV2\nCV3=4",
			expected: []CVEntry{
				{Number: 1, Value: 2},
				{Number: 2, Value: 0},
				{Number: 3, Value: 4},
			},
			separator: "",
		},
		{
			name:  "cv without value followed by cv with value - last wins",
			input: "CV1\nCV1=3",
			expected: []CVEntry{
				{Number: 1, Value: 3},
			},
			separator: "",
		},
		{
			name:  "cv with value followed by cv without value - last wins",
			input: "CV1=3\nCV1",
			expected: []CVEntry{
				{Number: 1, Value: 0},
			},
			separator: "",
		},
		{
			name:  "commented out cv line",
			input: "#CV1=2\nCV2=3",
			expected: []CVEntry{
				{Number: 2, Value: 3},
			},
			separator: "",
		},
		{
			name:  "cv range without value",
			input: "cv1-cv5",
			expected: []CVEntry{
				{Number: 1, Value: 0},
				{Number: 2, Value: 0},
				{Number: 3, Value: 0},
				{Number: 4, Value: 0},
				{Number: 5, Value: 0},
			},
			separator: "",
		},
		{
			name:  "cv range with value",
			input: "cv1-cv3=7",
			expected: []CVEntry{
				{Number: 1, Value: 7},
				{Number: 2, Value: 7},
				{Number: 3, Value: 7},
			},
			separator: "",
		},
		{
			name:  "cv range mixed with single",
			input: "cv1-cv2=5\ncv3=9",
			expected: []CVEntry{
				{Number: 1, Value: 5},
				{Number: 2, Value: 5},
				{Number: 3, Value: 9},
			},
			separator: "",
		},
		{
			name:  "cv range with separator",
			input: "cv1-cv3=2,cv4=8",
			expected: []CVEntry{
				{Number: 1, Value: 2},
				{Number: 2, Value: 2},
				{Number: 3, Value: 2},
				{Number: 4, Value: 8},
			},
			separator: ",",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseCVString(tt.input, tt.separator)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCVString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ParseCVString() = %v, want %v", result, tt.expected)
			}
		})
	}
}
