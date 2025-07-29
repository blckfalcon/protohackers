package challenge0

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/blckfalcon/protohackers/challenges"
)

// Echo Protocol: https://www.rfc-editor.org/rfc/rfc862.html
type Challenge struct{}

// Name returns the name of the challenge
func (c *Challenge) Name() string {
	return "Echo Server"
}

// Solve implements the solution for the challenge
func (c *Challenge) Solve(ctx context.Context) error {
	listener, err := net.Listen("tcp4", ":5001")
	if err != nil {
		return fmt.Errorf("failed to listen on port 5001: %v", err)
	}
	defer listener.Close()

	fmt.Println("Echo server listening on port 5001...")
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
		fmt.Println("\nShutting down echo server...")
	case <-ctx.Done():
		fmt.Println("\nContext cancelled, shutting down echo server...")
	}

	return nil
}

// handleConnection processes a single client connection
func (c *Challenge) handleConnection(conn net.Conn) {
	defer conn.Close()

	clientAddr := conn.RemoteAddr().String()
	fmt.Printf("Client connected: %s\n", clientAddr)

	reader := bufio.NewReader(conn)

	buffer := make([]byte, 4096)

	for {
		n, err := reader.Read(buffer)
		if err != nil {
			if err.Error() != "EOF" {
				log.Printf("Error reading from %s: %v", clientAddr, err)
			}
			fmt.Printf("Client disconnected: %s\n", clientAddr)
			return
		}

		fmt.Printf("Client %s wrote: \n", clientAddr)
		fmt.Printf("%s", buffer[:n])
		fmt.Printf("---------------")

		if _, err := conn.Write(buffer[:n]); err != nil {
			log.Printf("Error writing to %s: %v", clientAddr, err)
			return
		}
	}
}

func init() {
	challenges.Register(0, &Challenge{})
}
