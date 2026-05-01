import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AuthzCapabilityKey } from '../../authz/capabilities'
import { AppPreferencesProvider } from './AppPreferencesProvider'
import { useAppPreferences } from './AppPreferencesContext'

const authzApiMocks = vi.hoisted(() => ({
  loadCurrentAuthzCapabilities: vi.fn()
}))

vi.mock('../../api/authz', () => authzApiMocks)

function CapabilityProbe({ capabilityKey }: { capabilityKey: AuthzCapabilityKey }) {
  const { hasRequiredCapability } = useAppPreferences()
  return <div data-testid='capability-probe'>{String(hasRequiredCapability(capabilityKey))}</div>
}

describe('AppPreferencesProvider', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.localStorage.clear()
    window.history.pushState({}, '', '/app/org/units')
  })

  it('loads current session authz capability keys outside the login route', async () => {
    authzApiMocks.loadCurrentAuthzCapabilities.mockResolvedValue(['orgunit.orgunits:read'])

    render(
      <AppPreferencesProvider>
        <CapabilityProbe capabilityKey='orgunit.orgunits:read' />
      </AppPreferencesProvider>
    )

    expect(screen.getByTestId('capability-probe')).toHaveTextContent('false')
    await waitFor(() => expect(screen.getByTestId('capability-probe')).toHaveTextContent('true'))
    expect(authzApiMocks.loadCurrentAuthzCapabilities).toHaveBeenCalledTimes(1)
  })

  it('skips capability loading on the login route', () => {
    window.history.pushState({}, '', '/app/login')

    render(
      <AppPreferencesProvider>
        <CapabilityProbe capabilityKey='orgunit.orgunits:read' />
      </AppPreferencesProvider>
    )

    expect(screen.getByTestId('capability-probe')).toHaveTextContent('false')
    expect(authzApiMocks.loadCurrentAuthzCapabilities).not.toHaveBeenCalled()
  })
})
