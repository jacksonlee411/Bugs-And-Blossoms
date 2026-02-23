import { httpClient } from './httpClient'

export interface JobCatalogView {
  has_selection: boolean
  read_only: boolean
  setid?: string
  owner_setid?: string
}

export interface JobFamilyGroupItem {
  job_family_group_uuid: string
  job_family_group_code: string
  name: string
  is_active: boolean
  effective_day: string
}

export interface JobFamilyItem {
  job_family_uuid: string
  job_family_code: string
  job_family_group_code: string
  name: string
  is_active: boolean
  effective_day: string
}

export interface JobLevelItem {
  job_level_uuid: string
  job_level_code: string
  name: string
  is_active: boolean
  effective_day: string
}

export interface JobProfileItem {
  job_profile_uuid: string
  job_profile_code: string
  name: string
  is_active: boolean
  effective_day: string
  family_codes_csv: string
  primary_family_code: string
}

export interface JobCatalogResponse {
  as_of: string
  tenant_id: string
  view: JobCatalogView
  job_family_groups: JobFamilyGroupItem[]
  job_families: JobFamilyItem[]
  job_levels: JobLevelItem[]
  job_profiles: JobProfileItem[]
}

export async function getJobCatalog(options: {
  asOf: string
  setid?: string
}): Promise<JobCatalogResponse> {
  const query = new URLSearchParams({ as_of: options.asOf })
  if (options.setid) {
    query.set('setid', options.setid)
  }
  return httpClient.get<JobCatalogResponse>(`/jobcatalog/api/catalog?${query.toString()}`)
}

export async function applyJobCatalogAction(request: {
  action: string
  setid: string
  effective_date: string
  code?: string
  name?: string
  description?: string
  group_code?: string
  family_codes_csv?: string
  primary_family_code?: string
}): Promise<unknown> {
  return httpClient.post('/jobcatalog/api/catalog/actions', request)
}
