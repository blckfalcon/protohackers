package challenge0

import (
	"bufio"
	"context"
	"net"
	"testing"
)

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

	testMessages := []string{
		"Hello, World!\n",
		"This is a test message\n",
		"Another message\n",
	}

	conn, err := net.Dial("tcp", "localhost:5001")
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	for _, msg := range testMessages {
		_, err := conn.Write([]byte(msg))
		if err != nil {
			t.Fatalf("Failed to write to server: %v", err)
		}
		reader := bufio.NewReader(conn)
		response, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("Failed to read from server: %v", err)
		}
		if response != msg {
			t.Errorf("Expected response %q, got %q", msg, response)
		}
	}

	largeMessage := make([]byte, 5000)
	for i := range largeMessage {
		largeMessage[i] = byte(i % 256)
	}

	_, err = conn.Write(largeMessage)
	if err != nil {
		t.Fatalf("Failed to write large message to server: %v", err)
	}

	response := make([]byte, 0)
	buffer := make([]byte, 1024)
	totalRead := 0

	for totalRead < len(largeMessage) {
		n, err := conn.Read(buffer)
		if err != nil {
			t.Fatalf("Failed to read large message response: %v", err)
		}
		response = append(response, buffer[:n]...)
		totalRead += n
	}

	if len(response) != len(largeMessage) {
		t.Errorf("Expected response length %d, got %d", len(largeMessage), len(response))
	}
	for i := range largeMessage {
		if response[i] != largeMessage[i] {
			t.Errorf("Large message mismatch at byte %d: expected %d, got %d", i, largeMessage[i], response[i])
			break
		}
	}
}
