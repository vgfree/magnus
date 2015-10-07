package election_test

import (
	election "."
	etcd "github.com/coreos/etcd/client"
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

func TearDown(t *testing.T) {
	client, err := etcd.New(etcd.Config{
		Endpoints: endpoints,
		Transport: etcd.DefaultTransport,
	})
	if err != nil {
		t.Fatal(err)
	}

	api := etcd.NewKeysAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err = api.Delete(ctx, key, &etcd.DeleteOptions{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
}

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
		t.Fatal("timed out waiting for WaitGroup")
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

func AggNoms(t *testing.T, nominator *Nominator, value string) []*Nomination {
	nominations := []*Nomination{}
	for nomination := range nominator.Nominations {
		nomination.Result <- value
		t.Logf("agg nom %+v", nomination)
		nominations = append(nominations, nomination)
	}
	return nominations
}

func AggEvents(t *testing.T, nominator *Nominator) []*LeaderEvent {
	events := []*LeaderEvent{}
	for event := range nominator.LeaderEvents {
		t.Logf("agg event %+v", event)
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
			t.Errorf("nomination %d incorrect\nhave: %+v\nwant: %+v", n, h, w)
		} else {
			t.Logf("nomination %d correct\nhave: %+v", n, h)
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
			t.Errorf("event %d incorrect\nhave: %+v\nwant: %+v", n, h, w)
		} else {
			t.Logf("event %d correct\nhave: %+v", n, h)
		}
	}
}

func RunElection(t *testing.T, ctx context.Context, name, value string) ([]*Nomination, []*LeaderEvent) {
	nominator := &Nominator{
		Name:         name,
		Nominations:  make(chan *Nomination),
		LeaderEvents: make(chan *LeaderEvent),
		t:            t,
	}

	wg := &sync.WaitGroup{}
	wg.Add(3)

	nominations := []*Nomination{}
	go func() {
		defer wg.Done()
		nominations = AggNoms(t, nominator, value)
	}()

	events := []*LeaderEvent{}
	go func() {
		defer wg.Done()
		events = AggEvents(t, nominator)
	}()

	el := NewElection(t, name, nominator)
	go func() {
		defer wg.Done()
		el.Run()
		nominator.Close()
	}()

	<-ctx.Done()

	el.Resign()
	waitctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	AssertWait(t, waitctx, wg)
	return nominations, events
}

func TestOneElection(t *testing.T) {
	defer TearDown(t)
	name := "node1"
	value := "127.0.0.1"

	nominations := []*Nomination{}
	events := []*LeaderEvent{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	nominations, events = RunElection(t, ctx, name, value)

	AssertNoms(t, nominations, []*Nomination{
		{Name: name, Size: 1, Leaders: map[string]string{}},
	})

	AssertEvents(t, events, []*LeaderEvent{
		{1, map[string]string{name: value}},
		{1, map[string]string{}},
	})
}

func TestSecondElection(t *testing.T) {
	defer TearDown(t)
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

	// run first election immediately; resign at 10s
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		noms1, events1 = RunElection(t, ctx, name1, value1)
	}()

	// run second election at 5s; resign at 15s
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Second)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		noms2, events2 = RunElection(t, ctx, name2, value2)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	AssertWait(t, ctx, wg)

	t.Log("checking election 1")
	AssertNoms(t, noms1, []*Nomination{
		{Name: name1, Size: 1, Leaders: map[string]string{}},
	})

	AssertEvents(t, events1, []*LeaderEvent{
		{1, map[string]string{name1: value1}},
		{1, map[string]string{}},
	})

	t.Log("checking election 2")
	AssertNoms(t, noms2, []*Nomination{
		{Name: name2, Size: 1, Leaders: map[string]string{}},
	})

	AssertEvents(t, events2, []*LeaderEvent{
		{1, map[string]string{name1: value1}},
		{1, map[string]string{name2: value2}},
		{1, map[string]string{}},
	})
}
