package challenge3

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

// Client represents a connected chat client
type Client struct {
	conn     net.Conn
	name     string
	joined   bool
	messages chan string
}

// ChatRoom manages the chat room state
type ChatRoom struct {
	clients map[*Client]bool
	mu      sync.RWMutex
}

// Challenge implements the Challenge interface
type Challenge struct {
	room *ChatRoom
}

// Name returns the name of the challenge
func (c *Challenge) Name() string {
	return "TCP-based Chat Room"
}

// Solve implements the solution for the challenge
func (c *Challenge) Solve(ctx context.Context) error {
	c.room = &ChatRoom{
		clients: make(map[*Client]bool),
	}

	listener, err := net.Listen("tcp4", ":5001")
	if err != nil {
		return fmt.Errorf("failed to listen on port 5001: %v", err)
	}
	defer listener.Close()

	fmt.Println("Chat room server listening on port 5001...")
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
		fmt.Println("\nShutting down chat room server...")
	case <-ctx.Done():
		fmt.Println("\nContext cancelled, shutting down chat room server...")
	}

	return nil
}

// handleConnection processes a single client connection
func (c *Challenge) handleConnection(conn net.Conn) {
	defer conn.Close()

	clientAddr := conn.RemoteAddr().String()
	fmt.Printf("Client connected: %s\n", clientAddr)

	client := &Client{
		conn:     conn,
		messages: make(chan string, 100),
	}

	if _, err := conn.Write([]byte("Welcome to budgetchat! What shall I call you?\n")); err != nil {
		log.Printf("Error writing to %s: %v", clientAddr, err)
		return
	}

	go c.sendMessages(client)

	// Read messages from client
	reader := bufio.NewReader(conn)
	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			if client.joined {
				c.room.broadcast(fmt.Sprintf("* %s has left the room", client.name), client)
				c.room.removeClient(client)
			}
			fmt.Printf("Client disconnected: %s\n", clientAddr)
			return
		}

		message = strings.TrimRight(message, "\r\n")

		if !client.joined {
			if !c.isValidName(message) {
				conn.Write([]byte("Invalid name. Names must be alphanumeric and at least 1 character long.\n"))
				fmt.Printf("Client %s provided invalid name: %s\n", clientAddr, message)
				return
			}
			client.name = message
			client.joined = true
			c.room.addClient(client)

			userList := c.room.getUserList(client)
			client.messages <- fmt.Sprintf("* The room contains: %s", userList)

			c.room.broadcast(fmt.Sprintf("* %s has entered the room", client.name), client)
		} else {
			// This is a chat message
			c.room.broadcast(fmt.Sprintf("[%s] %s", client.name, message), client)
		}
	}
}

// sendMessages sends messages from the client's message channel to the client
func (c *Challenge) sendMessages(client *Client) {
	for msg := range client.messages {
		if _, err := client.conn.Write([]byte(msg + "\n")); err != nil {
			log.Printf("Error writing to %s: %v", client.name, err)
			return
		}
	}
}

// isValidName checks if a name is valid (alphanumeric, at least 1 char)
func (c *Challenge) isValidName(name string) bool {
	if len(name) == 0 || len(name) > 16 {
		return false
	}

	// Check if name contains only alphanumeric characters
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9]+$`, name)
	return matched
}

// addClient adds a client to the chat room
func (r *ChatRoom) addClient(client *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[client] = true
}

// removeClient removes a client from the chat room
func (r *ChatRoom) removeClient(client *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, client)
	close(client.messages)
}

// broadcast sends a message to all clients except the sender
func (r *ChatRoom) broadcast(message string, sender *Client) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for client := range r.clients {
		if client != sender && client.joined {
			client.messages <- message
		}
	}
}

// getUserList returns a comma-separated list of user names excluding the specified client
func (r *ChatRoom) getUserList(exclude *Client) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var users []string
	for client := range r.clients {
		if client != exclude && client.joined {
			users = append(users, client.name)
		}
	}

	return strings.Join(users, ", ")
}

func init() {
	challenges.Register(3, &Challenge{})
}
