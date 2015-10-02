package ballot

import (
	etcd "github.com/coreos/etcd/client"
)

func isEtcdErrorCode(err error, code int) bool {
	if err, ok := err.(etcd.Error); ok && err.Code == code {
		return true
	}
	return false
}

func hasEtcdPrefix(key, prefix string) bool {
	keyLen := len(key)
	prefixLen := len(prefix)
	return keyLen >= prefixLen && key[:prefixLen] == prefix
}

func getEtcdNode(node *etcd.Node, key string) *etcd.Node {
	if node.Key == key {
		return node
	}
	if !hasEtcdPrefix(key, node.Key) {
		return nil
	}
	if node.Dir {
		for _, node := range node.Nodes {
			node = getEtcdNode(node, key)
			if node != nil {
				return node
			}
		}
	}
	return nil
}

func isEtcdClusterError(clusterError, realError error) bool {
	if clusterError == realError {
		return true
	}
	if clusterError, ok := clusterError.(*etcd.ClusterError); ok {
		for _, err := range clusterError.Errors {
			if err == realError {
				return true
			}
		}
	}
	return false
}

func unrollEtcdClusterError(err error) error {
	if err, ok := err.(*etcd.ClusterError); ok {
		if len(err.Errors) > 0 {
			return err.Errors[0]
		}
	}
	return err
}
