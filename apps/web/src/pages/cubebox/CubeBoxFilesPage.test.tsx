import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const cubeboxAPIMocks = vi.hoisted(() => ({
  listCubeBoxFiles: vi.fn(),
  uploadCubeBoxFile: vi.fn(),
  deleteCubeBoxFile: vi.fn()
}))

vi.mock('../../api/cubebox', () => cubeboxAPIMocks)

import { CubeBoxFilesPage } from './CubeBoxFilesPage'

describe('CubeBoxFilesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    cubeboxAPIMocks.listCubeBoxFiles.mockResolvedValue({
      items: [
        {
          file_id: 'file_1',
          file_name: 'design.txt',
          media_type: 'text/plain',
          size_bytes: 16,
          sha256: 'abc',
          storage_key: 'tenant/file_1/design.txt',
          uploaded_by: 'actor_1',
          uploaded_at: '2026-04-13T02:00:00Z'
        }
      ]
    })
    cubeboxAPIMocks.deleteCubeBoxFile.mockResolvedValue(undefined)
  })

  it('renders files and deletes selected file', async () => {
    render(<CubeBoxFilesPage />)

    await waitFor(() => expect(cubeboxAPIMocks.listCubeBoxFiles).toHaveBeenCalled())
    expect(screen.getByTestId('cubebox-file-item')).toHaveTextContent('design.txt')

    fireEvent.click(screen.getByRole('button', { name: '删除' }))

    await waitFor(() => expect(cubeboxAPIMocks.deleteCubeBoxFile).toHaveBeenCalledWith('file_1'))
  })
})
