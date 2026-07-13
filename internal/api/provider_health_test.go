package api

import (
	"sync"
	"testing"
	"time"
)

func TestProviderHealthHealthyRegistersMissingProvider(t *testing.T) {
	health := newProviderHealth(ProviderHealthOptions{})

	if !health.Healthy("provider-a") {
		t.Fatal("new provider should be healthy")
	}

	snapshot := health.Snapshot()
	if len(snapshot) != 1 || snapshot[0].Provider != "provider-a" || !snapshot[0].Healthy {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
}

func TestProviderHealthRecoversAfterCooldown(t *testing.T) {
	now := time.Unix(100, 0)
	health := newProviderHealth(ProviderHealthOptions{FailureThreshold: 1, Cooldown: time.Minute})
	health.now = func() time.Time { return now }

	if opened := health.MarkFailure("provider-a"); !opened {
		t.Fatal("expected circuit to open")
	}
	if health.Healthy("provider-a") {
		t.Fatal("provider should be unhealthy during cooldown")
	}

	now = now.Add(time.Minute)
	if !health.Healthy("provider-a") {
		t.Fatal("provider should recover after cooldown")
	}
}

func TestProviderHealthConcurrentAccess(t *testing.T) {
	health := newProviderHealth(ProviderHealthOptions{FailureThreshold: 2, Cooldown: time.Millisecond})
	health.Register("provider-a")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = health.Healthy("provider-a")
				_ = health.MarkFailure("provider-a")
				health.MarkSuccess("provider-a")
				_ = health.Snapshot()
			}
		}()
	}
	wg.Wait()
}
