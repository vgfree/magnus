package election_test

import (
	"fmt"
	"golang.org/x/net/context"
)

type Nominator struct {
	Nominations  chan *Nomination
	LeaderEvents chan *LeaderEvent
}

func NewNominator() *Nominator {
	return &Nominator{
		Nominations:  make(chan *Nomination, 1),
		LeaderEvents: make(chan *LeaderEvent, 1),
	}
}

func (n *Nominator) Nominate(ctx context.Context, name string, size int, leaders map[string]string) (string, error) {
	nom := &Nomination{
		Name:    name,
		Size:    size,
		Leaders: leaders,
		Result:  make(chan interface{}),
	}

	select {
	case n.Nominations <- nom:
	case <-ctx.Done():
		return "", context.Canceled
	}

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
	event := &LeaderEvent{
		Size:    size,
		Leaders: leaders,
	}
	select {
	case n.LeaderEvents <- event:
	case <-ctx.Done():
		return context.Canceled
	}
	return nil
}

func (n *Nominator) Close() error {
	if n.Nominations != nil {
		close(n.Nominations)
	}
	if n.LeaderEvents != nil {
		close(n.LeaderEvents)
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
