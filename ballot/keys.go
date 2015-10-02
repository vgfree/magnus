package ballot

import (
	"path"
	"strings"
)

func cleanKey(key string) string {
	return "/" + strings.Trim(key, "/")
}

func lockKey(prefix string) string {
	return path.Join(prefix, "_lock")
}

func sizeKey(prefix string) string {
	return path.Join(prefix, "size")
}

func leadersKey(prefix string) string {
	return path.Join(prefix, "leaders")
}

func leaderKey(prefix, name string) string {
	return path.Join(leadersKey(prefix), name)
}
