// Author <dorzheho@cisco.com>

package mutex

import "sync"

const (
	LockActionUpgradeCluster    = "UpgradeCluster"
	LockActionEnableDisableApps = "EnableDisableApps"
	LockActionDeleteApps        = "DeleteApps"
	LockActionCreateNode        = "AddNode"
	LockActionUpdateNodeState   = "UpdateNodeState"
	LockActionDeleteNode        = "RemoveNode"
	LockActionAny               = "AnyAction"
)

type mutex map[string]interface{}

var (
	once     sync.Once
	instance mutex
)

func New() mutex {
	once.Do(func() {
		instance = make(map[string]interface{})
	})

	return instance
}

func Lock(key string, value interface{}) {
	instance[key] = value
}

func Unlock(key string) {
	if _, ok := instance[key]; ok {
		delete(instance, key)
	}
}

func IsLocked(key string) bool {
	for _, k := range []string{key, LockActionEnableDisableApps, LockActionDeleteApps,
		LockActionUpgradeCluster, LockActionCreateNode, LockActionDeleteNode} {
		if _, ok := instance[k]; ok {
			return true
		}
	}

	return false
}
