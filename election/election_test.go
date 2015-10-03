package election_test

import (
	election "."
	"reflect"
	"sync"
	"testing"
	"time"
)

var (
	endpoints []string = []string{"http://127.0.0.1:2379"}
	key       string   = "/tests/election"
)

func assertEventLeaders(t *testing.T, event *election.Event, size int, leaders map[string]string) {
	if event == nil {
		t.Error("event is nil")
	} else if event.Size != size {
		t.Errorf("event size %d != %d", event.Size, size)
	} else if !reflect.DeepEqual(event.Leaders, leaders) {
		t.Errorf("event leaders %+v != %+v", event.Leaders, leaders)
	}
}

func TestOneElection(t *testing.T) {
	name := "node1"
	value := "127.0.0.1"
	e, err := election.New(endpoints, key, name, value, nil)
	if err != nil {
		t.Fatal(err)
	}

	eventChan := e.Run()
	go func() {
		time.Sleep(5 * time.Second)
		b.Resign()
	}()

	events := make([]*election.Event, 0, 2)
	for event := range eventChan {
		t.Logf("event %+v", event)
		if event.Error == nil {
			events = append(events, event)
		} else {
			t.Fatal(event.Error)
		}
	}

	if len(events) != 2 {
		t.Errorf("received wrong number of events %d != 2", len(events))
	} else {
		assertEventLeaders(t, events[0], 1, map[string]string{name: value})
		assertEventLeaders(t, events[1], 1, map[string]string{})
	}
}

func TestSecondElection(t *testing.T) {
	node1 := "node1"
	value1 := "node1.example.com"
	node2 := "node2"
	value2 := "node2.example.com"

	e1, err := election.New(endpoints, key, node1, value1, nil)
	if err != nil {
		t.Fatal(err)
	}
	e2, err := election.New(endpoints, key, node2, value2, nil)
	if err != nil {
		t.Fatal(err)
	}

	wg := &sync.WaitGroup{}
	defer wg.Wait()

	// run first election in the background
	ready := make(chan *election.Event)
	wg.Add(1)
	go func() {
		defer wg.Done()
		sentReady := false
		events := e1.Run()
		for event := range events {
			if !sentReady {
				ready <- event
				sentReady = true
			}
			t.Logf("e1 event %+v", event)
		}
	}()

	event := <-ready
	if _, ok := event.Leaders[node1]; !ok {
		t.Fatal("node2 not a leader")
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Second)
		t.Log("e1 resigning")
		e1.Resign()
		time.Sleep(5 * time.Second)
		t.Log("e2 resigning")
		e2.Resign()
	}()

	eventChan := e2.Run()
	events := make([]*election.Event, 0, 3)
	for event := range eventChan {
		t.Logf("e2 event %+v", event)
		if event.Error == nil {
			events = append(events, event)
		} else {
			t.Fatal(event.Error)
		}
	}

	if len(events) != 3 {
		t.Errorf("received wrong number of events %d != 3", len(events))
	} else {
		assertEventLeaders(t, events[0], 1, map[string]string{node1: value1})
		assertEventLeaders(t, events[1], 1, map[string]string{node2: value2})
		assertEventLeaders(t, events[2], 1, map[string]string{})
	}
}
