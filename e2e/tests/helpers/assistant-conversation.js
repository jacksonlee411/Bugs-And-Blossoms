export function parseJSONSafe(raw) {
  const body = String(raw || "").trim()
  if (!body) {
    return null
  }
  try {
    return JSON.parse(body)
  } catch {
    return null
  }
}

export async function parseResponseBody(response) {
  const text = await response.text()
  return { text, json: parseJSONSafe(text) }
}

export function latestAssistantTurn(conversation) {
  if (!conversation || !Array.isArray(conversation.turns) || conversation.turns.length === 0) {
    return null
  }
  return conversation.turns[conversation.turns.length - 1]
}
