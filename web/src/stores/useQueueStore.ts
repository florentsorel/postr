import { defineStore } from "pinia"
import { ref, computed } from "vue"

export interface QueueItem {
  ratingKey: string
  title: string
  type: string
  seasonNumber?: number
  thumb: string
}

export const useQueueStore = defineStore("queue", () => {
  const items = ref<QueueItem[]>([])
  const count = computed(() => items.value.length)
  const pushing = ref<Set<string>>(new Set())
  const pulling = ref<Set<string>>(new Set())

  function isPushing(ratingKey: string) {
    return pushing.value.has(ratingKey)
  }

  function isPulling(ratingKey: string) {
    return pulling.value.has(ratingKey)
  }

  async function loadQueue() {
    const res = await fetch("/api/queue")
    if (res.ok) items.value = await res.json()
  }

  async function removeItem(ratingKey: string): Promise<string | null> {
    pulling.value = new Set([...pulling.value, ratingKey])
    try {
      const res = await fetch(`/api/queue/${ratingKey}`, { method: "DELETE" })
      items.value = items.value.filter((i) => i.ratingKey !== ratingKey)
      if (res.ok) {
        const data = await res.json().catch(() => ({}))
        return data.thumb ? data.thumb + "?t=" + Date.now() : null
      }
      return null
    } finally {
      pulling.value = new Set([...pulling.value].filter((k) => k !== ratingKey))
    }
  }

  async function pushAll(): Promise<void> {
    items.value.forEach((i) => pushing.value.add(i.ratingKey))
    pushing.value = new Set(pushing.value)
    try {
      await fetch("/api/queue/push-all", { method: "POST" })
      await loadQueue()
    } finally {
      pushing.value = new Set()
    }
  }

  async function pushOne(ratingKey: string): Promise<void> {
    pushing.value = new Set([...pushing.value, ratingKey])
    try {
      const res = await fetch(`/api/media/${ratingKey}/push`, { method: "POST" })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.error ?? "Push failed")
      }
      items.value = items.value.filter((i) => i.ratingKey !== ratingKey)
    } finally {
      pushing.value = new Set([...pushing.value].filter((k) => k !== ratingKey))
    }
  }

  function addItem(item: QueueItem) {
    const idx = items.value.findIndex((i) => i.ratingKey === item.ratingKey)
    if (idx >= 0) items.value[idx] = item
    else items.value.unshift(item)
  }

  return {
    items,
    count,
    pushing,
    isPushing,
    pulling,
    isPulling,
    loadQueue,
    removeItem,
    pushAll,
    pushOne,
    addItem,
  }
})
