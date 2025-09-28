package main

import (
	"sync"
	"time"
)

type DeviceID = string

type DeviceProvisioning struct {
	ClientID DeviceID
	ClientIP string
	Created  time.Time
}

type deviceStore struct {
	mu      sync.RWMutex
	entries map[DeviceID]DeviceProvisioning
	stop    chan struct{}
}

func NewDeviceStore() *deviceStore {
	ms := &deviceStore{
		entries: make(map[DeviceID]DeviceProvisioning),
		stop:    make(chan struct{}),
	}
	// go ms.janitor()
	return ms
}

func (ds *deviceStore) addEntry(id DeviceID, entry DeviceProvisioning) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.entries[id] = entry
}

func (ds *deviceStore) getEntry(id DeviceID) (DeviceProvisioning, bool) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	entry, exists := ds.entries[id]
	return entry, exists
}

func (ds *deviceStore) removeEntry(id DeviceID) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	delete(ds.entries, id)
}
