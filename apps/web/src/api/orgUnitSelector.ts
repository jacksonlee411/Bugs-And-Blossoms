import { httpClient } from './httpClient'

export type OrgUnitSelectorStatus = 'active' | 'disabled' | 'inactive'

export interface OrgUnitSelectorNode {
  org_code: string
  org_node_key: string
  name: string
  status: OrgUnitSelectorStatus
  has_visible_children: boolean
  path_org_codes?: string[]
}

interface OrgUnitSelectorAPIItem {
  org_code: string
  org_node_key?: string
  name: string
  status?: string
  has_visible_children?: boolean
  has_children?: boolean
  path_org_codes?: string[]
}

interface OrgUnitSelectorListResponse {
  as_of: string
  include_disabled?: boolean
  org_units: OrgUnitSelectorAPIItem[]
}

interface OrgUnitSelectorSearchAPIResponse {
  target_org_code: string
  target_name: string
  path_org_codes?: string[]
  tree_as_of: string
}

export interface OrgUnitSelectorSearchResult {
  org_code: string
  name: string
  path_org_codes: string[]
  tree_as_of: string
}

export interface OrgUnitSelectorReadOptions {
  asOf: string
  includeDisabled?: boolean
}

export interface OrgUnitSelectorChildrenOptions extends OrgUnitSelectorReadOptions {
  parentOrgCode: string
}

export interface OrgUnitSelectorSearchOptions extends OrgUnitSelectorReadOptions {
  query: string
}

function normalizeStatus(status: string | undefined): OrgUnitSelectorStatus {
  const normalized = (status ?? '').trim().toLowerCase()
  if (normalized === 'disabled' || normalized === 'inactive') {
    return normalized
  }
  return 'active'
}

function normalizePath(path: string[] | undefined): string[] | undefined {
  if (!path) {
    return undefined
  }
  const out = path.map((item) => item.trim()).filter((item) => item.length > 0)
  return out.length > 0 ? out : undefined
}

function selectorQuery(options: OrgUnitSelectorReadOptions): URLSearchParams {
  const query = new URLSearchParams({ as_of: options.asOf })
  if (options.includeDisabled) {
    query.set('include_disabled', '1')
  }
  return query
}

function toSelectorNode(item: OrgUnitSelectorAPIItem): OrgUnitSelectorNode {
  const orgNodeKey = item.org_node_key?.trim() ?? ''
  return {
    org_code: item.org_code.trim(),
    org_node_key: orgNodeKey,
    name: item.name.trim(),
    status: normalizeStatus(item.status),
    has_visible_children: item.has_visible_children ?? item.has_children === true,
    path_org_codes: normalizePath(item.path_org_codes)
  }
}

function toSelectorNodes(items: OrgUnitSelectorAPIItem[]): OrgUnitSelectorNode[] {
  return items.map(toSelectorNode).filter((item) => item.org_code.length > 0 && item.org_node_key.length > 0)
}

export async function listOrgUnitSelectorRoots(options: OrgUnitSelectorReadOptions): Promise<OrgUnitSelectorNode[]> {
  const query = selectorQuery(options)
  const response = await httpClient.get<OrgUnitSelectorListResponse>(`/org/api/org-units?${query.toString()}`)
  return toSelectorNodes(response.org_units)
}

export async function listOrgUnitSelectorChildren(options: OrgUnitSelectorChildrenOptions): Promise<OrgUnitSelectorNode[]> {
  const query = selectorQuery(options)
  query.set('parent_org_code', options.parentOrgCode)
  const response = await httpClient.get<OrgUnitSelectorListResponse>(`/org/api/org-units?${query.toString()}`)
  return toSelectorNodes(response.org_units)
}

export async function searchOrgUnitSelector(options: OrgUnitSelectorSearchOptions): Promise<OrgUnitSelectorSearchResult> {
  const query = selectorQuery(options)
  query.set('query', options.query)
  const response = await httpClient.get<OrgUnitSelectorSearchAPIResponse>(`/org/api/org-units/search?${query.toString()}`)
  return {
    org_code: response.target_org_code.trim(),
    name: response.target_name.trim(),
    path_org_codes: normalizePath(response.path_org_codes) ?? [],
    tree_as_of: response.tree_as_of.trim()
  }
}

export function formatOrgUnitSelectorLabel(node: Pick<OrgUnitSelectorNode, 'name' | 'org_code'>): string {
  return `${node.name} (${node.org_code})`
}
