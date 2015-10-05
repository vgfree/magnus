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

func CopyStringMap(m map[string]string) map[string]string {
	cp := make(map[string]string, len(m))
	for key, value := range m {
		cp[key] = value
	}
	return cp
}

func (n *Nominator) Nominate(ctx context.Context, name string, size int, leaders map[string]string) (string, error) {
	leadersCopy := CopyStringMap(leaders)
	n.t.Logf("%s nominate name=%s size=%d leaders=%+v", n.Name, name, size, leadersCopy)
	nom := &Nomination{
		Name:    name,
		Size:    size,
		Leaders: leadersCopy,
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
	leadersCopy := CopyStringMap(leaders)
	n.t.Logf("%s event size=%d leaders=%+v", n.Name, size, leadersCopy)
	n.LeaderEvents <- &LeaderEvent{
		Size:    size,
		Leaders: leadersCopy,
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
