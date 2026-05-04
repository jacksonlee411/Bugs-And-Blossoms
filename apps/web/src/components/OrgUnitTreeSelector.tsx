import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  TextField
} from '@mui/material'
import {
  formatOrgUnitSelectorLabel,
  listOrgUnitSelectorChildren,
  listOrgUnitSelectorRoots,
  searchOrgUnitSelector,
  type OrgUnitSelectorNode
} from '../api/orgUnitSelector'
import { useAppPreferences } from '../app/providers/AppPreferencesContext'
import { type TreePanelNode, TreePanel } from './TreePanel'

export type OrgUnitTreeSelectorValue = OrgUnitSelectorNode

export interface OrgUnitTreeSelectorProps {
  asOf: string
  includeDisabled?: boolean
  value?: OrgUnitSelectorNode | null
  onChange: (value: OrgUnitSelectorNode) => void
  minWidth?: number
}

export interface OrgUnitTreePickerDialogProps extends OrgUnitTreeSelectorProps {
  open: boolean
  title?: string
  onClose: () => void
}

export interface OrgUnitTreeFieldProps {
  asOf: string
  includeDisabled?: boolean
  value?: OrgUnitSelectorNode | null
  label: string
  disabled?: boolean
  onChange: (value: OrgUnitSelectorNode) => void
}

function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }
  return String(error)
}

function nodeLabel(node: OrgUnitSelectorNode): string {
  const suffix = node.status === 'disabled' || node.status === 'inactive' ? ' · Inactive' : ''
  return `${formatOrgUnitSelectorLabel(node)}${suffix}`
}

function buildTreeNodes(
  roots: OrgUnitSelectorNode[],
  childrenByParent: Record<string, OrgUnitSelectorNode[]>
): TreePanelNode[] {
  function build(node: OrgUnitSelectorNode, path: Set<string>): TreePanelNode {
    if (path.has(node.org_code)) {
      return { id: node.org_code, label: nodeLabel(node), hasChildren: false }
    }

    const nextPath = new Set(path)
    nextPath.add(node.org_code)
    const childrenLoaded = Object.hasOwn(childrenByParent, node.org_code)
    const children = childrenByParent[node.org_code] ?? []
    const childNodes = children.map((child) => build(child, nextPath))

    return {
      id: node.org_code,
      label: nodeLabel(node),
      hasChildren: childrenLoaded ? childNodes.length > 0 : node.has_visible_children,
      children: childNodes.length > 0 ? childNodes : undefined
    }
  }

  return roots.map((root) => build(root, new Set()))
}

function indexNodes(roots: OrgUnitSelectorNode[], childrenByParent: Record<string, OrgUnitSelectorNode[]>): Map<string, OrgUnitSelectorNode> {
  const out = new Map<string, OrgUnitSelectorNode>()
  const visit = (node: OrgUnitSelectorNode) => {
    out.set(node.org_code, node)
    for (const child of childrenByParent[node.org_code] ?? []) {
      visit(child)
    }
  }
  roots.forEach(visit)
  return out
}

export function OrgUnitTreeSelector({
  asOf,
  includeDisabled = false,
  value,
  onChange,
  minWidth = 320
}: OrgUnitTreeSelectorProps) {
  const { t } = useAppPreferences()
  const [roots, setRoots] = useState<OrgUnitSelectorNode[]>([])
  const [childrenByParent, setChildrenByParent] = useState<Record<string, OrgUnitSelectorNode[]>>({})
  const [selectedOrgCode, setSelectedOrgCode] = useState(value?.org_code ?? '')
  const [expandedOrgCodes, setExpandedOrgCodes] = useState<string[]>([])
  const [searchInput, setSearchInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setErrorMessage('')
    setChildrenByParent({})
    setExpandedOrgCodes([])
    listOrgUnitSelectorRoots({ asOf, includeDisabled })
      .then((items) => {
        if (cancelled) {
          return
        }
        setRoots(items)
      })
      .catch((error) => {
        if (!cancelled) {
          setErrorMessage(getErrorMessage(error))
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [asOf, includeDisabled])

  useEffect(() => {
    setSelectedOrgCode(value?.org_code ?? '')
  }, [value?.org_code])

  const nodesByCode = useMemo(() => indexNodes(roots, childrenByParent), [childrenByParent, roots])
  const treeNodes = useMemo(() => buildTreeNodes(roots, childrenByParent), [childrenByParent, roots])

  const ensureChildrenLoaded = useCallback(
    async (parentOrgCode: string): Promise<OrgUnitSelectorNode[]> => {
      if (Object.hasOwn(childrenByParent, parentOrgCode)) {
        return childrenByParent[parentOrgCode] ?? []
      }
      setLoading(true)
      setErrorMessage('')
      try {
        const children = await listOrgUnitSelectorChildren({ asOf, includeDisabled, parentOrgCode })
        setChildrenByParent((previous) => ({
          ...previous,
          [parentOrgCode]: children
        }))
        return children
      } catch (error) {
        setErrorMessage(getErrorMessage(error))
        return []
      } finally {
        setLoading(false)
      }
    },
    [asOf, childrenByParent, includeDisabled]
  )

  const ensurePathLoaded = useCallback(
    async (pathOrgCodes: string[]): Promise<Map<string, OrgUnitSelectorNode>> => {
      const loadedNodes = new Map(nodesByCode)
      for (const parentOrgCode of pathOrgCodes.slice(0, -1)) {
        const children = await ensureChildrenLoaded(parentOrgCode)
        children.forEach((child) => loadedNodes.set(child.org_code, child))
      }
      return loadedNodes
    },
    [ensureChildrenLoaded, nodesByCode]
  )

  const handleSelect = useCallback(
    (orgCode: string) => {
      const node = nodesByCode.get(orgCode)
      if (!node) {
        return
      }
      setSelectedOrgCode(node.org_code)
      onChange(node)
    },
    [nodesByCode, onChange]
  )

  const handleSearch = useCallback(async () => {
    const query = searchInput.trim()
    if (query.length === 0) {
      setErrorMessage(t('org_search_query_required'))
      return
    }
    setLoading(true)
    setErrorMessage('')
    try {
      const result = await searchOrgUnitSelector({ asOf, includeDisabled, query })
      const loadedNodes = await ensurePathLoaded(result.path_org_codes)
      setExpandedOrgCodes(result.path_org_codes.slice(0, -1))
      setSelectedOrgCode(result.org_code)
      const loadedNode = loadedNodes.get(result.org_code)
      if (loadedNode) {
        onChange(loadedNode)
      }
    } catch (error) {
      setErrorMessage(getErrorMessage(error))
    } finally {
      setLoading(false)
    }
  }, [asOf, ensurePathLoaded, includeDisabled, onChange, searchInput, t])

  return (
    <Stack spacing={1.5}>
      <Stack direction={{ sm: 'row', xs: 'column' }} spacing={1}>
        <TextField
          fullWidth
          label={t('org_search_label')}
          onChange={(event) => setSearchInput(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.preventDefault()
              void handleSearch()
            }
          }}
          value={searchInput}
        />
        <Button onClick={() => void handleSearch()} variant='outlined'>
          {t('org_search_action')}
        </Button>
      </Stack>
      {errorMessage ? <Alert severity='error'>{errorMessage}</Alert> : null}
      <TreePanel
        emptyLabel={t('text_no_data')}
        expandedItemIds={expandedOrgCodes}
        loading={loading}
        loadingLabel={t('text_loading')}
        minWidth={minWidth}
        nodes={treeNodes}
        onExpandedItemIdsChange={setExpandedOrgCodes}
        onExpand={(nodeId) => void ensureChildrenLoaded(nodeId)}
        onSelect={handleSelect}
        selectedItemId={selectedOrgCode || undefined}
        title={t('org_tree_title')}
      />
    </Stack>
  )
}

export function OrgUnitTreePickerDialog({
  open,
  onClose,
  ...contentProps
}: OrgUnitTreePickerDialogProps) {
  return (
    <Dialog fullWidth maxWidth='sm' onClose={onClose} open={open}>
      {open ? <OrgUnitTreePickerDialogContent {...contentProps} onClose={onClose} /> : null}
    </Dialog>
  )
}

function OrgUnitTreePickerDialogContent({
  title,
  onClose,
  onChange,
  value,
  ...selectorProps
}: Omit<OrgUnitTreePickerDialogProps, 'open'>) {
  const { t } = useAppPreferences()
  const [draft, setDraft] = useState<OrgUnitSelectorNode | null>(value ?? null)

  return (
    <>
      <DialogTitle>{title ?? t('org_tree_selector_title')}</DialogTitle>
      <DialogContent>
        <Box sx={{ pt: 1 }}>
          <OrgUnitTreeSelector {...selectorProps} onChange={setDraft} value={draft} />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('common_cancel')}</Button>
        <Button
          disabled={!draft}
          onClick={() => {
            if (!draft) {
              return
            }
            onChange(draft)
            onClose()
          }}
          variant='contained'
        >
          {t('common_confirm')}
        </Button>
      </DialogActions>
    </>
  )
}

export function OrgUnitTreeField({
  asOf,
  includeDisabled = false,
  value,
  label,
  disabled = false,
  onChange
}: OrgUnitTreeFieldProps) {
  const { t } = useAppPreferences()
  const [open, setOpen] = useState(false)
  return (
    <>
      <TextField
        disabled={disabled}
        fullWidth
        inputProps={{ readOnly: true }}
        label={label}
        onClick={() => {
          if (!disabled) {
            setOpen(true)
          }
        }}
        value={value ? formatOrgUnitSelectorLabel(value) : ''}
      />
      <OrgUnitTreePickerDialog
        asOf={asOf}
        includeDisabled={includeDisabled}
        onChange={onChange}
        onClose={() => setOpen(false)}
        open={open}
        title={t('org_tree_selector_title')}
        value={value}
      />
    </>
  )
}
