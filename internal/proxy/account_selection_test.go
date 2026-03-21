package proxy

import "testing"

func TestNextSkipsIneligibleAccounts(t *testing.T) {
	pool := &AccountPool{
		accounts: []*Account{
			{
				PlanType:  "personal",
				UserEmail: "blocked@example.com",
				QuotaInfo: &QuotaInfo{
					IsEligible:     false,
					HasPremium:     true,
					PremiumBalance: 1300000,
					PremiumLimit:   1300000,
				},
			},
			{
				UserEmail: "eligible@example.com",
				QuotaInfo: &QuotaInfo{
					IsEligible: true,
				},
			},
		},
	}

	got := pool.Next()
	if got == nil || got.UserEmail != "eligible@example.com" {
		t.Fatalf("expected eligible account, got %#v", got)
	}
}

func TestGetBestAccountPrefersEligibleAccount(t *testing.T) {
	pool := &AccountPool{
		accounts: []*Account{
			{
				PlanType:  "personal",
				UserEmail: "blocked@example.com",
				QuotaInfo: &QuotaInfo{
					IsEligible:     false,
					HasPremium:     true,
					PremiumBalance: 1300000,
					PremiumLimit:   1300000,
					SpaceLimit:     200,
					SpaceUsage:     180,
				},
			},
			{
				UserEmail: "eligible@example.com",
				QuotaInfo: &QuotaInfo{
					IsEligible: true,
					SpaceLimit: 200,
					SpaceUsage: 20,
				},
			},
		},
	}

	got := pool.GetBestAccount()
	if got == nil || got.UserEmail != "eligible@example.com" {
		t.Fatalf("expected eligible account, got %#v", got)
	}
}

func TestGetBestAccountReturnsNilWhenOnlyIneligibleAccounts(t *testing.T) {
	pool := &AccountPool{
		accounts: []*Account{
			{
				PlanType:  "business",
				UserEmail: "blocked@example.com",
				QuotaInfo: &QuotaInfo{
					IsEligible:     false,
					HasPremium:     true,
					PremiumBalance: 1300000,
					PremiumLimit:   1300000,
				},
			},
		},
	}

	if got := pool.GetBestAccount(); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}
}

func TestIsQuotaExhaustedUsesEligibilityFlag(t *testing.T) {
	pool := &AccountPool{
		accounts: []*Account{
			{
				PlanType:  "personal",
				UserEmail: "personal@example.com",
				QuotaInfo: &QuotaInfo{
					IsEligible:     false,
					HasPremium:     true,
					PremiumBalance: 1300000,
					PremiumLimit:   1300000,
				},
			},
		},
	}

	if got := pool.isQuotaExhausted(pool.accounts[0]); !got {
		t.Fatalf("expected account with is_eligible=false to be exhausted")
	}
}

func TestGetBestAccountUsesEffectiveRemaining(t *testing.T) {
	pool := &AccountPool{
		accounts: []*Account{
			{
				UserEmail: "space-rich-user-poor@example.com",
				QuotaInfo: &QuotaInfo{
					IsEligible: true,
					SpaceLimit: 200,
					SpaceUsage: 20,
					UserLimit:  200,
					UserUsage:  190,
				},
			},
			{
				UserEmail: "balanced@example.com",
				QuotaInfo: &QuotaInfo{
					IsEligible: true,
					SpaceLimit: 200,
					SpaceUsage: 60,
					UserLimit:  200,
					UserUsage:  60,
				},
			},
		},
	}

	got := pool.GetBestAccount()
	if got == nil || got.UserEmail != "balanced@example.com" {
		t.Fatalf("expected account with higher effective remaining, got %#v", got)
	}
}

func TestNextPrefersHigherEffectiveRemaining(t *testing.T) {
	pool := &AccountPool{
		accounts: []*Account{
			{
				UserEmail: "low@example.com",
				QuotaInfo: &QuotaInfo{
					IsEligible: true,
					SpaceLimit: 200,
					SpaceUsage: 160,
					UserLimit:  200,
					UserUsage:  160,
				},
			},
			{
				UserEmail: "high@example.com",
				QuotaInfo: &QuotaInfo{
					IsEligible: true,
					SpaceLimit: 200,
					SpaceUsage: 40,
					UserLimit:  200,
					UserUsage:  40,
				},
			},
		},
	}

	got := pool.Next()
	if got == nil || got.UserEmail != "high@example.com" {
		t.Fatalf("expected highest-remaining account, got %#v", got)
	}
}

func TestNextExcludingPrefersBestRemainingAmongUntried(t *testing.T) {
	low := &Account{
		UserEmail: "low@example.com",
		QuotaInfo: &QuotaInfo{
			IsEligible: true,
			SpaceLimit: 200,
			SpaceUsage: 170,
			UserLimit:  200,
			UserUsage:  170,
		},
	}
	mid := &Account{
		UserEmail: "mid@example.com",
		QuotaInfo: &QuotaInfo{
			IsEligible: true,
			SpaceLimit: 200,
			SpaceUsage: 80,
			UserLimit:  200,
			UserUsage:  80,
		},
	}
	high := &Account{
		UserEmail: "high@example.com",
		QuotaInfo: &QuotaInfo{
			IsEligible: true,
			SpaceLimit: 200,
			SpaceUsage: 20,
			UserLimit:  200,
			UserUsage:  20,
		},
	}
	pool := &AccountPool{
		accounts: []*Account{low, mid, high},
	}

	got := pool.NextExcluding(map[*Account]bool{high: true})
	if got == nil || got.UserEmail != "mid@example.com" {
		t.Fatalf("expected best remaining untried account, got %#v", got)
	}
}

func TestNextForResearchAllowsPremiumAtLimit(t *testing.T) {
	pool := &AccountPool{
		accounts: []*Account{
			{
				UserEmail: "premium@example.com",
				QuotaInfo: &QuotaInfo{
					IsEligible:        true,
					HasPremium:        true,
					ResearchModeUsage: 3,
				},
			},
		},
	}

	got := pool.NextForResearch()
	if got == nil || got.UserEmail != "premium@example.com" {
		t.Fatalf("expected premium account to remain research-capable, got %#v", got)
	}
}

func TestBasicRemainingUsesMostConstrainedQuota(t *testing.T) {
	info := &QuotaInfo{
		SpaceLimit: 200,
		SpaceUsage: 20,
		UserLimit:  200,
		UserUsage:  190,
	}

	if got := basicRemaining(info); got != 10 {
		t.Fatalf("expected effective remaining 10, got %d", got)
	}
}

func TestIsFreePlanTreatsPersonalWithPremiumAsPaid(t *testing.T) {
	acc := &Account{
		PlanType: "personal",
		QuotaInfo: &QuotaInfo{
			HasPremium:     true,
			PremiumBalance: 1300000,
			PremiumLimit:   1300000,
		},
	}

	if isFreePlan(acc) {
		t.Fatal("expected personal account with premium credits to be treated as paid")
	}
}
