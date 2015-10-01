package heartbeat

import (
	"errors"
	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"sync"
	"time"
)

var (
	RequestTimeout time.Duration = 5 * time.Second
	ErrRunning     error         = errors.New("heartbeat already running")
	ErrDeleted                   = errors.New("key deleted during heartbeat")
	ErrNotFound                  = errors.New("key not found")
)

// Heartbeat refreshes the TTL of an etcd key.
type Heartbeat interface {
	// Beat runs the heartbeat. It exits on error or when Stop is called. No
	// error is returned on Stop.
	Beat() error

	// Stop cancels the heartbeat and causes Beat to return. Calling Stop when
	// Beat is not running has no effect.
	Stop()
}

// actual heartbeat implementation
type heartbeat struct {
	api       etcd.KeysAPI
	key       string
	frequency time.Duration
	timeout   time.Duration
	value     string
	cancel    context.CancelFunc
	lock      *sync.Mutex
}

// New creates a new heartbeat which refreshes the TTL on an etcd key at the provided frequency. The timeout is the amount of time after an expected heartbeat that the key will expire. This the TTL is calculated as frequency+timeout.
func New(api etcd.KeysAPI, key string, frequency, timeout time.Duration) Heartbeat {
	return &heartbeat{
		api:       api,
		key:       key,
		frequency: frequency,
		timeout:   timeout,
		lock:      &sync.Mutex{},
	}
}

func (hb *heartbeat) get(ctx context.Context) (string, error) {
	if request, err := hb.api.Get(ctx, hb.key, &etcd.GetOptions{Quorum: true}); err == nil {
		if request.Node.Dir {
			return "", errors.New("key is a directory")
		}
		return request.Node.Value, nil
	} else {
		return "", err
	}
}

func (hb *heartbeat) set(ctx context.Context) (err error) {
	ttl := hb.frequency + hb.timeout
	for {
		// atomicly set the value with the new TTL to ensure we don't overwrite a changed value
		_, err = hb.api.Set(ctx, hb.key, hb.value, &etcd.SetOptions{PrevValue: hb.value, TTL: ttl})
		if isEtcdErrorCode(err, etcd.ErrorCodeTestFailed) {
			// the key has changed, get the new value and try again
			hb.value, err = hb.get(ctx)
			if err == nil {
				continue
			}
		} else if isEtcdErrorCode(err, etcd.ErrorCodeKeyNotFound) {
			err = ErrDeleted
		}
		break
	}
	return err
}

func isEtcdErrorCode(err error, code int) bool {
	if err, ok := err.(etcd.Error); ok && err.Code == code {
		return true
	}
	return false
}

func inClusterError(clusterError, checkError error) bool {
	if clusterError, ok := clusterError.(*etcd.ClusterError); ok {
		for _, err := range clusterError.Errors {
			if err == checkError {
				return true
			}
		}
	}
	return false
}

// Beat starts heartbeating and returns when it stops.
func (hb *heartbeat) Beat() error {
	ctx, err := func() (ctx context.Context, err error) {
		hb.lock.Lock()
		defer hb.lock.Unlock()
		if hb.cancel != nil {
			err = ErrRunning
		} else {
			ctx, hb.cancel = context.WithCancel(context.Background())
		}
		return
	}()
	if err != nil {
		return err
	}
	defer func() {
		hb.lock.Lock()
		defer hb.lock.Unlock()
		hb.cancel()
		hb.cancel = nil
	}()

	// get the value if we don't have it
	deadline, _ := context.WithDeadline(ctx, time.Now().Add(hb.frequency))
	hb.value, err = hb.get(deadline)
	if isEtcdErrorCode(err, etcd.ErrorCodeKeyNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	// set the ttl since the first tick is not immediate
	err = hb.set(deadline)
	if isEtcdErrorCode(err, etcd.ErrorCodeKeyNotFound) {
		return ErrDeleted
	} else if err != nil {
		return err
	}

	// set ttl on every timer tick
	ticker := time.NewTicker(hb.frequency)
	for {
		select {
		case tick := <-ticker.C:
			deadline, _ := context.WithDeadline(ctx, tick.Add(hb.frequency))
			err := hb.set(deadline)
			if inClusterError(err, context.Canceled) {
				return nil
			}
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
	return nil
}

// Stop the heartbeat.
func (hb *heartbeat) Stop() {
	hb.lock.Lock()
	defer hb.lock.Unlock()
	if hb.cancel != nil {
		hb.cancel()
	}
}
