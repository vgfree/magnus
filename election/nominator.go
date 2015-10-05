package election

import (
	"golang.org/x/net/context"
)

// Nominator determines whether or not a node should participate in an election
// when an open leadership slot is available.
type Nominator interface {
	// Nominate receives the current leadership state and determines if the
	// node should attempt to become a leader. If so a value is returned to
	// store in the node's leader key. Returns an error if the node does not
	// wish to become a leader.
	//
	// Nominate may also perform actions associated with a node becoming a
	// leader. If runtime of Nominate exceeds ElectionTimeout it is canceled by
	// the provided context.
	Nominate(ctx context.Context, name string, size int, leaders map[string]string) (string, error)

	// LeaderEvent is called on changes in leadership.
	LeaderEvent(ctx context.Context, size int, leaders map[string]string) error
}
