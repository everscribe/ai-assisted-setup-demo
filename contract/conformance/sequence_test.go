package conformance

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Run replays the lifecycle against one app, schema-checking every
// response, and returns them in order.
func Run(t *testing.T, c *Client) []*Response {
	t.Helper()
	steps := Lifecycle()
	out := make([]*Response, 0, len(steps))
	for _, s := range steps {
		resp := validated(t, c, s.Method, s.Path, s.Actor, s.Body)
		out = append(out, resp)
	}
	return out
}

func TestLifecycleRunsCleanAgainstEveryApp(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			responses := Run(t, NewClient(app))
			require.Len(t, responses, len(Lifecycle()))
			for i, r := range responses {
				require.NotEqual(t, 0, r.Status, "step %d (%s) produced no status", i, Lifecycle()[i].Name)
				require.Less(t, r.Status, 500, "step %d (%s) returned %d", i, Lifecycle()[i].Name, r.Status)
			}
		})
	}
}
