import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { MessageKey, MessageVars } from '../i18n/messages'

const selectorApiMocks = vi.hoisted(() => ({
  listOrgUnitSelectorChildren: vi.fn(),
  listOrgUnitSelectorRoots: vi.fn(),
  searchOrgUnitSelector: vi.fn()
}))

vi.mock('../api/orgUnitSelector', async () => {
  const actual = await vi.importActual<typeof import('../api/orgUnitSelector')>('../api/orgUnitSelector')
  return {
    ...actual,
    listOrgUnitSelectorChildren: selectorApiMocks.listOrgUnitSelectorChildren,
    listOrgUnitSelectorRoots: selectorApiMocks.listOrgUnitSelectorRoots,
    searchOrgUnitSelector: selectorApiMocks.searchOrgUnitSelector
  }
})

vi.mock('../app/providers/AppPreferencesContext', () => ({
  useAppPreferences: () => ({
    t: (key: MessageKey, vars?: MessageVars) => {
      const labels: Partial<Record<MessageKey, string>> = {
        common_cancel: 'Cancel',
        common_clear: 'Clear',
        common_confirm: 'Confirm',
        org_search_action: 'Locate',
        org_search_label: 'Search in tree',
        org_search_query_required: 'Please input a search query',
        org_tree_selector_title: 'Select organization',
        org_tree_title: 'Organization Tree',
        text_loading: 'Loading',
        text_no_data: 'No data'
      }
      let message = labels[key] ?? key
      for (const [name, value] of Object.entries(vars ?? {})) {
        message = message.replaceAll(`{${name}}`, String(value))
      }
      return message
    }
  })
}))

import { OrgUnitTreeField, OrgUnitTreeSelector } from './OrgUnitTreeSelector'

describe('OrgUnitTreeSelector', () => {
  beforeEach(() => {
    selectorApiMocks.listOrgUnitSelectorChildren.mockReset()
    selectorApiMocks.listOrgUnitSelectorRoots.mockReset()
    selectorApiMocks.searchOrgUnitSelector.mockReset()
  })

  it('loads roots, lazy-loads children, and emits the selected node', async () => {
    selectorApiMocks.listOrgUnitSelectorRoots.mockResolvedValue([
      {
        org_code: 'ROOT',
        org_node_key: '10000000',
        name: 'Root',
        status: 'active',
        has_visible_children: true
      }
    ])
    selectorApiMocks.listOrgUnitSelectorChildren.mockResolvedValue([
      {
        org_code: 'EAST',
        org_node_key: '10000001',
        name: 'East',
        status: 'active',
        has_visible_children: false
      }
    ])
    const onChange = vi.fn()

    render(<OrgUnitTreeSelector asOf='2026-05-04' onChange={onChange} />)

    const root = await screen.findByText('Root (ROOT)')
    fireEvent.click(root)
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ org_code: 'ROOT' }))

    await waitFor(() => expect(selectorApiMocks.listOrgUnitSelectorChildren).toHaveBeenCalledWith({
      asOf: '2026-05-04',
      includeDisabled: false,
      parentOrgCode: 'ROOT'
    }))
    const child = await screen.findByText('East (EAST)')
    fireEvent.click(child)

    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ org_code: 'EAST', org_node_key: '10000001' }))
  })

  it('uses search path to load ancestors and select the located node', async () => {
    selectorApiMocks.listOrgUnitSelectorRoots.mockResolvedValue([
      {
        org_code: 'ROOT',
        org_node_key: '10000000',
        name: 'Root',
        status: 'active',
        has_visible_children: true
      }
    ])
    selectorApiMocks.listOrgUnitSelectorChildren.mockResolvedValueOnce([
      {
        org_code: 'EAST',
        org_node_key: '10000001',
        name: 'East',
        status: 'active',
        has_visible_children: true
      }
    ]).mockResolvedValueOnce([
      {
        org_code: 'SH',
        org_node_key: '10000002',
        name: 'Shanghai',
        status: 'active',
        has_visible_children: false
      }
    ])
    selectorApiMocks.searchOrgUnitSelector.mockResolvedValue({
      org_code: 'SH',
      name: 'Shanghai',
      path_org_codes: ['ROOT', 'EAST', 'SH'],
      tree_as_of: '2026-05-04'
    })
    const onChange = vi.fn()

    render(<OrgUnitTreeSelector asOf='2026-05-04' onChange={onChange} />)

    await screen.findByText('Root (ROOT)')
    fireEvent.change(screen.getByLabelText('Search in tree'), { target: { value: 'Shanghai' } })
    fireEvent.click(screen.getByRole('button', { name: 'Locate' }))

    await waitFor(() => expect(selectorApiMocks.listOrgUnitSelectorChildren).toHaveBeenCalledTimes(2))
    expect(selectorApiMocks.listOrgUnitSelectorChildren).toHaveBeenNthCalledWith(1, {
      asOf: '2026-05-04',
      includeDisabled: false,
      parentOrgCode: 'ROOT'
    })
    expect(selectorApiMocks.listOrgUnitSelectorChildren).toHaveBeenNthCalledWith(2, {
      asOf: '2026-05-04',
      includeDisabled: false,
      parentOrgCode: 'EAST'
    })
    expect(screen.getByText('Shanghai (SH)')).toBeInTheDocument()
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ org_code: 'SH', org_node_key: '10000002' }))
  })

  it('clears a selected field value without opening the picker', () => {
    const onClear = vi.fn()

    render(
      <OrgUnitTreeField
        asOf='2026-05-04'
        clearable
        label='Parent'
        onChange={vi.fn()}
        onClear={onClear}
        value={{
          org_code: 'ROOT',
          org_node_key: '10000000',
          name: 'Root',
          status: 'active',
          has_visible_children: false
        }}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Clear' }))

    expect(onClear).toHaveBeenCalledOnce()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})
