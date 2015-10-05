package election

import (
	"errors"
	"github.com/WiFast/go-ballot/heartbeat"
	"github.com/WiFast/go-ballot/lock"
	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"strings"
	"time"
)

type Election interface {
	// Run for office! Monitors an election and attempts to vote this node as
	// leader. Call's the Nominators Nominate method when a the node attempts
	// to gain a leadership position. Changes in election state are queued via
	// the Nominator's LeaderEvent method. Should only be called once per
	// Election. The behavior of subsquent calls to Run is undefined but bound
	// to break something.
	Run()

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
}

// A hard Election implementation. Stores and synchronizes the election in an etcd directory.
type election struct {
	name      string              // A known value used to uniquely identify this node in the election.
	key       string              // The etcd key (directory) that stores the election.
	lock      lock.Lock           // Used to sychronize access to the etcd directory.
	heartbeat heartbeat.Heartbeat // Used to refresh the TTL on this node's key while the node is a leader.
	nominator Nominator           // The nominator in charge of nominating and configuring nodes.
	api       etcd.KeysAPI        // The client to use when communicating with etcd.
	context   context.Context     // Used to cancel all operations.
	cancel    context.CancelFunc  // Used to cancel all operations.
	options   *Options            // Timing options.
}

// New creates a new Election.
func New(endpoints []string, key, name string, nominator Nominator, opts *Options) (Election, error) {
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
		lock:      lock.New(api, lockKey(key), name),
		heartbeat: heartbeat.New(api, leaderKey(key, name), opts.HeartbeatFrequency, opts.HeartbeatTimeout),
		nominator: nominator,
		api:       api,
		context:   ctx,
		cancel:    cancel,
		options:   opts,
	}, nil
}

func (b *election) Resign() {
	b.cancel()
}
