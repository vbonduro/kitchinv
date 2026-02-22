package vision

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected *DetectedItem
	}{
		{
			name:     "full item",
			line:     "Milk | 2 liters | opened",
			expected: &DetectedItem{Name: "Milk", Quantity: "2 liters", Notes: "opened"},
		},
		{
			name:     "name and quantity only",
			line:     "Eggs | 12 count",
			expected: &DetectedItem{Name: "Eggs", Quantity: "12 count", Notes: ""},
		},
		{
			// Lines without a pipe separator are indistinguishable from preamble;
			// require at least one | for a line to be treated as an item.
			name:     "name only without pipe",
			line:     "Butter",
			expected: nil,
		},
		{
			name:     "empty line",
			line:     "",
			expected: nil,
		},
		{
			name:     "whitespace only",
			line:     "   ",
			expected: nil,
		},
		{
			name:     "header line Here",
			line:     "Here are the items:",
			expected: nil,
		},
		{
			name:     "header line I see",
			line:     "I see the following:",
			expected: nil,
		},
		{
			name:     "header line Based on",
			line:     "Based on the image:",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseLine(tt.line)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseResponse(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected []DetectedItem
	}{
		{
			name: "basic items",
			raw: `Milk | 2 liters | opened
Eggs | 12 count |
Cheese | 1 block | sharp cheddar`,
			expected: []DetectedItem{
				{Name: "Milk", Quantity: "2 liters", Notes: "opened"},
				{Name: "Eggs", Quantity: "12 count", Notes: ""},
				{Name: "Cheese", Quantity: "1 block", Notes: "sharp cheddar"},
			},
		},
		{
			name: "skip header lines",
			raw: `Here are the items I see:
Milk | 1 liter |
Butter | 1 block | `,
			expected: []DetectedItem{
				{Name: "Milk", Quantity: "1 liter", Notes: ""},
				{Name: "Butter", Quantity: "1 block", Notes: ""},
			},
		},
		{
			name: "empty lines",
			raw: `Apple | 6 |

Orange | 4 | `,
			expected: []DetectedItem{
				{Name: "Apple", Quantity: "6", Notes: ""},
				{Name: "Orange", Quantity: "4", Notes: ""},
			},
		},
		{
			name:     "no items with pipes",
			raw:      "Here are the items:",
			expected: []DetectedItem{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseResponse(tt.raw)
			assert.Equal(t, tt.expected, result)
		})
	}
}
