import { type PropsWithChildren, useCallback, useEffect, useMemo, useState } from 'react'
import AutoAwesomeIcon from '@mui/icons-material/AutoAwesome'
import DarkModeIcon from '@mui/icons-material/DarkMode'
import ExpandLessIcon from '@mui/icons-material/ExpandLess'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import LanguageIcon from '@mui/icons-material/Language'
import LightModeIcon from '@mui/icons-material/LightMode'
import LockIcon from '@mui/icons-material/Lock'
import LogoutIcon from '@mui/icons-material/Logout'
import SearchIcon from '@mui/icons-material/Search'
import { Link as RouterLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import {
  AppBar,
  Box,
  Chip,
  Collapse,
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
  Paper,
  Select,
  TextField,
  Toolbar,
  Tooltip,
  Typography
} from '@mui/material'
import useMediaQuery from '@mui/material/useMediaQuery'
import { useTheme } from '@mui/material/styles'
import { useAppPreferences } from '../app/providers/AppPreferencesContext'
import { buildNavigationSearchEntries, commonSearchEntries } from '../navigation/config'
import { trackUiEvent } from '../observability/tracker'
import { createLocalSearchProvider, mergeSearchProviders } from '../search/globalSearch'
import type { NavItem } from '../types/navigation'
import { CubeBoxProvider } from '../pages/cubebox/CubeBoxProvider'
import { CubeBoxPanel } from '../pages/cubebox/CubeBoxPanel'

const drawerWidth = 240
const cubeBoxPanelWidth = 416
const shellHeaderOffset = {
  xs: '56px',
  sm: '64px'
} as const
const cubeBoxPanelHeight = {
  xs: `calc(100% - ${shellHeaderOffset.xs})`,
  sm: `calc(100% - ${shellHeaderOffset.sm})`
} as const

type CubeBoxShellMode = 'wide' | 'medium' | 'compact'

interface AppShellProps {
  navItems: NavItem[]
}

export function AppShell({ navItems }: PropsWithChildren<AppShellProps>) {
  const navigate = useNavigate()
  const location = useLocation()
  const theme = useTheme()
  const { hasPermission, locale, navDebugMode, setLocale, t, tenantId, themeMode, toggleThemeMode } =
    useAppPreferences()
  const [searchOpen, setSearchOpen] = useState(false)
  const [cubeBoxOpen, setCubeBoxOpen] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState(() => buildNavigationSearchEntries(navItems))
  const cubeBoxWide = useMediaQuery(theme.breakpoints.up('lg'))
  const cubeBoxDesktop = useMediaQuery(theme.breakpoints.up('md'))
  const cubeBoxShellMode: CubeBoxShellMode = cubeBoxWide ? 'wide' : cubeBoxDesktop ? 'medium' : 'compact'
  const cubeBoxDrawerVariant = cubeBoxDesktop ? 'persistent' : 'temporary'
  const cubeBoxToggleLabel = t(cubeBoxOpen ? 'cubebox_close_drawer' : 'cubebox_open_drawer')
  const canAccessCubeBox = hasPermission('cubebox.conversations.read') || hasPermission('cubebox.conversations.use')

  const sortedNavItems = useMemo(() => [...navItems].sort((left, right) => left.order - right.order), [navItems])
  const visibleNavItems = useMemo(
    () => sortedNavItems.filter((item) => hasPermission(item.permissionKey)),
    [hasPermission, sortedNavItems]
  )
  const hiddenNavItems = useMemo(
    () => sortedNavItems.filter((item) => !hasPermission(item.permissionKey)),
    [hasPermission, sortedNavItems]
  )
  const topLevelNavItems = useMemo(
    () => visibleNavItems.filter((item) => !item.parentKey),
    [visibleNavItems]
  )
  const childNavItemsByParent = useMemo(() => {
    const groups: Record<string, NavItem[]> = {}
    visibleNavItems.forEach((item) => {
      const parentKey = item.parentKey
      if (!parentKey) {
        return
      }
      const bucket = groups[parentKey] ?? []
      bucket.push(item)
      groups[parentKey] = bucket
    })
    Object.values(groups).forEach((items) => items.sort((left, right) => left.order - right.order))
    return groups
  }, [visibleNavItems])
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({})

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

  useEffect(() => {
    if (!cubeBoxOpen || searchOpen) {
      return
    }

    function onEscape(event: KeyboardEvent) {
      if (event.defaultPrevented || event.key !== 'Escape') {
        return
      }
      setCubeBoxOpen(false)
    }

    window.addEventListener('keydown', onEscape)
    return () => window.removeEventListener('keydown', onEscape)
  }, [cubeBoxOpen, searchOpen])

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

  async function handleLogout() {
    await fetch('/logout', { credentials: 'include', method: 'POST' })
    navigate('/login', { replace: true })
  }

  const toggleCubeBox = useCallback(() => {
    const nextOpen = !cubeBoxOpen
    setCubeBoxOpen(nextOpen)
    trackUiEvent({
      eventName: 'nav_click',
      tenant: tenantId,
      module: 'shell',
      page: location.pathname,
      action: nextOpen ? 'cubebox_drawer_open' : 'cubebox_drawer_close',
      result: 'success',
      metadata: {
        shell_mode: cubeBoxShellMode
      }
    })
  }, [cubeBoxOpen, cubeBoxShellMode, location.pathname, tenantId])

  return (
    <CubeBoxProvider>
      <Box
        data-cubebox-shell-mode={cubeBoxShellMode}
        sx={{
          '--app-shell-header-offset': shellHeaderOffset,
          display: 'flex',
          minHeight: '100vh'
        }}
      >
        <AppBar
          enableColorOnDark
          color='primary'
          position='fixed'
          sx={{
            zIndex: (theme) => theme.zIndex.drawer + 1,
            color: 'common.white',
            '& .MuiSvgIcon-root': { color: 'common.white' }
          }}
        >
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
            {canAccessCubeBox ? (
              <Tooltip title={cubeBoxToggleLabel}>
                <IconButton
                  aria-label={cubeBoxToggleLabel}
                  aria-pressed={cubeBoxOpen}
                  color='inherit'
                  onClick={toggleCubeBox}
                >
                  <AutoAwesomeIcon />
                </IconButton>
              </Tooltip>
            ) : null}
            <Tooltip title={t(themeMode === 'light' ? 'theme_dark' : 'theme_light')}>
              <IconButton color='inherit' onClick={toggleThemeMode}>
                {themeMode === 'light' ? <DarkModeIcon /> : <LightModeIcon />}
              </IconButton>
            </Tooltip>
            <FormControl size='small' sx={{ minWidth: 120 }}>
              <Select
                sx={{
                  color: 'common.white',
                  '& .MuiSelect-icon': { color: 'common.white' },
                  '& .MuiOutlinedInput-notchedOutline': { borderColor: 'rgba(255,255,255,0.5)' },
                  '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: 'common.white' },
                  '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: 'common.white' }
                }}
                onChange={(event) => setLocale(event.target.value as 'en' | 'zh')}
                startAdornment={
                  <InputAdornment position='start'>
                    <LanguageIcon fontSize='small' sx={{ color: 'common.white' }} />
                  </InputAdornment>
                }
                value={locale}
              >
                <MenuItem value='zh'>{t('language_zh')}</MenuItem>
                <MenuItem value='en'>{t('language_en')}</MenuItem>
              </Select>
            </FormControl>
            <Tooltip title={t('action_logout')}>
              <IconButton color='inherit' onClick={() => void handleLogout()}>
                <LogoutIcon />
              </IconButton>
            </Tooltip>
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
            {topLevelNavItems.map((item) => {
              const children = childNavItemsByParent[item.key] ?? []
              const parentSelected = item.path === '/' ? location.pathname === '/' : location.pathname.startsWith(item.path)

              if (children.length === 0) {
                return (
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
                    selected={parentSelected}
                    to={item.path}
                  >
                    <ListItemIcon sx={{ minWidth: 34 }}>{item.icon}</ListItemIcon>
                    <ListItemText primary={t(item.labelKey)} />
                  </ListItemButton>
                )
              }

              const expanded = expandedGroups[item.key] ?? parentSelected
              return (
                <Box key={item.key}>
                  <ListItemButton
                    component={RouterLink}
                    onClick={() => {
                      setExpandedGroups((previous) => ({
                        ...previous,
                        [item.key]: !(previous[item.key] ?? parentSelected)
                      }))
                      trackUiEvent({
                        eventName: 'nav_click',
                        tenant: tenantId,
                        module: 'shell',
                        page: location.pathname,
                        action: `menu_navigate:${item.key}`,
                        result: 'success',
                        metadata: { target: item.path }
                      })
                    }}
                    selected={parentSelected}
                    to={item.path}
                  >
                    <ListItemIcon sx={{ minWidth: 34 }}>{item.icon}</ListItemIcon>
                    <ListItemText primary={t(item.labelKey)} />
                    {expanded ? <ExpandLessIcon fontSize='small' /> : <ExpandMoreIcon fontSize='small' />}
                  </ListItemButton>
                  <Collapse in={expanded} timeout='auto' unmountOnExit>
                    <List component='div' disablePadding>
                      {children.map((child) => (
                        <ListItemButton
                          key={child.key}
                          component={RouterLink}
                          onClick={() =>
                            trackUiEvent({
                              eventName: 'nav_click',
                              tenant: tenantId,
                              module: 'shell',
                              page: location.pathname,
                              action: `menu_navigate:${child.key}`,
                              result: 'success',
                              metadata: { target: child.path }
                            })
                          }
                          selected={location.pathname.startsWith(child.path)}
                          sx={{ pl: 6 }}
                          to={child.path}
                        >
                          <ListItemText primary={t(child.labelKey)} />
                        </ListItemButton>
                      ))}
                    </List>
                  </Collapse>
                </Box>
              )
            })}
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
        <Box
          component='main'
          data-cubebox-main-reserves-panel={cubeBoxOpen && cubeBoxWide ? 'true' : 'false'}
          sx={{
            flexGrow: 1,
            minWidth: 0,
            p: 3,
            pr: {
              xs: 3,
              lg: cubeBoxOpen ? `calc(${theme.spacing(3)} + ${cubeBoxPanelWidth}px)` : 3
            },
            transition: theme.transitions.create('padding-right', {
              duration: theme.transitions.duration.shorter
            })
          }}
        >
          <Toolbar />
          <Outlet />
        </Box>
        <Drawer
          anchor='right'
          onClose={() => setCubeBoxOpen(false)}
          open={cubeBoxOpen}
          variant={cubeBoxDrawerVariant}
          ModalProps={{ keepMounted: true }}
          PaperProps={{
            sx: {
              borderLeft: {
                md: '1px solid'
              },
              borderColor: 'divider',
              boxSizing: 'border-box',
              height: cubeBoxPanelHeight,
              p: 2,
              top: shellHeaderOffset,
              width: {
                xs: '100%',
                sm: cubeBoxPanelWidth,
                md: cubeBoxPanelWidth
              }
            },
            role: 'complementary',
            'aria-label': t('page_cubebox_title'),
            'data-cubebox-non-modal': cubeBoxDesktop ? 'true' : 'false',
            'data-cubebox-shell-mode': cubeBoxShellMode
          }}
        >
          <Paper elevation={0} sx={{ height: '100%' }}>
            <CubeBoxPanel />
          </Paper>
        </Drawer>
        <Dialog fullWidth maxWidth='sm' onClose={() => setSearchOpen(false)} open={searchOpen}>
          <DialogTitle>{t('global_search')}</DialogTitle>
          <DialogContent>
            <Box component='form' onSubmit={(event) => event.preventDefault()} sx={{ mb: 2 }}>
              <TextField
                fullWidth
                onKeyDown={(event) => {
                  const nativeEvent = event.nativeEvent
                  if (nativeEvent.isComposing || nativeEvent.keyCode === 229) {
                    return
                  }
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
    </CubeBoxProvider>
  )
}
