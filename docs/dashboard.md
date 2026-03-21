# Dashboard & Proxy

[← Back to README](../README.md)

## Dashboard auth model

- `/v1/messages` uses API key auth
- `/admin/*` uses dashboard session auth
- `/admin/*` is not an API-key endpoint for external clients

After logging in through `/dashboard/`, the frontend uses the `dashboard_session` cookie to access:

- `/admin/accounts`
- `/admin/models`
- `/admin/refresh`
- `/admin/settings`

## Opening the local Notion proxy

1. Click the best account or a specific account in the dashboard
2. The browser hits `/proxy/start?best=true` or `/proxy/start?email=<email>`
3. The server creates `np_session`
4. The browser is redirected to `/ai`
5. Notion HTML, API requests, assets, and realtime connections flow through that account
