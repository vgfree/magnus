package election

import (
	"golang.org/x/net/context"
)

// attempt to vote the node in as a leader
func (b *election) vote() (size int, leaders map[string]string, err error) {
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

	// check if we're already a leader
	if _, exists := leaders[b.name]; exists {
		debug.Printf("%s already leader of %s", b.name, b.key)
		return
	} else if len(leaders) >= size {
		debug.Printf("already %d leaders in %s", size, b.key)
		return
	}

	// nominate the node
	value := ""
	value, err = b.nominator.Nominate(ctx, b.name, size, leaders)
	if err != nil {
		logger.Printf("vote failed, nominator returned error: %s", err)
		return
	}

	// set us up the leader
	if err = b.setLeader(ctx, value); err != nil {
		logger.Printf("vote failed, could not set leader: %s", err)
		return
	}
	leaders[b.name] = value
	return
}

// remove the node as leader
func (b *election) resign() (size int, leaders map[string]string, err error) {
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
	if _, exists := leaders[b.name]; !exists {
		debug.Printf("resign complete, %s was not a leader of %s", b.name, b.key)
		return
	}

	err = b.delLeader(ctx)
	if err == nil {
		debug.Printf("resign complete, %s stepped down as leader of %s", b.name, b.key)
		delete(leaders, b.name)
	} else {
		debug.Printf("resign failed: %s", err)
	}
	return
}
