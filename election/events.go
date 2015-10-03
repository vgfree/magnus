package election

import (
	"time"
)

func (b *election) emitError(events chan<- *Event, err error) bool {
	event := &Event{
		Time:  time.Now().UTC(),
		Name:  b.name,
		Error: err,
	}
	select {
	case <-time.After(eventTimeout):
	case events <- event:
		return true
	}
	return false
}

func (b *election) emitEvent(events chan<- *Event, size int, leaders map[string]string) bool {
	event := &Event{
		Time:    time.Now().UTC(),
		Name:    b.name,
		Size:    size,
		Leaders: leaders,
	}
	select {
	case <-time.After(eventTimeout):
	case events <- event:
		return true
	}
	return false
}
