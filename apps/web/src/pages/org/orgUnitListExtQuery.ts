export function clearExtQueryParams(params: URLSearchParams) {
  params.delete('ext_filter_field_key')
  params.delete('ext_filter_value')
  const sortValue = params.get('sort')?.trim() ?? ''
  if (sortValue.startsWith('ext:')) {
    params.delete('sort')
    params.delete('order')
  }
}
