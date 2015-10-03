package election

import (
	"time"
)

func (b *election) emitError(events chan<- *Event, err error) {
	event := &Event{
		Time:  time.Now().UTC(),
		Name:  b.name,
		Error: err,
	}
	events <- event
}

func (b *election) emitEvent(events chan<- *Event, size int, leaders map[string]string) {
	event := &Event{
		Time:    time.Now().UTC(),
		Name:    b.name,
		Size:    size,
		Leaders: leaders,
	}
	events <- event
}
