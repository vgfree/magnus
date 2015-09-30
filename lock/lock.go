package lock

import (
	"errors"
	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"strings"
	"time"
)

var (
	ErrTimeout error = errors.New("timed out")
	ErrExists        = errors.New("lock exists")
)

var (
	lockAcquireTimeout time.Duration = 500 * time.Millisecond
)

// Lock allows a key in etcd to be treated as an exclusive lock. The lock is not reentrant.
type Lock interface {
	Acquire(ctx context.Context) error
	AcquireTTL(ctx context.Context, ttl time.Duration) error
	Release(ctx context.Context) error
}

type lock struct {
	key   string       // Path to the lock key.
	value string       // A value to place in the lock key when acquired.
	api   etcd.KeysAPI // Client to use when interacting with etcd.
}

func New(api etcd.KeysAPI, key, value string) Lock {
	key = strings.Trim(key, "/")
	return &lock{key, value, api}
}

// Acquire the lock. ErrTimeout is returned after `timeout` elapses without acquiring the lock.
func (l *lock) Acquire(ctx context.Context) error {
	return l.AcquireTTL(ctx, time.Duration(0))
}

// AcquireTTL acquires the lock as with `Acquire` but places a TTL on it. After the TTL expires the lock is released automatically.
func (l *lock) AcquireTTL(ctx context.Context, ttl time.Duration) error {
	var err error
	done := make(chan error)

Loop:
	for {
		acquireCtx, cancel := context.WithCancel(context.Background())
		go func() {
			done <- l.acquireTry(acquireCtx, ttl)
		}()

		select {
		case err = <-done:
			cancel()
			if err != ErrExists {
				break Loop
			}
		case <-ctx.Done():
			cancel()
			<-done
			if err != ErrExists {
				err = ErrTimeout
			}
			break Loop
		}
	}
	return err
}

// Release the lock.
func (l *lock) Release(ctx context.Context) error {
	_, err := l.api.Delete(ctx, l.key, &etcd.DeleteOptions{PrevValue: l.value})
	if apiErr, ok := err.(etcd.Error); ok && apiErr.Code == etcd.ErrorCodeKeyNotFound {
		err = nil
	}
	return err
}

// acquireTry attempte to acquire the lock. It returns an ErrExists if the lock is busy.
func (l *lock) acquireTry(ctx context.Context, ttl time.Duration) error {
	_, err := l.api.Set(ctx, l.key, l.value, &etcd.SetOptions{PrevExist: etcd.PrevNoExist, TTL: ttl})
	if err, ok := err.(etcd.Error); ok && err.Code == etcd.ErrorCodeNodeExist {
		return ErrExists
	}
	return err
}
