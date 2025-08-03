package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/blckfalcon/protohackers/challenges"
	_ "github.com/blckfalcon/protohackers/challenges/challenge0"
	_ "github.com/blckfalcon/protohackers/challenges/challenge1"
	_ "github.com/blckfalcon/protohackers/challenges/challenge2"
	_ "github.com/blckfalcon/protohackers/challenges/challenge3"
	_ "github.com/blckfalcon/protohackers/challenges/challenge4"
	_ "github.com/blckfalcon/protohackers/challenges/challenge5"
	_ "github.com/blckfalcon/protohackers/challenges/challenge6"
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
	ids := make([]int, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	for _, id := range ids {
		fmt.Printf("%d: %s\n", id, registry[id].Name())
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nSelect a challenge to run (enter number): ")
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Input error: %v\n", err)
		return
	}

	challengeID, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil {
		fmt.Printf("Invalid input\n")
		return
	}

	challenge, exists := challenges.GetByID(challengeID)
	if !exists {
		fmt.Printf("Challenge %d not found\n", challengeID)
		return
	}

	fmt.Printf("\nRunning Challenge %d: %s\n", challengeID, challenge.Name())
	fmt.Println("=============================")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, shutting down...")
		cancel()
	}()

	if err := challenge.Solve(ctx); err != nil && ctx.Err() == nil {
		fmt.Printf("Challenge error: %v\n", err)
	}
}
