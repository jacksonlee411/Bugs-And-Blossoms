import { httpClient } from './httpClient'

export interface PersonAPIItem {
  person_uuid: string
  pernr: string
  display_name: string
  status: string
  created_at: string
}

export interface PersonListResponse {
  tenant_id: string
  persons: PersonAPIItem[]
}

export async function listPersons(): Promise<PersonListResponse> {
  return httpClient.get<PersonListResponse>('/person/api/persons')
}

export async function createPerson(request: {
  pernr: string
  display_name: string
}): Promise<{ person_uuid: string; pernr: string; display_name: string; status: string }> {
  return httpClient.post<{ person_uuid: string; pernr: string; display_name: string; status: string }>(
    '/person/api/persons',
    request
  )
}

export async function getPersonByPernr(options: {
  pernr: string
}): Promise<{ person_uuid: string; pernr: string; display_name: string; status: string }> {
  const query = new URLSearchParams({ pernr: options.pernr })
  return httpClient.get<{ person_uuid: string; pernr: string; display_name: string; status: string }>(
    `/person/api/persons:by-pernr?${query.toString()}`
  )
}

