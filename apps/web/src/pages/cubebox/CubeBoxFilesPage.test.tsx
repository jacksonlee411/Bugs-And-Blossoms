import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AppProviders } from '../../app/providers/AppProviders'
import { ApiClientError } from '../../api/errors'

const cubeboxAPIMocks = vi.hoisted(() => ({
  listCubeBoxFiles: vi.fn(),
  uploadCubeBoxFile: vi.fn(),
  deleteCubeBoxFile: vi.fn()
}))

vi.mock('../../api/cubebox', () => cubeboxAPIMocks)

import { CubeBoxFilesPage } from './CubeBoxFilesPage'

function renderCubeBoxFilesPage() {
  return render(
    <AppProviders>
      <CubeBoxFilesPage />
    </AppProviders>
  )
}

describe('CubeBoxFilesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.localStorage.clear()
    cubeboxAPIMocks.listCubeBoxFiles.mockResolvedValue({
      items: [
        {
          file_id: 'file_1',
          filename: 'design.txt',
          content_type: 'text/plain',
          scan_status: 'ready',
          created_at: '2026-04-13T02:00:00Z',
          links: [
            {
              link_role: 'conversation_attachment',
              conversation_id: 'conv_1'
            }
          ],
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
    renderCubeBoxFilesPage()

    await waitFor(() => expect(cubeboxAPIMocks.listCubeBoxFiles).toHaveBeenCalled())
    expect(screen.getByTestId('cubebox-file-item')).toHaveTextContent('design.txt')

    fireEvent.click(screen.getByRole('button', { name: '删除' }))

    await waitFor(() => expect(cubeboxAPIMocks.deleteCubeBoxFile).toHaveBeenCalledWith('file_1'))
  }, 10000)

  it('uses normalized api error message when file load fails', async () => {
    cubeboxAPIMocks.listCubeBoxFiles.mockRejectedValue(
      new ApiClientError('legacy message', 'SERVER_ERROR', 500, 'trace_1', {
        code: 'cubebox_files_list_failed',
        message: '加载 CubeBox 文件列表失败，请稍后重试。'
      })
    )

    renderCubeBoxFilesPage()

    expect(await screen.findByText('加载 CubeBox 文件列表失败，请稍后重试。')).toBeInTheDocument()
  })

  it('keeps file-load fallback localized to the current app locale', async () => {
    window.localStorage.setItem('web-mui-locale', 'en')
    cubeboxAPIMocks.listCubeBoxFiles.mockRejectedValue(new ApiClientError('网络错误', 'NETWORK_ERROR'))

    renderCubeBoxFilesPage()

    expect(await screen.findByText('Failed to load files.')).toBeInTheDocument()
  })
})
