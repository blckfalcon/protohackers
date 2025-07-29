package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"

	"github.com/blckfalcon/protohackers/challenges"
	_ "github.com/blckfalcon/protohackers/challenges/challenge0"
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

	if err := challenge.Solve(); err != nil {
		fmt.Printf("Error running challenge: %v\n", err)
	}
}
