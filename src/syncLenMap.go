package main

import (
	"sync"
)

// список соединений с клиентами
type syncLenMap struct {
	count int
	m     map[*WebSocketConnection]bool
	mu    sync.Mutex
}

func initSyncLenMap() *syncLenMap {
	var s syncLenMap
	s.count = 0
	s.m = make(map[*WebSocketConnection]bool)
	return &s
}

func (s *syncLenMap) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

func (s *syncLenMap) Add(key *WebSocketConnection, value bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.m[key]
	if !exists {
		s.m[key] = value
		s.count++
	}
}

func (s *syncLenMap) Remove(key *WebSocketConnection) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.m[key]
	if exists {
		delete(s.m, key)
		s.count++
	}
	return exists
}

func (s *syncLenMap) Range(f func(key *WebSocketConnection, value bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.m {
		f(k, v)
	}
}
