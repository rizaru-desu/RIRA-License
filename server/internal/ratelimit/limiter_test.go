// SPDX-License-Identifier: AGPL-3.0-or-later
//
// RiraLicense — license server & API
// Copyright (C) 2026 RiraLab — Rizaru Desu. All rights reserved.

package ratelimit

import (
	"testing"
	"time"
)

func TestStoreRateLimitAndTTLEviction(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	s := newStoreWithClock(15*time.Minute, 5, 30*time.Minute, 0, 100, clock)
	defer s.Stop()

	for i := 0; i < 5; i++ {
		ok, delay := s.Allow("ip:1.2.3.4", "acct:a@b.com")
		if !ok || delay != 0 {
			t.Fatalf("attempt %d should be allowed, got ok=%v delay=%v", i+1, ok, delay)
		}
	}

	ok, delay := s.Allow("ip:1.2.3.4", "acct:a@b.com")
	if ok || delay <= 0 {
		t.Fatalf("6th attempt should be rate limited, got ok=%v delay=%v", ok, delay)
	}

	// Because the IP failed, the other account should NOT have lost its token.
	ok, _ = s.Allow("ip:9.9.9.9", "acct:other@b.com")
	if !ok {
		t.Fatal("unrelated key should be allowed")
	}

	// Advance clock by 3 minutes (one refill interval = 15m/5 = 3m).
	now = now.Add(3 * time.Minute)
	ok, _ = s.Allow("ip:1.2.3.4", "acct:a@b.com")
	if !ok {
		t.Fatal("expected token refill after 3 minutes")
	}

	// Advance past TTL and verify eviction cleans up idle entries.
	now = now.Add(31 * time.Minute)
	s.evictExpired()
	if s.len() != 0 {
		t.Fatalf("expected 0 entries after TTL eviction, got %d", s.len())
	}
}
