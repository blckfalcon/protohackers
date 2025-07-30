package challenge2

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestProcessMessage(t *testing.T) {
	c := &Challenge{}
	session := &Session{
		data: make([]PriceData, 0),
	}

	// Test insert message
	insertMsg := make([]byte, 9)
	insertMsg[0] = 'I'
	binary.BigEndian.PutUint32(insertMsg[1:5], 12345)
	binary.BigEndian.PutUint32(insertMsg[5:9], 101)

	response, err := c.processMessage(insertMsg, session)
	if err != nil {
		t.Errorf("processMessage(insert) returned error: %v", err)
	}
	if response != nil {
		t.Errorf("processMessage(insert) should return nil response, got %v", response)
	}

	// Check that data was inserted
	session.mu.RLock()
	if len(session.data) != 1 {
		t.Errorf("Expected 1 data point, got %d", len(session.data))
	}
	if session.data[0].Timestamp != 12345 {
		t.Errorf("Expected timestamp 12345, got %d", session.data[0].Timestamp)
	}
	if session.data[0].Price != 101 {
		t.Errorf("Expected price 101, got %d", session.data[0].Price)
	}
	session.mu.RUnlock()

	// Test query message
	queryMsg := make([]byte, 9)
	queryMsg[0] = 'Q'
	binary.BigEndian.PutUint32(queryMsg[1:5], 12000)
	binary.BigEndian.PutUint32(queryMsg[5:9], 13000)

	response, err = c.processMessage(queryMsg, session)
	if err != nil {
		t.Errorf("processMessage(query) returned error: %v", err)
	}
	if response == nil {
		t.Error("processMessage(query) should return a response")
	} else if len(response) != 4 {
		t.Errorf("processMessage(query) should return 4 bytes, got %d", len(response))
	} else {
		mean := binary.BigEndian.Uint32(response)
		if mean != 101 {
			t.Errorf("Expected mean 101, got %d", mean)
		}
	}

	// Test query with no data in range
	queryMsg2 := make([]byte, 9)
	queryMsg2[0] = 'Q'
	binary.BigEndian.PutUint32(queryMsg2[1:5], 1000)
	binary.BigEndian.PutUint32(queryMsg2[5:9], 2000)

	response, err = c.processMessage(queryMsg2, session)
	if err != nil {
		t.Errorf("processMessage(query2) returned error: %v", err)
	}
	if response == nil {
		t.Error("processMessage(query2) should return a response")
	} else if len(response) != 4 {
		t.Errorf("processMessage(query2) should return 4 bytes, got %d", len(response))
	} else {
		mean := binary.BigEndian.Uint32(response)
		if mean != 0 {
			t.Errorf("Expected mean 0 for no data, got %d", mean)
		}
	}

	// Test invalid message type
	invalidMsg := make([]byte, 9)
	invalidMsg[0] = 'X'
	binary.BigEndian.PutUint32(invalidMsg[1:5], 12345)
	binary.BigEndian.PutUint32(invalidMsg[5:9], 101)

	_, err = c.processMessage(invalidMsg, session)
	if err == nil {
		t.Error("processMessage(invalid) should return an error")
	}
}

func TestCalculateMean(t *testing.T) {
	c := &Challenge{}
	session := &Session{
		data: []PriceData{
			{Timestamp: 1000, Price: 100},
			{Timestamp: 2000, Price: 200},
			{Timestamp: 3000, Price: 300},
			{Timestamp: 4000, Price: 400},
		},
	}

	tests := []struct {
		name     string
		mintime  int32
		maxtime  int32
		expected int32
	}{
		{"Single value", 1000, 1000, 100},
		{"Two values", 1000, 2000, 150},
		{"All values", 1000, 4000, 250},
		{"No values", 5000, 6000, 0},
		{"Invalid range", 3000, 2000, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.calculateMean(session, tt.mintime, tt.maxtime)
			if result != tt.expected {
				t.Errorf("calculateMean(%d, %d) = %d, expected %d", tt.mintime, tt.maxtime, result, tt.expected)
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
		// Use a different port to avoid conflicts
		// We need to modify the Solve method to accept a port parameter for testing
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

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Test the example from the specification
	conn, err := net.Dial("tcp", "localhost:5001")
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Insert (timestamp,price): (12345,101)
	insert1 := make([]byte, 9)
	insert1[0] = 'I'
	binary.BigEndian.PutUint32(insert1[1:5], 12345)
	binary.BigEndian.PutUint32(insert1[5:9], 101)
	_, err = conn.Write(insert1)
	if err != nil {
		t.Fatalf("Failed to write insert1: %v", err)
	}

	// Insert (timestamp,price): (12346,102)
	insert2 := make([]byte, 9)
	insert2[0] = 'I'
	binary.BigEndian.PutUint32(insert2[1:5], 12346)
	binary.BigEndian.PutUint32(insert2[5:9], 102)
	_, err = conn.Write(insert2)
	if err != nil {
		t.Fatalf("Failed to write insert2: %v", err)
	}

	// Insert (timestamp,price): (12347,100)
	insert3 := make([]byte, 9)
	insert3[0] = 'I'
	binary.BigEndian.PutUint32(insert3[1:5], 12347)
	binary.BigEndian.PutUint32(insert3[5:9], 100)
	_, err = conn.Write(insert3)
	if err != nil {
		t.Fatalf("Failed to write insert3: %v", err)
	}

	// Insert (timestamp,price): (40960,5)
	insert4 := make([]byte, 9)
	insert4[0] = 'I'
	binary.BigEndian.PutUint32(insert4[1:5], 40960)
	binary.BigEndian.PutUint32(insert4[5:9], 5)
	_, err = conn.Write(insert4)
	if err != nil {
		t.Fatalf("Failed to write insert4: %v", err)
	}

	// Query between T=12288 and T=16384
	query := make([]byte, 9)
	query[0] = 'Q'
	binary.BigEndian.PutUint32(query[1:5], 12288)
	binary.BigEndian.PutUint32(query[5:9], 16384)
	_, err = conn.Write(query)
	if err != nil {
		t.Fatalf("Failed to write query: %v", err)
	}

	// Read response
	response := make([]byte, 4)
	_, err = conn.Read(response)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	mean := binary.BigEndian.Uint32(response)
	if mean != 101 {
		t.Errorf("Expected mean 101, got %d", mean)
	}
}
