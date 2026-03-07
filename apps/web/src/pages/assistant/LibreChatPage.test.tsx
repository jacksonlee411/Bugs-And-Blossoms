import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { LibreChatPage } from './LibreChatPage'

describe('LibreChatPage', () => {
  it('shows retirement notice instead of iframe bridge shell', () => {
    render(<LibreChatPage />)

    expect(screen.getByRole('heading', { name: 'LibreChat 旧桥接入口已下线' })).toBeInTheDocument()
    expect(screen.getByText(/不再承担正式对话交互职责/)).toBeInTheDocument()
    expect(screen.queryByTestId('librechat-standalone-frame')).not.toBeInTheDocument()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  })
})
