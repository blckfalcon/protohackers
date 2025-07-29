package challenge1

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/blckfalcon/protohackers/challenges"
)

// Request represents the incoming JSON request
type Request struct {
	Method string `json:"method"`
	Number any    `json:"number"`
}

// Response represents the outgoing JSON response
type Response struct {
	Method string `json:"method"`
	Prime  bool   `json:"prime"`
}

// Challenge implements the Challenge interface
type Challenge struct{}

// Name returns the name of the challenge
func (c *Challenge) Name() string {
	return "Prime Number JSON Server"
}

// Solve implements the solution for the challenge
func (c *Challenge) Solve(ctx context.Context) error {
	listener, err := net.Listen("tcp4", ":5001")
	if err != nil {
		return fmt.Errorf("failed to listen on port 5001: %v", err)
	}
	defer listener.Close()

	fmt.Println("Prime number JSON server listening on port 5001...")
	fmt.Println("Press Ctrl+C to stop the server")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					if opErr, ok := err.(*net.OpError); ok && opErr.Op == "accept" {
						return
					}
					log.Printf("Error accepting connection: %v", err)
					continue
				}

				go c.handleConnection(conn)
			}
		}
	}()

	select {
	case <-shutdownChan:
		fmt.Println("\nShutting down prime number server...")
	case <-ctx.Done():
		fmt.Println("\nContext cancelled, shutting down prime number server...")
	}

	return nil
}

// handleConnection processes a single client connection
func (c *Challenge) handleConnection(conn net.Conn) {
	defer conn.Close()

	clientAddr := conn.RemoteAddr().String()
	fmt.Printf("Client connected: %s\n", clientAddr)

	reader := bufio.NewReader(conn)

	for {
		data, err := reader.ReadBytes('\n')
		if err != nil {
			if err.Error() != "EOF" {
				log.Printf("Error reading from %s: %v", clientAddr, err)
			}
			fmt.Printf("Client disconnected: %s\n", clientAddr)
			return
		}

		if len(data) == 0 || (len(data) == 1 && data[0] == '\n') {
			continue
		}

		fmt.Printf("Client %s sent: %s", clientAddr, data)

		response, malformed := c.processRequest(data)

		if _, err := conn.Write([]byte(response + "\n")); err != nil {
			log.Printf("Error writing to %s: %v", clientAddr, err)
			return
		}

		if malformed {
			fmt.Printf("Malformed request from %s, disconnecting\n", clientAddr)
			return
		}
	}
}

// processRequest processes a JSON request and returns a response
func (c *Challenge) processRequest(data []byte) (string, bool) {
	var req Request
	err := json.Unmarshal(data, &req)
	if err != nil {
		return c.createMalformedResponse(), true
	}

	if req.Method != "isPrime" {
		return c.createMalformedResponse(), true
	}

	var number float64
	switch v := req.Number.(type) {
	case float64:
		number = v
	case float32:
		number = float64(v)
	case int:
		number = float64(v)
	case int64:
		number = float64(v)
	case int32:
		number = float64(v)
	case json.Number:
		var err error
		number, err = v.Float64()
		if err != nil {
			return c.createMalformedResponse(), true
		}
	default:
		return c.createMalformedResponse(), true
	}

	if math.Floor(number) != number {
		return c.createResponse(false), false
	}

	intNum := int(number)
	isPrime := c.isPrime(intNum)

	return c.createResponse(isPrime), false
}

// isPrime checks if a number is prime
func (c *Challenge) isPrime(n int) bool {
	if n <= 1 {
		return false
	}
	if n <= 3 {
		return true
	}
	if n%2 == 0 || n%3 == 0 {
		return false
	}

	i := 5
	w := 2
	for i*i <= n {
		if n%i == 0 {
			return false
		}
		i += w
		w = 6 - w
	}

	return true
}

// createResponse creates a valid JSON response
func (c *Challenge) createResponse(prime bool) string {
	resp := Response{
		Method: "isPrime",
		Prime:  prime,
	}

	data, _ := json.Marshal(resp)

	return string(data)
}

// createMalformedResponse creates a malformed JSON response
func (c *Challenge) createMalformedResponse() string {
	// Create a response that is malformed according to the specification
	// We'll make it malformed by using an invalid method name
	resp := struct {
		Method string `json:"method"`
		Prime  bool   `json:"prime"`
	}{
		Method: "invalid",
		Prime:  false,
	}

	data, _ := json.Marshal(resp)
	return string(data)
}

func init() {
	challenges.Register(1, &Challenge{})
}
