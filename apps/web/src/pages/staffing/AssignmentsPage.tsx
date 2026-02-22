import { type FormEvent, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Alert, Box, Button, Paper, Stack, TextField, Typography } from '@mui/material'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { listAssignments, upsertAssignment } from '../../api/assignments'
import { getPersonByPernr } from '../../api/persons'
import { listPositions } from '../../api/positions'
import { PageHeader } from '../../components/PageHeader'
import { SetIDExplainPanel } from '../../components/SetIDExplainPanel'

function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

function parseDateOrDefault(raw: string | null, fallback: string): string {
  if (!raw) {
    return fallback
  }
  const value = raw.trim()
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) {
    return fallback
  }
  return value
}

function parseOptionalValue(raw: string | null): string {
  if (!raw) {
    return ''
  }
  return raw.trim()
}

export function AssignmentsPage() {
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()
  const fallbackAsOf = useMemo(() => todayISO(), [])

  const asOf = parseDateOrDefault(searchParams.get('as_of'), fallbackAsOf)
  const pernr = parseOptionalValue(searchParams.get('pernr'))

  const [asOfInput, setAsOfInput] = useState(asOf)
  const [pernrInput, setPernrInput] = useState(pernr)
  const [effectiveDate, setEffectiveDate] = useState(asOf)
  const [positionUUID, setPositionUUID] = useState('')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    setAsOfInput(asOf)
    setEffectiveDate(asOf)
  }, [asOf])

  useEffect(() => {
    setPernrInput(pernr)
  }, [pernr])

  const personQuery = useQuery({
    enabled: pernr.trim().length > 0,
    queryKey: ['staffing', 'person-by-pernr', pernr],
    queryFn: async () => getPersonByPernr({ pernr })
  })

  const positionsQuery = useQuery({
    queryKey: ['staffing', 'positions', asOf],
    queryFn: async () => listPositions({ asOf })
  })

  const assignmentsQuery = useQuery({
    enabled: Boolean(personQuery.data?.person_uuid),
    queryKey: ['staffing', 'assignments', asOf, personQuery.data?.person_uuid ?? ''],
    queryFn: async () => listAssignments({ asOf, personUUID: personQuery.data!.person_uuid })
  })

  const createMutation = useMutation({
    mutationFn: upsertAssignment,
    onSuccess: async () => {
      const personUUID = personQuery.data?.person_uuid
      if (personUUID) {
        await queryClient.invalidateQueries({ queryKey: ['staffing', 'assignments', asOf, personUUID] })
      }
    }
  })

  function applyFilters() {
    const nextParams = new URLSearchParams(searchParams)
    nextParams.set('as_of', asOfInput)
    if (pernrInput.trim().length > 0) {
      nextParams.set('pernr', pernrInput.trim())
    } else {
      nextParams.delete('pernr')
    }
    setSearchParams(nextParams)
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)

    const personUUID = personQuery.data?.person_uuid
    if (!personUUID) {
      setError('person_uuid missing (load pernr first)')
      return
    }
    if (positionUUID.trim().length === 0) {
      setError('position_uuid required')
      return
    }

    try {
      await createMutation.mutateAsync({
        effective_date: effectiveDate,
        person_uuid: personUUID,
        position_uuid: positionUUID,
        status: 'active'
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  const positions = positionsQuery.data?.positions ?? []
  const assignments = assignmentsQuery.data?.assignments ?? []

  return (
    <Box>
      <PageHeader title='Staffing / Assignments' subtitle='MUI-only · create + list via JSON API' />

      <Stack spacing={2}>
        {error ? <Alert severity='error'>{error}</Alert> : null}

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Load
          </Typography>
          <Stack direction='row' spacing={1.5} alignItems='center'>
            <TextField
              label='as_of'
              name='as_of'
              type='date'
              value={asOfInput}
              onChange={(e) => setAsOfInput(e.target.value)}
            />
            <TextField
              label='pernr'
              name='pernr'
              value={pernrInput}
              onChange={(e) => setPernrInput(e.target.value)}
              placeholder='1001'
            />
            <Button onClick={applyFilters} variant='contained'>
              Load
            </Button>
          </Stack>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Context
          </Typography>
          <Typography variant='body2'>As-of: {asOf}</Typography>
          <Typography variant='body2'>Pernr: {pernr || '(none)'}</Typography>
          {personQuery.isError ? <Alert severity='error'>Person load failed</Alert> : null}
          {personQuery.data ? (
            <>
              <Typography variant='body2'>Person UUID: {personQuery.data.person_uuid}</Typography>
              <Typography variant='body2'>Name: {personQuery.data.display_name}</Typography>
            </>
          ) : null}
        </Paper>

        <SetIDExplainPanel
          initialAsOf={asOf}
          initialScopeCode='staffing'
          title='SetID Explain（Assignments）'
          subtitle='用于排查任职写入时的上下文拒绝与字段策略命中。'
        />

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Create
          </Typography>
          <Stack component='form' spacing={1.5} onSubmit={(e) => void onSubmit(e)}>
            <TextField
              label='effective_date'
              name='effective_date'
              type='date'
              value={effectiveDate}
              onChange={(e) => setEffectiveDate(e.target.value)}
            />
            <label>
              position_uuid
              <select
                name='position_uuid'
                value={positionUUID}
                onChange={(e) => setPositionUUID(e.target.value)}
                style={{ marginLeft: 8, padding: 6, minWidth: 360 }}
              >
                <option value=''>-- select --</option>
                {positions.map((p) => (
                  <option key={p.position_uuid} value={p.position_uuid}>
                    {p.name} · {p.job_profile_code} · {p.org_code}
                  </option>
                ))}
              </select>
            </label>
            <Button disabled={createMutation.isPending} type='submit' variant='contained'>
              Create
            </Button>
          </Stack>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h2' variant='h6' sx={{ mb: 1 }}>
            Timeline / List
          </Typography>
          {assignmentsQuery.isError ? <Alert severity='error'>List failed</Alert> : null}
          <table border={1} cellPadding={6} cellSpacing={0}>
            <thead>
              <tr>
                <th>effective_date</th>
                <th>status</th>
                <th>position_uuid</th>
                <th>assignment_uuid</th>
              </tr>
            </thead>
            <tbody>
              {assignments.map((a) => (
                <tr key={a.assignment_uuid}>
                  <td>{a.effective_date}</td>
                  <td>{a.status}</td>
                  <td>
                    <code>{a.position_uuid}</code>
                  </td>
                  <td>
                    <code>{a.assignment_uuid}</code>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Paper>
      </Stack>
    </Box>
  )
}
