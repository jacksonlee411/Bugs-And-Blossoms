import { type FormEvent, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Alert, Box, Button, Container, Paper, Stack, TextField, Typography } from '@mui/material'
import { ApiClientError } from '../api/errors'
import { httpClient } from '../api/httpClient'
import { useAppPreferences } from '../app/providers/AppPreferencesContext'

export function LoginPage() {
  const navigate = useNavigate()
  const { t } = useAppPreferences()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)

    if (email.trim().length === 0 || password.trim().length === 0) {
      setError(t('login_failed'))
      return
    }

    setSubmitting(true)
    try {
      await httpClient.post('/iam/api/sessions', { email: email.trim(), password })
      navigate('/', { replace: true })
    } catch (raw) {
      const err = raw instanceof ApiClientError ? raw : new ApiClientError(t('login_failed'), 'UNKNOWN_ERROR')
      setError(err.message || t('login_failed'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Container maxWidth='sm' sx={{ py: 10 }}>
      <Paper elevation={3} sx={{ p: 4 }}>
        <Stack spacing={3} component='form' onSubmit={onSubmit}>
          <Box>
            <Typography variant='h4' component='h1'>
              {t('page_login_title')}
            </Typography>
            <Typography variant='body2' color='text.secondary'>
              {t('page_login_subtitle')}
            </Typography>
          </Box>

          {error ? <Alert severity='error'>{error}</Alert> : null}

          <TextField
            autoComplete='username'
            disabled={submitting}
            label={t('login_email')}
            name='email'
            onChange={(event) => setEmail(event.target.value)}
            required
            type='email'
            value={email}
          />
          <TextField
            autoComplete='current-password'
            disabled={submitting}
            label={t('login_password')}
            name='password'
            onChange={(event) => setPassword(event.target.value)}
            required
            type='password'
            value={password}
          />

          <Button disabled={submitting} type='submit' variant='contained' size='large'>
            {t('login_submit')}
          </Button>
        </Stack>
      </Paper>
    </Container>
  )
}
