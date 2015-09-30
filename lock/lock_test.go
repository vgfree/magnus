package lock_test

import (
	lock "."
	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"sync"
	"testing"
	"time"
)

var (
	endpoints []string = []string{"http://127.0.0.1:2379"}
	key       string   = "/tests/_lock"
	client    etcd.Client
	api       etcd.KeysAPI
)

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

func get() (string, error) {
	ctx := context.Background()
	if response, err := api.Get(ctx, key, &etcd.GetOptions{Quorum: true}); err == nil {
		return response.Node.Value, nil
	} else {
		return "", err
	}
}

func set(value string) error {
	ctx := context.Background()
	_, err := api.Set(ctx, key, value, nil)
	return err
}

func del() error {
	ctx := context.Background()
	_, err := api.Delete(ctx, key, nil)
	return err
}

func TestLockAcquire(t *testing.T) {
	SetUp(t)
	defer TearDown(t)

	want := "node1"
	l := lock.New(api, key, want)
	if err := l.Acquire(500 * time.Millisecond); err == nil {
		if have, err := get(); err != nil {
			t.Error(err)
		} else if have != want {
			t.Error("%s != %s", have, want)
		}
	} else {
		t.Error(err)
	}
}

func TestLockAcquireExists(t *testing.T) {
	SetUp(t)
	defer TearDown(t)
	if err := set("node2"); err != nil {
		t.Fatal(err)
	}

	l := lock.New(api, key, "node1")
	if err := l.Acquire(500 * time.Millisecond); err == nil {
		t.Error("lock acquired")
	} else if err != lock.ErrExists {
		t.Error(err)
	}
}

func TestLockExclusive(t *testing.T) {
	SetUp(t)
	defer TearDown(t)

	node1 := lock.New(api, key, "node1")
	node2 := lock.New(api, key, "node2")
	if err := node1.Acquire(500 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := node2.Acquire(500 * time.Millisecond); err != lock.ErrExists {
		t.Error(err)
	}
}

func TestLockRelease(t *testing.T) {
	SetUp(t)
	defer TearDown(t)

	node1 := lock.New(api, key, "node1")
	node2 := lock.New(api, key, "node2")
	if err := node1.Acquire(500 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := node2.Acquire(500 * time.Millisecond); err != nil {
			t.Error(err)
		}
	}()
	time.Sleep(200 * time.Millisecond)
	if err := node1.Release(500 * time.Millisecond); err != nil {
		t.Error(err)
	}
	wg.Wait()
}

func TestLockAcquireTTL(t *testing.T) {

	SetUp(t)
	defer TearDown(t)

	node1 := lock.New(api, key, "node1")
	node2 := lock.New(api, key, "node2")
	if err := node1.AcquireTTL(500*time.Millisecond, time.Second); err != nil {
		t.Fatal(err)
	}
	if err := node2.Acquire(2 * time.Second); err != nil {
		t.Error(err)
	}
}
