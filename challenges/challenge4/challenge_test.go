package challenge4

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestChallenge_Solve(t *testing.T) {
	challenge := &Challenge{
		Address: GetAvailableUDPPort(t),
	}

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

	time.Sleep(100 * time.Millisecond)

	serverAddr, err := net.ResolveUDPAddr("udp", challenge.Address)
	if err != nil {
		t.Fatalf("Failed to resolve server address: %v", err)
	}

	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	testRetrieveKey(t, conn, "version", "Ken's Key-Value Store 1.0")

	testInsertAndRetrieve(t, conn, "foo", "bar")
	testInsertAndRetrieve(t, conn, "hello", "world")
	testInsertAndRetrieve(t, conn, "empty", "")

	testInsertAndRetrieve(t, conn, "foo", "newvalue")

	testInsertAndRetrieve(t, conn, "equals", "a=b=c")
	testInsertAndRetrieve(t, conn, "", "emptykey")
	testInsertAndRetrieve(t, conn, "emptyvalue", "")

	testRetrieveNonExistentKey(t, conn, "nonexistent")

	testVersionImmutable(t, conn)

	testPacketSizeLimits(t, conn)
}

func testInsertAndRetrieve(t *testing.T, conn *net.UDPConn, key, value string) {
	t.Helper()

	insertRequest := key + "=" + value
	_, err := conn.Write([]byte(insertRequest))
	if err != nil {
		t.Fatalf("Failed to insert key %q: %v", key, err)
	}

	time.Sleep(10 * time.Millisecond)

	retrieveRequest := key
	_, err = conn.Write([]byte(retrieveRequest))
	if err != nil {
		t.Fatalf("Failed to retrieve key %q: %v", key, err)
	}

	buffer := make([]byte, 1000)
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read response for key %q: %v", key, err)
	}

	response := string(buffer[:n])
	expected := key + "=" + value
	if response != expected {
		t.Errorf("Expected response %q, got %q", expected, response)
	}
}

func testRetrieveKey(t *testing.T, conn *net.UDPConn, key, expectedValue string) {
	t.Helper()

	_, err := conn.Write([]byte(key))
	if err != nil {
		t.Fatalf("Failed to retrieve key %q: %v", key, err)
	}

	buffer := make([]byte, 1000)
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read response for key %q: %v", key, err)
	}

	response := string(buffer[:n])
	expected := key + "=" + expectedValue
	if response != expected {
		t.Errorf("Expected response %q, got %q", expected, response)
	}
}

func testRetrieveNonExistentKey(t *testing.T, conn *net.UDPConn, key string) {
	t.Helper()

	_, err := conn.Write([]byte(key))
	if err != nil {
		t.Fatalf("Failed to retrieve key %q: %v", key, err)
	}

	// Set read deadline to avoid blocking indefinitely
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

	buffer := make([]byte, 1000)
	_, err = conn.Read(buffer)

	// Reset deadline
	conn.SetReadDeadline(time.Time{})

	// For non-existent keys, the server may choose not to respond
	// So we don't fail if we get a timeout, but we do fail if we get an unexpected response
	if err == nil {
		response := string(buffer)
		// The server might return an empty value
		if response != key+"=" {
			t.Errorf("Unexpected response for non-existent key %q: %q", key, response)
		}
	}
}

func testVersionImmutable(t *testing.T, conn *net.UDPConn) {
	t.Helper()

	_, err := conn.Write([]byte("version=modified"))
	if err != nil {
		t.Fatalf("Failed to send version modification request: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	_, err = conn.Write([]byte("version"))
	if err != nil {
		t.Fatalf("Failed to retrieve version key: %v", err)
	}

	buffer := make([]byte, 1000)
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read version response: %v", err)
	}

	response := string(buffer[:n])
	expected := "version=Ken's Key-Value Store 1.0"
	if response != expected {
		t.Errorf("Version key should be immutable. Expected %q, got %q", expected, response)
	}
}

func testPacketSizeLimits(t *testing.T, conn *net.UDPConn) {
	t.Helper()

	// Test with a request that's exactly 1000 bytes (should be rejected)
	largeKey := strings.Repeat("a", 500)
	largeValue := strings.Repeat("b", 499) // 500 + 1 (for '=') + 499 = 1000
	largeRequest := largeKey + "=" + largeValue

	_, err := conn.Write([]byte(largeRequest))
	if err != nil {
		t.Fatalf("Failed to send large request: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	// Try to retrieve the key - it should not have been stored
	_, err = conn.Write([]byte(largeKey))
	if err != nil {
		t.Fatalf("Failed to retrieve large key: %v", err)
	}

	// Set read deadline to avoid blocking indefinitely
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

	buffer := make([]byte, 1000)
	_, err = conn.Read(buffer)

	// Reset deadline
	conn.SetReadDeadline(time.Time{})

	// We expect no response since the large request should have been rejected
	if err == nil {
		response := string(buffer)
		t.Errorf("Unexpected response for oversized request: %q", response)
	}
}

func GetAvailableUDPPort(t *testing.T) string {
	t.Helper()
	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		t.Fatal(err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	port := strconv.Itoa(conn.LocalAddr().(*net.UDPAddr).Port)
	return ":" + port
}
