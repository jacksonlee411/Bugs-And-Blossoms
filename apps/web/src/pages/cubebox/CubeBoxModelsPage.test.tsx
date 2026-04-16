import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AppProviders } from '../../app/providers/AppProviders'
import { ApiClientError } from '../../api/errors'

const cubeboxAPIMocks = vi.hoisted(() => ({
  getCubeBoxModels: vi.fn(),
  getCubeBoxRuntimeStatus: vi.fn()
}))

vi.mock('../../api/cubebox', () => cubeboxAPIMocks)

import { CubeBoxModelsPage } from './CubeBoxModelsPage'

function renderCubeBoxModelsPage() {
  return render(
    <AppProviders>
      <CubeBoxModelsPage />
    </AppProviders>
  )
}

describe('CubeBoxModelsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.localStorage.clear()
    cubeboxAPIMocks.getCubeBoxModels.mockResolvedValue({
      models: [
        { provider: 'openai', model: 'gpt-5.4' },
        { provider: 'openai', model: 'gpt-5.3-codex' }
      ]
    })
    cubeboxAPIMocks.getCubeBoxRuntimeStatus.mockResolvedValue({
      status: 'healthy',
      checked_at: '2026-04-17T01:00:00Z',
      frontend: { healthy: 'healthy' },
      backend: { healthy: 'healthy' },
      knowledge_runtime: { healthy: 'healthy' },
      model_gateway: { healthy: 'healthy' },
      file_store: { healthy: 'healthy' },
      retired_capabilities: [],
      capabilities: {
        conversation_enabled: true,
        files_enabled: true,
        agents_ui_enabled: false,
        agents_write_enabled: false,
        memory_enabled: false,
        web_search_enabled: false,
        file_search_enabled: false,
        mcp_enabled: false
      }
    })
  })

  it('renders runtime summary and model list', async () => {
    renderCubeBoxModelsPage()

    await waitFor(() => expect(cubeboxAPIMocks.getCubeBoxModels).toHaveBeenCalled())
    await waitFor(() => expect(cubeboxAPIMocks.getCubeBoxRuntimeStatus).toHaveBeenCalled())

    expect(screen.getByRole('heading', { name: 'CubeBox 模型' })).toBeInTheDocument()
    expect(screen.getByText('openai: gpt-5.4')).toBeInTheDocument()
    expect(screen.getByText('openai: gpt-5.3-codex')).toBeInTheDocument()
    expect(screen.getByText('model_gateway=healthy')).toBeInTheDocument()
  })

  it('keeps runtime summary visible when model list fails', async () => {
    cubeboxAPIMocks.getCubeBoxModels.mockRejectedValue(
      new ApiClientError('legacy message', 'SERVER_ERROR', 500, 'trace_1', {
        code: 'cubebox_models_unavailable',
        message: 'CubeBox 模型清单暂不可用，请稍后重试。'
      })
    )

    renderCubeBoxModelsPage()

    expect(await screen.findByText('CubeBox 模型清单暂不可用，请稍后重试。')).toBeInTheDocument()
    expect(screen.getByText('model_gateway=healthy')).toBeInTheDocument()
  })

  it('keeps model list visible when runtime status fails', async () => {
    cubeboxAPIMocks.getCubeBoxRuntimeStatus.mockRejectedValue(
      new ApiClientError('legacy message', 'SERVER_ERROR', 503, 'trace_1', {
        code: 'cubebox_service_missing',
        message: 'CubeBox 服务暂不可用，请稍后重试。'
      })
    )

    renderCubeBoxModelsPage()

    expect(await screen.findByText('CubeBox 服务暂不可用，请稍后重试。')).toBeInTheDocument()
    expect(screen.getByText('openai: gpt-5.4')).toBeInTheDocument()
  })

  it('keeps fallback errors localized to the current app locale', async () => {
    window.localStorage.setItem('web-mui-locale', 'en')
    cubeboxAPIMocks.getCubeBoxModels.mockRejectedValue(new ApiClientError('网络错误', 'NETWORK_ERROR'))

    renderCubeBoxModelsPage()

    expect(await screen.findByText('Failed to load CubeBox models.')).toBeInTheDocument()
  })
})
