export type UiEventName = 'bulk_action' | 'detail_open' | 'filter_submit' | 'nav_click'

export type UiEventResult = 'cancel' | 'failure' | 'success'

export interface UiEvent {
  eventName: UiEventName
  tenant: string
  module: string
  page: string
  action: string
  result: UiEventResult
  latencyMs?: number
  metadata?: Record<string, string | number | boolean>
}

export interface TrackedUiEvent extends UiEvent {
  timestamp: string
}

declare global {
  interface Window {
    __WEB_MUI_UI_EVENTS__?: TrackedUiEvent[]
  }
}

export function trackUiEvent(event: UiEvent): TrackedUiEvent {
  const trackedEvent: TrackedUiEvent = {
    ...event,
    timestamp: new Date().toISOString()
  }

  if (window.__WEB_MUI_UI_EVENTS__) {
    window.__WEB_MUI_UI_EVENTS__.push(trackedEvent)
  } else {
    window.__WEB_MUI_UI_EVENTS__ = [trackedEvent]
  }

  if (import.meta.env.DEV && import.meta.env.MODE !== 'test') {
    console.info('[ui-event]', trackedEvent)
  }

  return trackedEvent
}
