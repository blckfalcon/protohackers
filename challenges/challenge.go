package challenges

// Challenge defines the interface that all challenges must implement
type Challenge interface {
	Name() string
	Solve() error
}

// Registry holds all registered challenges
var Registry = make(map[int]Challenge)

// Register adds a challenge to the registry
func Register(id int, challenge Challenge) {
	Registry[id] = challenge
}

// GetByID returns a challenge by ID
func GetByID(id int) (Challenge, bool) {
	challenge, exists := Registry[id]
	return challenge, exists
}

// GetAll returns all registered challenges
func GetAll() map[int]Challenge {
	return Registry
}
