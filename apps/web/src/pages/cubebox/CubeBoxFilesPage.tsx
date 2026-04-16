import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline'
import UploadFileIcon from '@mui/icons-material/UploadFile'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  List,
  ListItem,
  ListItemText,
  Stack,
  Typography
} from '@mui/material'
import { useEffect, useRef, useState } from 'react'
import { useAppPreferences } from '../../app/providers/AppPreferencesContext'
import { deleteCubeBoxFile, listCubeBoxFiles, uploadCubeBoxFile, type CubeBoxFile } from '../../api/cubebox'
import { cubeBoxErrorMessage } from './errorMessage'

function fileLabel(item: CubeBoxFile): string {
  return item.filename ?? item.file_name
}

function fileContentType(item: CubeBoxFile): string {
  return item.content_type ?? item.media_type
}

function fileCreatedAt(item: CubeBoxFile): string {
  return item.created_at ?? item.uploaded_at
}

function fileConversationID(item: CubeBoxFile): string | undefined {
  if (typeof item.conversation_id === 'string' && item.conversation_id.trim().length > 0) {
    return item.conversation_id
  }
  const firstLink = Array.isArray(item.links) ? item.links[0] : undefined
  return firstLink?.conversation_id
}

export function CubeBoxFilesPage() {
  const { locale, t } = useAppPreferences()
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const [items, setItems] = useState<CubeBoxFile[]>([])
  const [busy, setBusy] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')

  async function load() {
    const response = await listCubeBoxFiles()
    setItems(response.items)
  }

  useEffect(() => {
    void load().catch((error) => setErrorMessage(cubeBoxErrorMessage(error, t('cubebox_error_files_load'), locale)))
  }, [locale, t])

  async function handleUpload(fileList: FileList | null) {
    const file = fileList?.item(0)
    if (!file) {
      return
    }
    setBusy(true)
    setErrorMessage('')
    try {
      await uploadCubeBoxFile(file)
      await load()
    } catch (error) {
      setErrorMessage(cubeBoxErrorMessage(error, t('cubebox_error_files_upload'), locale))
    } finally {
      setBusy(false)
      if (fileInputRef.current) {
        fileInputRef.current.value = ''
      }
    }
  }

  async function handleDelete(fileID: string) {
    setBusy(true)
    setErrorMessage('')
    try {
      await deleteCubeBoxFile(fileID)
      await load()
    } catch (error) {
      setErrorMessage(cubeBoxErrorMessage(error, t('cubebox_error_files_delete'), locale))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Stack spacing={2}>
      <Stack alignItems='center' direction='row' spacing={1}>
        <Typography variant='h5'>{t('cubebox_files_title')}</Typography>
        <Box sx={{ flex: 1 }} />
        <input
          hidden
          onChange={(event) => void handleUpload(event.target.files)}
          ref={fileInputRef}
          type='file'
        />
        <Button
          disabled={busy}
          onClick={() => fileInputRef.current?.click()}
          startIcon={<UploadFileIcon />}
          variant='contained'
        >
          {t('cubebox_files_upload')}
        </Button>
      </Stack>

      <Typography color='text.secondary' variant='body2'>
        {t('cubebox_files_subtitle')}
      </Typography>

      {errorMessage ? <Alert severity='warning'>{errorMessage}</Alert> : null}

      <Card>
        <CardContent>
          <List disablePadding>
            {items.map((item) => (
              <ListItem
                data-testid='cubebox-file-item'
                divider
                key={item.file_id}
                secondaryAction={(
                  <Button
                    color='error'
                    disabled={busy}
                    onClick={() => void handleDelete(item.file_id)}
                    size='small'
                    startIcon={<DeleteOutlineIcon />}
                  >
                    {t('cubebox_files_delete')}
                  </Button>
                )}
              >
                <ListItemText
                  primary={fileLabel(item)}
                  secondary={`${fileContentType(item)} · ${item.size_bytes} bytes · ${fileCreatedAt(item)}`}
                />
                {fileConversationID(item) ? <Chip label={fileConversationID(item)} size='small' variant='outlined' /> : null}
              </ListItem>
            ))}
            {items.length === 0 ? (
              <Typography color='text.secondary' variant='body2'>
                {t('cubebox_files_empty')}
              </Typography>
            ) : null}
          </List>
        </CardContent>
      </Card>
    </Stack>
  )
}
