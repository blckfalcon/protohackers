package challenge4

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/blckfalcon/protohackers/challenges"
)

const (
	maxPacketSize = 1000
	versionKey    = "version"
	versionValue  = "Ken's Key-Value Store 1.0"
)

type Challenge struct {
	Address string
}

// Name returns the name of the challenge
func (c *Challenge) Name() string {
	return "Key-Value Store over UDP"
}

// Solve implements the solution for the challenge
func (c *Challenge) Solve(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", c.Address)
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address %s: %v", c.Address, err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", c.Address, err)
	}
	defer conn.Close()

	fmt.Printf("Key-Value Store server listening on %s...\n", c.Address)
	fmt.Println("Press Ctrl+C to stop the server")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	kvStore := &KVStore{
		data: make(map[string]string),
		mu:   sync.RWMutex{},
	}
	kvStore.data[versionKey] = versionValue

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				buffer := make([]byte, maxPacketSize)
				n, clientAddr, err := conn.ReadFromUDP(buffer)
				if err != nil {
					if opErr, ok := err.(*net.OpError); ok && opErr.Op == "read" {
						return
					}
					log.Printf("Error reading from UDP: %v", err)
					continue
				}

				go c.handleRequest(conn, clientAddr, buffer[:n], kvStore)
			}
		}
	}()

	select {
	case <-shutdownChan:
		fmt.Println("\nShutting down key-value store server...")
	case <-ctx.Done():
		fmt.Println("\nContext cancelled, shutting down key-value store server...")
	}

	return nil
}

// handleRequest processes a single UDP packet request
func (c *Challenge) handleRequest(conn *net.UDPConn, clientAddr *net.UDPAddr, data []byte, kvStore *KVStore) {
	if len(data) >= maxPacketSize {
		return
	}

	request := string(data)

	if strings.Contains(request, "=") {
		kvStore.Insert(request)
		return
	}

	value, exists := kvStore.Retrieve(request)
	if exists {
		response := fmt.Sprintf("%s=%s", request, value)
		if len(response) < maxPacketSize {
			conn.WriteToUDP([]byte(response), clientAddr)
		}
	}
}

// KVStore is a thread-safe key-value store
type KVStore struct {
	data map[string]string
	mu   sync.RWMutex
}

// Insert stores a key-value pair from a request string
func (kv *KVStore) Insert(request string) {
	index := strings.Index(request, "=")
	if index == -1 {
		return
	}

	key := request[:index]
	value := request[index+1:]

	if key == versionKey {
		return
	}

	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.data[key] = value
}

// Retrieve retrieves a value by key
func (kv *KVStore) Retrieve(key string) (string, bool) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	value, exists := kv.data[key]
	return value, exists
}

func init() {
	challenges.Register(4, &Challenge{Address: ":5001"})
}
