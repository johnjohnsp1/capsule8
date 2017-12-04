package perf

import (
	"sync"
	"sync/atomic"
)

//
// safeUInt64Map
// map[uint64]uint64
//

type uint64Map map[uint64]uint64

func newUInt64Map() uint64Map {
	return make(uint64Map)
}

type safeUInt64Map struct {
	sync.Mutex              // used only by writers
	active     atomic.Value // map[uint64]uint64
}

func newSafeUInt64Map() *safeUInt64Map {
	return &safeUInt64Map{}
}

func (m *safeUInt64Map) getMap() uint64Map {
	value := m.active.Load()
	if value == nil {
		return nil
	}
	return value.(uint64Map)
}

func (m *safeUInt64Map) removeInPlace(ids []uint64) {
	em := m.getMap()
	if em == nil {
		return
	}

	for _, id := range ids {
		delete(em, id)
	}
}

func (m *safeUInt64Map) remove(ids []uint64) {
	m.Lock()
	defer m.Unlock()

	om := m.getMap()
	nm := newUInt64Map()
	if om != nil {
		for k, v := range om {
			nm[k] = v
		}
	}
	for _, id := range ids {
		delete(nm, id)
	}

	m.active.Store(nm)
}

func (m *safeUInt64Map) updateInPlace(mfrom uint64Map) {
	mto := m.getMap()
	if mto == nil {
		mto = newUInt64Map()
		m.active.Store(mto)
	}

	for k, v := range mfrom {
		mto[k] = v
	}
}

func (m *safeUInt64Map) update(mfrom uint64Map) {
	m.Lock()
	defer m.Unlock()

	om := m.getMap()
	nm := newUInt64Map()
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

//
// safeEventAttrMap
// map[uint64]*EventAttr
//

type eventAttrMap map[uint64]*EventAttr

func newEventAttrMap() eventAttrMap {
	return make(eventAttrMap)
}

type safeEventAttrMap struct {
	sync.Mutex              // used only by writers
	active     atomic.Value // map[uint64]*EventAttr
}

func newSafeEventAttrMap() *safeEventAttrMap {
	return &safeEventAttrMap{}
}

func (m *safeEventAttrMap) getMap() eventAttrMap {
	value := m.active.Load()
	if value == nil {
		return nil
	}
	return value.(eventAttrMap)
}

func (m *safeEventAttrMap) removeInPlace(ids []uint64) {
	em := m.getMap()
	if em == nil {
		return
	}

	for _, id := range ids {
		delete(em, id)
	}
}

func (m *safeEventAttrMap) remove(ids []uint64) {
	m.Lock()
	defer m.Unlock()

	oem := m.getMap()
	nem := newEventAttrMap()
	if oem != nil {
		for k, v := range oem {
			nem[k] = v
		}
	}
	for _, id := range ids {
		delete(nem, id)
	}

	m.active.Store(nem)
}

func (m *safeEventAttrMap) updateInPlace(emfrom eventAttrMap) {
	em := m.getMap()
	if em == nil {
		em = newEventAttrMap()
		m.active.Store(em)
	}

	for k, v := range emfrom {
		em[k] = v
	}
}

func (m *safeEventAttrMap) update(emfrom eventAttrMap) {
	m.Lock()
	defer m.Unlock()

	oem := m.getMap()
	nem := newEventAttrMap()
	if oem != nil {
		for k, v := range oem {
			nem[k] = v
		}
	}
	for k, v := range emfrom {
		nem[k] = v
	}

	m.active.Store(nem)
}
