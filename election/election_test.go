package election_test

import (
	election "."
	"golang.org/x/net/context"
	"reflect"
	"sync"
	"testing"
	"time"
)

var (
	endpoints []string = []string{"http://127.0.0.1:2379"}
	key       string   = "/tests/election"
)

func WaitChan(wg *sync.WaitGroup) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		wg.Wait()
		ch <- struct{}{}
	}()
	return ch
}

func AssertWait(t *testing.T, ctx context.Context, wg *sync.WaitGroup) {
	select {
	case <-ctx.Done():
		t.Error("timed out waiting for WaitGroup")
	case <-WaitChan(wg):
	}
}

func NewElection(t *testing.T, name string, nominator *Nominator) election.Election {
	election, err := election.New(endpoints, key, name, nominator, nil)
	if err != nil {
		t.Fatal(err)
	}
	return election
}

func AssertNom(t *testing.T, nom *Nomination, size int, leaders map[string]string) {
	if nom == nil {
		t.Error("nomination is nil")
	} else if nom.Size != size {
		t.Errorf("nomination size %d != %d", nom.Size, size)
	} else if !reflect.DeepEqual(nom.Leaders, leaders) {
		t.Errorf("nomination leaders %+v != %+v", nom.Leaders, leaders)
	}
}

func AssertEvent(t *testing.T, event *LeaderEvent, size int, leaders map[string]string) {
	if event == nil {
		t.Error("event is nil")
	} else if event.Size != size {
		t.Errorf("event size %d != %d", event.Size, size)
	} else if !reflect.DeepEqual(event.Leaders, leaders) {
		t.Errorf("event leaders %+v != %+v", event.Leaders, leaders)
	}
}

func AggNoms(nominator *Nominator, value string) []*Nomination {
	nominations := []*Nomination{}
	for nomination := range nominator.Nominations {
		nomination.Result <- value
		nominations = append(nominations, nomination)
	}
	return nominations
}

func AggEvents(nominator *Nominator) []*LeaderEvent {
	events := []*LeaderEvent{}
	for event := range nominator.LeaderEvents {
		events = append(events, event)
	}
	return events
}

func AssertNoms(t *testing.T, have []*Nomination, want []*Nomination) {
	if len(have) != len(want) {
		t.Errorf("wrong number of nominations: %d != %d", len(have), len(want))
		return
	}
	for n, w := range want {
		h := have[n]
		if h.Name != w.Name || h.Size != w.Size || !reflect.DeepEqual(h.Leaders, w.Leaders) {
			t.Errorf("nomination %d incorret: %+v != %+v", n, h, w)
		}
	}
}

func AssertEvents(t *testing.T, have []*LeaderEvent, want []*LeaderEvent) {
	if len(have) != len(want) {
		t.Errorf("wrong number of events: %d != %d", len(have), len(want))
		return
	}
	for n, w := range want {
		h := have[n]
		if !reflect.DeepEqual(h, w) {
			t.Errorf("event %d incorret: %+v != %+v", n, h, w)
		}
	}
}

func RunElection(t *testing.T, ctx context.Context, name, value string) ([]*Nomination, []*LeaderEvent) {
	nominator := &Nominator{
		Nominations:  make(chan *Nomination),
		LeaderEvents: make(chan *LeaderEvent),
	}

	aggwg := &sync.WaitGroup{}
	aggwg.Add(2)

	nominations := []*Nomination{}
	go func() {
		defer aggwg.Done()
		nominations = AggNoms(nominator, value)
	}()

	events := []*LeaderEvent{}
	go func() {
		defer aggwg.Done()
		events = AggEvents(nominator)
	}()

	el := NewElection(t, name, nominator)

	elwg := &sync.WaitGroup{}
	elwg.Add(1)
	go func() {
		defer elwg.Done()
		el.Run()
	}()

	<-ctx.Done()

	el.Resign()
	waitctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	AssertWait(t, waitctx, elwg)
	nominator.Close()
	AssertWait(t, waitctx, aggwg)
	return nominations, events
}

func TestOneElection(t *testing.T) {
	name := "node1"
	value := "127.0.0.1"

	nominations := []*Nomination{}
	events := []*LeaderEvent{}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	nominations, events = RunElection(t, ctx, name, value)

	AssertNoms(t, nominations, []*Nomination{
		{Name: name, Size: 1, Leaders: map[string]string{name: value}},
	})

	AssertEvents(t, events, []*LeaderEvent{
		{1, map[string]string{name: value}},
		{1, map[string]string{}},
	})
}

func TestSecondElection(t *testing.T) {
	name1 := "node1"
	value1 := "10.1.1.10"
	noms1 := []*Nomination{}
	events1 := []*LeaderEvent{}

	name2 := "node2"
	value2 := "10.1.1.11"
	noms2 := []*Nomination{}
	events2 := []*LeaderEvent{}

	wg := &sync.WaitGroup{}
	wg.Add(2)

	// run first election immediately; resign at 4s
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		noms1, events1 = RunElection(t, ctx, name1, value1)
	}()

	// run second election at 2s; resign at 6s
	go func() {
		defer wg.Done()
		time.Sleep(3 * time.Second)
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		noms2, events2 = RunElection(t, ctx, name2, value2)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	AssertWait(t, ctx, wg)

	t.Log("checking election 1")
	AssertNoms(t, noms1, []*Nomination{
		{Name: name1, Size: 1, Leaders: map[string]string{name1: value1}},
	})

	AssertEvents(t, events1, []*LeaderEvent{
		{1, map[string]string{name1: value1}},
		{1, map[string]string{}},
	})

	t.Log("checking election 2")
	AssertNoms(t, noms2, []*Nomination{
		{Name: name2, Size: 1, Leaders: map[string]string{name2: value2}},
	})

	AssertEvents(t, events2, []*LeaderEvent{
		{1, map[string]string{name1: value1}},
		{1, map[string]string{name2: value2}},
		{1, map[string]string{}},
	})
}
