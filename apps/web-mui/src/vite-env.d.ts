/// <reference types='vite/client' />

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL?: string
  readonly VITE_API_TIMEOUT_MS?: string
  readonly VITE_TENANT_ID?: string
  readonly VITE_PERMISSIONS?: string
  readonly VITE_NAV_DEBUG?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
