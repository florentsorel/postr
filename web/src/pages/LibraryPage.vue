<script setup lang="ts">
import { ref, computed, onMounted } from "vue"
import { useRoute, useRouter } from "vue-router"
import { useToast } from "@nuxt/ui/composables/useToast"
import MediaCard from "../components/MediaCard.vue"
import ChangePosterModal from "../components/ChangePosterModal.vue"
import ImportModal from "../components/ImportModal.vue"
import QueuePanel from "../components/QueuePanel.vue"
import { useLibraryUiStore } from "@/stores/useLibraryUiStore"
import { useQueueStore } from "@/stores/useQueueStore"

type MediaType = "all" | "movie" | "show" | "season" | "collection"
type SortKey = "title" | "year" | "added"

interface MediaItem {
  id: number
  ratingKey: string
  title: string
  type: Exclude<MediaType, "all">
  year?: number
  seasonNumber?: number
  thumb?: string
  addedAt?: number
}

const route = useRoute()
const router = useRouter()
const uiStore = useLibraryUiStore()
const queueStore = useQueueStore()

const VALID_TABS: MediaType[] = ["all", "movie", "show", "season", "collection"]
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

const loading = ref(false)
const importModalOpen = ref(false)
const queuePanelOpen = ref(false)
const plexConfigured = ref<boolean | null>(null)

onMounted(async () => {
  loading.value = true
  const [statusRes, mediaRes] = await Promise.allSettled([
    fetch("/api/plex/status"),
    fetch("/api/media"),
  ])

  plexConfigured.value =
    statusRes.status === "fulfilled" && statusRes.value.ok
      ? (await statusRes.value.json()).configured
      : false

  if (mediaRes.status === "fulfilled" && mediaRes.value.ok) {
    media.value = await mediaRes.value.json()
  }
  loading.value = false
  queueStore.loadQueue()
})
const toast = useToast()

async function sendToPlex(item: MediaItem) {
  try {
    await queueStore.pushOne(item.ratingKey)
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
  const thumb = await queueStore.removeItem(item.ratingKey)
  if (thumb) {
    const found = media.value.find((m) => m.ratingKey === item.ratingKey)
    if (found) found.thumb = thumb
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
  const item = media.value.find((m) => m.ratingKey === payload.ratingKey)
  if (item) {
    item.thumb = cacheBusted
  }
  if (selectedItem.value && selectedItem.value.ratingKey === payload.ratingKey) {
    queueStore.addItem({
      ratingKey: payload.ratingKey,
      title: selectedItem.value.title,
      type: selectedItem.value.type,
      seasonNumber: selectedItem.value.seasonNumber,
      thumb: cacheBusted,
    })
  }
}

// Will be replaced by real API data
const media = ref<MediaItem[]>([])

const tabs = [
  { label: "All", value: "all" },
  { label: "Movies", value: "movie" },
  { label: "TV Series", value: "show" },
  { label: "Seasons", value: "season" },
  { label: "Collections", value: "collection" },
]

const search = ref("")
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
const titleTypeOrder: Record<Exclude<MediaType, "all">, number> = {
  collection: 0,
  movie: 1,
  show: 2,
  season: 3,
}

const filtered = computed(() => {
  const byTab =
    activeTab.value === "all" ? media.value : media.value.filter((m) => m.type === activeTab.value)

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

const TYPE_LABEL: Record<Exclude<MediaType, "all">, string> = {
  movie: "Movie",
  show: "TV Series",
  season: "Season",
  collection: "Collection",
}

const activeTabLabel = computed(
  () => TYPE_LABEL[activeTab.value as Exclude<MediaType, "all">] ?? "items"
)

async function onImported() {
  const res = await fetch("/api/media")
  if (res.ok) media.value = await res.json()
  queueStore.loadQueue()
}
</script>

<template>
  <div class="min-h-screen bg-[#1f1f1f] text-white">
    <!-- Header -->
    <header class="border-b border-neutral-800 px-6 py-4 flex items-center gap-4">
      <div class="flex items-center gap-2">
        <div class="w-7 h-7 rounded-lg bg-primary-500 flex items-center justify-center">
          <UIcon name="i-lucide-image" class="w-4 h-4 text-white" />
        </div>
        <span class="font-bold text-white text-lg">Postr</span>
      </div>

      <div class="ml-auto flex items-center gap-3">
        <UButton
          icon="i-lucide-refresh-cw"
          variant="outline"
          color="neutral"
          size="sm"
          @click="importModalOpen = true"
        >
          Import from Plex
        </UButton>
        <UButton
          icon="i-lucide-upload-cloud"
          variant="ghost"
          color="neutral"
          size="sm"
          @click="queuePanelOpen = true"
        >
          <template v-if="queueStore.count > 0">
            <UBadge :label="String(queueStore.count)" color="primary" size="xs" class="ml-1" />
          </template>
        </UButton>
        <UButton
          to="/settings"
          icon="i-lucide-settings"
          variant="ghost"
          color="neutral"
          size="sm"
        />
      </div>
    </header>

    <!-- Content -->
    <div class="max-w-7xl mx-auto px-6 py-8">
      <!-- Empty state -->
      <template v-if="!loading && media.length === 0">
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
            v-model="sort"
            :items="sortOptions"
            size="lg"
            class="w-full sm:w-44 sm:shrink-0"
          />
        </div>

        <!-- Tabs + count -->
        <div class="flex items-center justify-between mb-6">
          <UTabs v-model="activeTab" :items="tabs" variant="link" />
          <span class="text-sm text-neutral-500 shrink-0">
            {{ filtered.length }} item{{ filtered.length !== 1 ? "s" : "" }}
          </span>
        </div>

        <!-- Skeleton loading -->
        <div
          v-if="loading"
          class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4"
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
          class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4"
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
            :in-queue="queueStore.items.some((q) => q.ratingKey === item.ratingKey)"
            @change-poster="openPosterModal(item)"
            @send-to-plex="sendToPlex(item)"
            @get-from-plex="getFromPlex(item)"
          />
        </div>

        <!-- Pagination -->
        <div v-if="filtered.length > PER_PAGE" class="flex justify-center mt-8">
          <UPagination
            v-model:page="currentPage"
            :total="filtered.length"
            :items-per-page="PER_PAGE"
            :sibling-count="1"
            show-edges
          />
        </div>
      </template>
    </div>

    <ChangePosterModal v-model:open="posterModal" :item="selectedItem" @uploaded="onUploaded" />
    <ImportModal v-model:open="importModalOpen" @imported="onImported" />
    <QueuePanel
      v-model:open="queuePanelOpen"
      @restored="
        ({ ratingKey, thumb }) => {
          const m = media.find((i) => i.ratingKey === ratingKey)
          if (m) m.thumb = thumb
        }
      "
    />
  </div>
</template>
