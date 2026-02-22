import { type FormEvent, useMemo, useState } from 'react'
import { Alert, Box, Button, Paper, Stack, TextField, Typography } from '@mui/material'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { bindSetID, createSetID, listSetIDBindings, listSetIDs } from '../../api/setids'
import { PageHeader } from '../../components/PageHeader'

function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

function newRequestID(prefix: string): string {
  return `${prefix}:${Date.now()}`
}

export function SetIDGovernancePage() {
  const queryClient = useQueryClient()

  const [asOf, setAsOf] = useState(todayISO())
  const [createSetIDValue, setCreateSetIDValue] = useState('')
  const [createName, setCreateName] = useState('')

  const [bindOrgCode, setBindOrgCode] = useState('')
  const [bindSetIDValue, setBindSetIDValue] = useState('')

  const [error, setError] = useState<string | null>(null)

  const setidsQuery = useQuery({
    queryKey: ['setids'],
    queryFn: () => listSetIDs(),
    staleTime: 30_000
  })

  const bindingsQuery = useQuery({
    queryKey: ['setid-bindings', asOf],
    queryFn: () => listSetIDBindings({ asOf }),
    staleTime: 10_000
  })

  const createMutation = useMutation({
    mutationFn: (req: { setid: string; name: string; effective_date: string; request_id: string }) => createSetID(req),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['setids'] })
      setCreateSetIDValue('')
      setCreateName('')
    }
  })

  const bindMutation = useMutation({
    mutationFn: (req: { org_code: string; setid: string; effective_date: string; request_id: string }) => bindSetID(req),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['setid-bindings', asOf] })
      setBindOrgCode('')
      setBindSetIDValue('')
    }
  })

  const setids = useMemo(() => setidsQuery.data?.setids ?? [], [setidsQuery.data])
  const bindings = useMemo(() => bindingsQuery.data?.bindings ?? [], [bindingsQuery.data])

  async function onCreate(event: FormEvent) {
    event.preventDefault()
    setError(null)
    try {
      await createMutation.mutateAsync({
        setid: createSetIDValue.trim(),
        name: createName.trim(),
        effective_date: asOf,
        request_id: newRequestID('mui-setid-create')
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  async function onBind(event: FormEvent) {
    event.preventDefault()
    setError(null)
    try {
      await bindMutation.mutateAsync({
        org_code: bindOrgCode.trim(),
        setid: bindSetIDValue.trim(),
        effective_date: asOf,
        request_id: newRequestID('mui-setid-bind')
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <Box>
      <PageHeader title='SetID Governance' subtitle='List + create + bind (MUI-only)' />

      <Stack spacing={2}>
        {error ? <Alert severity='error'>{error}</Alert> : null}

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            As-of
          </Typography>
          <TextField label='as_of' name='as_of' type='date' value={asOf} onChange={(e) => setAsOf(e.target.value)} />
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Create SetID
          </Typography>
          <Stack
            component='form'
            spacing={1.5}
            onSubmit={(event) => {
              void onCreate(event)
            }}
          >
            <TextField label='setid' name='setid' value={createSetIDValue} onChange={(e) => setCreateSetIDValue(e.target.value)} />
            <TextField label='name' name='name' value={createName} onChange={(e) => setCreateName(e.target.value)} />
            <Button disabled={createMutation.isPending} type='submit' variant='contained'>
              Create
            </Button>
          </Stack>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Bind SetID
          </Typography>
          <Stack
            component='form'
            spacing={1.5}
            onSubmit={(event) => {
              void onBind(event)
            }}
          >
            <TextField label='org_code' name='org_code' value={bindOrgCode} onChange={(e) => setBindOrgCode(e.target.value)} />
            <TextField label='setid' name='setid' value={bindSetIDValue} onChange={(e) => setBindSetIDValue(e.target.value)} />
            <Button disabled={bindMutation.isPending} type='submit' variant='contained'>
              Bind
            </Button>
          </Stack>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h2' variant='h6' sx={{ mb: 1 }}>
            SetIDs
          </Typography>
          {setidsQuery.isError ? <Alert severity='error'>SetID list failed</Alert> : null}
          <table border={1} cellPadding={6} cellSpacing={0}>
            <thead>
              <tr>
                <th>setid</th>
                <th>name</th>
                <th>status</th>
              </tr>
            </thead>
            <tbody>
              {setids.map((s) => (
                <tr key={s.setid}>
                  <td>{s.setid}</td>
                  <td>{s.name}</td>
                  <td>{s.status}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h2' variant='h6' sx={{ mb: 1 }}>
            Bindings
          </Typography>
          {bindingsQuery.isError ? <Alert severity='error'>Binding list failed</Alert> : null}
          <table border={1} cellPadding={6} cellSpacing={0}>
            <thead>
              <tr>
                <th>org_unit_id</th>
                <th>setid</th>
                <th>valid_from</th>
                <th>valid_to</th>
              </tr>
            </thead>
            <tbody>
              {bindings.map((b) => (
                <tr key={`${b.org_unit_id}:${b.setid}:${b.valid_from}`}>
                  <td>{b.org_unit_id}</td>
                  <td>{b.setid}</td>
                  <td>{b.valid_from}</td>
                  <td>{b.valid_to}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Paper>
      </Stack>
    </Box>
  )
}

