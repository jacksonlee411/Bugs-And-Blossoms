import ApartmentIcon from '@mui/icons-material/Apartment'
import ChecklistIcon from '@mui/icons-material/Checklist'
import HomeWorkIcon from '@mui/icons-material/HomeWork'
import ManageAccountsIcon from '@mui/icons-material/ManageAccounts'
import MenuBookIcon from '@mui/icons-material/MenuBook'
import PendingActionsIcon from '@mui/icons-material/PendingActions'
import RouteIcon from '@mui/icons-material/Route'
import SecurityIcon from '@mui/icons-material/Security'
import TuneIcon from '@mui/icons-material/Tune'
import { AUTHZ_CAPABILITY_KEYS } from '../authz/capabilities'
import type { NavItem, SearchEntry } from '../types/navigation'

export const navItems: NavItem[] = [
  {
    key: 'foundation-demo',
    path: '/',
    labelKey: 'nav_foundation_demo',
    icon: <HomeWorkIcon fontSize='small' />,
    order: 10,
    keywords: ['foundation', 'demo', 'mui', '基座', '示例']
  },
  {
    key: 'org-units',
    path: '/org/units',
    labelKey: 'nav_org_units',
    icon: <ApartmentIcon fontSize='small' />,
    order: 20,
    requiredCapabilityKey: AUTHZ_CAPABILITY_KEYS.orgunitOrgUnitsRead,
    keywords: ['org', 'unit', 'department', '组织', '部门']
  },
  {
    key: 'org-field-configs',
    path: '/org/units/field-configs',
    labelKey: 'nav_org_field_configs',
    icon: <TuneIcon fontSize='small' />,
    order: 21,
    requiredCapabilityKey: AUTHZ_CAPABILITY_KEYS.orgunitOrgUnitsAdmin,
    keywords: ['field', 'config', 'metadata', 'orgunit', '字段', '配置', '元数据']
  },
  {
    key: 'dict-configs',
    path: '/dicts',
    labelKey: 'nav_dicts',
    icon: <MenuBookIcon fontSize='small' />,
    order: 22,
    requiredCapabilityKey: AUTHZ_CAPABILITY_KEYS.iamDictsAdmin,
    keywords: ['dict', 'dictionary', 'value', '配置', '字典', '编码']
  },
  {
    key: 'authz-roles',
    path: '/authz/roles',
    labelKey: 'nav_authz_roles',
    icon: <SecurityIcon fontSize='small' />,
    order: 23,
    requiredCapabilityKey: AUTHZ_CAPABILITY_KEYS.iamAuthzAdmin,
    keywords: ['authz', 'role', 'permission', '角色', '授权']
  },
  {
    key: 'authz-user-assignments',
    path: '/authz/user-assignments',
    labelKey: 'nav_authz_user_assignments',
    icon: <ManageAccountsIcon fontSize='small' />,
    order: 24,
    requiredCapabilityKey: AUTHZ_CAPABILITY_KEYS.iamAuthzAdmin,
    keywords: ['authz', 'user', 'assignment', '角色', '用户授权']
  },
  {
    key: 'authz-capabilities',
    path: '/authz/capabilities',
    labelKey: 'nav_authz_capabilities',
    icon: <ChecklistIcon fontSize='small' />,
    order: 25,
    requiredCapabilityKey: AUTHZ_CAPABILITY_KEYS.iamAuthzRead,
    keywords: ['authz', 'capability', 'permission', '功能', '授权项', '权限']
  },
  {
    key: 'authz-api-catalog',
    path: '/authz/api-catalog',
    labelKey: 'nav_authz_api_catalog',
    icon: <RouteIcon fontSize='small' />,
    order: 26,
    requiredCapabilityKey: AUTHZ_CAPABILITY_KEYS.iamAuthzRead,
    keywords: ['authz', 'api', 'route', 'catalog', '授权', '接口', '目录']
  },
  {
    key: 'approval-inbox',
    path: '/approvals',
    labelKey: 'nav_approvals',
    icon: <PendingActionsIcon fontSize='small' />,
    order: 30,
    keywords: ['approval', 'workflow', 'task', '审批', '待办']
  }
]

export const commonSearchEntries: SearchEntry[] = [
  {
    key: 'common-my-tasks',
    labelKey: 'common_my_tasks',
    path: '/approvals',
    source: 'common',
    keywords: ['task', 'todo', 'approval', '任务', '待办']
  },
  {
    key: 'common-recent-changes',
    labelKey: 'common_recent_changes',
    path: '/org/units',
    source: 'common',
    requiredCapabilityKey: AUTHZ_CAPABILITY_KEYS.orgunitOrgUnitsRead,
    keywords: ['change', 'history', 'audit', '变更', '审计']
  },
  {
    key: 'common-foundation-demo',
    labelKey: 'nav_foundation_demo',
    path: '/',
    source: 'common',
    keywords: ['demo', 'example', '示例']
  }
]

export function buildNavigationSearchEntries(items: NavItem[]): SearchEntry[] {
  return items.map((item) => ({
    key: `nav-${item.key}`,
    labelKey: item.labelKey,
    path: item.path,
    source: 'navigation',
    requiredCapabilityKey: item.requiredCapabilityKey,
    keywords: item.keywords
  }))
}
