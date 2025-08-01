package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/blckfalcon/protohackers/challenges"
	_ "github.com/blckfalcon/protohackers/challenges/challenge0"
	_ "github.com/blckfalcon/protohackers/challenges/challenge1"
	_ "github.com/blckfalcon/protohackers/challenges/challenge2"
	_ "github.com/blckfalcon/protohackers/challenges/challenge3"
	_ "github.com/blckfalcon/protohackers/challenges/challenge4"
	_ "github.com/blckfalcon/protohackers/challenges/challenge5"
)

func main() {
	fmt.Println("Protohackers Challenge Solver")
	fmt.Println("=============================")

	registry := challenges.GetAll()

	if len(registry) == 0 {
		fmt.Println("No challenges registered.")
		return
	}

	fmt.Println("\nAvailable Challenges:")
	for id, challenge := range registry {
		fmt.Printf("%d: %s\n", id, challenge.Name())
	}

	// Get user selection
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nSelect a challenge to run (enter number): ")
	input, _ := reader.ReadString('\n')
	input = input[:len(input)-1] // Remove newline

	challengeID, err := strconv.Atoi(input)
	if err != nil {
		fmt.Printf("Invalid input: %v\n", err)
		return
	}

	challenge, exists := challenges.GetByID(challengeID)
	if !exists {
		fmt.Printf("Challenge %d not found\n", challengeID)
		return
	}

	fmt.Printf("\nRunning Challenge %d: %s\n", challengeID, challenge.Name())
	fmt.Println("=============================")

	// Create a context that can be cancelled with Ctrl+C
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handler for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, shutting down...")
		cancel()
	}()

	if err := challenge.Solve(ctx); err != nil {
		fmt.Printf("Error running challenge: %v\n", err)
	}
}
