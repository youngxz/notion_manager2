package proxy

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	mrand "math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type AccountPool struct {
	mu       sync.RWMutex
	accounts []*Account
	index    atomic.Uint64

	// Refresh state (protected by refreshMu)
	refreshMu      sync.RWMutex
	refreshing     bool
	refreshDone    int
	refreshTotal   int
	lastRefreshAt  *time.Time
	lastRefreshErr string
}

func NewAccountPool() *AccountPool {
	return &AccountPool{}
}

func (p *AccountPool) LoadFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read accounts dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[account] skip %s: %v", entry.Name(), err)
			continue
		}
		var acc Account
		if err := json.Unmarshal(data, &acc); err != nil {
			log.Printf("[account] skip %s: %v", entry.Name(), err)
			continue
		}
		if acc.TokenV2 == "" || acc.UserID == "" || acc.SpaceID == "" {
			log.Printf("[account] skip %s: missing required fields", entry.Name())
			continue
		}
		if acc.TokenV2 == "YOUR_TOKEN_V2_HERE" || strings.HasPrefix(acc.UserID, "xxxxxxxx") {
			log.Printf("[account] skip %s: placeholder/example config", entry.Name())
			continue
		}
		if acc.BrowserID == "" {
			acc.BrowserID = generateUUIDv4()
		}
		if acc.ClientVersion == "" || acc.ClientVersion == "unknown" {
			acc.ClientVersion = DefaultClientVersion
		}
		// Load persisted quota info (snake_case keys) into runtime QuotaInfo
		acc.QuotaInfo = loadPersistedQuotaInfo(data)
		p.accounts = append(p.accounts, &acc)
		log.Printf("[account] loaded: %s (%s) [%s]", acc.UserName, acc.UserEmail, acc.PlanType)
	}

	if len(p.accounts) == 0 {
		return fmt.Errorf("no valid accounts found in %s", dir)
	}
	log.Printf("[account] total: %d accounts loaded", len(p.accounts))
	return nil
}

func (p *AccountPool) LoadSingle(tokenFile string) error {
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		return err
	}

	var acc Account
	if err := json.Unmarshal(data, &acc); err != nil {
		// Treat as plain token file
		acc = Account{
			TokenV2:       string(data),
			UserID:        "322d872b-594c-816e-b8ce-00022c725bb3",
			SpaceID:       "176faced-55bd-8161-bbbf-000339934d27",
			UserName:      "default",
			SpaceName:     "default",
			Timezone:      "UTC",
			ClientVersion: DefaultClientVersion,
			BrowserID:     generateUUIDv4(),
		}
	}
	if acc.BrowserID == "" {
		acc.BrowserID = generateUUIDv4()
	}
	if acc.ClientVersion == "" || acc.ClientVersion == "unknown" {
		acc.ClientVersion = DefaultClientVersion
	}
	p.accounts = append(p.accounts, &acc)
	log.Printf("[account] loaded single account: %s", acc.UserName)
	return nil
}

func (p *AccountPool) Next() *Account {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pickBestAccountLocked(nil)
}

// NextForResearch returns the next available account suitable for research mode.
// Premium accounts are treated as research-capable regardless of research_usage.
// For non-premium accounts, it prefers research_usage < 3 and falls back if needed.
func (p *AccountPool) NextForResearch() *Account {
	p.mu.RLock()
	defer p.mu.RUnlock()
	n := len(p.accounts)
	if n == 0 {
		return nil
	}
	start := p.index.Add(1) - 1
	// First pass: prefer premium accounts and accounts with lower research usage.
	var fallback *Account
	bestUsage := int(^uint(0) >> 1)
	for i := 0; i < n; i++ {
		acc := p.accounts[(start+uint64(i))%uint64(n)]
		if p.isQuotaExhausted(acc) {
			continue
		}
		if acc.QuotaInfo == nil {
			if fallback == nil {
				fallback = acc
			}
			continue
		}
		if acc.QuotaInfo.HasPremium {
			return acc
		}
		if acc.QuotaInfo.ResearchModeUsage < 3 {
			if fallback == nil || acc.QuotaInfo.ResearchModeUsage < bestUsage {
				fallback = acc
				bestUsage = acc.QuotaInfo.ResearchModeUsage
			}
			continue
		}
		if fallback == nil {
			fallback = acc // keep a usable fallback if no fresher account exists
		}
	}
	return fallback // nil if all exhausted
}

// NextExcluding returns the next available account excluding the given ones (for retry)
func (p *AccountPool) NextExcluding(exclude map[*Account]bool) *Account {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pickBestAccountLocked(exclude)
}

// pickBestAccountLocked returns the available account with the highest
// effective remaining quota. Ties are broken by the rotating start index so
// equally healthy accounts still share traffic.
func (p *AccountPool) pickBestAccountLocked(exclude map[*Account]bool) *Account {
	n := len(p.accounts)
	if n == 0 {
		return nil
	}
	start := p.index.Add(1) - 1
	var best *Account
	bestScore := -1
	for i := 0; i < n; i++ {
		acc := p.accounts[(start+uint64(i))%uint64(n)]
		if exclude != nil && exclude[acc] {
			continue
		}
		if p.isQuotaExhausted(acc) {
			continue
		}
		score := accountQuotaPriority(acc)
		if best == nil || score > bestScore {
			best = acc
			bestScore = score
		}
	}
	return best
}

// MarkQuotaExhausted marks an account as quota-exhausted with a timestamp.
// Recovery only happens when RefreshAll confirms isEligible=true via API.
// Free plan accounts (200 lifetime credits) will stay exhausted permanently.
// Paid plan accounts recover when monthly credits reset at billing cycle boundary.
func (p *AccountPool) MarkQuotaExhausted(acc *Account) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if acc.QuotaExhaustedAt != nil {
		return // already marked
	}
	now := time.Now()
	acc.QuotaExhaustedAt = &now
	log.Printf("[quota] marked %s (%s) as exhausted (recovery via API re-check only)", acc.UserName, acc.UserEmail)
}

// ClearQuotaExhausted removes the exhausted mark (called when API confirms recovery)
func (p *AccountPool) ClearQuotaExhausted(acc *Account) {
	p.mu.Lock()
	defer p.mu.Unlock()
	acc.QuotaExhaustedAt = nil
	acc.PermanentlyExhausted = false
}

func (p *AccountPool) isQuotaExhausted(acc *Account) bool {
	if acc.PermanentlyExhausted {
		return true
	}
	if acc.QuotaInfo != nil {
		return !acc.QuotaInfo.IsEligible
	}
	if acc.QuotaExhaustedAt == nil {
		return false
	}
	return true
}

// MarkPermanentlyExhausted marks a free-plan account as permanently exhausted (never recovers).
func (p *AccountPool) MarkPermanentlyExhausted(acc *Account) {
	p.mu.Lock()
	defer p.mu.Unlock()
	acc.PermanentlyExhausted = true
	now := time.Now()
	acc.QuotaExhaustedAt = &now
	log.Printf("[quota] marked %s (%s) as PERMANENTLY exhausted (free plan, no recovery)", acc.UserName, acc.UserEmail)
}

// AvailableCount returns the number of accounts not currently quota-exhausted
func (p *AccountPool) AvailableCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	count := 0
	for _, acc := range p.accounts {
		if !p.isQuotaExhausted(acc) {
			count++
		}
	}
	return count
}

func (p *AccountPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.accounts)
}

// GetByEmail returns an available (non-exhausted) account by email, or nil if not found/exhausted.
func (p *AccountPool) GetByEmail(email string) *Account {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, acc := range p.accounts {
		if acc.UserEmail == email && !p.isQuotaExhausted(acc) {
			return acc
		}
	}
	return nil
}

func (p *AccountPool) AllModels() []ModelEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()
	seen := map[string]bool{}
	var models []ModelEntry
	for _, acc := range p.accounts {
		for _, m := range acc.Models {
			if !seen[m.ID] {
				seen[m.ID] = true
				models = append(models, m)
			}
		}
	}
	if len(models) == 0 {
		// Return default models if none loaded
		for name, id := range DefaultModelMap {
			models = append(models, ModelEntry{Name: name, ID: id})
		}
	}
	return models
}

// GetRefreshStatus returns the current refresh state for the API
func (p *AccountPool) GetRefreshStatus() map[string]interface{} {
	p.refreshMu.RLock()
	defer p.refreshMu.RUnlock()
	status := map[string]interface{}{
		"refreshing": p.refreshing,
		"done":       p.refreshDone,
		"total":      p.refreshTotal,
	}
	if p.lastRefreshAt != nil {
		status["last_refresh_at"] = p.lastRefreshAt.Format(time.RFC3339)
	}
	if p.lastRefreshErr != "" {
		status["error"] = p.lastRefreshErr
	}
	return status
}

// TriggerRefresh starts RefreshAll in a background goroutine if not already running.
// Returns true if a new refresh was started, false if one is already in progress.
func (p *AccountPool) TriggerRefresh(accountsDir string) bool {
	p.refreshMu.Lock()
	if p.refreshing {
		p.refreshMu.Unlock()
		return false
	}
	p.refreshMu.Unlock()
	go p.RefreshAll(accountsDir)
	return true
}

// RefreshAll proactively checks AI quota and fetches models for all accounts via Notion API.
// It also persists updated info back to account JSON files.
func (p *AccountPool) RefreshAll(accountsDir string) {
	p.refreshMu.Lock()
	if p.refreshing {
		p.refreshMu.Unlock()
		return // already running
	}
	p.refreshing = true
	p.refreshDone = 0
	p.lastRefreshErr = ""
	p.refreshMu.Unlock()

	defer func() {
		p.refreshMu.Lock()
		p.refreshing = false
		now := time.Now()
		p.lastRefreshAt = &now
		p.refreshMu.Unlock()
	}()

	p.mu.RLock()
	accs := make([]*Account, len(p.accounts))
	copy(accs, p.accounts)
	p.mu.RUnlock()

	p.refreshMu.Lock()
	p.refreshTotal = len(accs)
	p.refreshMu.Unlock()

	log.Printf("[refresh] refreshing %d accounts (quota + models)...", len(accs))
	modelsUpdated := false

	for _, acc := range accs {
		// Skip permanently exhausted accounts (free plan, no recovery possible)
		if acc.PermanentlyExhausted {
			if isFreePlan(acc) {
				log.Printf("[refresh] %s (%s): ⛔ permanently exhausted (skipped)", acc.UserName, acc.UserEmail)
				continue
			}
			log.Printf("[refresh] %s (%s): clearing stale permanent exhaustion flag before re-check", acc.UserName, acc.UserEmail)
			p.ClearQuotaExhausted(acc)
		}

		// 1. Check quota
		info, err := CheckQuota(acc)
		if err != nil {
			log.Printf("[refresh] %s (%s): quota check failed: %v", acc.UserName, acc.UserEmail, err)
		} else {
			now := time.Now()
			acc.QuotaInfo = info
			acc.QuotaCheckedAt = &now

			if info.IsEligible {
				remaining := basicRemaining(info)
				premiumInfo := ""
				if info.HasPremium {
					premiumInfo = fmt.Sprintf(", premium %d/%d", info.PremiumUsage, info.PremiumLimit)
				}
				if info.ResearchModeUsage > 0 {
					premiumInfo += fmt.Sprintf(", research=%d", info.ResearchModeUsage)
				}
				// If was exhausted, clear the flag — API confirmed recovery
				if acc.QuotaExhaustedAt != nil {
					log.Printf("[refresh] %s (%s): ✅ RECOVERED! (space %d/%d, user %d/%d, remaining ~%d%s)",
						acc.UserName, acc.UserEmail, info.SpaceUsage, info.SpaceLimit, info.UserUsage, info.UserLimit, remaining, premiumInfo)
					p.ClearQuotaExhausted(acc)
				} else {
					log.Printf("[refresh] %s (%s): ✅ eligible (space %d/%d, user %d/%d, remaining ~%d%s)",
						acc.UserName, acc.UserEmail, info.SpaceUsage, info.SpaceLimit, info.UserUsage, info.UserLimit, remaining, premiumInfo)
				}
			} else {
				if isFreePlan(acc) {
					log.Printf("[refresh] %s (%s): ❌ NOT eligible (space %d/%d, user %d/%d, marking permanent)",
						acc.UserName, acc.UserEmail, info.SpaceUsage, info.SpaceLimit, info.UserUsage, info.UserLimit)
					p.MarkPermanentlyExhausted(acc)
				} else {
					log.Printf("[refresh] %s (%s): ❌ NOT eligible (space %d/%d, user %d/%d)",
						acc.UserName, acc.UserEmail, info.SpaceUsage, info.SpaceLimit, info.UserUsage, info.UserLimit)
					p.MarkQuotaExhausted(acc)
				}
			}
		}

		p.refreshMu.Lock()
		p.refreshDone++
		p.refreshMu.Unlock()

		// 2. Fetch models
		models, err := FetchModels(acc)
		if err != nil {
			log.Printf("[refresh] %s (%s): model fetch failed: %v", acc.UserName, acc.UserEmail, err)
		} else if len(models) > 0 {
			acc.Models = models
			modelsUpdated = true
			log.Printf("[refresh] %s (%s): fetched %d models", acc.UserName, acc.UserEmail, len(models))
		}
	}

	if modelsUpdated {
		// Update DefaultModelMap from fetched models
		p.mu.Lock()
		for _, acc := range p.accounts {
			for _, m := range acc.Models {
				// Build reverse mapping: display name → internal ID
				normalizedName := normalizeModelName(m.Name)
				if normalizedName != "" {
					DefaultModelMap[normalizedName] = m.ID
				}
			}
		}
		p.mu.Unlock()
	}

	// 3. Persist to disk
	if accountsDir != "" {
		p.SaveAccounts(accountsDir)
	}

	log.Printf("[refresh] complete: %d/%d accounts available", p.AvailableCount(), len(accs))
}

// normalizeModelName converts display name like "GPT-5.2" to a user-friendly alias like "gpt-5.2"
func normalizeModelName(displayName string) string {
	s := strings.ToLower(strings.TrimSpace(displayName))
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

// SaveAccounts persists current account state (models, quota) back to JSON files
func (p *AccountPool) SaveAccounts(dir string) {
	p.mu.RLock()
	accs := make([]*Account, len(p.accounts))
	copy(accs, p.accounts)
	p.mu.RUnlock()

	for _, acc := range accs {
		// Find the matching file by user_email
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var existing map[string]interface{}
			if err := json.Unmarshal(data, &existing); err != nil {
				continue
			}
			email, _ := existing["user_email"].(string)
			if email != acc.UserEmail {
				continue
			}

			// Update models
			if len(acc.Models) > 0 {
				var modelEntries []map[string]string
				for _, m := range acc.Models {
					modelEntries = append(modelEntries, map[string]string{"id": m.ID, "name": m.Name})
				}
				existing["available_models"] = modelEntries
			}

			// Update quota info
			if acc.QuotaInfo != nil {
				existing["quota_info"] = map[string]interface{}{
					"is_eligible":         acc.QuotaInfo.IsEligible,
					"space_usage":         acc.QuotaInfo.SpaceUsage,
					"space_limit":         acc.QuotaInfo.SpaceLimit,
					"user_usage":          acc.QuotaInfo.UserUsage,
					"user_limit":          acc.QuotaInfo.UserLimit,
					"last_usage_at":       acc.QuotaInfo.LastUsageAtMs,
					"research_mode_usage": acc.QuotaInfo.ResearchModeUsage,
					"has_premium":         acc.QuotaInfo.HasPremium,
					"premium_balance":     acc.QuotaInfo.PremiumBalance,
					"premium_usage":       acc.QuotaInfo.PremiumUsage,
					"premium_limit":       acc.QuotaInfo.PremiumLimit,
				}
			}
			if acc.QuotaCheckedAt != nil {
				existing["quota_checked_at"] = acc.QuotaCheckedAt.Format(time.RFC3339)
			}

			// Write back
			out, err := json.MarshalIndent(existing, "", "  ")
			if err != nil {
				continue
			}
			os.WriteFile(path, append(out, '\n'), 0644)
			break
		}
	}
}

// StartRefreshLoop runs a background goroutine that periodically refreshes all accounts
func (p *AccountPool) StartRefreshLoop(interval time.Duration, accountsDir string) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			p.RefreshAll(accountsDir)
		}
	}()
}

// GetAccountDetails returns detailed info for all accounts (for admin dashboard)
func (p *AccountPool) GetAccountDetails() []map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var details []map[string]interface{}
	for _, acc := range p.accounts {
		entry := map[string]interface{}{
			"email":     acc.UserEmail,
			"name":      acc.UserName,
			"plan":      acc.PlanType,
			"space":     acc.SpaceName,
			"exhausted": p.isQuotaExhausted(acc),
			"permanent": acc.PermanentlyExhausted,
		}
		if acc.QuotaInfo != nil {
			entry["eligible"] = acc.QuotaInfo.IsEligible
			entry["usage"] = acc.QuotaInfo.SpaceUsage
			entry["limit"] = acc.QuotaInfo.SpaceLimit
			entry["space_usage"] = acc.QuotaInfo.SpaceUsage
			entry["space_limit"] = acc.QuotaInfo.SpaceLimit
			entry["space_remaining"] = quotaRemaining(acc.QuotaInfo.SpaceLimit, acc.QuotaInfo.SpaceUsage)
			entry["user_usage"] = acc.QuotaInfo.UserUsage
			entry["user_limit"] = acc.QuotaInfo.UserLimit
			entry["user_remaining"] = quotaRemaining(acc.QuotaInfo.UserLimit, acc.QuotaInfo.UserUsage)
			entry["remaining"] = basicRemaining(acc.QuotaInfo)
			entry["last_usage_at"] = acc.QuotaInfo.LastUsageAtMs
			// Research mode (V1)
			entry["research_usage"] = acc.QuotaInfo.ResearchModeUsage
			// Premium credit data (V2)
			entry["has_premium"] = acc.QuotaInfo.HasPremium
			entry["premium_balance"] = acc.QuotaInfo.PremiumBalance
			entry["premium_usage"] = acc.QuotaInfo.PremiumUsage
			entry["premium_limit"] = acc.QuotaInfo.PremiumLimit
		}
		if acc.QuotaCheckedAt != nil {
			entry["checked_at"] = acc.QuotaCheckedAt.Format(time.RFC3339)
		}
		if p.isQuotaExhausted(acc) && acc.QuotaExhaustedAt != nil {
			entry["exhausted_at"] = acc.QuotaExhaustedAt.Format(time.RFC3339)
		}
		// Models
		var models []map[string]string
		for _, m := range acc.Models {
			models = append(models, map[string]string{"id": m.ID, "name": m.Name})
		}
		entry["models"] = models
		details = append(details, entry)
	}
	return details
}

// GetQuotaSummary returns quota summary for all accounts (for /health endpoint)
func (p *AccountPool) GetQuotaSummary() []map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var summary []map[string]interface{}
	for _, acc := range p.accounts {
		entry := map[string]interface{}{
			"email":     acc.UserEmail,
			"name":      acc.UserName,
			"plan":      acc.PlanType,
			"exhausted": p.isQuotaExhausted(acc),
			"permanent": acc.PermanentlyExhausted,
		}
		if acc.QuotaInfo != nil {
			entry["eligible"] = acc.QuotaInfo.IsEligible
			entry["usage"] = acc.QuotaInfo.SpaceUsage
			entry["limit"] = acc.QuotaInfo.SpaceLimit
			entry["space_usage"] = acc.QuotaInfo.SpaceUsage
			entry["space_limit"] = acc.QuotaInfo.SpaceLimit
			entry["space_remaining"] = quotaRemaining(acc.QuotaInfo.SpaceLimit, acc.QuotaInfo.SpaceUsage)
			entry["user_usage"] = acc.QuotaInfo.UserUsage
			entry["user_limit"] = acc.QuotaInfo.UserLimit
			entry["user_remaining"] = quotaRemaining(acc.QuotaInfo.UserLimit, acc.QuotaInfo.UserUsage)
			entry["remaining"] = basicRemaining(acc.QuotaInfo)
			entry["last_usage_at"] = acc.QuotaInfo.LastUsageAtMs
			// Research mode (V1)
			entry["research_usage"] = acc.QuotaInfo.ResearchModeUsage
			// Premium credit data (V2)
			entry["has_premium"] = acc.QuotaInfo.HasPremium
			entry["premium_balance"] = acc.QuotaInfo.PremiumBalance
			entry["premium_usage"] = acc.QuotaInfo.PremiumUsage
			entry["premium_limit"] = acc.QuotaInfo.PremiumLimit
		}
		if acc.QuotaCheckedAt != nil {
			entry["checked_at"] = acc.QuotaCheckedAt.Format(time.RFC3339)
		}
		summary = append(summary, entry)
	}
	return summary
}

// loadPersistedQuotaInfo parses the persisted quota_info (snake_case keys) from raw account JSON.
// Returns nil if quota_info is not present or cannot be parsed.
func loadPersistedQuotaInfo(data []byte) *QuotaInfo {
	var raw struct {
		QuotaInfo *struct {
			IsEligible        bool  `json:"is_eligible"`
			SpaceUsage        int   `json:"space_usage"`
			SpaceLimit        int   `json:"space_limit"`
			UserUsage         int   `json:"user_usage"`
			UserLimit         int   `json:"user_limit"`
			LastUsageAt       int64 `json:"last_usage_at"`
			ResearchModeUsage int   `json:"research_mode_usage"`
			HasPremium        bool  `json:"has_premium"`
			PremiumBalance    int   `json:"premium_balance"`
			PremiumUsage      int   `json:"premium_usage"`
			PremiumLimit      int   `json:"premium_limit"`
		} `json:"quota_info"`
	}
	if err := json.Unmarshal(data, &raw); err != nil || raw.QuotaInfo == nil {
		return nil
	}
	return &QuotaInfo{
		IsEligible:        raw.QuotaInfo.IsEligible,
		SpaceUsage:        raw.QuotaInfo.SpaceUsage,
		SpaceLimit:        raw.QuotaInfo.SpaceLimit,
		UserUsage:         raw.QuotaInfo.UserUsage,
		UserLimit:         raw.QuotaInfo.UserLimit,
		LastUsageAtMs:     raw.QuotaInfo.LastUsageAt,
		ResearchModeUsage: raw.QuotaInfo.ResearchModeUsage,
		HasPremium:        raw.QuotaInfo.HasPremium,
		PremiumBalance:    raw.QuotaInfo.PremiumBalance,
		PremiumUsage:      raw.QuotaInfo.PremiumUsage,
		PremiumLimit:      raw.QuotaInfo.PremiumLimit,
	}
}

func quotaRemaining(limit, usage int) int {
	if limit <= 0 {
		return 0
	}
	remaining := limit - usage
	if remaining < 0 {
		return 0
	}
	return remaining
}

func basicRemaining(info *QuotaInfo) int {
	if info == nil {
		return 0
	}
	remaining := []int{}
	if info.SpaceLimit > 0 {
		remaining = append(remaining, quotaRemaining(info.SpaceLimit, info.SpaceUsage))
	}
	if info.UserLimit > 0 {
		remaining = append(remaining, quotaRemaining(info.UserLimit, info.UserUsage))
	}
	if len(remaining) == 0 {
		return 0
	}
	best := remaining[0]
	for _, value := range remaining[1:] {
		if value < best {
			best = value
		}
	}
	return best
}

func accountQuotaPriority(acc *Account) int {
	if acc == nil || acc.QuotaInfo == nil {
		return -1
	}
	return basicRemaining(acc.QuotaInfo)
}

func generateUUIDv4() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		// Fallback
		for i := range b {
			b[i] = byte(mrand.Intn(256))
		}
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
