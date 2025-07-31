package challenge2

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/blckfalcon/protohackers/challenges"
)

// PriceData represents a timestamped price
type PriceData struct {
	Timestamp int32
	Price     int32
}

// Session represents a client session with its own price data
type Session struct {
	data []PriceData
	mu   sync.RWMutex
}

// Challenge implements the Challenge interface
type Challenge struct {
	Address string
}

// Name returns the name of the challenge
func (c *Challenge) Name() string {
	return "Price Tracking Server"
}

// Solve implements the solution for the challenge
func (c *Challenge) Solve(ctx context.Context) error {
	listener, err := net.Listen("tcp4", c.Address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", c.Address, err)
	}
	defer listener.Close()

	fmt.Printf("Echo server listening on %s...\n", c.Address)
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
		fmt.Println("\nShutting down price tracking server...")
	case <-ctx.Done():
		fmt.Println("\nContext cancelled, shutting down price tracking server...")
	}

	return nil
}

// handleConnection processes a single client connection
func (c *Challenge) handleConnection(conn net.Conn) {
	defer conn.Close()

	clientAddr := conn.RemoteAddr().String()
	fmt.Printf("Client connected: %s\n", clientAddr)

	// Create a new session for this connection
	session := &Session{
		data: make([]PriceData, 0),
	}

	buffer := make([]byte, 9)

	for {
		_, err := io.ReadFull(conn, buffer)
		if err != nil {
			if err.Error() != "EOF" {
				log.Printf("Error reading from %s: %v", clientAddr, err)
			}
			fmt.Printf("Client disconnected: %s\n", clientAddr)
			return
		}

		response, err := c.processMessage(buffer, session)
		if err != nil {
			log.Printf("Error processing message from %s: %v", clientAddr, err)
			return
		}

		if response != nil {
			if _, err := conn.Write(response); err != nil {
				log.Printf("Error writing to %s: %v", clientAddr, err)
				return
			}
		}
	}

}

// processMessage processes a binary message and returns a response if needed
func (c *Challenge) processMessage(buffer []byte, session *Session) ([]byte, error) {
	msgType := buffer[0]

	switch msgType {
	case 'I': // Insert message
		return c.processInsert(buffer, session)
	case 'Q': // Query message
		return c.processQuery(buffer, session)
	default:
		return nil, fmt.Errorf("invalid message type: %c", msgType)
	}
}

// processInsert handles insert messages
func (c *Challenge) processInsert(buffer []byte, session *Session) ([]byte, error) {
	// Bytes 1-4: timestamp (big endian int32)
	timestamp := int32(binary.BigEndian.Uint32(buffer[1:5]))

	// Bytes 5-8: price (big endian int32)
	price := int32(binary.BigEndian.Uint32(buffer[5:9]))

	session.mu.Lock()
	session.data = append(session.data, PriceData{
		Timestamp: timestamp,
		Price:     price,
	})
	session.mu.Unlock()

	fmt.Printf("Inserted price %d at timestamp %d\n", price, timestamp)

	return nil, nil
}

// processQuery handles query messages
func (c *Challenge) processQuery(buffer []byte, session *Session) ([]byte, error) {
	// Bytes 1-4: mintime (big endian int32)
	mintime := int32(binary.BigEndian.Uint32(buffer[1:5]))

	// Bytes 5-8: maxtime (big endian int32)
	maxtime := int32(binary.BigEndian.Uint32(buffer[5:9]))

	fmt.Printf("Query for time range [%d, %d]\n", mintime, maxtime)

	mean := c.calculateMean(session, mintime, maxtime)

	response := make([]byte, 4)
	binary.BigEndian.PutUint32(response, uint32(mean))

	fmt.Printf("Returning mean price: %d\n", mean)

	return response, nil
}

// calculateMean calculates the mean price in the given time range
func (c *Challenge) calculateMean(session *Session, mintime, maxtime int32) int32 {
	if mintime > maxtime {
		return 0
	}

	session.mu.RLock()
	defer session.mu.RUnlock()

	var sum int64 = 0
	var count int64 = 0

	for _, data := range session.data {
		if data.Timestamp >= mintime && data.Timestamp <= maxtime {
			sum += int64(data.Price)
			count++
		}
	}

	if count == 0 {
		return 0
	}

	// Calculate mean and round (we'll use truncation toward zero)
	mean := sum / count
	return int32(mean)
}

func init() {
	challenges.Register(2, &Challenge{Address: ":5001"})
}
