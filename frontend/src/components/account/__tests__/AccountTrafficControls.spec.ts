import { describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import AccountTrafficControls from '../AccountTrafficControls.vue'
import { accountTrafficAPI, defaultTrafficPolicy, type AccountTrafficResponse } from '@/api/admin/accountTraffic'
vi.mock('vue-i18n', async (original) => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/admin/accountTraffic', async (original) => ({
  ...await original<typeof import('@/api/admin/accountTraffic')>(),
  accountTrafficAPI: { get: vi.fn() }
}))
describe('traffic observations', () => {
  it('does not overwrite an unsaved policy when a delayed refresh returns', async () => {
    let resolve!: (value: AccountTrafficResponse) => void
    vi.mocked(accountTrafficAPI.get).mockReturnValue(new Promise(r => { resolve = r }))
    const policy = defaultTrafficPolicy()
    const wrapper = mount(AccountTrafficControls, { props: { accountId: 1, hardLimit: 3, platform: 'openai', modelValue: policy } })
    await wrapper.get('button').trigger('click')
    await wrapper.get('[data-testid="traffic-rpm-toggle"]').setValue(true)
    resolve({ policy: defaultTrafficPolicy(), state: null, state_available: false, hard_limit: 3 })
    await flushPromises()
    expect(policy.strict_rpm_enabled).toBe(true)
    expect(wrapper.get('[data-testid="traffic-rpm-toggle"]').element).toHaveProperty('checked', true)
    wrapper.unmount()
  })
})
