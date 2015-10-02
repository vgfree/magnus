package ballot

import (
	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

// attempt to vote the node in as a leader
func (b *ballot) vote() (size int, leaders map[string]string, err error) {
	// give the vote operation a time limit
	debug.Printf("%s voting in %s", b.name, b.key)
	ctx, cancel := context.WithTimeout(b.context, b.options.ElectionTimeout)
	defer cancel()

	// acquire the lock
	if err = b.lock.AcquireTTL(ctx, b.options.ElectionTimeout); err == nil {
		debug.Printf("acquired lock with ttl of %s", b.options.ElectionTimeout)
	} else {
		err = unrollEtcdClusterError(err)
		debug.Printf("vote failed, could not acquire lock: %s", err)
		return
	}
	defer b.lock.Release(ctx)

	// retrieve election state
	size, leaders, err = b.getState(ctx)
	if err != nil {
		err = unrollEtcdClusterError(err)
		debug.Printf("vote failed, could not get state: %s", err)
		return
	}

	// set us up the leader
	value, exists := leaders[b.name]
	if (exists && value != b.value) || (!exists && len(leaders) < size) {
		err = b.setLeader(ctx, b.value)
		if err == nil {
			if exists {
				debug.Printf("%s already leader of %s", b.name, b.key)
			}
			debug.Printf("elected %s leader of %s", b.name, b.key)
			leaders[b.name] = b.value
		} else {
			err = unrollEtcdClusterError(err)
			debug.Printf("vote failed, could not set leader: %s", err)
		}
	} else if exists {
		debug.Printf("%s already leader of %s", b.name, b.key)
	} else {
		debug.Printf("lost election, already %d leaders in %s", size, b.key)
	}
	return
}

// remove the node as leader
func (b *ballot) resign() (size int, leaders map[string]string, err error) {
	debug.Printf("resigning %s as leader of %s", b.name, b.key)
	ctx, cancel := context.WithTimeout(context.Background(), b.options.ElectionTimeout)
	defer cancel()

	// acquire the lock
	if err = b.lock.AcquireTTL(ctx, b.options.ElectionTimeout); err == nil {
		debug.Printf("acquired lock with ttl of %s", b.options.ElectionTimeout)
	} else {
		debug.Printf("resign failed, could not acquire lock: %s", err)
		return
	}
	defer b.lock.Release(ctx)

	// retrieve election state
	size, leaders, err = b.getState(ctx)
	if err != nil {
		debug.Printf("resign failed, could not get state: %s", err)
		return
	}

	// delete the leader key
	if _, ok := leaders[b.name]; ok {
		_, err := b.api.Delete(ctx, leaderKey(b.key, b.name), nil)
		if isEtcdErrorCode(err, etcd.ErrorCodeKeyNotFound) {
			err = nil
			debug.Printf("resign complete, %s was not a leader of %s", b.name, b.key)
		} else {
			debug.Printf("resign complete, %s stepped down as leader of %s", b.name, b.key)
		}
		if err == nil {
			delete(leaders, b.name)
		}
	} else {
		debug.Printf("resign complete, %s was not a leader of %s", b.name, b.key)
	}
	return
}
