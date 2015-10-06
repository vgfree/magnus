package election

import (
	"time"
)

var (
	DefaultElectionTimeout    = 15 * time.Second
	DefaultHeartbeatFrequency = 30 * time.Second
	DefaultHeartbeatTimeout   = 5 * time.Second
)

type Options struct {
	ElectionTimeout    time.Duration
	HeartbeatFrequency time.Duration
	HeartbeatTimeout   time.Duration
}
