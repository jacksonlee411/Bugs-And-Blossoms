import { type FormEvent, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Alert,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  List,
  ListItemButton,
  ListItemText,
  Paper,
  Stack,
  TextField,
  Typography
} from '@mui/material'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createDict, createDictValue, disableDict, listDicts, listDictValues } from '../../api/dicts'
import { PageHeader } from '../../components/PageHeader'

function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

function newRequestCode(prefix: string): string {
  return `${prefix}:${Date.now()}`
}

function parseApiError(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }
  return String(error)
}

function statusColor(status: string): 'success' | 'default' {
  return status.trim().toLowerCase() === 'active' ? 'success' : 'default'
}

export function DictConfigsPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [asOf, setAsOf] = useState(todayISO())
  const [keyword, setKeyword] = useState('')
  const [selectedDictCode, setSelectedDictCode] = useState('')
  const [error, setError] = useState<string | null>(null)

  const [createDictOpen, setCreateDictOpen] = useState(false)
  const [createValueOpen, setCreateValueOpen] = useState(false)

  const [createDictCode, setCreateDictCode] = useState('')
  const [createDictName, setCreateDictName] = useState('')
  const [createDictEnabledOn, setCreateDictEnabledOn] = useState(todayISO())
  const [disableDictDay, setDisableDictDay] = useState(todayISO())

  const [createValueCode, setCreateValueCode] = useState('')
  const [createValueLabel, setCreateValueLabel] = useState('')
  const [createValueEnabledOn, setCreateValueEnabledOn] = useState(todayISO())

  const dictsQuery = useQuery({
    queryKey: ['dicts', asOf],
    queryFn: () => listDicts(asOf),
    staleTime: 10_000
  })

  const dicts = useMemo(() => dictsQuery.data?.dicts ?? [], [dictsQuery.data])

  useEffect(() => {
    if (dicts.length === 0) {
      setSelectedDictCode('')
      return
    }
    const first = dicts[0]
    if (!first) {
      setSelectedDictCode('')
      return
    }
    if (!dicts.some((item) => item.dict_code === selectedDictCode)) {
      setSelectedDictCode(first.dict_code)
    }
  }, [dicts, selectedDictCode])

  const valuesQuery = useQuery({
    enabled: selectedDictCode.trim().length > 0,
    queryKey: ['dict-values', selectedDictCode, asOf, keyword],
    queryFn: () =>
      listDictValues({
        dictCode: selectedDictCode,
        asOf,
        q: keyword,
        status: 'all',
        limit: 50
      }),
    staleTime: 5_000
  })

  const values = useMemo(() => valuesQuery.data?.values ?? [], [valuesQuery.data])

  const createDictMutation = useMutation({
    mutationFn: (request: { dict_code: string; name: string; enabled_on: string; request_code: string }) => createDict(request),
    onSuccess: async (result) => {
      setSelectedDictCode(result.dict_code)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['dicts', asOf] }),
        queryClient.invalidateQueries({ queryKey: ['dict-values', result.dict_code, asOf] })
      ])
    }
  })

  const disableDictMutation = useMutation({
    mutationFn: (request: { dict_code: string; disabled_on: string; request_code: string }) => disableDict(request),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['dicts', asOf] }),
        queryClient.invalidateQueries({ queryKey: ['dict-values', selectedDictCode, asOf] })
      ])
    }
  })

  const createValueMutation = useMutation({
    mutationFn: (request: { dict_code: string; code: string; label: string; enabled_on: string; request_code: string }) =>
      createDictValue(request),
    onSuccess: async (_, variables) => {
      await queryClient.invalidateQueries({ queryKey: ['dict-values', selectedDictCode, asOf] })
      navigate({
        pathname: `/dicts/${variables.dict_code}/values/${encodeURIComponent(variables.code)}`,
        search: `?as_of=${asOf}`
      })
    }
  })

  async function onCreateDict(event: FormEvent) {
    event.preventDefault()
    setError(null)
    try {
      await createDictMutation.mutateAsync({
        dict_code: createDictCode.trim().toLowerCase(),
        name: createDictName.trim(),
        enabled_on: createDictEnabledOn,
        request_code: newRequestCode('mui-dict-code-create')
      })
      setCreateDictOpen(false)
      setCreateDictCode('')
      setCreateDictName('')
      setCreateDictEnabledOn(todayISO())
    } catch (mutationError) {
      setError(parseApiError(mutationError))
    }
  }

  async function onDisableDict(event: FormEvent) {
    event.preventDefault()
    setError(null)
    if (selectedDictCode.trim().length === 0) {
      setError('请先选择字典字段')
      return
    }
    try {
      await disableDictMutation.mutateAsync({
        dict_code: selectedDictCode,
        disabled_on: disableDictDay,
        request_code: newRequestCode('mui-dict-code-disable')
      })
    } catch (mutationError) {
      setError(parseApiError(mutationError))
    }
  }

  async function onCreateValue(event: FormEvent) {
    event.preventDefault()
    setError(null)
    if (selectedDictCode.trim().length === 0) {
      setError('请先选择字典字段')
      return
    }
    try {
      await createValueMutation.mutateAsync({
        dict_code: selectedDictCode,
        code: createValueCode.trim(),
        label: createValueLabel.trim(),
        enabled_on: createValueEnabledOn,
        request_code: newRequestCode('mui-dict-value-create')
      })
      setCreateValueOpen(false)
      setCreateValueCode('')
      setCreateValueLabel('')
      setCreateValueEnabledOn(todayISO())
    } catch (mutationError) {
      setError(parseApiError(mutationError))
    }
  }

  return (
    <Box>
      <PageHeader
        title='字典配置'
        subtitle='左侧字典字段列表，右侧值列表（点击值进入详情页）'
        actions={
          <>
            <Button onClick={() => setCreateDictOpen(true)} size='small' variant='outlined'>
              新增字典字段
            </Button>
            <Button disabled={selectedDictCode.trim().length === 0} onClick={() => setCreateValueOpen(true)} size='small' variant='outlined'>
              新增字典值
            </Button>
          </>
        }
      />

      <Stack spacing={2}>
        {error ? <Alert severity='error'>{error}</Alert> : null}

        <Paper sx={{ p: 1.5 }} variant='outlined'>
          <Stack alignItems='center' direction={{ xs: 'column', md: 'row' }} spacing={1.5}>
            <TextField label='as_of' type='date' value={asOf} onChange={(event) => setAsOf(event.target.value)} />
            <TextField label='q' value={keyword} onChange={(event) => setKeyword(event.target.value)} />
            <Typography color='text.secondary' variant='body2'>
              当前字典字段数：{dicts.length}
            </Typography>
          </Stack>
        </Paper>

        <Box
          sx={{
            display: 'grid',
            gap: 2,
            gridTemplateColumns: {
              xs: '1fr',
              md: '280px minmax(0, 1fr)'
            }
          }}
        >
          <Paper sx={{ p: 1.5 }} variant='outlined'>
            <Typography sx={{ mb: 1 }} variant='subtitle2'>
              字典字段列表
            </Typography>
            {dictsQuery.isLoading ? <Typography variant='body2'>加载中...</Typography> : null}
            {dictsQuery.error ? <Alert severity='error'>{parseApiError(dictsQuery.error)}</Alert> : null}

            {dicts.length > 0 ? (
              <List dense sx={{ border: 1, borderColor: 'divider', borderRadius: 1, maxHeight: 420, overflow: 'auto', p: 0.5 }}>
                {dicts.map((dictItem) => (
                  <ListItemButton
                    key={dictItem.dict_code}
                    onClick={() => setSelectedDictCode(dictItem.dict_code)}
                    selected={dictItem.dict_code === selectedDictCode}
                    sx={{ borderRadius: 1, mb: 0.5 }}
                  >
                    <Box sx={{ alignItems: 'center', display: 'flex', gap: 1, justifyContent: 'space-between', width: '100%' }}>
                      <ListItemText
                        primary={dictItem.name}
                        primaryTypographyProps={{ fontWeight: 600, variant: 'body2' }}
                        secondary={dictItem.dict_code}
                        secondaryTypographyProps={{ variant: 'caption' }}
                      />
                      <Chip color={statusColor(dictItem.status)} label={dictItem.status} size='small' variant='outlined' />
                    </Box>
                  </ListItemButton>
                ))}
              </List>
            ) : (
              <Typography color='text.secondary' variant='body2'>
                暂无字典字段
              </Typography>
            )}

            <Divider sx={{ my: 1.5 }} />
            <Typography sx={{ mb: 1 }} variant='subtitle2'>
              停用字典字段
            </Typography>
            <Stack component='form' onSubmit={(event) => void onDisableDict(event)} spacing={1}>
              <TextField disabled label='dict_code' value={selectedDictCode} />
              <TextField label='disabled_on' type='date' value={disableDictDay} onChange={(event) => setDisableDictDay(event.target.value)} />
              <Button disabled={disableDictMutation.isPending || selectedDictCode.trim().length === 0} type='submit' variant='outlined'>
                停用字段
              </Button>
            </Stack>
          </Paper>

          <Paper sx={{ p: 1.5 }} variant='outlined'>
            <Typography sx={{ mb: 1 }} variant='subtitle2'>
              字典值列表（点击行进入详情）
            </Typography>
            {valuesQuery.isLoading ? <Typography variant='body2'>加载中...</Typography> : null}
            {valuesQuery.error ? <Alert severity='error'>{parseApiError(valuesQuery.error)}</Alert> : null}

            <Box sx={{ border: 1, borderColor: 'divider', borderRadius: 1, overflow: 'auto', maxHeight: 420 }}>
              <table border={0} cellPadding={8} cellSpacing={0} style={{ borderCollapse: 'collapse', width: '100%' }}>
                <thead>
                  <tr style={{ position: 'sticky', top: 0, background: '#fff' }}>
                    <th align='left'>code</th>
                    <th align='left'>label</th>
                    <th align='left'>status</th>
                    <th align='left'>enabled_on</th>
                    <th align='left'>disabled_on</th>
                    <th align='left'>updated_at</th>
                  </tr>
                </thead>
                <tbody>
                  {values.map((value) => (
                    <tr
                      key={`${value.dict_code}:${value.code}:${value.enabled_on}`}
                      style={{ borderTop: '1px solid #eee', cursor: 'pointer' }}
                      onClick={() =>
                        navigate({
                          pathname: `/dicts/${value.dict_code}/values/${encodeURIComponent(value.code)}`,
                          search: `?as_of=${asOf}`
                        })
                      }
                    >
                      <td>{value.code}</td>
                      <td>{value.label}</td>
                      <td>{value.status}</td>
                      <td>{value.enabled_on}</td>
                      <td>{value.disabled_on ?? '-'}</td>
                      <td>{value.updated_at}</td>
                    </tr>
                  ))}
                  {values.length === 0 ? (
                    <tr>
                      <td colSpan={6} style={{ padding: 16, textAlign: 'center' }}>
                        暂无字典值
                      </td>
                    </tr>
                  ) : null}
                </tbody>
              </table>
            </Box>
          </Paper>
        </Box>
      </Stack>

      <Dialog fullWidth maxWidth='sm' onClose={() => setCreateDictOpen(false)} open={createDictOpen}>
        <DialogTitle>新增字典字段</DialogTitle>
        <Box component='form' onSubmit={(event) => void onCreateDict(event)}>
          <DialogContent>
            <Stack spacing={1.5}>
              <TextField label='dict_code' required value={createDictCode} onChange={(event) => setCreateDictCode(event.target.value)} />
              <TextField label='name' required value={createDictName} onChange={(event) => setCreateDictName(event.target.value)} />
              <TextField
                label='enabled_on'
                required
                type='date'
                value={createDictEnabledOn}
                onChange={(event) => setCreateDictEnabledOn(event.target.value)}
              />
            </Stack>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setCreateDictOpen(false)}>取消</Button>
            <Button disabled={createDictMutation.isPending} type='submit' variant='contained'>
              提交
            </Button>
          </DialogActions>
        </Box>
      </Dialog>

      <Dialog fullWidth maxWidth='sm' onClose={() => setCreateValueOpen(false)} open={createValueOpen}>
        <DialogTitle>新增字典值</DialogTitle>
        <Box component='form' onSubmit={(event) => void onCreateValue(event)}>
          <DialogContent>
            <Stack spacing={1.5}>
              <TextField disabled label='dict_code' value={selectedDictCode} />
              <TextField label='code' required value={createValueCode} onChange={(event) => setCreateValueCode(event.target.value)} />
              <TextField label='label' required value={createValueLabel} onChange={(event) => setCreateValueLabel(event.target.value)} />
              <TextField
                label='enabled_on'
                required
                type='date'
                value={createValueEnabledOn}
                onChange={(event) => setCreateValueEnabledOn(event.target.value)}
              />
            </Stack>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setCreateValueOpen(false)}>取消</Button>
            <Button disabled={createValueMutation.isPending} type='submit' variant='contained'>
              提交
            </Button>
          </DialogActions>
        </Box>
      </Dialog>
    </Box>
  )
}
