import { type FormEvent, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Alert, Box, Button, Paper, Stack, TextField, Typography } from '@mui/material'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getPositionOptions, listPositions, upsertPosition } from '../../api/positions'
import { PageHeader } from '../../components/PageHeader'

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

export function PositionsPage() {
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()
  const fallbackAsOf = useMemo(() => todayISO(), [])

  const asOf = parseDateOrDefault(searchParams.get('as_of'), fallbackAsOf)
  const orgCode = parseOptionalValue(searchParams.get('org_code'))

  const [asOfInput, setAsOfInput] = useState(asOf)
  const [orgCodeInput, setOrgCodeInput] = useState(orgCode)
  const [effectiveDate, setEffectiveDate] = useState(asOf)
  const [jobProfileUUID, setJobProfileUUID] = useState('')
  const [name, setName] = useState('')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    setAsOfInput(asOf)
    setEffectiveDate(asOf)
  }, [asOf])

  useEffect(() => {
    setOrgCodeInput(orgCode)
  }, [orgCode])

  const positionsQuery = useQuery({
    queryKey: ['staffing', 'positions', asOf],
    queryFn: async () => listPositions({ asOf })
  })

  const optionsQuery = useQuery({
    enabled: orgCode.trim().length > 0,
    queryKey: ['staffing', 'positions-options', asOf, orgCode],
    queryFn: async () => getPositionOptions({ asOf, orgCode })
  })

  const createMutation = useMutation({
    mutationFn: upsertPosition,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['staffing', 'positions', asOf] })
      setName('')
    }
  })

  function applyFilters() {
    const nextParams = new URLSearchParams(searchParams)
    nextParams.set('as_of', asOfInput)
    if (orgCodeInput.trim().length > 0) {
      nextParams.set('org_code', orgCodeInput.trim())
    } else {
      nextParams.delete('org_code')
    }
    setSearchParams(nextParams)
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)

    if (orgCode.trim().length === 0) {
      setError('org_code required')
      return
    }
    if (jobProfileUUID.trim().length === 0) {
      setError('job_profile_uuid required')
      return
    }
    if (name.trim().length === 0) {
      setError('name required')
      return
    }

    try {
      await createMutation.mutateAsync({
        effective_date: effectiveDate,
        org_code: orgCode,
        job_profile_uuid: jobProfileUUID,
        name: name.trim()
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  const options = optionsQuery.data ?? null
  const positions = positionsQuery.data?.positions ?? []
  const filteredPositions = orgCode.trim().length === 0 ? positions : positions.filter((p) => p.org_code === orgCode)

  return (
    <Box>
      <PageHeader title='Staffing / Positions' subtitle='MUI-only · create + list via JSON API' />

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
              label='org_code'
              name='org_code'
              value={orgCodeInput}
              onChange={(e) => setOrgCodeInput(e.target.value)}
              placeholder='ROOT'
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
          <Typography variant='body2'>Org Code: {orgCode || '(none)'}</Typography>
          <Typography variant='body2'>SetID: {options?.jobcatalog_setid ?? '-'}</Typography>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Create
          </Typography>

          {optionsQuery.isError ? <Alert severity='error'>Options load failed</Alert> : null}

          <Stack component='form' spacing={1.5} onSubmit={(e) => void onSubmit(e)}>
            <TextField
              label='effective_date'
              name='effective_date'
              type='date'
              value={effectiveDate}
              onChange={(e) => setEffectiveDate(e.target.value)}
            />

            <label>
              job_profile_uuid
              <select
                name='job_profile_uuid'
                value={jobProfileUUID}
                onChange={(e) => setJobProfileUUID(e.target.value)}
                style={{ marginLeft: 8, padding: 6, minWidth: 320 }}
              >
                <option value=''>-- select --</option>
                {(options?.job_profiles ?? []).map((p) => (
                  <option key={p.job_profile_uuid} value={p.job_profile_uuid}>
                    {p.job_profile_code} · {p.name}
                  </option>
                ))}
              </select>
            </label>

            <TextField label='name' name='name' value={name} onChange={(e) => setName(e.target.value)} />
            <Button disabled={createMutation.isPending} type='submit' variant='contained'>
              Create
            </Button>
          </Stack>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h2' variant='h6' sx={{ mb: 1 }}>
            List
          </Typography>
          {positionsQuery.isError ? <Alert severity='error'>List failed</Alert> : null}
          <table border={1} cellPadding={6} cellSpacing={0}>
            <thead>
              <tr>
                <th>name</th>
                <th>position_uuid</th>
                <th>org_code</th>
                <th>job_profile_code</th>
                <th>setid</th>
                <th>effective_date</th>
              </tr>
            </thead>
            <tbody>
              {filteredPositions.map((p) => (
                <tr key={p.position_uuid}>
                  <td>{p.name}</td>
                  <td>
                    <code>{p.position_uuid}</code>
                  </td>
                  <td>{p.org_code}</td>
                  <td>{p.job_profile_code}</td>
                  <td>{p.jobcatalog_setid}</td>
                  <td>{p.effective_date}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Paper>
      </Stack>
    </Box>
  )
}

