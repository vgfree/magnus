package election

import (
	"github.com/WiFast/go-ballot/heartbeat"
	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"reflect"
	"sync"
	"time"
)

// watch queues election changes into `events`.
func (b *election) watch(events chan<- string) {
	debug.Printf("watch on %s started", b.key)
	watcher := b.api.Watcher(b.key, &etcd.WatcherOptions{Recursive: true})
	for {
		res, err := watcher.Next(b.context)
		err = unrollEtcdClusterError(err)
		if err == context.Canceled {
			debug.Printf("watch on %s canceled, returning", b.key)
			break
		}
		if isEtcdErrorCode(err, etcd.ErrorCodeKeyNotFound) {
			debug.Printf("wath on %s failed, key not found, retrying", b.key)
			continue
		}
		if err == nil {
			if hasEtcdPrefix(res.Node.Key, sizeKey(b.key)) || hasEtcdPrefix(res.Node.Key, leadersKey(b.key)) {
				debug.Printf("watcher emitting event %s}", res.Node.Key)
				events <- res.Node.Key
			} else {
				debug.Printf("watcher discarding event %s}", res.Node.Key)
			}
		} else {
			logger.Printf("watch error on %s: %s", res.Node.Key, err)
		}
	}
	close(events)
}

// beat heartbeat the node's leader key. It exits when stopped or when the key
// key is deleted.
func (b *election) beat() {
	for {
		err := b.heartbeat.Beat()
		if err == nil {
			debug.Print("heartbeat stopped")
			break
		} else if err == heartbeat.ErrRunning {
			debug.Print("heartbeat already running")
			break
		} else if err == heartbeat.ErrDeleted {
			debug.Print("heartbeat stopped, key deleted")
			break
		} else if err == heartbeat.ErrNotFound {
			debug.Print("heartbeat failed, key not found")
			break
		}
		logger.Printf(`heartbeat exited with error, retrying: %s`, unrollEtcdClusterError(err))
	}
	return
}

func (b *election) Run() {
	logger.Printf("%s is running for leader of %s", b.name, b.key)
	wg := &sync.WaitGroup{}
	watchEvents := make(chan string, 1)
	size := 0
	leaders := map[string]string{}

	defer func() {
		wg.Wait()
	}()

	doBeat := func() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.beat()
		}()
	}

	doVote := func() error {
		newSize, newLeaders, err := b.vote()
		if err == nil {
			// success, do the beat
			if _, ok := newLeaders[b.name]; ok {
				doBeat()
			}
			if newSize != size || !reflect.DeepEqual(newLeaders, leaders) {
				logger.Printf("leadership changed size=%d leaders=%+v", newSize, newLeaders)
				if nomErr := b.nominator.LeaderEvent(b.context, newSize, newLeaders); nomErr == context.Canceled {
					debug.Print("canceled sending leadership change event")
					return nil
				} else if nomErr != nil {
					logger.Printf("leadership change event failed: %s", nomErr)
				}
				size = newSize
				leaders = newLeaders
			}
		} else if err == context.Canceled {
			// exit loop on cancelation
			logger.Print("election canceled, returning")
			err = nil
		} else {
			// queue errors downstream
			logger.Printf("election error, retrying: %s", err)
		}
		return err
	}

	// resign on completion
	defer func() {
		b.heartbeat.Stop()
		newSize, newLeaders, err := b.resign()
		if err != nil {
			logger.Printf("resign failed: %s", b.name)
		} else if newSize != size || !reflect.DeepEqual(newLeaders, leaders) {
			logger.Printf("%s resigned leadership of %s", b.name, b.key)
			b.nominator.LeaderEvent(b.context, newSize, newLeaders)
			if nomErr := b.nominator.LeaderEvent(b.context, newSize, newLeaders); nomErr != nil && nomErr != context.Canceled {
				logger.Printf("leadership change event failed: %s", nomErr)
			}
		}
	}()

	// watch for leadership changes
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.watch(watchEvents)
	}()

	// try and vote ourselves in before we take any events
	if doVote() == context.Canceled {
		return
	}

	// process leadership changes
	tick := time.NewTicker(60 * time.Second)
	for {
		select {
		case _, ok := <-watchEvents:
			if !ok {
				// exit loop on channel close
				debug.Print("watch event channel closed, returning")
				return
			} else if doVote() == context.Canceled {
				return
			}
		case <-tick.C:
			if doVote() == context.Canceled {
				return
			}
		}
	}
}
