import { useCallback, useMemo, useState } from 'react'
import { Box, CircularProgress, Stack, Typography } from '@mui/material'
import {
  DataGrid,
  type DataGridProps,
  type GridColumnOrderChangeParams,
  type GridColumnResizeParams,
  type GridColumnVisibilityModel,
  type GridColDef,
  type GridDensity,
  type GridRowsProp
} from '@mui/x-data-grid'

interface GridColumnDimension {
  width?: number
}

interface GridPreferencesState {
  columnVisibilityModel?: GridColumnVisibilityModel
  orderedFields?: string[]
  dimensions?: Record<string, GridColumnDimension>
  density?: GridDensity
}

interface DataGridPageProps {
  columns: GridColDef[]
  rows: GridRowsProp
  noRowsLabel?: string
  loadingLabel?: string
  loading?: boolean
  storageKey?: string
  gridProps?: Partial<DataGridProps>
}

const GRID_PREFERENCES_STORAGE_PREFIX = 'web-mui-grid-prefs'

function loadGridPreferences(storageKey: string): GridPreferencesState {
  if (typeof window === 'undefined') {
    return {}
  }

  const key = `${GRID_PREFERENCES_STORAGE_PREFIX}/${storageKey}`
  const raw = window.localStorage.getItem(key)
  if (!raw) {
    return {}
  }

  try {
    const parsed = JSON.parse(raw) as GridPreferencesState
    if (!parsed || typeof parsed !== 'object') {
      return {}
    }
    return parsed
  } catch {
    return {}
  }
}

function persistGridPreferences(storageKey: string, value: GridPreferencesState) {
  if (typeof window === 'undefined') {
    return
  }
  const key = `${GRID_PREFERENCES_STORAGE_PREFIX}/${storageKey}`
  window.localStorage.setItem(key, JSON.stringify(value))
}

function resolveOrderedFields(candidate: string[] | undefined, fallbackFields: string[]): string[] {
  const knownFields = new Set(fallbackFields)
  const ordered: string[] = []

  if (candidate) {
    candidate.forEach((field) => {
      if (knownFields.has(field) && !ordered.includes(field)) {
        ordered.push(field)
      }
    })
  }

  fallbackFields.forEach((field) => {
    if (!ordered.includes(field)) {
      ordered.push(field)
    }
  })

  return ordered
}

function sanitizeGridPreferences(preferences: GridPreferencesState, columns: GridColDef[]): GridPreferencesState {
  const fields = columns.map((column) => String(column.field))
  const knownFields = new Set(fields)

  let visibilityModel: GridColumnVisibilityModel | undefined
  if (preferences.columnVisibilityModel) {
    const nextModel: GridColumnVisibilityModel = {}
    Object.entries(preferences.columnVisibilityModel).forEach(([field, visible]) => {
      if (knownFields.has(field)) {
        nextModel[field] = Boolean(visible)
      }
    })
    if (Object.keys(nextModel).length > 0) {
      visibilityModel = nextModel
    }
  }

  const orderedFields = resolveOrderedFields(preferences.orderedFields, fields)

  let dimensions: Record<string, GridColumnDimension> | undefined
  if (preferences.dimensions) {
    const nextDimensions: Record<string, GridColumnDimension> = {}
    Object.entries(preferences.dimensions).forEach(([field, value]) => {
      if (!knownFields.has(field) || !value || typeof value !== 'object') {
        return
      }
      if (typeof value.width === 'number' && Number.isFinite(value.width) && value.width > 0) {
        nextDimensions[field] = { width: value.width }
      }
    })
    if (Object.keys(nextDimensions).length > 0) {
      dimensions = nextDimensions
    }
  }

  const density = preferences.density
  const sanitizedDensity = density === 'compact' || density === 'comfortable' || density === 'standard' ? density : undefined

  return {
    columnVisibilityModel: visibilityModel,
    orderedFields,
    dimensions,
    density: sanitizedDensity
  }
}

function NoRowsOverlay({ label }: { label: string }) {
  return (
    <Box sx={{ p: 3, textAlign: 'center' }}>
      <Typography color='text.secondary' variant='body2'>
        {label}
      </Typography>
    </Box>
  )
}

function LoadingOverlay({ label }: { label: string }) {
  return (
    <Stack alignItems='center' spacing={1} sx={{ p: 3 }}>
      <CircularProgress size={20} />
      <Typography color='text.secondary' variant='body2'>
        {label}
      </Typography>
    </Stack>
  )
}

export function DataGridPage({
  columns,
  rows,
  noRowsLabel = 'No data',
  loadingLabel = 'Loading...',
  loading = false,
  storageKey,
  gridProps
}: DataGridPageProps) {
  const normalizedStorageKey = storageKey?.trim() ?? ''
  const storageEnabled = normalizedStorageKey.length > 0

  const [preferences, setPreferences] = useState<GridPreferencesState>(() =>
    storageEnabled ? loadGridPreferences(normalizedStorageKey) : {}
  )

  const sanitizedPreferences = useMemo(
    () => sanitizeGridPreferences(preferences, columns),
    [columns, preferences]
  )

  const defaultOrder = useMemo(() => columns.map((column) => String(column.field)), [columns])

  const updatePreferences = useCallback(
    (updater: (previous: GridPreferencesState) => GridPreferencesState) => {
      if (!storageEnabled) {
        return
      }
      setPreferences((previous) => {
        const next = updater(previous)
        persistGridPreferences(normalizedStorageKey, next)
        return next
      })
    },
    [normalizedStorageKey, storageEnabled]
  )

  const resolvedInitialState = useMemo<DataGridProps['initialState']>(() => {
    if (!storageEnabled) {
      return gridProps?.initialState
    }

    const baseColumnsState = gridProps?.initialState?.columns ?? {}
    const nextColumnsState: NonNullable<DataGridProps['initialState']>['columns'] = {
      ...baseColumnsState,
      orderedFields: sanitizedPreferences.orderedFields,
      columnVisibilityModel: sanitizedPreferences.columnVisibilityModel,
      dimensions: sanitizedPreferences.dimensions
    }

    return {
      ...gridProps?.initialState,
      columns: nextColumnsState,
      density: sanitizedPreferences.density ?? gridProps?.initialState?.density
    }
  }, [gridProps?.initialState, sanitizedPreferences, storageEnabled])

  const handleColumnVisibilityModelChange = useCallback<NonNullable<DataGridProps['onColumnVisibilityModelChange']>>(
    (model, details) => {
      updatePreferences((previous) => ({
        ...previous,
        columnVisibilityModel: model
      }))
      gridProps?.onColumnVisibilityModelChange?.(model, details)
    },
    [gridProps, updatePreferences]
  )

  const handleColumnOrderChange = useCallback<NonNullable<DataGridProps['onColumnOrderChange']>>(
    (params, event, details) => {
      const orderParams = params as GridColumnOrderChangeParams
      const field = String(orderParams.column.field)
      updatePreferences((previous) => {
        const currentOrder = resolveOrderedFields(previous.orderedFields, defaultOrder)
        const existingIndex = currentOrder.indexOf(field)
        if (existingIndex < 0) {
          return previous
        }

        const nextOrder = [...currentOrder]
        nextOrder.splice(existingIndex, 1)
        const targetIndex = Math.max(0, Math.min(orderParams.targetIndex, nextOrder.length))
        nextOrder.splice(targetIndex, 0, field)
        return {
          ...previous,
          orderedFields: nextOrder
        }
      })
      gridProps?.onColumnOrderChange?.(params, event, details)
    },
    [defaultOrder, gridProps, updatePreferences]
  )

  const handleColumnWidthChange = useCallback<NonNullable<DataGridProps['onColumnWidthChange']>>(
    (params, event, details) => {
      const resizeParams = params as GridColumnResizeParams
      const field = String(resizeParams.colDef.field)
      const width = resizeParams.width

      if (Number.isFinite(width) && width > 0) {
        updatePreferences((previous) => ({
          ...previous,
          dimensions: {
            ...(previous.dimensions ?? {}),
            [field]: { width }
          }
        }))
      }

      gridProps?.onColumnWidthChange?.(params, event, details)
    },
    [gridProps, updatePreferences]
  )

  const handleDensityChange = useCallback<NonNullable<DataGridProps['onDensityChange']>>(
    (density) => {
      updatePreferences((previous) => ({
        ...previous,
        density
      }))
      gridProps?.onDensityChange?.(density)
    },
    [gridProps, updatePreferences]
  )

  return (
    <Box
      sx={{
        bgcolor: 'background.paper',
        border: 1,
        borderColor: 'divider',
        borderRadius: 1,
        minHeight: 480,
        overflow: 'hidden'
      }}
    >
      <DataGrid
        columns={columns}
        disableRowSelectionOnClick
        loading={loading}
        pageSizeOptions={[10, 20, 50]}
        rows={rows}
        slots={{
          loadingOverlay: () => <LoadingOverlay label={loadingLabel} />,
          noRowsOverlay: () => <NoRowsOverlay label={noRowsLabel} />
        }}
        {...gridProps}
        initialState={resolvedInitialState}
        onColumnOrderChange={handleColumnOrderChange}
        onColumnVisibilityModelChange={handleColumnVisibilityModelChange}
        onColumnWidthChange={handleColumnWidthChange}
        onDensityChange={handleDensityChange}
      />
    </Box>
  )
}
