package ballot

import (
	"time"
)

func (b *ballot) emitError(events chan<- *Event, err error) bool {
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

func (b *ballot) emitEvent(events chan<- *Event, size int, leaders map[string]string) bool {
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
