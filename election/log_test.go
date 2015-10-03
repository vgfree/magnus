package election_test

import (
	election "."
)

// Enable debug logging for tests.
func init() {
	election.SetLogger(nil, true)
}
