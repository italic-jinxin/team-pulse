package credentials

import (
	"context"
	"errors"
	"sync"
)

var ErrNotFound = errors.New("credential not found")

type Credential struct {
	Token  string
	Source string
	Login  string
}

type Store interface {
	Get(context.Context) (Credential, error)
	Set(context.Context, Credential) error
	Delete(context.Context) error
}

type MemoryStore struct {
	mu         sync.RWMutex
	credential Credential
	set        bool
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (s *MemoryStore) Get(context.Context) (Credential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.set {
		return Credential{}, ErrNotFound
	}
	return s.credential, nil
}

func (s *MemoryStore) Set(_ context.Context, credential Credential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.credential = credential
	s.set = true
	return nil
}

func (s *MemoryStore) Delete(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.credential = Credential{}
	s.set = false
	return nil
}
