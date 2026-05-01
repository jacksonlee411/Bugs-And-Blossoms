export type AuthzCapabilityKey = `${string}:${string}`

export const AUTHZ_CAPABILITY_KEYS = {
  cubeboxConversationsRead: 'cubebox.conversations:read',
  cubeboxConversationsUse: 'cubebox.conversations:use',
  iamDictReleaseAdmin: 'iam.dict_release:admin',
  iamDictsAdmin: 'iam.dicts:admin',
  orgunitOrgUnitsAdmin: 'orgunit.orgunits:admin',
  orgunitOrgUnitsRead: 'orgunit.orgunits:read'
} as const satisfies Record<string, AuthzCapabilityKey>

const AUTHZ_CAPABILITY_KEY_PATTERN = /^[a-z][a-z0-9_]*(?:\.[a-z][a-z0-9_]*)*:[a-z][a-z0-9_]*$/

export function isAuthzCapabilityKey(value: string): value is AuthzCapabilityKey {
  return AUTHZ_CAPABILITY_KEY_PATTERN.test(value)
}
