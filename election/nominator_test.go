package election_test

import (
	"fmt"
	"golang.org/x/net/context"
	"testing"
)

type Nominator struct {
	Name         string
	Nominations  chan *Nomination
	LeaderEvents chan *LeaderEvent
	t            *testing.T
}

func (n *Nominator) Nominate(ctx context.Context, name string, size int, leaders map[string]string) (string, error) {
	n.t.Logf("%s nominate name=%s size=%d leaders=%+v", n.Name, name, size, leaders)
	nom := &Nomination{
		Name:    name,
		Size:    size,
		Leaders: leaders,
		Result:  make(chan interface{}),
	}
	n.Nominations <- nom

	select {
	case result := <-nom.Result:
		switch result := result.(type) {
		case string:
			return result, nil
		case error:
			return "", result
		default:
			return "", fmt.Errorf("invalid result %+v", result)
		}
	case <-ctx.Done():
		return "", context.Canceled
	}
}

func (n *Nominator) LeaderEvent(ctx context.Context, size int, leaders map[string]string) error {
	n.t.Logf("%s event size=%d leaders=%+v", n.Name, size, leaders)
	n.LeaderEvents <- &LeaderEvent{
		Size:    size,
		Leaders: leaders,
	}
	return nil
}

func (n *Nominator) Close() error {
	if n.Nominations != nil {
		close(n.Nominations)
		n.t.Logf("%s closed nominations", n.Name)
	}
	if n.LeaderEvents != nil {
		close(n.LeaderEvents)
		n.t.Logf("%s closed events", n.Name)
	}
	return nil
}

type Nomination struct {
	Name    string
	Size    int
	Leaders map[string]string
	Result  chan interface{}
}

type LeaderEvent struct {
	Size    int
	Leaders map[string]string
}
