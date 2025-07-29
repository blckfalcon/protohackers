# Protohackers Challenge Solver

This is a Go project structure for solving programming challenges, where each challenge has its own package and can be selected from the main program.

## Project Structure

```
.
├── main.go                 # Main program with challenge selection
├── go.mod                  # Go module file
├── challenges/             # Directory containing all challenge packages
│   ├── challenge.go        # Challenge interface and registry
│   └── challenge0/         # Example challenge package
│       └── challenge.go    # Challenge implementation
└── README.md               # This file
```

## How to Run

1. Build and run the program:
   ```bash
   go run .
   ```

2. Select a challenge by entering its number when prompted.

## Adding New Challenges

To add a new challenge:

1. Create a new directory under `challenges/` with the challenge name (e.g., `challenges/challenge1/`).

2. Create a `challenge.go` file in that directory with the following structure:

   ```go
   package challenge1

   import (
       "fmt"
       "github.com/blckfalcon/protohackers/challenges"
   )

   // Challenge1 implements the Challenge interface
   type Challenge1 struct{}

   // Name returns the name of the challenge
   func (c *Challenge1) Name() string {
       return "Challenge 1 Name"
   }
   // Solve implements the solution for the challenge
   func (c *Challenge1) Solve() error {
       fmt.Println("Running challenge 1 solution...")
       // Implement your solution here
       return nil
   }

   func init() {
       // Register this challenge when the package is imported
       challenges.Register(1, &Challenge1{})
   }
   ```

3. Import the new challenge package in `main.go`:

   ```go
   import (
       // ... other imports
       _ "github.com/blckfalcon/protohackers/challenges/challenge1"
   )
   ```

4. The challenge will automatically appear in the selection menu when you run the program.

## Challenge Interface

All challenges must implement the `Challenge` interface defined in `challenges/challenge.go`:

```go
type Challenge interface {
    Name() string
    Solve() error
}
```

- `Name()`: Returns the name of the challenge
- `Solve()`: Implements the solution for the challenge

## Challenge Registry

The challenge registry in `challenges/challenge.go` keeps track of all registered challenges. Each challenge package registers itself in its `init()` function using:

```go
challenges.Register(challengeID, &ChallengeImplementation{})
```

The `challengeID` should be a unique integer that identifies the challenge.