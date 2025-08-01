package challenge5

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"regexp"

	"strings"
	"sync"
	"syscall"

	"github.com/blckfalcon/protohackers/challenges"
)

const (
	upstreamHost = "chat.protohackers.com"
	upstreamPort = "16963"
	tonyAddress  = "7YWHMfk9JZe0LM0g1ZauHuiSxhI"
)

// Challenge implements the Challenge interface
type Challenge struct {
	Address string
}

// Name returns the name of the challenge
func (c *Challenge) Name() string {
	return "Boguscoin Address Proxy"
}

// Solve implements the solution for the challenge
func (c *Challenge) Solve(ctx context.Context) error {
	listener, err := net.Listen("tcp4", c.Address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", c.Address, err)
	}
	defer listener.Close()

	fmt.Printf("Boguscoin proxy server listening on %s...\n", c.Address)
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

				go c.handleConnection(ctx, conn)
			}
		}
	}()

	select {
	case <-shutdownChan:
		fmt.Println("\nShutting down boguscoin proxy server...")
	case <-ctx.Done():
		fmt.Println("\nContext cancelled, shutting down boguscoin proxy server...")
	}

	return nil
}

// handleConnection processes a single client connection
func (c *Challenge) handleConnection(ctx context.Context, clientConn net.Conn) {
	defer clientConn.Close()

	clientAddr := clientConn.RemoteAddr().String()
	fmt.Printf("Client connected: %s\n", clientAddr)

	upstreamConn, err := net.Dial("tcp4", net.JoinHostPort(upstreamHost, upstreamPort))
	if err != nil {
		log.Printf("Failed to connect to upstream server: %v", err)
		return
	}
	defer upstreamConn.Close()

	fmt.Printf("Connected to upstream server for client %s\n", clientAddr)

	var wg sync.WaitGroup
	wg.Add(2)

	connCtx, connCancel := context.WithCancel(ctx)
	defer connCancel()

	go func() {
		defer wg.Done()
		c.forwardMessages(connCtx, clientConn, upstreamConn, clientAddr, "client->upstream")
	}()

	go func() {
		defer wg.Done()
		c.forwardMessages(connCtx, upstreamConn, clientConn, clientAddr, "upstream->client")
	}()

	wg.Wait()
	fmt.Printf("Connection closed for client %s\n", clientAddr)
}

// forwardMessages forwards messages from source to destination, replacing Boguscoin addresses
func (c *Challenge) forwardMessages(ctx context.Context, source, destination net.Conn, clientAddr, direction string) {
	defer func() {
		source.Close()
		destination.Close()
	}()

	reader := bufio.NewReader(source)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			message, err := reader.ReadString('\n')
			if err != nil {
				if err.Error() != "EOF" {
					log.Printf("Error reading from %s (%s): %v", clientAddr, direction, err)
				}
				return
			}

			processedMessage := c.replaceBoguscoinAddresses(message)

			if message != processedMessage {
				fmt.Printf("[%s] Original: %s", direction, message)
				fmt.Printf("[%s] Modified: %s", direction, processedMessage)
			}

			if _, err := destination.Write([]byte(processedMessage)); err != nil {
				log.Printf("Error writing to %s (%s): %v", clientAddr, direction, err)
				return
			}
		}
	}
}

// replaceBoguscoinAddresses finds and replaces all Boguscoin addresses in a message
func (c *Challenge) replaceBoguscoinAddresses(message string) string {
	endsWithNewline := strings.HasSuffix(message, "\n")

	cleanMessage := message
	if endsWithNewline {
		cleanMessage = message[:len(message)-1]
	}

	words := strings.Fields(cleanMessage)

	pattern := regexp.MustCompile(`^7[A-Za-z0-9]{25,34}$`)
	for i, word := range words {
		if pattern.MatchString(word) {
			words[i] = tonyAddress
		}
	}

	result := strings.Join(words, " ")

	if endsWithNewline {
		result += "\n"
	}

	return result
}

func init() {
	challenges.Register(5, &Challenge{Address: ":5001"})
}
