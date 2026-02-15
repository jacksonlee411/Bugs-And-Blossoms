import { type FormEvent, useMemo, useState } from 'react'
import { Alert, Box, Button, Paper, Stack, TextField, Typography } from '@mui/material'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createPerson, listPersons } from '../../api/persons'
import { PageHeader } from '../../components/PageHeader'

function newRequestCode(prefix: string): string {
  return `${prefix}:${Date.now()}`
}

export function PersonsPage() {
  const queryClient = useQueryClient()
  const [pernr, setPernr] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [error, setError] = useState<string | null>(null)

  const personsQuery = useQuery({
    queryKey: ['persons'],
    queryFn: () => listPersons(),
    staleTime: 30_000
  })

  const createMutation = useMutation({
    mutationFn: (req: { pernr: string; display_name: string }) => createPerson(req),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['persons'] })
      setPernr('')
      setDisplayName('')
    }
  })

  const rows = useMemo(() => personsQuery.data?.persons ?? [], [personsQuery.data])

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    try {
      await createMutation.mutateAsync({ pernr: pernr.trim(), display_name: displayName.trim() })
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <Box>
      <PageHeader title='Person' subtitle='Create + list (MUI-only)' />

      <Stack spacing={2}>
        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Create
          </Typography>

          {error ? (
            <Alert severity='error' sx={{ mb: 1 }}>
              {error}
            </Alert>
          ) : null}

          <Stack
            component='form'
            onSubmit={(event) => {
              void onSubmit(event)
            }}
            spacing={1.5}
          >
            <TextField label='Pernr' name='pernr' value={pernr} onChange={(e) => setPernr(e.target.value)} />
            <TextField
              label='Display Name'
              name='display_name'
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
            />
            <input type='hidden' name='request_code' value={newRequestCode('mui-person')} />
            <Button disabled={createMutation.isPending} type='submit' variant='contained'>
              Create
            </Button>
          </Stack>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            List
          </Typography>

          {personsQuery.isError ? <Alert severity='error'>List failed</Alert> : null}

          <table border={1} cellPadding={6} cellSpacing={0}>
            <thead>
              <tr>
                <th>pernr</th>
                <th>display_name</th>
                <th>status</th>
                <th>person_uuid</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((p) => (
                <tr key={p.person_uuid}>
                  <td>{p.pernr}</td>
                  <td>{p.display_name}</td>
                  <td>{p.status}</td>
                  <td>
                    <code>{p.person_uuid}</code>
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
