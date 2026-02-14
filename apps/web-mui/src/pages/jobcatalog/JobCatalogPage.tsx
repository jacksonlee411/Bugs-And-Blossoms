import { type FormEvent, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Alert, Box, Button, Paper, Stack, TextField, Typography } from '@mui/material'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { applyJobCatalogAction, getJobCatalog, listOwnedJobCatalogPackages } from '../../api/jobCatalog'
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

function newRequestCode(prefix: string): string {
  return `${prefix}:${Date.now()}`
}

export function JobCatalogPage() {
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()
  const fallbackAsOf = useMemo(() => todayISO(), [])

  const asOf = parseDateOrDefault(searchParams.get('as_of'), fallbackAsOf)
  const packageCode = parseOptionalValue(searchParams.get('package_code'))

  const [asOfInput, setAsOfInput] = useState(asOf)
  const [packageCodeInput, setPackageCodeInput] = useState(packageCode)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    setAsOfInput(asOf)
  }, [asOf])

  useEffect(() => {
    setPackageCodeInput(packageCode)
  }, [packageCode])

  const packagesQuery = useQuery({
    queryKey: ['jobcatalog', 'owned-packages', asOf],
    queryFn: async () => listOwnedJobCatalogPackages({ asOf })
  })

  const catalogQuery = useQuery({
    enabled: packageCode.length > 0,
    queryKey: ['jobcatalog', 'catalog', asOf, packageCode],
    queryFn: async () => getJobCatalog({ asOf, packageCode })
  })

  const actionMutation = useMutation({
    mutationFn: applyJobCatalogAction,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['jobcatalog', 'catalog', asOf, packageCode] })
    }
  })

  const [actionEffectiveDate, setActionEffectiveDate] = useState(asOf)

  // Create Job Family Group
  const [groupCode, setGroupCode] = useState('')
  const [groupName, setGroupName] = useState('')

  // Create Job Family
  const [familyCode, setFamilyCode] = useState('')
  const [familyName, setFamilyName] = useState('')
  const [familyGroupCode, setFamilyGroupCode] = useState('')

  // Update Job Family Group
  const [updateFamilyCode, setUpdateFamilyCode] = useState('')
  const [updateFamilyGroupCode, setUpdateFamilyGroupCode] = useState('')

  // Create Job Level
  const [levelCode, setLevelCode] = useState('')
  const [levelName, setLevelName] = useState('')

  // Create Job Profile
  const [profileCode, setProfileCode] = useState('')
  const [profileName, setProfileName] = useState('')
  const [profileFamilyCodesCSV, setProfileFamilyCodesCSV] = useState('')
  const [profilePrimaryFamilyCode, setProfilePrimaryFamilyCode] = useState('')

  function applyFilters() {
    const nextParams = new URLSearchParams(searchParams)
    nextParams.set('as_of', asOfInput)
    if (packageCodeInput.trim().length > 0) {
      nextParams.set('package_code', packageCodeInput.trim())
    } else {
      nextParams.delete('package_code')
    }
    setSearchParams(nextParams)
    setActionEffectiveDate(asOfInput)
  }

  async function submitAction(request: Parameters<typeof applyJobCatalogAction>[0]) {
    if (packageCode.trim().length === 0) {
      setError('package_code required')
      return
    }
    setError(null)
    try {
      await actionMutation.mutateAsync(request)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  async function onCreateGroup(event: FormEvent) {
    event.preventDefault()
    await submitAction({
      action: 'create_job_family_group',
      package_code: packageCode,
      effective_date: actionEffectiveDate,
      code: groupCode.trim(),
      name: groupName.trim()
    })
  }

  async function onCreateFamily(event: FormEvent) {
    event.preventDefault()
    await submitAction({
      action: 'create_job_family',
      package_code: packageCode,
      effective_date: actionEffectiveDate,
      code: familyCode.trim(),
      name: familyName.trim(),
      group_code: familyGroupCode.trim()
    })
  }

  async function onUpdateFamilyGroup(event: FormEvent) {
    event.preventDefault()
    await submitAction({
      action: 'update_job_family_group',
      package_code: packageCode,
      effective_date: actionEffectiveDate,
      code: updateFamilyCode.trim(),
      group_code: updateFamilyGroupCode.trim()
    })
  }

  async function onCreateLevel(event: FormEvent) {
    event.preventDefault()
    await submitAction({
      action: 'create_job_level',
      package_code: packageCode,
      effective_date: actionEffectiveDate,
      code: levelCode.trim(),
      name: levelName.trim()
    })
  }

  async function onCreateProfile(event: FormEvent) {
    event.preventDefault()
    await submitAction({
      action: 'create_job_profile',
      package_code: packageCode,
      effective_date: actionEffectiveDate,
      code: profileCode.trim(),
      name: profileName.trim(),
      family_codes_csv: profileFamilyCodesCSV.trim(),
      primary_family_code: profilePrimaryFamilyCode.trim()
    })
  }

  const ownedPackages = packagesQuery.data ?? []
  const catalog = catalogQuery.data ?? null

  return (
    <Box>
      <PageHeader title='Job Catalog' subtitle='MUI-only · list + actions (create/update) via JSON API' />

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
              label='package_code'
              name='package_code'
              value={packageCodeInput}
              onChange={(e) => setPackageCodeInput(e.target.value)}
              placeholder='DEFLT'
            />
            <Button onClick={applyFilters} variant='contained'>
              Load
            </Button>
          </Stack>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h2' variant='h6' sx={{ mb: 1 }}>
            Owned Packages (scope=jobcatalog)
          </Typography>
          {packagesQuery.isError ? <Alert severity='error'>Owned packages load failed</Alert> : null}
          <table border={1} cellPadding={6} cellSpacing={0}>
            <thead>
              <tr>
                <th>package_code</th>
                <th>owner_setid</th>
                <th>status</th>
                <th>effective_date</th>
              </tr>
            </thead>
            <tbody>
              {ownedPackages.map((p) => (
                <tr key={p.package_id}>
                  <td>{p.package_code}</td>
                  <td>{p.owner_setid}</td>
                  <td>{p.status}</td>
                  <td>{p.effective_date}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Selection
          </Typography>
          <Typography variant='body2'>Package: {packageCode || '(none)'}</Typography>
          <Typography variant='body2'>As-of: {asOf}</Typography>
          {catalog ? (
            <>
              <Typography variant='body2'>Owner SetID: {catalog.view.owner_setid || '-'}</Typography>
              <Typography variant='body2'>Read-only: {catalog.view.read_only ? 'true' : 'false'}</Typography>
            </>
          ) : null}
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Action Effective Date
          </Typography>
          <TextField
            label='effective_date'
            name='effective_date'
            type='date'
            value={actionEffectiveDate}
            onChange={(e) => setActionEffectiveDate(e.target.value)}
          />
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Create Job Family Group
          </Typography>
          <Stack component='form' spacing={1.5} onSubmit={(e) => void onCreateGroup(e)}>
            <TextField label='code' name='code' value={groupCode} onChange={(e) => setGroupCode(e.target.value)} />
            <TextField label='name' name='name' value={groupName} onChange={(e) => setGroupName(e.target.value)} />
            <input type='hidden' name='request_code' value={newRequestCode('mui-jobcatalog-group')} />
            <Button disabled={actionMutation.isPending} type='submit' variant='contained'>
              Create
            </Button>
          </Stack>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Create Job Family
          </Typography>
          <Stack component='form' spacing={1.5} onSubmit={(e) => void onCreateFamily(e)}>
            <TextField label='code' name='code' value={familyCode} onChange={(e) => setFamilyCode(e.target.value)} />
            <TextField label='name' name='name' value={familyName} onChange={(e) => setFamilyName(e.target.value)} />
            <TextField
              label='group_code'
              name='group_code'
              value={familyGroupCode}
              onChange={(e) => setFamilyGroupCode(e.target.value)}
            />
            <input type='hidden' name='request_code' value={newRequestCode('mui-jobcatalog-family')} />
            <Button disabled={actionMutation.isPending} type='submit' variant='contained'>
              Create
            </Button>
          </Stack>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Update Job Family Group
          </Typography>
          <Stack component='form' spacing={1.5} onSubmit={(e) => void onUpdateFamilyGroup(e)}>
            <TextField
              label='job_family_code'
              name='code'
              value={updateFamilyCode}
              onChange={(e) => setUpdateFamilyCode(e.target.value)}
            />
            <TextField
              label='job_family_group_code'
              name='group_code'
              value={updateFamilyGroupCode}
              onChange={(e) => setUpdateFamilyGroupCode(e.target.value)}
            />
            <input type='hidden' name='request_code' value={newRequestCode('mui-jobcatalog-family-reparent')} />
            <Button disabled={actionMutation.isPending} type='submit' variant='contained'>
              Update
            </Button>
          </Stack>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Create Job Level
          </Typography>
          <Stack component='form' spacing={1.5} onSubmit={(e) => void onCreateLevel(e)}>
            <TextField label='code' name='code' value={levelCode} onChange={(e) => setLevelCode(e.target.value)} />
            <TextField label='name' name='name' value={levelName} onChange={(e) => setLevelName(e.target.value)} />
            <input type='hidden' name='request_code' value={newRequestCode('mui-jobcatalog-level')} />
            <Button disabled={actionMutation.isPending} type='submit' variant='contained'>
              Create
            </Button>
          </Stack>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h3' variant='subtitle1' sx={{ mb: 1 }}>
            Create Job Profile
          </Typography>
          <Stack component='form' spacing={1.5} onSubmit={(e) => void onCreateProfile(e)}>
            <TextField
              label='code'
              name='code'
              value={profileCode}
              onChange={(e) => setProfileCode(e.target.value)}
            />
            <TextField
              label='name'
              name='name'
              value={profileName}
              onChange={(e) => setProfileName(e.target.value)}
            />
            <TextField
              label='family_codes_csv'
              name='family_codes_csv'
              value={profileFamilyCodesCSV}
              onChange={(e) => setProfileFamilyCodesCSV(e.target.value)}
              placeholder='JF-BE,JF-FE'
            />
            <TextField
              label='primary_family_code'
              name='primary_family_code'
              value={profilePrimaryFamilyCode}
              onChange={(e) => setProfilePrimaryFamilyCode(e.target.value)}
              placeholder='JF-BE'
            />
            <input type='hidden' name='request_code' value={newRequestCode('mui-jobcatalog-profile')} />
            <Button disabled={actionMutation.isPending} type='submit' variant='contained'>
              Create
            </Button>
          </Stack>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h2' variant='h6' sx={{ mb: 1 }}>
            Job Family Groups
          </Typography>
          {catalogQuery.isError ? <Alert severity='error'>Catalog load failed</Alert> : null}
          <table border={1} cellPadding={6} cellSpacing={0}>
            <thead>
              <tr>
                <th>code</th>
                <th>name</th>
                <th>effective_day</th>
              </tr>
            </thead>
            <tbody>
              {(catalog?.job_family_groups ?? []).map((g) => (
                <tr key={g.job_family_group_uuid}>
                  <td>{g.job_family_group_code}</td>
                  <td>{g.name}</td>
                  <td>{g.effective_day}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h2' variant='h6' sx={{ mb: 1 }}>
            Job Families
          </Typography>
          <table border={1} cellPadding={6} cellSpacing={0}>
            <thead>
              <tr>
                <th>code</th>
                <th>name</th>
                <th>group_code</th>
                <th>effective_day</th>
              </tr>
            </thead>
            <tbody>
              {(catalog?.job_families ?? []).map((f) => (
                <tr key={f.job_family_uuid}>
                  <td>{f.job_family_code}</td>
                  <td>{f.name}</td>
                  <td>{f.job_family_group_code}</td>
                  <td>{f.effective_day}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h2' variant='h6' sx={{ mb: 1 }}>
            Job Levels
          </Typography>
          <table border={1} cellPadding={6} cellSpacing={0}>
            <thead>
              <tr>
                <th>code</th>
                <th>name</th>
                <th>effective_day</th>
              </tr>
            </thead>
            <tbody>
              {(catalog?.job_levels ?? []).map((l) => (
                <tr key={l.job_level_uuid}>
                  <td>{l.job_level_code}</td>
                  <td>{l.name}</td>
                  <td>{l.effective_day}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography component='h2' variant='h6' sx={{ mb: 1 }}>
            Job Profiles
          </Typography>
          <table border={1} cellPadding={6} cellSpacing={0}>
            <thead>
              <tr>
                <th>code</th>
                <th>name</th>
                <th>families</th>
                <th>primary</th>
                <th>effective_day</th>
              </tr>
            </thead>
            <tbody>
              {(catalog?.job_profiles ?? []).map((p) => (
                <tr key={p.job_profile_uuid}>
                  <td>{p.job_profile_code}</td>
                  <td>{p.name}</td>
                  <td>{p.family_codes_csv}</td>
                  <td>{p.primary_family_code}</td>
                  <td>{p.effective_day}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Paper>
      </Stack>
    </Box>
  )
}

