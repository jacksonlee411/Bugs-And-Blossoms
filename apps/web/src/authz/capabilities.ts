export type AuthzCapabilityKey = `${string}:${string}`

export const AUTHZ_CAPABILITY_KEYS = {
  cubeboxConversationsRead: 'cubebox.conversations:read',
  cubeboxConversationsUse: 'cubebox.conversations:use',
  cubeboxModelCredentialDeactivate: 'cubebox.model_credential:deactivate',
  cubeboxModelCredentialRead: 'cubebox.model_credential:read',
  cubeboxModelCredentialRotate: 'cubebox.model_credential:rotate',
  cubeboxModelProviderUpdate: 'cubebox.model_provider:update',
  cubeboxModelSelectionSelect: 'cubebox.model_selection:select',
  cubeboxModelSelectionVerify: 'cubebox.model_selection:verify',
  iamAuthzAdmin: 'iam.authz:admin',
  iamAuthzRead: 'iam.authz:read',
  iamDictReleaseAdmin: 'iam.dict_release:admin',
  iamDictsAdmin: 'iam.dicts:admin',
  iamDictsRead: 'iam.dicts:read',
  orgunitOrgUnitsAdmin: 'orgunit.orgunits:admin',
  orgunitOrgUnitsRead: 'orgunit.orgunits:read'
} as const satisfies Record<string, AuthzCapabilityKey>

const AUTHZ_CAPABILITY_KEY_PATTERN = /^[a-z][a-z0-9_]*(?:\.[a-z][a-z0-9_]*)*:[a-z][a-z0-9_]*$/

export function isAuthzCapabilityKey(value: string): value is AuthzCapabilityKey {
  return AUTHZ_CAPABILITY_KEY_PATTERN.test(value)
}
