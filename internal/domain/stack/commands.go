package stack

import "fmt"

// TestCommand is the human-readable command SpecForge runs for filter (a
// scenario marker, or "" for the whole suite). It is shown to the agent and
// the developer; the adapter builds the real invocation.
func (p Profile) TestCommand(filter string) string {
	switch p.Runner {
	case RunnerGo:
		if filter == "" {
			return "go test ./..."
		}
		return fmt.Sprintf("go test -run %s ./...", filter)
	case RunnerMaven:
		if filter == "" {
			return "mvn test"
		}
		return fmt.Sprintf("mvn test -Dtest='*%s*'", filter)
	case RunnerGradle:
		if filter == "" {
			return "gradle test"
		}
		return fmt.Sprintf("gradle test --tests '*%s*'", filter)
	case RunnerVitest:
		if filter == "" {
			return "npx vitest run"
		}
		return fmt.Sprintf("npx vitest run -t %s", filter)
	case RunnerJest:
		if filter == "" {
			return "npx jest"
		}
		return fmt.Sprintf("npx jest -t %s", filter)
	case RunnerNPM:
		return "npm test"
	case RunnerPytest:
		if filter == "" {
			return "pytest"
		}
		return fmt.Sprintf("pytest -k %s", filter)
	}
	return ""
}

// MarkerExample shows how a test carrying marker is named in this stack.
func (p Profile) MarkerExample(marker string) string {
	switch p.Kind {
	case Go:
		return fmt.Sprintf("func Test%s_RequestsResetLink(t *testing.T)", marker)
	case Maven, Gradle:
		return fmt.Sprintf("@Test void %s_requestsResetLink()", marker)
	case Node:
		return fmt.Sprintf(`it("%s requests a reset link", () => { … })`, marker)
	case Python:
		return fmt.Sprintf("def test_%s_requests_reset_link():", marker)
	}
	return ""
}
