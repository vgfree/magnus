package election

import (
	"fmt"
	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"path"
	"strconv"
)

const (
	DefaultSize = 1
)

func (b *election) setSize(ctx context.Context, size int) error {
	opts := &etcd.SetOptions{PrevExist: etcd.PrevNoExist}
	_, err := b.api.Set(ctx, sizeKey(b.key), fmt.Sprintf("%d", size), opts)
	return unrollEtcdClusterError(err)
}

func (b *election) setLeader(ctx context.Context, value string) error {
	ttl := b.options.HeartbeatFrequency + b.options.HeartbeatTimeout
	_, err := b.api.Set(ctx, leaderKey(b.key, b.name), value, &etcd.SetOptions{TTL: ttl})
	return unrollEtcdClusterError(err)
}

func (b *election) getNode(ctx context.Context) (*etcd.Node, error) {
	opts := &etcd.GetOptions{Recursive: true, Quorum: true}
	if res, err := b.api.Get(ctx, b.key, opts); err == nil {
		return res.Node, nil
	} else {
		return nil, unrollEtcdClusterError(err)
	}
}

func (b *election) parseState(ctx context.Context, node *etcd.Node) (size int, leaders map[string]string, err error) {
	// parse size
	size = DefaultSize
	key := sizeKey(b.key)
	sizeNode := getEtcdNode(node, key)
	if sizeNode == nil {
		debug.Printf("key %s not found, setting to %d", key, size)
		if err = b.setSize(ctx, size); err != nil {
			err = unrollEtcdClusterError(err)
			debug.Printf("set size key %s failed: %s", key, err)
			return
		}
	} else if size, err = strconv.Atoi(sizeNode.Value); err != nil {
		debug.Printf("key %s has invalid value %s, setting to %d", key, sizeNode.Value, size)
		if err = b.setSize(ctx, size); err != nil {
			err = unrollEtcdClusterError(err)
			debug.Printf("set size key %s failed: %s", key, err)
			return
		}
	}

	// parse leaders
	leaders = make(map[string]string, size)
	leadersNode := getEtcdNode(node, leadersKey(b.key))
	if leadersNode != nil {
		for _, leaderNode := range leadersNode.Nodes {
			if !leaderNode.Dir {
				leaders[path.Base(leaderNode.Key)] = leaderNode.Value
			}
		}
	}
	return
}

func (b *election) getState(ctx context.Context) (size int, leaders map[string]string, err error) {
	size = DefaultSize
	leaders = map[string]string{}

	node, err := b.getNode(ctx)
	err = unrollEtcdClusterError(err)
	if isEtcdErrorCode(err, etcd.ErrorCodeKeyNotFound) {
		debug.Printf("key %s not found, setting to %d", sizeKey(b.key), size)
		err = b.setSize(ctx, size)
		if err != nil {
			debug.Printf("set size key %s failed: %s", sizeKey(b.key), err)
		}
	} else if err == nil {
		size, leaders, err = b.parseState(ctx, node)
	}
	return
}
