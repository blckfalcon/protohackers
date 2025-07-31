package challenge3

import (
	"bufio"
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestChatRoom(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	challenge := &Challenge{}
	go challenge.Solve(ctx)

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	t.Run("ValidName", func(t *testing.T) {
		conn, err := net.Dial("tcp", "localhost:5001")
		if err != nil {
			t.Fatalf("Failed to connect to server: %v", err)
		}
		defer conn.Close()

		// Read welcome message
		reader := bufio.NewReader(conn)
		welcome, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("Failed to read welcome message: %v", err)
		}

		expectedWelcome := "Welcome to budgetchat! What shall I call you?\n"
		if welcome != expectedWelcome {
			t.Errorf("Expected welcome message %q, got %q", expectedWelcome, welcome)
		}

		// Send a valid name
		name := "alice"
		if _, err := conn.Write([]byte(name + "\n")); err != nil {
			t.Fatalf("Failed to send name: %v", err)
		}

		// Read room contents (should be empty)
		roomContents, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("Failed to read room contents: %v", err)
		}

		expectedRoomContents := "* The room contains: \n"
		if roomContents != expectedRoomContents {
			t.Errorf("Expected room contents %q, got %q", expectedRoomContents, roomContents)
		}
	})

	t.Run("InvalidName", func(t *testing.T) {
		conn, err := net.Dial("tcp", "localhost:5001")
		if err != nil {
			t.Fatalf("Failed to connect to server: %v", err)
		}
		defer conn.Close()

		// Read welcome message
		reader := bufio.NewReader(conn)
		_, err = reader.ReadString('\n')
		if err != nil {
			t.Fatalf("Failed to read welcome message: %v", err)
		}

		// Send an invalid name (contains special characters)
		invalidName := "alice@123"
		if _, err := conn.Write([]byte(invalidName + "\n")); err != nil {
			t.Fatalf("Failed to send invalid name: %v", err)
		}

		// Read error message
		errorMsg, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("Failed to read error message: %v", err)
		}

		expectedError := "Invalid name. Names must be alphanumeric and at least 1 character long.\n"
		if errorMsg != expectedError {
			t.Errorf("Expected error message %q, got %q", expectedError, errorMsg)
		}
	})

	t.Run("MultipleClients", func(t *testing.T) {
		var wg sync.WaitGroup

		// First client, bob
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.Dial("tcp", "localhost:5001")
			if err != nil {
				t.Errorf("Failed to connect: %v", err)
				return
			}
			defer conn.Close()

			reader := bufio.NewReader(conn)

			// Welcome message
			_, _ = reader.ReadString('\n')
			// Set name
			_, _ = conn.Write([]byte("bob\n"))
			// List of users connected
			_, _ = reader.ReadString('\n')

			var msg string
			msg, err = reader.ReadString('\n')
			if err != nil {
				t.Errorf("Failed to read message: %v", err)
				return
			}
			if !strings.Contains(msg, "alice has entered the room") {
				t.Errorf("Expected to be notified by alice joining the room")
			}

			msg, err = reader.ReadString('\n')
			if err != nil {
				t.Errorf("Failed to read message: %v", err)
				return
			}
			if !strings.Contains(msg, "Hello from alice") {
				t.Errorf("Expected message from alice, got: %s", msg)
			}
		}()

		time.Sleep(50 * time.Millisecond)

		// Second client, alice
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.Dial("tcp", "localhost:5001")
			if err != nil {
				t.Errorf("Failed to connect: %v", err)
				return
			}
			defer conn.Close()

			reader := bufio.NewReader(conn)

			// Welcome message
			_, _ = reader.ReadString('\n')
			// Set name
			_, _ = conn.Write([]byte("alice\n"))

			// List of users connected
			roomContents, err := reader.ReadString('\n')
			if err != nil {
				t.Errorf("Failed to read room contents: %v", err)
				return
			}
			if !strings.Contains(roomContents, "bob") {
				t.Errorf("Expected room to contain bob, got: %s", roomContents)
			}

			_, _ = conn.Write([]byte("Hello from alice\n"))

			var msg string
			msg, err = reader.ReadString('\n')
			if err != nil {
				t.Errorf("Failed to read join message: %v", err)
				return
			}
			if !strings.Contains(msg, "bob has left the room") {
				t.Errorf("Expected leaving notification from bob, got: %s", msg)
			}
		}()

		wg.Wait()
	})

	t.Run("EmptyName", func(t *testing.T) {
		conn, err := net.Dial("tcp", "localhost:5001")
		if err != nil {
			t.Fatalf("Failed to connect to server: %v", err)
		}
		defer conn.Close()

		// Read welcome message
		reader := bufio.NewReader(conn)
		_, err = reader.ReadString('\n')
		if err != nil {
			t.Fatalf("Failed to read welcome message: %v", err)
		}

		// Send an empty name
		if _, err := conn.Write([]byte("\n")); err != nil {
			t.Fatalf("Failed to send empty name: %v", err)
		}

		// Read error message
		errorMsg, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("Failed to read error message: %v", err)
		}

		expectedError := "Invalid name. Names must be alphanumeric and at least 1 character long.\n"
		if errorMsg != expectedError {
			t.Errorf("Expected error message %q, got %q", expectedError, errorMsg)
		}
	})

	t.Run("NameTooLong", func(t *testing.T) {
		conn, err := net.Dial("tcp", "localhost:5001")
		if err != nil {
			t.Fatalf("Failed to connect to server: %v", err)
		}
		defer conn.Close()

		// Read welcome message
		reader := bufio.NewReader(conn)
		_, err = reader.ReadString('\n')
		if err != nil {
			t.Fatalf("Failed to read welcome message: %v", err)
		}

		// Send a name that's too long
		longName := "thisnameistoolongtobevalid"
		if _, err := conn.Write([]byte(longName + "\n")); err != nil {
			t.Fatalf("Failed to send long name: %v", err)
		}

		// Read error message
		errorMsg, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("Failed to read error message: %v", err)
		}

		expectedError := "Invalid name. Names must be alphanumeric and at least 1 character long.\n"
		if errorMsg != expectedError {
			t.Errorf("Expected error message %q, got %q", expectedError, errorMsg)
		}
	})
}

func TestIsValidName(t *testing.T) {
	challenge := &Challenge{}

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"ValidName", "alice", true},
		{"ValidNameWithNumbers", "alice123", true},
		{"ValidNameUpperCase", "ALICE", true},
		{"ValidNameMixedCase", "Alice123", true},
		{"EmptyName", "", false},
		{"NameWithSpecialChars", "alice@123", false},
		{"NameWithSpaces", "alice smith", false},
		{"NameTooLong", "thisnameistoolongtobevalid", false},
		{"NameExactly16Chars", "1234567890123456", true},
		{"Name17Chars", "12345678901234567", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := challenge.isValidName(test.input)
			if result != test.expected {
				t.Errorf("isValidName(%q) = %v, expected %v", test.input, result, test.expected)
			}
		})
	}
}
