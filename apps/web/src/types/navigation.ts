import type { ReactNode } from 'react'
import type { AuthzCapabilityKey } from '../authz/capabilities'
import type { MessageKey } from '../i18n/messages'

export interface NavItem {
  key: string
  path: string
  labelKey: MessageKey
  icon: ReactNode
  order: number
  parentKey?: string
  requiredCapabilityKey?: AuthzCapabilityKey
  keywords: string[]
}

export interface SearchEntry {
  key: string
  labelKey: MessageKey
  path: string
  source: 'navigation' | 'common'
  requiredCapabilityKey?: AuthzCapabilityKey
  keywords: string[]
}
