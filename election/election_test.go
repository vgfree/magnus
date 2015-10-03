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
	wg := &sync.WaitGroup{}
	defer wg.Wait()

	name := "node1"
	value := "127.0.0.1"
	e, err := election.New(endpoints, key, name, value, nil)
	if err != nil {
		t.Fatal(err)
	}

	wg.Add(2)
	eventChan := make(chan *election.Event)
	go func() {
		defer wg.Done()
		e.Run(eventChan)
	}()
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Second)
		e.Resign()
	}()

	events := make([]*election.Event, 0, 2)
	for event := range eventChan {
		t.Logf("event %+v", event)
		events = append(events, event)
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
	eventChan1 := make(chan *election.Event)
	eventChan2 := make(chan *election.Event)

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

	wg.Add(1)
	go func() {
		defer wg.Done()
		e1.Run(eventChan1)
	}()

	// run first election in the background
	ready := make(chan *election.Event)
	wg.Add(1)
	go func() {
		defer wg.Done()
		sentReady := false
		for event := range eventChan1 {
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

	wg.Add(1)
	go func() {
		defer wg.Done()
		e2.Run(eventChan2)
	}()

	events := make([]*election.Event, 0, 3)
	for event := range eventChan2 {
		t.Logf("e2 event %+v", event)
		events = append(events, event)
	}

	if len(events) != 3 {
		t.Errorf("received wrong number of events %d != 3", len(events))
	} else {
		assertEventLeaders(t, events[0], 1, map[string]string{node1: value1})
		assertEventLeaders(t, events[1], 1, map[string]string{node2: value2})
		assertEventLeaders(t, events[2], 1, map[string]string{})
	}
}
