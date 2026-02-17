import { type FormEvent, useMemo, useState } from 'react'
import { Alert, Box, Button, Divider, Paper, Stack, TextField, Typography } from '@mui/material'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { PageHeader } from '../../components/PageHeader'
import { correctDictValue, createDictValue, disableDictValue, listDictAudit, listDicts, listDictValues } from '../../api/dicts'

function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

function newRequestCode(prefix: string): string {
  return `${prefix}:${Date.now()}`
}

export function DictConfigsPage() {
  const queryClient = useQueryClient()

  const [asOf, setAsOf] = useState(todayISO())
  const [selectedDictCode, setSelectedDictCode] = useState('org_type')
  const [selectedValueCode, setSelectedValueCode] = useState('')
  const [keyword, setKeyword] = useState('')
  const [error, setError] = useState<string | null>(null)

  const dictsQuery = useQuery({
    queryKey: ['dicts', asOf],
    queryFn: () => listDicts(asOf),
    staleTime: 10_000
  })

  const valuesQuery = useQuery({
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

  const auditQuery = useQuery({
    enabled: selectedValueCode.trim().length > 0,
    queryKey: ['dict-audit', selectedDictCode, selectedValueCode],
    queryFn: () => listDictAudit({ dictCode: selectedDictCode, code: selectedValueCode, limit: 50 }),
    staleTime: 5_000
  })

  const createMutation = useMutation({
    mutationFn: (req: { dict_code: string; code: string; label: string; enabled_on: string; request_code: string }) => createDictValue(req),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['dict-values', selectedDictCode, asOf] }),
        queryClient.invalidateQueries({ queryKey: ['dicts', asOf] })
      ])
    }
  })

  const disableMutation = useMutation({
    mutationFn: (req: { dict_code: string; code: string; disabled_on: string; request_code: string }) => disableDictValue(req),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['dict-values', selectedDictCode, asOf] })
      if (selectedValueCode.trim().length > 0) {
        await queryClient.invalidateQueries({ queryKey: ['dict-audit', selectedDictCode, selectedValueCode] })
      }
    }
  })

  const correctMutation = useMutation({
    mutationFn: (req: { dict_code: string; code: string; label: string; correction_day: string; request_code: string }) => correctDictValue(req),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['dict-values', selectedDictCode, asOf] })
      if (selectedValueCode.trim().length > 0) {
        await queryClient.invalidateQueries({ queryKey: ['dict-audit', selectedDictCode, selectedValueCode] })
      }
    }
  })

  const dicts = useMemo(() => dictsQuery.data?.dicts ?? [], [dictsQuery.data])
  const values = useMemo(() => valuesQuery.data?.values ?? [], [valuesQuery.data])
  const auditEvents = useMemo(() => auditQuery.data?.events ?? [], [auditQuery.data])

  const [createCode, setCreateCode] = useState('')
  const [createLabel, setCreateLabel] = useState('')
  const [createEnabledOn, setCreateEnabledOn] = useState(todayISO())

  const [disableDay, setDisableDay] = useState(todayISO())
  const [correctLabel, setCorrectLabel] = useState('')
  const [correctDay, setCorrectDay] = useState(todayISO())

  async function onCreate(event: FormEvent) {
    event.preventDefault()
    setError(null)
    try {
      await createMutation.mutateAsync({
        dict_code: selectedDictCode,
        code: createCode.trim(),
        label: createLabel.trim(),
        enabled_on: createEnabledOn,
        request_code: newRequestCode('mui-dict-create')
      })
      setCreateCode('')
      setCreateLabel('')
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  async function onDisable(event: FormEvent) {
    event.preventDefault()
    setError(null)
    if (selectedValueCode.trim().length === 0) {
      setError('Select a value first')
      return
    }
    try {
      await disableMutation.mutateAsync({
        dict_code: selectedDictCode,
        code: selectedValueCode,
        disabled_on: disableDay,
        request_code: newRequestCode('mui-dict-disable')
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  async function onCorrect(event: FormEvent) {
    event.preventDefault()
    setError(null)
    if (selectedValueCode.trim().length === 0) {
      setError('Select a value first')
      return
    }
    try {
      await correctMutation.mutateAsync({
        dict_code: selectedDictCode,
        code: selectedValueCode,
        label: correctLabel.trim(),
        correction_day: correctDay,
        request_code: newRequestCode('mui-dict-correct')
      })
      setCorrectLabel('')
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <Box>
      <PageHeader title='Dictionary Configs' subtitle='DICT values (effective day) + audit (tx_time)' />

      <Stack spacing={2}>
        {error ? <Alert severity='error'>{error}</Alert> : null}

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Context
          </Typography>
          <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} alignItems='center'>
            <TextField label='as_of' name='as_of' type='date' value={asOf} onChange={(e) => setAsOf(e.target.value)} />
            <TextField
              label='dict_code'
              name='dict_code'
              value={selectedDictCode}
              onChange={(e) => setSelectedDictCode(e.target.value)}
              helperText={`Available: ${dicts.map((d) => d.dict_code).join(', ') || 'n/a'}`}
            />
            <TextField label='q' name='q' value={keyword} onChange={(e) => setKeyword(e.target.value)} />
          </Stack>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Values (click a row to select)
          </Typography>
          {valuesQuery.isError ? <Alert severity='error'>Values load failed</Alert> : null}
          <table border={1} cellPadding={6} cellSpacing={0}>
            <thead>
              <tr>
                <th>code</th>
                <th>label</th>
                <th>status</th>
                <th>enabled_on</th>
                <th>disabled_on</th>
                <th>updated_at</th>
              </tr>
            </thead>
            <tbody>
              {values.map((v) => (
                <tr
                  key={`${v.dict_code}:${v.code}:${v.enabled_on}`}
                  style={{ background: v.code === selectedValueCode ? '#e6f5f5' : 'transparent', cursor: 'pointer' }}
                  onClick={() => {
                    setSelectedValueCode(v.code)
                    setCorrectLabel(v.label)
                  }}
                >
                  <td>{v.code}</td>
                  <td>{v.label}</td>
                  <td>{v.status}</td>
                  <td>{v.enabled_on}</td>
                  <td>{v.disabled_on ?? ''}</td>
                  <td>{v.updated_at}</td>
                </tr>
              ))}
            </tbody>
          </table>
          <Divider sx={{ my: 2 }} />
          <Typography variant='body2'>Selected: {selectedValueCode || '(none)'}</Typography>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Create
          </Typography>
          <Stack component='form' spacing={1.5} onSubmit={(e) => void onCreate(e)}>
            <TextField label='code' name='code' value={createCode} onChange={(e) => setCreateCode(e.target.value)} />
            <TextField label='label' name='label' value={createLabel} onChange={(e) => setCreateLabel(e.target.value)} />
            <TextField
              label='enabled_on'
              name='enabled_on'
              type='date'
              value={createEnabledOn}
              onChange={(e) => setCreateEnabledOn(e.target.value)}
            />
            <Button disabled={createMutation.isPending} type='submit' variant='contained'>
              Create
            </Button>
          </Stack>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Disable
          </Typography>
          <Stack component='form' spacing={1.5} onSubmit={(e) => void onDisable(e)}>
            <TextField
              label='disabled_on'
              name='disabled_on'
              type='date'
              value={disableDay}
              onChange={(e) => setDisableDay(e.target.value)}
            />
            <Button disabled={disableMutation.isPending} type='submit' variant='contained'>
              Disable Selected
            </Button>
          </Stack>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Correct Label
          </Typography>
          <Stack component='form' spacing={1.5} onSubmit={(e) => void onCorrect(e)}>
            <TextField label='label' name='label' value={correctLabel} onChange={(e) => setCorrectLabel(e.target.value)} />
            <TextField
              label='correction_day'
              name='correction_day'
              type='date'
              value={correctDay}
              onChange={(e) => setCorrectDay(e.target.value)}
            />
            <Button disabled={correctMutation.isPending} type='submit' variant='contained'>
              Correct Selected
            </Button>
          </Stack>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Audit
          </Typography>
          {auditQuery.isError ? <Alert severity='error'>Audit load failed</Alert> : null}
          <table border={1} cellPadding={6} cellSpacing={0}>
            <thead>
              <tr>
                <th>tx_time</th>
                <th>event_type</th>
                <th>request_code</th>
                <th>initiator_uuid</th>
              </tr>
            </thead>
            <tbody>
              {auditEvents.map((e) => (
                <tr key={e.event_uuid}>
                  <td>{e.tx_time}</td>
                  <td>{e.event_type}</td>
                  <td>{e.request_code}</td>
                  <td>{e.initiator_uuid}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Paper>
      </Stack>
    </Box>
  )
}

