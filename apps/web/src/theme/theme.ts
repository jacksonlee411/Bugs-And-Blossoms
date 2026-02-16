import type {} from '@mui/x-data-grid/themeAugmentation'
import { createTheme, type PaletteMode } from '@mui/material/styles'

export function buildAppTheme(mode: PaletteMode) {
  return createTheme({
    palette: {
      mode,
      primary: {
        main: '#09a7a3',
        contrastText: '#ffffff'
      },
      background: {
        default: mode === 'light' ? '#f7f9fb' : '#0f172a',
        paper: mode === 'light' ? '#fff' : '#0b1220'
      }
    },
    shape: {
      borderRadius: 10
    },
    typography: {
      fontFamily: 'Inter, system-ui, -apple-system, Segoe UI, Roboto, sans-serif'
    },
    components: {
      MuiAppBar: {
        styleOverrides: {
          root: {
            boxShadow: 'none',
            borderBottom: mode === 'light' ? '1px solid #e5e7eb' : '1px solid #1f2937'
          }
        }
      },
      MuiDataGrid: {
        styleOverrides: {
          root: {
            border: 'none',
            backgroundColor: 'transparent'
          }
        }
      }
    },
  })
}
