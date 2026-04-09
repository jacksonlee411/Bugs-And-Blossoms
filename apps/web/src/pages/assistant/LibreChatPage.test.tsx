import { render, screen } from '@testing-library/react'
import { vi } from 'vitest'
import { describe, expect, it } from 'vitest'

import { LibreChatPage } from './LibreChatPage'

describe('LibreChatPage', () => {
  it('redirects to formal entry and keeps fallback link', () => {
    const replaceMock = vi.fn()
    const originalLocation = window.location
    const locationStub = {
      ...originalLocation,
      replace: replaceMock
    } as unknown as Location
    Object.defineProperty(window, 'location', {
      value: locationStub,
      configurable: true
    })

    try {
      render(<LibreChatPage />)

      expect(replaceMock).toHaveBeenCalledWith('/app/assistant/librechat')
      expect(screen.getByRole('heading', { name: '正在进入 LibreChat 正式入口' })).toBeInTheDocument()
      expect(screen.getByRole('link', { name: '打开正式入口' })).toHaveAttribute('href', '/app/assistant/librechat')
      expect(screen.queryByTestId('librechat-standalone-frame')).not.toBeInTheDocument()
    } finally {
      Object.defineProperty(window, 'location', {
        value: originalLocation,
        configurable: true
      })
    }
  }, 20000)
})
