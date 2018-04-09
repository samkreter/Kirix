package types

type ComputeInstance struct {
	Name  string
	State string
}

const (
	StateComplete   = "Completed"
	StateInProgress = "InProgress"
)
