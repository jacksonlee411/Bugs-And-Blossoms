import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { PageHeader } from './PageHeader'

describe('PageHeader', () => {
  it('renders title and subtitle', () => {
    render(<PageHeader subtitle='sub' title='header' />)

    expect(screen.getByRole('heading', { name: 'header' })).toBeInTheDocument()
    expect(screen.getByText('sub')).toBeInTheDocument()
  })
})
