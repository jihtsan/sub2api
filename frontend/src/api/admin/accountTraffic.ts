import { apiClient } from '../client'

export interface AccountTrafficPolicy {
  strict_rpm_enabled: boolean; rpm: number; burst: number
  adaptive_enabled: boolean; adaptive_mode: 'observe'
  min_concurrency: number; failure_threshold: number; failure_window_seconds: number; recovery_seconds: number
}
export interface AccountTrafficState {
  effective_concurrency: number; recommended_concurrency: number; in_flight: number; requests_last_minute: number
  accepted: number; rejected_rpm: number; rejected_concurrency: number; upstream_429: number; upstream_5xx: number
  completed: number; average_duration_ms: number; last_adjustment_at?: string
}
export interface AccountTrafficResponse { policy: AccountTrafficPolicy; state?: AccountTrafficState | null; state_available: boolean; hard_limit: number }
export const defaultTrafficPolicy = (): AccountTrafficPolicy => ({ strict_rpm_enabled: false, rpm: 60, burst: 5, adaptive_enabled: false, adaptive_mode: 'observe', min_concurrency: 1, failure_threshold: 3, failure_window_seconds: 60, recovery_seconds: 60 })
const integerIn = (value: number, min: number, max: number) => Number.isInteger(value) && value >= min && value <= max
export function normalizeTrafficDraft(value: AccountTrafficPolicy): AccountTrafficPolicy {
  const policy = { ...value }, defaults = defaultTrafficPolicy()
  if (!policy.strict_rpm_enabled) {
    if (!integerIn(policy.rpm, 1, 60000)) policy.rpm = defaults.rpm
    if (!integerIn(policy.burst, 1, policy.rpm)) policy.burst = Math.min(defaults.burst, policy.rpm)
  }
  if (!policy.adaptive_enabled) {
    if (!integerIn(policy.min_concurrency, 1, 10000)) policy.min_concurrency = defaults.min_concurrency
    if (!integerIn(policy.failure_threshold, 1, 100)) policy.failure_threshold = defaults.failure_threshold
    if (!integerIn(policy.failure_window_seconds, 10, 3600)) policy.failure_window_seconds = defaults.failure_window_seconds
    if (!integerIn(policy.recovery_seconds, 10, 3600)) policy.recovery_seconds = defaults.recovery_seconds
  }
  return policy
}
export const accountTrafficAPI = {
  async get(id: number, signal?: AbortSignal) { return (await apiClient.get<AccountTrafficResponse>(`/admin/accounts/${id}/traffic-control`, { signal })).data }
}
