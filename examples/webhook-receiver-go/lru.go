package main

import (
	"container/list"
	"sync"
)

// seenSet remembers the most recent event ids so that redelivered events are
// acknowledged without being processed twice. It is a plain LRU: once more than
// capacity ids are stored, the least recently touched one is dropped.
//
// The user center retries a failed delivery for roughly nine hours (30s, 2m,
// 10m, 30m, 2h, 6h), so a few thousand ids are plenty for one endpoint. Replace
// it with a shared store (Redis SET NX with a TTL, a unique DB column, ...) when
// several receiver instances sit behind one load balancer.
type seenSet struct {
	mu    sync.Mutex
	cap   int
	order *list.List
	items map[string]*list.Element
}

func newSeenSet(capacity int) *seenSet {
	if capacity <= 0 {
		capacity = 1
	}
	return &seenSet{cap: capacity, order: list.New(), items: make(map[string]*list.Element, capacity)}
}

// Seen reports whether id was already recorded and refreshes its position.
func (s *seenSet) Seen(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	el, ok := s.items[id]
	if ok {
		s.order.MoveToFront(el)
	}
	return ok
}

// Add records id, evicting the least recently used entry when full.
func (s *seenSet) Add(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if el, ok := s.items[id]; ok {
		s.order.MoveToFront(el)
		return
	}
	s.items[id] = s.order.PushFront(id)
	for s.order.Len() > s.cap {
		last := s.order.Back()
		s.order.Remove(last)
		delete(s.items, last.Value.(string))
	}
}
