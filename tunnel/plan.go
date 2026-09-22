package tunnel

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/muhammad-deve/server/cmd/config"
)

// planInfo is what the local inspector shows about the account's allowance:
// which plan is active, how much traffic it includes per month, and how much of
// that has been used so far.
//
// The CLI only knows about this tunnel's own traffic, so the figures come from
// the GoPort API. Known stays false whenever that call has not succeeded --
// offline, unauthenticated, or self-hosted without billing -- so the UI can
// stay silent rather than invent a quota.
type planInfo struct {
	Known        bool   `json:"known"`
	Plan         string `json:"plan"`
	IsPro        bool   `json:"isPro"`
	MonthlyBytes int64  `json:"monthlyBytes"`
	MonthBytes   int64  `json:"monthBytes"`
}

// Only the fields the inspector needs; the endpoint returns a lot more.
type dashboardPayload struct {
	MonthBytes int64 `json:"monthBytes"`
	Billing    *struct {
		IsPro bool `json:"isPro"`
		Plan  struct {
			Key          string `json:"key"`
			MonthlyBytes int64  `json:"monthlyBytes"`
		} `json:"plan"`
	} `json:"billing"`
}

type planWatcher struct {
	mu   sync.RWMutex
	info planInfo
}

func (p *planWatcher) get() planInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.info
}

func (p *planWatcher) set(info planInfo) {
	p.mu.Lock()
	p.info = info
	p.mu.Unlock()
}

// fetch asks the GoPort API for the account's plan and month-to-date traffic.
// The same BytesForPeriod query backs the enforcement check on the server, so
// the number shown here is the one the limit is actually measured against.
func (p *planWatcher) fetch(ctx context.Context) error {
	if config.Token == "" || config.APIBaseURL == "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, config.APIBaseURL+"/api/v1/dashboard", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", config.Token)

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil // not signed in, or the API is unhappy; stay silent
	}

	var payload dashboardPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}
	if payload.Billing == nil || payload.Billing.Plan.MonthlyBytes <= 0 {
		return nil
	}

	p.set(planInfo{
		Known:        true,
		Plan:         payload.Billing.Plan.Key,
		IsPro:        payload.Billing.IsPro,
		MonthlyBytes: payload.Billing.Plan.MonthlyBytes,
		MonthBytes:   payload.MonthBytes,
	})
	return nil
}

// watch refreshes the plan in the background. Failures are ignored on purpose:
// a tunnel must keep working when the API is unreachable, and the inspector
// simply omits the allowance until a refresh succeeds.
func (p *planWatcher) watch(interval time.Duration) {
	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			_ = p.fetch(ctx)
			cancel()
			time.Sleep(interval)
		}
	}()
}
