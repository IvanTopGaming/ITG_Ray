package bindings

import "sync"

// StoreLock serializes read-modify-write cycles over the shared on-disk
// stores (servers.json, subscriptions.json).
//
// Both files are mutated by load → modify → save sequences in SubsService and
// ServersService, and the bridge dispatcher runs handlers concurrently, so
// without a lock two overlapping cycles each merge into a snapshot taken
// before the other one wrote and the later Save drops the earlier change.
// The write itself is atomic, so the file never tears — the loss is logical,
// which also means the race detector cannot see it.
//
// One lock covers both files on purpose: subscription edits cascade into
// servers.json (Edit's URL change, Remove's orphan cleanup), so the two are
// mutated as a unit and a per-file lock would still leave that window open.
//
// Slow work must stay outside the lock. SyncOne fetches over the network and
// TestLatency probes servers before taking it, then re-reads under the lock —
// holding it across a 30s fetch would serialize exactly what this is meant to
// keep concurrent.
type StoreLock struct{ mu sync.Mutex }

// NewStoreLock returns a lock ready for use. main.go creates one and hands
// the same instance to every service that writes the shared stores.
func NewStoreLock() *StoreLock { return &StoreLock{} }

// Lock acquires the store lock.
func (l *StoreLock) Lock() { l.mu.Lock() }

// Unlock releases the store lock.
func (l *StoreLock) Unlock() { l.mu.Unlock() }
