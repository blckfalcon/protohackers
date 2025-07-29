package challenge1

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"testing"
)

func TestProcessRequest(t *testing.T) {
	c := &Challenge{}

	tests := []struct {
		name      string
		request   string
		expected  string
		malformed bool
	}{
		{
			name:      "Valid prime number",
			request:   `{"method":"isPrime","number":7}`,
			expected:  `{"method":"isPrime","prime":true}`,
			malformed: false,
		},
		{
			name:      "Valid non-prime number",
			request:   `{"method":"isPrime","number":8}`,
			expected:  `{"method":"isPrime","prime":false}`,
			malformed: false,
		},
		{
			name:      "Non-integer number",
			request:   `{"method":"isPrime","number":7.5}`,
			expected:  `{"method":"isPrime","prime":false}`,
			malformed: false,
		},
		{
			name:      "Negative number",
			request:   `{"method":"isPrime","number":-5}`,
			expected:  `{"method":"isPrime","prime":false}`,
			malformed: false,
		},
		{
			name:      "Zero",
			request:   `{"method":"isPrime","number":0}`,
			expected:  `{"method":"isPrime","prime":false}`,
			malformed: false,
		},
		{
			name:      "One",
			request:   `{"method":"isPrime","number":1}`,
			expected:  `{"method":"isPrime","prime":false}`,
			malformed: false,
		},
		{
			name:      "Two (smallest prime)",
			request:   `{"method":"isPrime","number":2}`,
			expected:  `{"method":"isPrime","prime":true}`,
			malformed: false,
		},
		{
			name:      "Large prime number",
			request:   `{"method":"isPrime","number":7919}`,
			expected:  `{"method":"isPrime","prime":true}`,
			malformed: false,
		},
		{
			name:      "Invalid method",
			request:   `{"method":"invalid","number":7}`,
			expected:  `{"method":"invalid","prime":false}`,
			malformed: true,
		},
		{
			name:      "Missing method",
			request:   `{"number":7}`,
			expected:  `{"method":"invalid","prime":false}`,
			malformed: true,
		},
		{
			name:      "Missing number",
			request:   `{"method":"isPrime"}`,
			expected:  `{"method":"invalid","prime":false}`,
			malformed: true,
		},
		{
			name:      "Invalid JSON",
			request:   `{"method":"isPrime","number":}`,
			expected:  `{"method":"invalid","prime":false}`,
			malformed: true,
		},
		{
			name:      "Non-number value",
			request:   `{"method":"isPrime","number":"seven"}`,
			expected:  `{"method":"invalid","prime":false}`,
			malformed: true,
		},
		{
			name:      "With extra fields",
			request:   `{"method":"isPrime","number":7,"extra":"field"}`,
			expected:  `{"method":"isPrime","prime":true}`,
			malformed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, malformed := c.processRequest([]byte(tt.request))

			// Parse both responses to compare them structurally
			var respMap, expectedMap map[string]interface{}
			if err := json.Unmarshal([]byte(response), &respMap); err != nil {
				t.Fatalf("Failed to parse response JSON: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.expected), &expectedMap); err != nil {
				t.Fatalf("Failed to parse expected JSON: %v", err)
			}

			if malformed != tt.malformed {
				t.Errorf("Expected malformed=%v, got malformed=%v", tt.malformed, malformed)
			}

			if !malformed && !equalJSONMaps(respMap, expectedMap) {
				t.Errorf("Expected response %s, got %s", tt.expected, response)
			}
		})
	}
}

func TestChallenge_Solve(t *testing.T) {
	challenge := &Challenge{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverReady := make(chan struct{})

	go func() {
		close(serverReady)

		if err := challenge.Solve(ctx); err != nil {
			select {
			case <-ctx.Done():
				// This is expected during shutdown
				return
			default:
				t.Errorf("Server error: %v", err)
			}
		}
	}()
	<-serverReady

	tests := []struct {
		name     string
		requests []string
		expected []string
	}{
		{
			name:     "Valid prime number",
			requests: []string{`{"method":"isPrime","number":7}` + "\n"},
			expected: []string{`{"method":"isPrime","prime":true}` + "\n"},
		},
		{
			name: "Multiple Requests per connection",
			requests: []string{
				`{"method":"isPrime","number":2}` + "\n",
				`{"method":"isPrime","number":7919}` + "\n",
			},
			expected: []string{
				`{"method":"isPrime","prime":true}` + "\n",
				`{"method":"isPrime","prime":true}` + "\n",
			},
		},
		{
			name:     "Malformed request",
			requests: []string{`{"method":"invalid","number":7}` + "\n"},
			expected: []string{`{"method":"invalid","prime":false}` + "\n"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new connection for each test case
			conn, err := net.Dial("tcp", "localhost:5001")
			if err != nil {
				t.Fatalf("Failed to connect to server: %v", err)
			}
			defer conn.Close()

			reader := bufio.NewReader(conn)

			// Send all requests and collect responses
			for i, request := range tt.requests {
				_, err := conn.Write([]byte(request))
				if err != nil {
					t.Fatalf("Failed to write to server: %v", err)
				}

				response, err := reader.ReadString('\n')
				if err != nil {
					t.Fatalf("Failed to read from server: %v", err)
				}

				if response != tt.expected[i] {
					t.Errorf("Expected response %q, got %q", tt.expected[i], response)
				}
			}
		})
	}
}

func TestIsPrime(t *testing.T) {
	c := &Challenge{}

	tests := []struct {
		name     string
		number   int
		expected bool
	}{
		{"Negative", -5, false},
		{"Zero", 0, false},
		{"One", 1, false},
		{"Two", 2, true},
		{"Three", 3, true},
		{"Four", 4, false},
		{"Five", 5, true},
		{"Nine", 9, false},
		{"Eleven", 11, true},
		{"Large prime", 7919, true},
		{"Large non-prime", 7920, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.isPrime(tt.number)
			if result != tt.expected {
				t.Errorf("Expected %d to be prime=%v, got prime=%v", tt.number, tt.expected, result)
			}
		})
	}
}

func equalJSONMaps(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		if bv, ok := b[k]; !ok || v != bv {
			return false
		}
	}

	return true
}
