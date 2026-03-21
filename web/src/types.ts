export interface Model {
  id: string
  name: string
}

export interface AccountInfo {
  email: string
  name: string
  plan: string
  space: string
  exhausted: boolean
  permanent: boolean
  eligible?: boolean
  usage?: number
  limit?: number
  remaining?: number
  space_usage?: number
  space_limit?: number
  space_remaining?: number
  user_usage?: number
  user_limit?: number
  user_remaining?: number
  checked_at?: string
  exhausted_at?: string
  last_usage_at?: number
  models?: Model[]
  research_usage?: number
  has_premium?: boolean
  premium_balance?: number
  premium_usage?: number
  premium_limit?: number
}

export interface RefreshStatus {
  refreshing: boolean
  done: number
  total: number
  last_refresh_at?: string
  error?: string
}

export interface DashboardData {
  total: number
  available: number
  models: Model[]
  accounts: AccountInfo[]
  refresh?: RefreshStatus
}
