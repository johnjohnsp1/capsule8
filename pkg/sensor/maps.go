package sensor

import (
	"sync"
	"sync/atomic"
)

//
// safeSubscriptionMap
// map[uint64]chan interface{}
//

type subscriptionMap map[uint64]chan interface{}

type safeSubscriptionMap struct {
	sync.Mutex              // used only by writers
	active     atomic.Value // map[uint64]chan interface{}
}

func newSafeSubscriptionMap() *safeSubscriptionMap {
	return &safeSubscriptionMap{}
}

func (m *safeSubscriptionMap) getMap() subscriptionMap {
	value := m.active.Load()
	if value == nil {
		return nil
	}
	return value.(subscriptionMap)
}

func (m *safeSubscriptionMap) remove(mfrom subscriptionMap) {
	m.Lock()
	defer m.Unlock()

	om := m.getMap()
	if om != nil {
		nm := make(subscriptionMap, len(om)-len(mfrom))
		for k, v := range om {
			if _, ok := mfrom[k]; !ok {
				nm[k] = v
			}
		}
		m.active.Store(nm)
	}
}

func (m *safeSubscriptionMap) update(mfrom subscriptionMap) {
	m.Lock()
	defer m.Unlock()

	om := m.getMap()
	nm := make(subscriptionMap, len(om)+len(mfrom))
	if om != nil {
		for k, v := range om {
			nm[k] = v
		}
	}
	for k, v := range mfrom {
		nm[k] = v
	}

	m.active.Store(nm)
}
