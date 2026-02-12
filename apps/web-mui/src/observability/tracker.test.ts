import { describe, expect, it } from 'vitest'
import { trackUiEvent } from './tracker'

describe('trackUiEvent', () => {
  it('writes events to window buffer', () => {
    window.__WEB_MUI_UI_EVENTS__ = []
    const event = trackUiEvent({
      eventName: 'nav_click',
      tenant: 'tenant-a',
      module: 'shell',
      page: '/people',
      action: 'menu_navigate:people',
      result: 'success'
    })

    expect(event.timestamp).toBeTypeOf('string')
    expect(window.__WEB_MUI_UI_EVENTS__).toHaveLength(1)
    expect(window.__WEB_MUI_UI_EVENTS__?.[0]?.action).toBe('menu_navigate:people')
  })
})
