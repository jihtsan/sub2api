<template>
  <details class="rounded-xl border border-gray-200 p-4 dark:border-dark-700" data-testid="account-traffic-controls" @toggle="onToggle">
    <summary class="cursor-pointer text-sm font-medium">{{ t('admin.accounts.traffic.title') }}</summary>
    <p class="mt-3 text-xs leading-relaxed text-gray-500">{{ t('admin.accounts.traffic.description') }}</p>
    <p v-if="platform === 'grok'" class="mt-2 text-xs text-amber-700">{{ t('admin.accounts.traffic.grok') }}</p>
    <fieldset :disabled="disabled" class="mt-4 space-y-4">
      <label class="flex items-center gap-2 text-sm"><input v-model="policy.strict_rpm_enabled" type="checkbox" data-testid="traffic-rpm-toggle" />{{ t('admin.accounts.traffic.strict') }}</label>
      <div v-if="policy.strict_rpm_enabled" class="grid gap-3 sm:grid-cols-2">
        <label class="text-sm">{{ t('admin.accounts.traffic.rpm') }}<input v-model.number="policy.rpm" class="input mt-1" type="number" required min="1" max="60000" /></label>
        <label class="text-sm">{{ t('admin.accounts.traffic.burst') }}<input v-model.number="policy.burst" class="input mt-1" type="number" required min="1" :max="policy.rpm" /></label>
        <p class="text-xs text-gray-500 sm:col-span-2">{{ t('admin.accounts.traffic.budgetHint') }}</p>
      </div>
      <label class="flex items-center gap-2 text-sm"><input v-model="policy.adaptive_enabled" type="checkbox" data-testid="traffic-observe-toggle" />{{ t('admin.accounts.traffic.observe') }}</label>
      <div v-if="policy.adaptive_enabled" class="grid gap-3 sm:grid-cols-2">
        <label v-for="field in observationFields" :key="field.key" class="text-sm">{{ t(`admin.accounts.traffic.${field.key}`) }}<input v-model.number="policy[field.key]" class="input mt-1" type="number" required :min="field.min" :max="field.max" /></label>
        <p class="text-xs text-gray-500 sm:col-span-2">{{ t('admin.accounts.traffic.observeHint', { limit: hardLimit }) }}</p>
      </div>
      <p class="text-xs text-gray-500">{{ t('admin.accounts.traffic.saveHint') }}</p>
    </fieldset>
    <div class="mt-4 rounded-lg bg-gray-50 p-3 text-xs dark:bg-dark-900">
      <button type="button" class="text-primary-600" :disabled="loading" @click="refresh">{{ t(loading ? 'admin.accounts.traffic.loading' : 'admin.accounts.traffic.refresh') }}</button>
      <p v-if="unavailable" role="status" class="mt-2">{{ t('admin.accounts.traffic.unavailable') }}</p>
      <p v-else-if="state && !active" class="mt-2">{{ t('admin.accounts.traffic.inactive') }}</p>
      <dl v-else-if="state" class="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-3">
        <div v-for="key in stateFields" :key="key"><dt>{{ t(`admin.accounts.traffic.${key}`) }}</dt><dd>{{ state[key] }}</dd></div>
        <div><dt>{{ t('admin.accounts.traffic.duration') }}</dt><dd>{{ (state.average_duration_ms / 1000).toFixed(2) }} s</dd></div>
      </dl>
      <p v-if="state && active" class="mt-2 text-gray-500">{{ t('admin.accounts.traffic.statsHint') }}</p>
    </div>
  </details>
</template>

<script setup lang="ts">
import { onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { accountTrafficAPI, type AccountTrafficPolicy, type AccountTrafficState } from '@/api/admin/accountTraffic'
const props = defineProps<{ accountId: number; hardLimit: number; platform: string; disabled?: boolean }>()
const policy = defineModel<AccountTrafficPolicy>({ required: true })
const { t } = useI18n()
const state = ref<AccountTrafficState | null>(null)
const loading = ref(false)
const unavailable = ref(false)
const active = ref(false)
let controller: AbortController | undefined
let opened = false
const observationFields = [
  { key: 'min_concurrency', min: 1, max: 10000 },
  { key: 'failure_threshold', min: 1, max: 100 },
  { key: 'failure_window_seconds', min: 10, max: 3600 },
  { key: 'recovery_seconds', min: 10, max: 3600 }
] as const
const stateFields = ['effective_concurrency', 'recommended_concurrency', 'in_flight', 'requests_last_minute', 'upstream_429', 'upstream_5xx', 'rejected_rpm'] as const
async function refresh() {
  controller?.abort()
  const request = new AbortController()
  controller = request
  loading.value = true
  unavailable.value = false
  try {
    const result = await accountTrafficAPI.get(props.accountId, request.signal)
    if (request.signal.aborted) return
    // Refresh telemetry only: an asynchronous response must never overwrite unsaved edits.
    state.value = result.state ?? null
    unavailable.value = !result.state_available
    active.value = result.policy.strict_rpm_enabled || result.policy.adaptive_enabled
  } catch {
    if (!request.signal.aborted) unavailable.value = true
  } finally {
    if (controller === request) loading.value = false
  }
}
function onToggle(event: Event) {
  opened = (event.target as HTMLDetailsElement).open
  if (opened) void refresh()
}
watch(() => props.accountId, () => {
  controller?.abort()
  state.value = null
  if (opened) void refresh()
})
onUnmounted(() => controller?.abort())
</script>
