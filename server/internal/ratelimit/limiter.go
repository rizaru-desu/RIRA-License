// SPDX-License-Identifier: AGPL-3.0-or-later
//
// RiraLicense — license server & API
// Copyright (C) 2026 RiraLab — Rizaru Desu. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// For full license text and ownership/attribution terms, see LICENSE and NOTICE.

package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	LoginMaxAttempts          = 5
	LoginWindow               = 15 * time.Minute
	ForgotPasswordMaxAttempts = 3
	ForgotPasswordWindow      = time.Hour
	RegisterMaxAttempts       = 5
	RegisterWindow            = time.Hour

	defaultCleanupInterval = 5 * time.Minute
	defaultMaxEntries      = 50_000
)

type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Store is an in-memory per-key rate limiter backed by golang.org/x/time/rate
// with TTL eviction and a stoppable background cleanup ticker.
type Store struct {
	mu         sync.Mutex
	entries    map[string]*entry
	limit      rate.Limit
	burst      int
	ttl        time.Duration
	maxEntries int
	now        func() time.Time
	done       chan struct{}
	stopOnce   sync.Once
}

// NewStore creates a rate limiter Store that permits burst requests per window
// and evicts idle entries after ttl. Call Stop() on shutdown.
func NewStore(window time.Duration, burst int, ttl time.Duration) *Store {
	return newStoreWithClock(window, burst, ttl, defaultCleanupInterval, defaultMaxEntries, time.Now)
}

func newStoreWithClock(
	window time.Duration,
	burst int,
	ttl time.Duration,
	cleanupInterval time.Duration,
	maxEntries int,
	now func() time.Time,
) *Store {
	interval := window / time.Duration(burst)
	s := &Store{
		entries:    make(map[string]*entry),
		limit:      rate.Every(interval),
		burst:      burst,
		ttl:        ttl,
		maxEntries: maxEntries,
		now:        now,
		done:       make(chan struct{}),
	}

	if cleanupInterval > 0 {
		go s.cleanupLoop(cleanupInterval)
	}
	return s
}

// Stop terminates the background eviction goroutine.
func (s *Store) Stop() {
	s.stopOnce.Do(func() {
		close(s.done)
	})
}

// Allow checks and consumes one token for every non-empty key atomically.
// If any key is rate-limited, all reservations are cancelled and Retry-After is returned.
func (s *Store) Allow(keys ...string) (bool, time.Duration) {
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()

	type pending struct {
		res *rate.Reservation
	}
	reservations := make([]pending, 0, len(keys))
	var maxDelay time.Duration

	for _, key := range keys {
		if key == "" {
			continue
		}
		e := s.getOrCreateLocked(key, now)
		res := e.limiter.ReserveN(now, 1)
		reservations = append(reservations, pending{res: res})

		if !res.OK() {
			maxDelay = s.ttl
			continue
		}
		if d := res.DelayFrom(now); d > 0 {
			if d > maxDelay {
				maxDelay = d
			}
		}
	}

	if maxDelay > 0 {
		for _, p := range reservations {
			p.res.CancelAt(now)
		}
		if maxDelay < time.Second {
			maxDelay = time.Second
		}
		return false, maxDelay
	}

	return true, 0
}

func (s *Store) getOrCreateLocked(key string, now time.Time) *entry {
	if e, ok := s.entries[key]; ok {
		e.lastSeen = now
		return e
	}
	if len(s.entries) >= s.maxEntries {
		s.evictExpiredLocked(now)
	}
	// If still at capacity after TTL eviction, drop the oldest entry to bound memory.
	if len(s.entries) >= s.maxEntries {
		var oldestKey string
		var oldestTime time.Time
		first := true
		for k, v := range s.entries {
			if first || v.lastSeen.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.lastSeen
				first = false
			}
		}
		if oldestKey != "" {
			delete(s.entries, oldestKey)
		}
	}

	e := &entry{
		limiter:  rate.NewLimiter(s.limit, s.burst),
		lastSeen: now,
	}
	s.entries[key] = e
	return e
}

func (s *Store) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.evictExpired()
		case <-s.done:
			return
		}
	}
}

func (s *Store) evictExpired() {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictExpiredLocked(now)
}

func (s *Store) evictExpiredLocked(now time.Time) {
	for key, e := range s.entries {
		if now.Sub(e.lastSeen) > s.ttl {
			delete(s.entries, key)
		}
	}
}

func (s *Store) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

// AuthLimiters bundles the rate limiter stores for authentication endpoints.
type AuthLimiters struct {
	Login          *Store
	ForgotPassword *Store
	Register       *Store
}

// NewAuthLimiters constructs the three configured auth rate limiters.
func NewAuthLimiters() *AuthLimiters {
	return &AuthLimiters{
		Login:          NewStore(LoginWindow, LoginMaxAttempts, 30*time.Minute),
		ForgotPassword: NewStore(ForgotPasswordWindow, ForgotPasswordMaxAttempts, 2*time.Hour),
		Register:       NewStore(RegisterWindow, RegisterMaxAttempts, 2*time.Hour),
	}
}

// Stop stops all background cleanup tickers.
func (a *AuthLimiters) Stop() {
	if a == nil {
		return
	}
	a.Login.Stop()
	a.ForgotPassword.Stop()
	a.Register.Stop()
}
