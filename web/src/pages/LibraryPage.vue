<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from "vue"
import { isAllowedPosterMimeType } from "@/utils/poster"
import type { DropdownMenuItem } from "@nuxt/ui"
import { useRoute, useRouter } from "vue-router"
import { useToast } from "@nuxt/ui/composables/useToast"
import MediaCard from "../components/MediaCard.vue"
import ChangePosterModal from "../components/ChangePosterModal.vue"
import ImportModal from "../components/ImportModal.vue"
import QueuePanel from "../components/QueuePanel.vue"
import SyncCheckModal from "../components/SyncCheckModal.vue"
import HelpModal from "../components/HelpModal.vue"
import ErrorLayout from "../components/ErrorLayout.vue"
import { useLibraryUiStore } from "@/stores/useLibraryUiStore"
import { useQueueStore } from "@/stores/useQueueStore"
import { useAuthStore } from "@/stores/useAuthStore"

type MediaType = "all" | "movie" | "show" | "season" | "collection" | "orphan"
type SortKey = "title" | "year" | "added"

interface MediaItem {
  id: number
  ratingKey: string
  title: string
  type: Exclude<MediaType, "all" | "orphan">
  year?: number
  seasonNumber?: number
  thumb?: string
  addedAt?: number
  locallyModified: boolean
  isOrphan: boolean
}

const route = useRoute()
const router = useRouter()
const uiStore = useLibraryUiStore()
const queueStore = useQueueStore()
const authStore = useAuthStore()

async function logout() {
  await authStore.logout()
  router.push("/login")
}

const VALID_TABS: MediaType[] = ["all", "movie", "show", "season", "collection", "orphan"]
const VALID_SORTS: SortKey[] = ["title", "year", "added"]

function parseTab(v: unknown): MediaType {
  return VALID_TABS.includes(v as MediaType) ? (v as MediaType) : "all"
}
function parseSort(v: unknown): SortKey {
  return VALID_SORTS.includes(v as SortKey) ? (v as SortKey) : "added"
}
function parsePage(v: unknown): number {
  const n = parseInt(v as string, 10)
  return Number.isFinite(n) && n > 0 ? n : 1
}

// URL is the single source of truth — writable computed refs sync directly
// with route.query, no intermediate refs needed.
const activeTab = computed<MediaType>({
  get: () => parseTab(route.query.tab),
  set: (newTab) => {
    // Save current sort for current tab, then restore the new tab's sort.
    uiStore.setSortForTab(parseTab(route.query.tab), parseSort(route.query.sort))
    const restored = uiStore.getSortForTab(newTab)
    const newSort = uiStore.isSortAvailable(newTab, restored) ? restored : "added"
    router.replace({
      query: {
        ...(newTab !== "all" ? { tab: newTab } : {}),
        ...(newSort !== "added" ? { sort: newSort } : {}),
      },
    })
  },
})

const sort = computed<SortKey>({
  get: () => parseSort(route.query.sort),
  set: (v) => {
    uiStore.setSortForTab(parseTab(route.query.tab), v)
    router.replace({
      query: {
        ...(route.query.tab ? { tab: route.query.tab } : {}),
        ...(v !== "added" ? { sort: v } : {}),
        ...(route.query.page ? { page: route.query.page } : {}),
      },
    })
  },
})

const page = computed<number>({
  get: () => parsePage(route.query.page),
  set: (v) =>
    router.replace({
      query: {
        ...(route.query.tab ? { tab: route.query.tab } : {}),
        ...(route.query.sort ? { sort: route.query.sort } : {}),
        ...(v !== 1 ? { page: String(v) } : {}),
      },
    }),
})

const settings = ref({ autoResize: true, resizeWidth: 1000 })

async function resizeIfNeeded(file: File): Promise<Blob> {
  if (!settings.value.autoResize) return file
  return new Promise((resolve, reject) => {
    const img = new Image()
    const url = URL.createObjectURL(file)
    img.onload = () => {
      URL.revokeObjectURL(url)
      const targetWidth = settings.value.resizeWidth
      if (img.width <= targetWidth) {
        resolve(file)
        return
      }
      const targetHeight = Math.round((targetWidth * 3) / 2)
      const canvas = document.createElement("canvas")
      canvas.width = targetWidth
      canvas.height = targetHeight
      const ctx = canvas.getContext("2d")!
      ctx.drawImage(img, 0, 0, targetWidth, targetHeight)
      canvas.toBlob((blob) => resolve(blob ?? file), file.type, 0.9)
    }
    img.onerror = () => {
      URL.revokeObjectURL(url)
      reject(new Error("Failed to load image for resizing"))
    }
    img.src = url
  })
}

const uploadingKeys = ref(new Set<string>())

const isDraggingFileOnPage = ref(false)

function onDocumentDragEnter(e: DragEvent) {
  const item = Array.from(e.dataTransfer?.items ?? []).find((i) => i.kind === "file")
  if (item && isAllowedPosterMimeType(item.type)) isDraggingFileOnPage.value = true
}

function onDocumentDragLeave(e: DragEvent) {
  if (!e.relatedTarget) isDraggingFileOnPage.value = false
}

function onDocumentDragOver(e: DragEvent) {
  e.preventDefault()
}

function onDocumentDrop(e: DragEvent) {
  e.preventDefault()
  isDraggingFileOnPage.value = false
}

onMounted(() => {
  document.addEventListener("dragenter", onDocumentDragEnter)
  document.addEventListener("dragleave", onDocumentDragLeave)
  document.addEventListener("dragover", onDocumentDragOver)
  document.addEventListener("drop", onDocumentDrop)
})

onUnmounted(() => {
  document.removeEventListener("dragenter", onDocumentDragEnter)
  document.removeEventListener("dragleave", onDocumentDragLeave)
  document.removeEventListener("dragover", onDocumentDragOver)
  document.removeEventListener("drop", onDocumentDrop)
})

const loading = ref(false)
const backendError = ref(false)
const importModalOpen = ref(false)
const queuePanelOpen = ref(false)
const syncModalOpen = ref(false)
const helpModalOpen = ref(false)

const menuItems = computed<DropdownMenuItem[][]>(() => {
  const items: DropdownMenuItem[] = [
    {
      label: "Import from Plex",
      icon: "i-lucide-refresh-cw",
      onSelect: () => {
        importModalOpen.value = true
      },
    },
  ]
  if (plexConfigured.value && activeItems.value.length > 0) {
    items.push({
      label: "Sync from Plex",
      icon: "i-lucide-scan-search",
      onSelect: () => {
        syncModalOpen.value = true
      },
    })
  }
  if (queueStore.count > 0) {
    items.push({
      label: `Queue (${queueStore.count})`,
      icon: "i-lucide-upload-cloud",
      onSelect: () => {
        queuePanelOpen.value = true
      },
    })
  }
  const secondGroup: DropdownMenuItem[] = [
    { label: "Settings", icon: "i-lucide-settings", to: "/settings" },
    {
      label: "Help",
      icon: "i-lucide-circle-help",
      onSelect: () => {
        helpModalOpen.value = true
      },
    },
  ]
  if (authStore.authEnabled) {
    return [items, secondGroup, [{ label: "Logout", icon: "i-lucide-log-out", onSelect: logout }]]
  }
  return [items, secondGroup]
})

defineShortcuts({
  "?": () => {
    helpModalOpen.value = !helpModalOpen.value
  },
  meta_k: () => {
    searchInput.value?.inputRef?.focus()
  },
})
const plexConfigured = ref<boolean | null>(null)

onMounted(async () => {
  loading.value = true
  try {
    const [statusRes, mediaRes, settingsRes] = await Promise.all([
      fetch("/api/plex/status"),
      fetch("/api/media"),
      fetch("/api/settings"),
    ])

    if (settingsRes.ok) {
      const data = await settingsRes.json()
      settings.value.autoResize = data.auto_resize ?? true
      settings.value.resizeWidth = data.resize_width ?? 1000
    }

    if (mediaRes.status >= 500) {
      backendError.value = true
      return
    }

    plexConfigured.value = statusRes.ok ? (await statusRes.json()).configured : false

    if (mediaRes.ok) {
      mediaItems.value = await mediaRes.json()
    }
    queueStore.loadQueue()
  } catch {
    backendError.value = true
  } finally {
    loading.value = false
  }
})
const toast = useToast()

async function sendToPlex(item: MediaItem) {
  try {
    const { orphaned } = await queueStore.pushOne(item.ratingKey)
    if (orphaned) {
      item.isOrphan = true
      toast.add({
        title: "Media not found in Plex",
        description: "This item no longer exists in Plex and has been moved to the Orphaned tab.",
        color: "warning",
        icon: "i-lucide-unlink",
      })
    } else {
      item.locallyModified = false
    }
  } catch (e) {
    toast.add({
      title: "Failed to push to Plex",
      description: e instanceof Error ? e.message : undefined,
      color: "error",
      icon: "i-lucide-circle-x",
    })
  }
}

async function getFromPlex(item: MediaItem) {
  try {
    const { thumb, warning, orphaned } = await queueStore.removeItem(item.ratingKey)
    const found = mediaItems.value.find((m) => m.ratingKey === item.ratingKey)
    if (found) {
      if (orphaned) {
        found.isOrphan = true
        toast.add({
          title: "Media not found in Plex",
          description: "This item no longer exists in Plex and has been moved to the Orphaned tab.",
          color: "warning",
          icon: "i-lucide-unlink",
        })
      } else {
        if (thumb) found.thumb = thumb
        found.locallyModified = false
        if (warning) {
          toast.add({
            title: "Could not restore Plex poster",
            description: warning,
            color: "warning",
            icon: "i-lucide-alert-triangle",
          })
        }
      }
    }
  } catch (e) {
    toast.add({
      title: "Could not restore Plex poster",
      description: e instanceof Error ? e.message : undefined,
      color: "error",
      icon: "i-lucide-circle-x",
    })
  }
}

async function deleteOrphan(item: MediaItem) {
  const res = await fetch(`/api/media/${item.ratingKey}`, { method: "DELETE" })
  if (res.ok) {
    mediaItems.value = mediaItems.value.filter((m) => m.ratingKey !== item.ratingKey)
  } else {
    toast.add({ title: "Failed to delete orphan", color: "error", icon: "i-lucide-circle-x" })
  }
}

const posterModal = ref(false)
const selectedItem = ref<MediaItem | null>(null)

function openPosterModal(item: MediaItem) {
  selectedItem.value = item
  posterModal.value = true
}

function onUploaded(payload: { ratingKey: string; thumb: string }) {
  const cacheBusted = payload.thumb + "?t=" + Date.now()
  const item = mediaItems.value.find((m) => m.ratingKey === payload.ratingKey)
  if (item) {
    item.thumb = cacheBusted
    item.locallyModified = true
    queueStore.addItem({
      ratingKey: item.ratingKey,
      title: item.title,
      type: item.type,
      seasonNumber: item.seasonNumber,
      thumb: cacheBusted,
    })
  }
}

async function onDropFile(item: MediaItem, file: File) {
  uploadingKeys.value = new Set(uploadingKeys.value).add(item.ratingKey)
  try {
    const blob = await resizeIfNeeded(file)
    const formData = new FormData()
    formData.append("file", blob, file.name)
    const res = await fetch(`/api/media/${item.ratingKey}/upload`, {
      method: "POST",
      body: formData,
    })
    if (res.ok) {
      const data = await res.json()
      onUploaded({ ratingKey: item.ratingKey, thumb: data.thumb })
    } else {
      toast.add({ title: "Upload failed", color: "error", icon: "i-lucide-circle-x" })
    }
  } catch {
    toast.add({ title: "Upload failed", color: "error", icon: "i-lucide-circle-x" })
  } finally {
    const next = new Set(uploadingKeys.value)
    next.delete(item.ratingKey)
    uploadingKeys.value = next
  }
}

const mediaItems = ref<MediaItem[]>([])

const activeItems = computed(() => mediaItems.value.filter((m) => !m.isOrphan))
const orphanItems = computed(() => mediaItems.value.filter((m) => m.isOrphan))

watch(orphanItems, (items) => {
  if (items.length === 0 && activeTab.value === "orphan") activeTab.value = "all"
})

const tabs = computed(() => {
  const base = [
    { label: "All", value: "all" },
    { label: "Movies", value: "movie" },
    { label: "TV Series", value: "show" },
    { label: "Seasons", value: "season" },
    { label: "Collections", value: "collection" },
  ]
  if (orphanItems.value.length > 0) {
    base.push({ label: `Orphaned (${orphanItems.value.length})`, value: "orphan" })
  }
  return base
})

const search = ref("")
const searchInput = ref<{ inputRef: HTMLInputElement } | null>(null)
const PER_PAGE = 18

const ALL_SORT_OPTIONS = [
  { label: "Title (A–Z)", value: "title" },
  { label: "Year", value: "year" },
  { label: "Recently added", value: "added" },
]

const sortOptions = computed(() =>
  ALL_SORT_OPTIONS.filter((o) => uiStore.isSortAvailable(activeTab.value, o.value as SortKey))
)

// For title sort: collections surface first when title is identical
const titleTypeOrder: Record<Exclude<MediaType, "all" | "orphan">, number> = {
  collection: 0,
  movie: 1,
  show: 2,
  season: 3,
}

const filtered = computed(() => {
  if (activeTab.value === "orphan") {
    const s = search.value.trim().toLowerCase()
    return s
      ? orphanItems.value.filter((m) => m.title.toLowerCase().includes(s))
      : orphanItems.value
  }

  const byTab =
    activeTab.value === "all"
      ? activeItems.value
      : activeItems.value.filter((m) => m.type === activeTab.value)

  const searched = search.value.trim()
    ? byTab.filter((m) => m.title.toLowerCase().includes(search.value.toLowerCase().trim()))
    : byTab

  return [...searched].sort((a, b) => {
    if (sort.value === "year")
      return (b.year ?? 0) - (a.year ?? 0) || a.title.localeCompare(b.title)
    if (sort.value === "added") return (b.addedAt ?? 0) - (a.addedAt ?? 0)
    return (
      a.title.localeCompare(b.title) ||
      titleTypeOrder[a.type] - titleTypeOrder[b.type] ||
      (a.seasonNumber != null && b.seasonNumber != null ? a.seasonNumber - b.seasonNumber : 0)
    )
  })
})

// Effective page: clamped between 1 and the total number of available pages.
// Search narrows results without touching the URL — clearing the search
// restores the original page.
const currentPage = computed({
  get: () => {
    const totalPages = Math.ceil(filtered.value.length / PER_PAGE) || 1
    return Math.min(page.value, totalPages)
  },
  set: (v: number) => {
    page.value = v
  },
})

const paginated = computed(() => {
  const start = (currentPage.value - 1) * PER_PAGE
  return filtered.value.slice(start, start + PER_PAGE)
})

const TYPE_LABEL: Record<Exclude<MediaType, "all" | "orphan">, string> = {
  movie: "Movie",
  show: "TV Series",
  season: "Season",
  collection: "Collection",
}

const activeTabLabel = computed(
  () => TYPE_LABEL[activeTab.value as Exclude<MediaType, "all" | "orphan">] ?? "items"
)

async function onImported() {
  const res = await fetch("/api/media")
  if (res.ok) mediaItems.value = await res.json()
  queueStore.loadQueue()
}

function onSynced(items: Array<{ ratingKey: string; updatedAt: number }>) {
  for (const changed of items) {
    const m = mediaItems.value.find((i) => i.ratingKey === changed.ratingKey)
    if (m) {
      m.thumb = `/api/media/${changed.ratingKey}/thumb?v=${changed.updatedAt}`
      m.locallyModified = false
    }
  }
}

function onSyncOrphaned(ratingKeys: string[]) {
  for (const ratingKey of ratingKeys) {
    const m = mediaItems.value.find((i) => i.ratingKey === ratingKey)
    if (m) m.isOrphan = true
  }
}
</script>

<template>
  <ErrorLayout v-if="backendError" :code="502" message="The backend is unreachable." />
  <div v-else class="min-h-screen bg-[#1f1f1f] text-white">
    <!-- Header -->
    <header
      class="border-b border-neutral-800 px-6 py-4 flex items-center gap-4 sm:sticky sm:top-0 sm:z-10 bg-[#1f1f1f]"
    >
      <div class="flex items-center gap-2">
        <div class="w-7 h-7 rounded-lg bg-primary-500 flex items-center justify-center">
          <UIcon name="i-lucide-image" class="w-4 h-4 text-white" />
        </div>
        <span class="font-bold text-white text-lg">Postr</span>
      </div>

      <!-- Hamburger menu (xs only) -->
      <UDropdownMenu class="sm:hidden ml-auto" :items="menuItems" :content="{ align: 'end' }">
        <UButton icon="i-lucide-menu" variant="ghost" color="neutral" size="sm" />
      </UDropdownMenu>

      <div class="ml-auto hidden sm:flex items-center gap-3">
        <UTooltip text="Import library from Plex and download posters">
          <UButton
            icon="i-lucide-refresh-cw"
            variant="outline"
            color="neutral"
            size="sm"
            @click="importModalOpen = true"
          >
            Import from Plex
          </UButton>
        </UTooltip>
        <UTooltip
          v-if="plexConfigured && activeItems.length > 0"
          text="Detect poster changes made directly in Plex"
        >
          <UButton
            icon="i-lucide-scan-search"
            variant="ghost"
            color="neutral"
            size="sm"
            @click="syncModalOpen = true"
          >
            Sync from Plex
          </UButton>
        </UTooltip>
        <UTooltip v-if="queueStore.count > 0" text="Posters pending push to Plex">
          <UButton
            icon="i-lucide-upload-cloud"
            variant="ghost"
            color="neutral"
            size="sm"
            @click="queuePanelOpen = true"
          >
            <UBadge :label="String(queueStore.count)" color="primary" size="xs" class="ml-1" />
          </UButton>
        </UTooltip>
        <UTooltip text="Settings">
          <UButton
            to="/settings"
            icon="i-lucide-settings"
            variant="ghost"
            color="neutral"
            size="sm"
          />
        </UTooltip>
        <UTooltip text="Help">
          <UButton
            icon="i-lucide-circle-help"
            variant="ghost"
            color="neutral"
            size="sm"
            @click="helpModalOpen = true"
          />
        </UTooltip>
        <UTooltip v-if="authStore.authEnabled" text="Logout">
          <UButton
            icon="i-lucide-log-out"
            variant="ghost"
            color="neutral"
            size="sm"
            @click="logout"
          />
        </UTooltip>
      </div>
    </header>

    <!-- Content -->
    <div class="max-w-7xl mx-auto px-6 py-8">
      <!-- Empty state -->
      <template v-if="!loading && activeItems.length === 0 && orphanItems.length === 0">
        <div class="flex flex-col items-center justify-center py-32 gap-6 text-center">
          <div class="w-20 h-20 rounded-2xl bg-neutral-800 flex items-center justify-center">
            <UIcon name="i-lucide-film" class="w-10 h-10 text-neutral-600" />
          </div>
          <div>
            <h2 class="text-xl font-semibold text-white">No media imported yet</h2>
            <p class="text-sm text-neutral-500 mt-2 max-w-sm">
              Connect to your Plex server and import your library to start managing poster artwork.
            </p>
          </div>
          <UButton icon="i-lucide-refresh-cw" size="lg" @click="importModalOpen = true">
            Import from Plex
          </UButton>
          <p v-if="plexConfigured === false" class="text-xs text-neutral-600">
            Plex server not configured —
            <UButton to="/settings" variant="link" color="primary" size="xs" class="px-0">
              open Settings
            </UButton>
          </p>
        </div>
      </template>

      <!-- Library -->
      <template v-else>
        <!-- Search + Sort -->
        <div class="mb-6 flex flex-col sm:flex-row sm:items-center gap-3">
          <UInput
            ref="searchInput"
            v-model="search"
            placeholder="Search media..."
            icon="i-lucide-search"
            size="lg"
            class="flex-1"
            :ui="{ base: 'bg-neutral-800/60' }"
          >
            <template v-if="search" #trailing>
              <UButton
                icon="i-lucide-x"
                variant="ghost"
                color="neutral"
                size="sm"
                @click="search = ''"
              />
            </template>
          </UInput>
          <USelect
            v-if="activeTab !== 'orphan'"
            v-model="sort"
            :items="sortOptions"
            size="lg"
            class="w-full sm:w-44 sm:shrink-0"
          />
        </div>

        <!-- Tabs + count -->
        <div class="flex items-center justify-between mb-6 gap-2">
          <!-- xs: dropdown select -->
          <USelect
            v-model="activeTab"
            :items="tabs"
            value-key="value"
            label-key="label"
            class="sm:hidden flex-1 min-w-0"
            size="lg"
          />
          <!-- sm+: tabs -->
          <UTabs v-model="activeTab" :items="tabs" variant="link" class="hidden sm:flex" />
          <span class="text-sm text-neutral-500 shrink-0">
            {{ filtered.length }} item{{ filtered.length !== 1 ? "s" : "" }}
          </span>
        </div>

        <!-- Skeleton loading -->
        <div
          v-if="loading"
          class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4 items-start"
        >
          <div v-for="n in 12" :key="n" class="flex flex-col gap-2">
            <USkeleton class="w-full aspect-[2/3] rounded-xl" />
            <USkeleton class="h-4 w-3/4 rounded" />
            <USkeleton class="h-3 w-1/2 rounded" />
          </div>
        </div>

        <!-- Empty filtered state -->
        <div
          v-else-if="filtered.length === 0"
          class="flex flex-col items-center justify-center py-24 gap-3 text-center"
        >
          <UIcon name="i-lucide-search-x" class="w-10 h-10 text-neutral-600" />
          <p class="text-neutral-500 text-sm">No {{ activeTabLabel }} found</p>
        </div>

        <!-- Grid -->
        <div
          v-else
          class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4 items-start"
        >
          <MediaCard
            v-for="item in paginated"
            :key="item.id"
            :title="item.title"
            :type="item.type"
            :year="item.year"
            :season-number="item.seasonNumber"
            :thumb="item.thumb"
            :syncing="queueStore.isPushing(item.ratingKey)"
            :pulling="queueStore.isPulling(item.ratingKey)"
            :uploading="uploadingKeys.has(item.ratingKey)"
            :file-dragging="isDraggingFileOnPage"
            :in-queue="queueStore.items.some((q) => q.ratingKey === item.ratingKey)"
            :locally-modified="item.locallyModified"
            :is-orphan="item.isOrphan"
            @change-poster="openPosterModal(item)"
            @send-to-plex="sendToPlex(item)"
            @get-from-plex="getFromPlex(item)"
            @delete-orphan="deleteOrphan(item)"
            @drop-file="onDropFile(item, $event)"
          />
        </div>

        <!-- Pagination -->
        <div v-if="filtered.length > PER_PAGE" class="flex justify-center mt-8">
          <UPagination
            v-model:page="currentPage"
            :total="filtered.length"
            :items-per-page="PER_PAGE"
            :sibling-count="2"
            :ui="{ first: 'hidden', last: 'hidden' }"
            show-edges
            class="hidden sm:flex"
          />
          <UPagination
            v-model:page="currentPage"
            :total="filtered.length"
            :items-per-page="PER_PAGE"
            :sibling-count="1"
            class="sm:hidden"
          />
        </div>
      </template>
    </div>

    <ChangePosterModal
      v-if="posterModal"
      v-model:open="posterModal"
      :item="selectedItem"
      @uploaded="onUploaded"
    />
    <ImportModal v-model:open="importModalOpen" @imported="onImported" />
    <SyncCheckModal
      v-if="syncModalOpen"
      v-model:open="syncModalOpen"
      @synced="onSynced"
      @orphaned="onSyncOrphaned"
    />
    <HelpModal v-model:open="helpModalOpen" />
    <QueuePanel
      v-model:open="queuePanelOpen"
      @restored="
        ({ ratingKey, thumb }) => {
          const m = mediaItems.find((i) => i.ratingKey === ratingKey)
          if (m) {
            if (thumb) m.thumb = thumb
            m.locallyModified = false
          }
        }
      "
    />
  </div>
</template>
