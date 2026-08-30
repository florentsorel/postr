import { defineStore } from "pinia"
import { ref } from "vue"

export type Provider = "plex" | "jellyfin"

/**
 * Holds which media server Postr is configured against. Postr talks to one
 * server at a time — the backend decides which from MEDIA_SERVER — and every
 * user-facing label is derived from `name` rather than hardcoded.
 */
export const useServerStore = defineStore("server", () => {
  const provider = ref<Provider>("plex")
  // Pre-load fallback only: the router guard loads the real name before the
  // first page renders, so this value is never shown on a working backend.
  const name = ref("Plex")
  const configured = ref<boolean | null>(null)
  const loaded = ref(false)

  async function load(force = false): Promise<void> {
    if (loaded.value && !force) return
    try {
      const res = await fetch("/api/server/status")
      if (!res.ok) return
      const data = await res.json()
      provider.value = data.provider ?? "plex"
      name.value = data.name ?? "Plex"
      configured.value = data.configured ?? false
      loaded.value = true
    } catch {
      configured.value = false
    }
  }

  return { provider, name, configured, loaded, load }
})
