package tdd

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors that stop the loop. The CLI maps each one to an exit code
// and a remediation box; tests assert them with errors.Is.
var (
	// ErrPrematureGreen: the test passed before any implementation.
	ErrPrematureGreen = errors.New("the test passed before any implementation was written")
	// ErrAttemptsExhausted: the agent could not complete a phase.
	ErrAttemptsExhausted = errors.New("the agent ran out of attempts for this phase")
	// ErrNoTestWritten: the agent changed no test file during RED.
	ErrNoTestWritten = errors.New("the agent did not write a test")
	// ErrUnsupportedStack: SpecForge cannot build or test this project.
	ErrUnsupportedStack = errors.New("the project stack is not supported")
	// ErrStateMismatch: the saved state belongs to another specification.
	ErrStateMismatch = errors.New("the saved loop state belongs to a different specification")
)

// TamperingError reports test files changed outside RED.
type TamperingError struct {
	Phase   Phase
	Changed []string
}

func (e *TamperingError) Error() string {
	return fmt.Sprintf("test files were modified during %s: %s", e.Phase, strings.Join(e.Changed, ", "))
}

// OpenQuestionsError reports a specification that still has open questions.
type OpenQuestionsError struct {
	Questions []string
}

func (e *OpenQuestionsError) Error() string {
	return fmt.Sprintf("the specification has %d open question(s)", len(e.Questions))
}

// AgentBlockedError reports an agent that declared it cannot continue.
type AgentBlockedError struct {
	Phase           Phase
	Reason          string
	SuggestedAction string
}

func (e *AgentBlockedError) Error() string {
	return fmt.Sprintf("the agent is blocked during %s: %s", e.Phase, e.Reason)
}
