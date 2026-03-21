# notion-manager Dashboard — Frontend Development Guide

## Tech Stack

- **Framework**: React 19 + TypeScript
- **Build**: Vite 6
- **Styling**: TailwindCSS v4 (via `@tailwindcss/vite` plugin)
- **Embedding**: Go `//go:embed` — compiled dist is embedded into the Go binary

## Project Structure

```
web/
├── index.html          # HTML entry point
├── package.json        # Dependencies
├── vite.config.ts      # Vite config (base: /dashboard/, proxy to :8081)
├── tsconfig.json       # TypeScript config
└── src/
    ├── main.tsx        # React root mount
    ├── App.tsx         # Main dashboard component (Header, Summary, Grid)
    ├── api.ts          # API calls (/admin/accounts, /proxy/start)
    ├── types.ts        # TypeScript interfaces (AccountInfo, DashboardData, Model)
    ├── utils.ts        # Helpers (avatarColor, quota status, formatting)
    ├── index.css       # TailwindCSS imports + theme variables
    └── vite-env.d.ts   # Vite type reference
```

## Build & Deploy

```bash
# Development (hot reload, proxies API to localhost:8081)
cd web && npm run dev

# Production build
make web          # builds frontend + copies to internal/web/dist/
make build        # builds frontend + Go binary
make build-go     # Go binary only (uses existing internal/web/dist/)
```

The build pipeline:
1. `npm run build` → `web/dist/`
2. `xcopy web/dist internal/web/dist/` (embedded into Go binary)
3. `go build` with `//go:embed dist/*` in `internal/web/embed.go`
4. Go serves at `/dashboard/` with API key injected into `<meta name="api-key">`

## Design System

### Theme (Notion Dark)

All theme colors are defined as CSS custom properties in `src/index.css` under `@theme`:

| Token              | Value                    | Usage                     |
|--------------------|--------------------------|---------------------------|
| `--color-bg-primary`   | `#191919`            | Page background           |
| `--color-bg-secondary` | `#202020`            | Header background         |
| `--color-bg-card`      | `#252525`            | Card background           |
| `--color-bg-card-hover`| `#2f2f2f`            | Card hover state          |
| `--color-bg-exhausted` | `#2a1f1f`            | Exhausted account card bg |
| `--color-text-primary` | `#ebebea`            | Primary text              |
| `--color-text-secondary`| `#9b9a97`           | Secondary/label text      |
| `--color-text-muted`   | `#5a5a5a`            | Muted/timestamp text      |
| `--color-notion-blue`  | `#2383e2`            | Accent / links / buttons  |
| `--color-ok`           | `#4dab9a`            | Available / healthy       |
| `--color-warn`         | `#d9a651`            | Low quota warning         |
| `--color-err`          | `#eb5757`            | Exhausted / error         |
| `--color-research`     | `#b39ddb`            | Research mode badge       |

Use TailwindCSS utility classes with these tokens, e.g. `bg-bg-card`, `text-text-primary`, `text-ok`.

### Typography

- Font: system font stack (`-apple-system, BlinkMacSystemFont, "Segoe UI", ...`)
- Summary stat values: `text-2xl font-bold tabular-nums`
- Card title: `text-[13px] font-semibold`
- Labels: `text-[11px] text-text-secondary uppercase tracking-wider`
- Timestamps: `text-[10px] text-text-muted`

### Components

- **StatCard**: Summary metric with label, value, sub-text
- **TotalQuotaBar**: Full-width progress bar with color-coded fill
- **AccountCard**: Clickable card with avatar, badges, quota bar, model pills
- **Badge**: Small pill with variant styling (`paid`, `free`, `research`, `exhausted`, `ok`, `model`)
- **QuotaBar**: Thin progress bar with color based on usage percentage

### Quota Display Convention

**Important**: Quota bars and numbers show **已使用 / 总额度** (used / total) format.
- The progress bar fill represents **usage** (how much has been consumed)
- Color coding: green (<70%), yellow (70-90%), red (>90%)
- Do NOT label as "配额" alone — it's ambiguous. Use "已使用" or "used / total" phrasing
- The summary card shows **剩余** (remaining) which is `total - used`

### Account Status

| Status    | Dot Color | Card Style              | Badge       |
|-----------|-----------|-------------------------|-------------|
| Available | Green     | Default bg              | `✓ 可用`    |
| Low quota | Yellow    | Default bg              | `✓ 可用`    |
| Exhausted | Red       | `bg-bg-exhausted`       | `⛔ 耗尽`   |
| Permanent | Red       | `bg-bg-exhausted` + 55% opacity | `⛔ 耗尽` |

### Research Mode

Research mode is available on **paid plans only** (`plus`, `business`, `enterprise`, `team`).
- Show `🔬 Research` badge on paid plan accounts
- Research mode uses the same AI quota but consumes significantly more per query (7+ search rounds, 3-5 min per query)
- Research mode uses request type `"researcher"` instead of `"workflow"`

## API Integration

### Data Source

The dashboard fetches from `GET /admin/accounts` with `Authorization: Bearer <api-key>`.

Response shape:
```json
{
  "total": 11,
  "available": 11,
  "models": [{ "id": "...", "name": "..." }],
  "accounts": [{
    "email": "...",
    "name": "...",
    "plan": "personal",
    "space": "...",
    "exhausted": false,
    "permanent": false,
    "eligible": true,
    "usage": 103,
    "limit": 200,
    "remaining": 97,
    "checked_at": "2026-03-17T...",
    "models": [{ "id": "...", "name": "..." }]
  }]
}
```

### Proxy Navigation

- Click account card → `GET /proxy/start?email=<email>` (opens in new tab)
- "Open Best" button → `GET /proxy/start?best=true`
- Both create a `np_session` cookie and redirect to `/ai` (Notion reverse proxy)

## Conventions

- All component code lives in `App.tsx` (single-file for now; split when it grows)
- No external UI library — TailwindCSS utility classes only
- Responsive: 5-col summary grid → 2-col on tablet → 1-col on mobile
- Account cards sorted: available first (by remaining desc), then exhausted
- Search filters client-side on name, email, plan, workspace
- Keyboard shortcut: `/` focuses search, `Escape` blurs
