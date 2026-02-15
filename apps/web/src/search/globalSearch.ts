import type { SearchEntry } from '../types/navigation'

export interface SearchProvider {
  search: (query: string) => Promise<SearchEntry[]>
}

function normalizeText(value: string): string {
  return value.trim().toLowerCase()
}

export function createLocalSearchProvider(entries: SearchEntry[]): SearchProvider {
  return {
    async search(query: string) {
      const normalizedQuery = normalizeText(query)
      if (normalizedQuery.length === 0) {
        return entries
      }

      return entries.filter((entry) => entry.keywords.some((keyword) => normalizeText(keyword).includes(normalizedQuery)))
    }
  }
}

export function mergeSearchProviders(providers: SearchProvider[]): SearchProvider {
  return {
    async search(query: string) {
      const result = await Promise.all(providers.map((provider) => provider.search(query)))
      const deduped = new Map<string, SearchEntry>()
      result.flat().forEach((entry) => {
        deduped.set(entry.key, entry)
      })
      return [...deduped.values()]
    }
  }
}
