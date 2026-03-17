export async function readSSEStream(
  url: string,
  options: RequestInit,
  onEvent: (event: Record<string, unknown>) => void
): Promise<void> {
  const response = await fetch(url, options)
  const reader = response.body!.getReader()
  const decoder = new TextDecoder()
  let buffer = ""

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split("\n")
    buffer = lines.pop() ?? ""
    for (const line of lines) {
      if (!line.startsWith("data: ")) continue
      try {
        onEvent(JSON.parse(line.slice(6)))
      } catch {
        // ignore malformed events
      }
    }
  }
}
