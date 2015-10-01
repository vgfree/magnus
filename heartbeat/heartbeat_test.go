package heartbeat_test

import (
	heartbeat "."
	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"sync"
	"testing"
	"time"
)

var (
	endpoints []string = []string{"http://127.0.0.1:2379"}
	key       string   = "/tests/heartbeat_value"
	client    etcd.Client
	api       etcd.KeysAPI
)

func get() (string, error) {
	ctx := context.Background()
	if response, err := api.Get(ctx, key, &etcd.GetOptions{Quorum: true}); err == nil {
		return response.Node.Value, nil
	} else {
		return "", err
	}
}

func set(value string, ttl time.Duration) error {
	ctx := context.Background()
	_, err := api.Set(ctx, key, value, &etcd.SetOptions{TTL: ttl})
	return err
}

func del() error {
	ctx := context.Background()
	_, err := api.Delete(ctx, key, nil)
	return err
}

func waitChan(wg *sync.WaitGroup) chan struct{} {
	ch := make(chan struct{})
	go func() {
		wg.Wait()
		close(ch)
	}()
	return ch
}

func SetUp(t *testing.T) {
	var err error
	client, err = etcd.New(etcd.Config{
		Endpoints: endpoints,
		Transport: etcd.DefaultTransport,
	})
	if err != nil {
		t.Fatal(err)
	}
	api = etcd.NewKeysAPI(client)
}

func TearDown(t *testing.T) {
	del()
}

// TTL's are set and expire on a key with an existing TTL.
func TestTTL(t *testing.T) {
	SetUp(t)
	defer TearDown(t)

	freq := 2 * time.Second
	to := 500 * time.Millisecond
	want := "testing 123"
	if err := set(want, freq+to); err != nil {
		t.Fatal(err)
	}

	hb := heartbeat.New(api, key, freq, to)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := hb.Beat(); err != nil {
			t.Error(err)
		}
	}()

	time.Sleep(6 * time.Second)
	if have, err := get(); err != nil {
		t.Error(err)
	} else if have != want {
		t.Errorf("%s != %s", have, want)
	}

	hb.Stop()
	select {
	case <-time.After(1 * time.Second):
		t.Error("beat did not exist")
	case <-waitChan(wg):
	}
}

// TTL's are set and expire on a key without an existing TTL.
func TestNoTTL(t *testing.T) {
	SetUp(t)
	defer TearDown(t)

	freq := 2 * time.Second
	to := 500 * time.Millisecond
	want := "testing 123"
	if err := set(want, time.Duration(0)); err != nil {
		t.Fatal(err)
	}

	hb := heartbeat.New(api, key, freq, to)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := hb.Beat(); err != nil {
			t.Error(err)
		}
	}()

	time.Sleep(6 * time.Second)
	if have, err := get(); err != nil {
		t.Error(err)
	} else if have != want {
		t.Errorf("%s != %s", have, want)
	}

	hb.Stop()
	select {
	case <-time.After(1 * time.Second):
		t.Error("beat did not exist")
	case <-waitChan(wg):
	}
}

// Make sure Stop doesn't panic.
func TestStop(t *testing.T) {
	SetUp(t)
	defer TearDown(t)
	freq := 2 * time.Second
	to := 500 * time.Millisecond
	hb := heartbeat.New(api, key, freq, to)
	hb.Stop()
}
