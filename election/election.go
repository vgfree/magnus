package election

import (
	"errors"
	"github.com/WiFast/go-ballot/heartbeat"
	"github.com/WiFast/go-ballot/lock"
	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"strings"
	"sync"
	"time"
)

type Election interface {
	// Run for office! Monitors an election and attempts to vote this node as
	// leader. Changes in election state are queued via the returend Event
	// channel. If the channel blocks on receive for 100ms the event is
	// discarded. The same channel is returned on subsequent calls.
	Run() <-chan *Event

	// Resign the node's spot as a leader. Stops any running goroutines and
	// closed the Event channel.
	Resign()
}

// Event is sent on the channel returned by Run. An Event is sent any time
// there is a change in leadership or an error occurs.
type Event struct {
	Time    time.Time         // The time at which this event occurred.
	Name    string            // The name of this node.
	Size    int               // How many leaders are allowed.
	Leaders map[string]string // The leaders at the time of this event.
	Error   error             // If there was an error, this is it.
}

// A hard Election implementation. Stores synchronizes the election in an etcd directory.
type election struct {
	name      string              // A known value used to uniquely identify this node in the election.
	key       string              // The etcd key (directory) that stores the election.
	value     string              // A value to set in the leader key.
	lock      lock.Lock           // Used to sychronize access to the etcd directory.
	heartbeat heartbeat.Heartbeat // Used to refresh the TTL on this node's key while the node is a leader.
	api       etcd.KeysAPI        // The client to use when communicating with etcd.
	context   context.Context     // Used to cancel all operations.
	cancel    context.CancelFunc  // Used to cancel all operations.
	wg        *sync.WaitGroup     // Used to wait for the Run function to complete.
	options   *Options            // Timing options.
}

// New creates a new Election.
func New(endpoints []string, key, name, value string, opts *Options) (Election, error) {
	if strings.ContainsAny(name, "/") {
		return nil, errors.New("name may not contain a forward slash")
	}

	if opts == nil {
		opts = &Options{}
	}
	if opts.ElectionTimeout <= time.Duration(0) {
		opts.ElectionTimeout = DefaultElectionTimeout
	}
	if opts.HeartbeatFrequency <= time.Duration(0) {
		opts.HeartbeatFrequency = DefaultHeartbeatFrequency
	}
	if opts.HeartbeatTimeout <= time.Duration(0) {
		opts.HeartbeatTimeout = DefaultHeartbeatTimeout
	}

	client, err := etcd.New(etcd.Config{
		Endpoints:               endpoints,
		Transport:               etcd.DefaultTransport,
		HeaderTimeoutPerRequest: opts.ElectionTimeout,
	})
	if err != nil {
		return nil, err
	}

	key = cleanKey(key)
	api := etcd.NewKeysAPI(client)
	ctx, cancel := context.WithCancel(context.Background())

	return &election{
		name:      name,
		key:       key,
		value:     value,
		lock:      lock.New(api, lockKey(key), name),
		heartbeat: heartbeat.New(api, leaderKey(key, name), opts.HeartbeatFrequency, opts.HeartbeatTimeout),
		api:       api,
		context:   ctx,
		cancel:    cancel,
		wg:        &sync.WaitGroup{},
		options:   opts,
	}, nil
}

func (b *election) Run() <-chan *Event {
	b.wg.Add(1)
	events := make(chan *Event)
	go func() {
		defer b.wg.Done()
		b.run(events)
	}()
	return events
}

func (b *election) Resign() {
	b.cancel()
	b.wg.Wait()
}
