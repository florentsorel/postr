import { describe, it, expect, beforeEach, vi } from "vitest"
import { setActivePinia, createPinia } from "pinia"
import { useServerStore } from "./useServerStore"

describe("useServerStore", () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it("defaults to Plex before the status endpoint answers", () => {
    const store = useServerStore()
    expect(store.provider).toBe("plex")
    expect(store.name).toBe("Plex")
    expect(store.configured).toBeNull()
  })

  it("adopts the provider reported by the backend", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ provider: "jellyfin", name: "Jellyfin", configured: true }),
      })
    )

    const store = useServerStore()
    await store.load()

    expect(store.provider).toBe("jellyfin")
    expect(store.name).toBe("Jellyfin")
    expect(store.configured).toBe(true)
  })

  it("only calls the backend once unless forced", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ provider: "jellyfin", name: "Jellyfin", configured: true }),
    })
    vi.stubGlobal("fetch", fetchMock)

    const store = useServerStore()
    await store.load()
    await store.load()
    expect(fetchMock).toHaveBeenCalledTimes(1)

    await store.load(true)
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it("marks the server as unconfigured when the request fails", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("offline")))

    const store = useServerStore()
    await store.load()

    expect(store.configured).toBe(false)
    expect(store.loaded).toBe(false)
  })
})
