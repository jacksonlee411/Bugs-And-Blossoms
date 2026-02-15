import { httpClient } from './httpClient'

export interface AssignmentAPIItem {
  assignment_uuid: string
  person_uuid: string
  position_uuid: string
  status: string
  effective_date: string
}

export interface AssignmentListResponse {
  as_of: string
  tenant: string
  person_uuid: string
  assignments: AssignmentAPIItem[]
}

export async function listAssignments(options: {
  asOf: string
  personUUID: string
}): Promise<AssignmentListResponse> {
  const query = new URLSearchParams({ as_of: options.asOf, person_uuid: options.personUUID })
  return httpClient.get<AssignmentListResponse>(`/org/api/assignments?${query.toString()}`)
}

export async function upsertAssignment(request: {
  effective_date: string
  person_uuid: string
  position_uuid: string
  status?: string
  allocated_fte?: string
}): Promise<AssignmentAPIItem> {
  return httpClient.post<AssignmentAPIItem>('/org/api/assignments', request)
}

