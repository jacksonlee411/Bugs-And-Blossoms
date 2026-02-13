import { type PropsWithChildren, useCallback, useEffect, useMemo, useState } from 'react'
import DarkModeIcon from '@mui/icons-material/DarkMode'
import LanguageIcon from '@mui/icons-material/Language'
import LightModeIcon from '@mui/icons-material/LightMode'
import LockIcon from '@mui/icons-material/Lock'
import SearchIcon from '@mui/icons-material/Search'
import { Link as RouterLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import {
  AppBar,
  Box,
  Chip,
  Dialog,
  DialogContent,
  DialogTitle,
  Drawer,
  FormControl,
  IconButton,
  InputAdornment,
  List,
  ListItemIcon,
  ListItemButton,
  ListItemText,
  MenuItem,
  Select,
  TextField,
  Toolbar,
  Tooltip,
  Typography
} from '@mui/material'
import { useAppPreferences } from '../app/providers/AppPreferencesContext'
import { buildNavigationSearchEntries, commonSearchEntries } from '../navigation/config'
import { trackUiEvent } from '../observability/tracker'
import { createLocalSearchProvider, mergeSearchProviders } from '../search/globalSearch'
import type { NavItem } from '../types/navigation'

const drawerWidth = 240

interface AppShellProps {
  navItems: NavItem[]
}

export function AppShell({ navItems }: PropsWithChildren<AppShellProps>) {
  const navigate = useNavigate()
  const location = useLocation()
  const { hasPermission, locale, navDebugMode, setLocale, t, tenantId, themeMode, toggleThemeMode } =
    useAppPreferences()
  const [searchOpen, setSearchOpen] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState(() => buildNavigationSearchEntries(navItems))

  const sortedNavItems = useMemo(() => [...navItems].sort((left, right) => left.order - right.order), [navItems])
  const visibleNavItems = useMemo(
    () => sortedNavItems.filter((item) => hasPermission(item.permissionKey)),
    [hasPermission, sortedNavItems]
  )
  const hiddenNavItems = useMemo(
    () => sortedNavItems.filter((item) => !hasPermission(item.permissionKey)),
    [hasPermission, sortedNavItems]
  )

  const searchEntries = useMemo(
    () => [...buildNavigationSearchEntries(sortedNavItems), ...commonSearchEntries],
    [sortedNavItems]
  )
  const localizedSearchEntries = useMemo(
    () =>
      searchEntries.map((entry) => ({
        ...entry,
        keywords: [...entry.keywords, t(entry.labelKey)]
      })),
    [searchEntries, t]
  )
  const searchProvider = useMemo(
    () => mergeSearchProviders([createLocalSearchProvider(localizedSearchEntries)]),
    [localizedSearchEntries]
  )

  const runSearch = useCallback(
    async (query: string, shouldTrack: boolean) => {
      const startedAt = performance.now()
      const results = await searchProvider.search(query)
      setSearchResults(results)
      if (shouldTrack) {
        trackUiEvent({
          eventName: 'filter_submit',
          tenant: tenantId,
          module: 'shell',
          page: 'global-search',
          action: 'search_submit',
          result: 'success',
          latencyMs: Math.round(performance.now() - startedAt),
          metadata: {
            query_length: query.trim().length,
            result_count: results.length
          }
        })
      }
    },
    [searchProvider, tenantId]
  )

  useEffect(() => {
    function onShortcut(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        setSearchOpen(true)
      }
    }

    window.addEventListener('keydown', onShortcut)
    return () => window.removeEventListener('keydown', onShortcut)
  }, [])

  const handleSearchSelect = useCallback(
    (path: string) => {
      navigate(path)
      setSearchOpen(false)
      trackUiEvent({
        eventName: 'nav_click',
        tenant: tenantId,
        module: 'shell',
        page: 'global-search',
        action: `search_navigate:${path}`,
        result: 'success'
      })
    },
    [navigate, tenantId]
  )

  return (
    <Box sx={{ display: 'flex', minHeight: '100vh' }}>
      <AppBar enableColorOnDark color='primary' position='fixed' sx={{ zIndex: (theme) => theme.zIndex.drawer + 1 }}>
        <Toolbar sx={{ gap: 1 }}>
          <Typography component='h1' variant='h6'>
            {t('app_title')}
          </Typography>
          <Box sx={{ flex: 1 }} />
          <Tooltip title='Ctrl/Cmd + K'>
            <IconButton
              color='inherit'
              onClick={() => {
                setSearchOpen(true)
                void runSearch(searchQuery, false)
              }}
            >
              <SearchIcon />
            </IconButton>
          </Tooltip>
          <Tooltip title={t(themeMode === 'light' ? 'theme_dark' : 'theme_light')}>
            <IconButton color='inherit' onClick={toggleThemeMode}>
              {themeMode === 'light' ? <DarkModeIcon /> : <LightModeIcon />}
            </IconButton>
          </Tooltip>
          <FormControl size='small' sx={{ minWidth: 120 }}>
            <Select
              onChange={(event) => setLocale(event.target.value as 'en' | 'zh')}
              startAdornment={
                <InputAdornment position='start'>
                  <LanguageIcon fontSize='small' />
                </InputAdornment>
              }
              value={locale}
            >
              <MenuItem value='zh'>{t('language_zh')}</MenuItem>
              <MenuItem value='en'>{t('language_en')}</MenuItem>
            </Select>
          </FormControl>
        </Toolbar>
      </AppBar>
      <Drawer
        sx={{
          width: drawerWidth,
          flexShrink: 0,
          '& .MuiDrawer-paper': { boxSizing: 'border-box', width: drawerWidth }
        }}
        variant='permanent'
      >
        <Toolbar />
        <List>
          {visibleNavItems.map((item) => (
            <ListItemButton
              key={item.key}
              component={RouterLink}
              onClick={() =>
                trackUiEvent({
                  eventName: 'nav_click',
                  tenant: tenantId,
                  module: 'shell',
                  page: location.pathname,
                  action: `menu_navigate:${item.key}`,
                  result: 'success',
                  metadata: { target: item.path }
                })
              }
              selected={item.path === '/' ? location.pathname === '/' : location.pathname.startsWith(item.path)}
              to={item.path}
            >
              <ListItemIcon sx={{ minWidth: 34 }}>{item.icon}</ListItemIcon>
              <ListItemText primary={t(item.labelKey)} />
            </ListItemButton>
          ))}
          {navDebugMode
            ? hiddenNavItems.map((item) => (
                <ListItemButton disabled key={item.key}>
                  <ListItemIcon sx={{ minWidth: 34 }}>
                    <LockIcon fontSize='small' />
                  </ListItemIcon>
                  <ListItemText primary={t(item.labelKey)} secondary={t('nav_debug_mode')} />
                </ListItemButton>
              ))
            : null}
        </List>
      </Drawer>
      <Box component='main' sx={{ flexGrow: 1, p: 3 }}>
        <Toolbar />
        <Outlet />
      </Box>
      <Dialog fullWidth maxWidth='sm' onClose={() => setSearchOpen(false)} open={searchOpen}>
        <DialogTitle>{t('global_search')}</DialogTitle>
        <DialogContent>
          <Box component='form' onSubmit={(event) => event.preventDefault()} sx={{ mb: 2 }}>
            <TextField
              fullWidth
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  event.preventDefault()
                  void runSearch(searchQuery, true)
                }
              }}
              onChange={(event) => {
                const nextQuery = event.target.value
                setSearchQuery(nextQuery)
                void runSearch(nextQuery, false)
              }}
              placeholder={t('global_search_placeholder')}
              value={searchQuery}
            />
          </Box>
          <List>
            {searchResults.length === 0 ? (
              <Typography color='text.secondary' sx={{ px: 2, py: 1.5 }} variant='body2'>
                {t('global_search_empty')}
              </Typography>
            ) : null}
            {searchResults.map((entry) => (
              <ListItemButton key={entry.key} onClick={() => handleSearchSelect(entry.path)}>
                <ListItemText primary={t(entry.labelKey)} />
                <Chip
                  label={entry.source === 'navigation' ? t('search_source_navigation') : t('search_source_common')}
                  size='small'
                  variant='outlined'
                />
              </ListItemButton>
            ))}
          </List>
        </DialogContent>
      </Dialog>
    </Box>
  )
}
