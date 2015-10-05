package main

import (
	"flag"
	"github.com/WiFast/go-ballot/election"
	"github.com/WiFast/go-ballot/privateip"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
)

var (
	DefaultEndpoints []string = []string{"http://127.0.0.1:2379"}
)

func main() {
	var (
		endpoints StringsOpt
		key string
		privateIPs StringsOpt
		options *election.Options = &election.Options{}
	)

	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags.Var(&endpoints, "endpoints", "A list of etcd endpoints to connect to.")
	flags.StringVar(&key, "key", "", "Path to the etcd directory where the election is stored.")
	flags.Var(&privateIPs, "private-ips", "A list of private IP addresses to choose from.")
	flags.DurationVar(&options.ElectionTimeout, "election-timeout", election.DefaultElectionTimeout, "How long to wait for etcd to respond during an election.")
	flags.DurationVar(&options.HeartbeatFrequency, "heartbeat-frequency", election.DefaultHeartbeatFrequency, "How frequently to refresh the leader key in etcd.")
	flags.DurationVar(&options.HeartbeatTimeout, "heartbeat-timeout", election.DefaultHeartbeatTimeout, "How long after a missed heartbeat the leader node should remain.")
	flags.Parse(os.Args[1:])

	if key == "" {
		logger.Fatal("key is required")
	}
	if len(privateIPs) == 0 {
		logger.Fatal("at least one private ip must be provided")
	}

	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt)

	nominator, err := privateip.New([]string(privateIPs))
	if err != nil {
		logger.Fatal(err)
	}

	el, err := election.New([]string(endpoints), key, nominator.InstanceID(), nominator, options)
	if err != nil {
		logger.Fatal(err)
	}

	// spin up services
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		el.Run()
	}()

	stopped := make(chan struct{})
	go func() {
		wg.Wait()
		close(stopped)
	}()

	<-sigChan
	el.Resign()

	select {
	case <-time.After(60 * time.Second):
		logger.Print("timed out waiting for stop event")
	case <-stopped:
	}
	logger.Print("ballot stopped")
}

// A string array flag capable of being appended to.
type StringsOpt []string

func (strs *StringsOpt) Set(value string) error {
	*strs = append(*strs, strings.Split(value, ",")...)
	return nil
}

func (strs *StringsOpt) String() string {
	return strings.Join(*strs, ",")
}
