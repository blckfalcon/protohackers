package challenge5

import (
	"testing"
)

func TestReplaceBoguscoinAddresses(t *testing.T) {
	challenge := &Challenge{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Address at start of message",
			input:    "7F1u3wSD5RbOHQmupo9nx4TnhQ is my address",
			expected: "7YWHMfk9JZe0LM0g1ZauHuiSxhI is my address",
		},
		{
			name:     "Address at end of message",
			input:    "My address is 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX",
			expected: "My address is 7YWHMfk9JZe0LM0g1ZauHuiSxhI",
		},
		{
			name:     "Address in middle of message",
			input:    "Send to 7LOrwbDlS8NujgjddyogWgIM93MV5N2VR please",
			expected: "Send to 7YWHMfk9JZe0LM0g1ZauHuiSxhI please",
		},
		{
			name:     "Two addresses",
			input:    "7F1u3wSD5RbOHQmupo9nx4TnhQ and 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX",
			expected: "7YWHMfk9JZe0LM0g1ZauHuiSxhI and 7YWHMfk9JZe0LM0g1ZauHuiSxhI",
		},
		{
			name:     "Multiple addresses",
			input:    "Please pay the ticket price of 15 Boguscoins to one of these addresses: 7YWHMfk9JZe0LM0g1ZauHuiSxhI 7knNDYk0y5ABcYq8hRbip4ThbiRI2S 7YWHMfk9JZe0LM0g1ZauHuiSxhI\n",
			expected: "Please pay the ticket price of 15 Boguscoins to one of these addresses: 7YWHMfk9JZe0LM0g1ZauHuiSxhI 7YWHMfk9JZe0LM0g1ZauHuiSxhI 7YWHMfk9JZe0LM0g1ZauHuiSxhI\n",
		},
		{
			name:     "Address with surrounding spaces",
			input:    "Send to 7adNeSwJkMakpEcln9HEtthSRtxdmEHOT8T now",
			expected: "Send to 7YWHMfk9JZe0LM0g1ZauHuiSxhI now",
		},
		{
			name:     "No valid address",
			input:    "This is not a valid address 123",
			expected: "This is not a valid address 123",
		},
		{
			name:     "Address too short",
			input:    "This is too short 7abc123",
			expected: "This is too short 7abc123",
		},
		{
			name:     "Address too long",
			input:    "This is too long 7abcdefghijklmnopqrstuvwxyz0123456789",
			expected: "This is too long 7abcdefghijklmnopqrstuvwxyz0123456789",
		},
		{
			name:     "Address with non-alphanumeric characters",
			input:    "This has invalid chars 7abc-def!@#",
			expected: "This has invalid chars 7abc-def!@#",
		},
		{
			name:     "Empty message",
			input:    "",
			expected: "",
		},
		{
			name:     "New line at the end of message",
			input:    "Please send the payment of 750 Boguscoins to 7pvKhFstuG82dZ4zlSveZnTeGtVWKwk\n",
			expected: "Please send the payment of 750 Boguscoins to 7YWHMfk9JZe0LM0g1ZauHuiSxhI\n",
		},
		{
			name:     "Product ID is not a Boguscoin",
			input:    "This is a product ID, not a Boguscoin: 7m3qxcswVYpbbQOBAB5QAVC2sr6mKxz7G-cgpDuq8GyxlQzseZwzOOd81Ib1VWayIc5-1234",
			expected: "This is a product ID, not a Boguscoin: 7m3qxcswVYpbbQOBAB5QAVC2sr6mKxz7G-cgpDuq8GyxlQzseZwzOOd81Ib1VWayIc5-1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := challenge.replaceBoguscoinAddresses(tt.input)
			if result != tt.expected {
				t.Errorf("replaceBoguscoinAddresses() = %v, want %v", result, tt.expected)
			}
		})
	}
}
