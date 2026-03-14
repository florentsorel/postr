<script setup lang="ts">
import { ref, computed } from "vue"
import { useRoute, useRouter } from "vue-router"
import MediaCard from "../components/MediaCard.vue"
import ChangePosterModal from "../components/ChangePosterModal.vue"

type MediaType = "all" | "movie" | "show" | "season" | "collection"
type SortKey = "title" | "type" | "year" | "added"

interface MediaItem {
  id: string
  title: string
  type: Exclude<MediaType, "all">
  year?: number
  thumb?: string
  addedAt?: number
}

const route = useRoute()
const router = useRouter()

const VALID_TABS: MediaType[] = ["all", "movie", "show", "season", "collection"]
const VALID_SORTS: SortKey[] = ["title", "type", "year", "added"]

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
  set: (v) =>
    router.replace({
      query: {
        ...(v !== "all" ? { tab: v } : {}),
        ...(route.query.sort ? { sort: route.query.sort } : {}),
        ...(route.query.page ? { page: route.query.page } : {}),
      },
    }),
})

const sort = computed<SortKey>({
  get: () => parseSort(route.query.sort),
  set: (v) =>
    router.replace({
      query: {
        ...(route.query.tab ? { tab: route.query.tab } : {}),
        ...(v !== "added" ? { sort: v } : {}),
        ...(route.query.page ? { page: route.query.page } : {}),
      },
    }),
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
const importing = ref(false)

const posterModal = ref(false)
const selectedItem = ref<MediaItem | null>(null)

function openPosterModal(item: MediaItem) {
  selectedItem.value = item
  posterModal.value = true
}

function onPosterConfirm() {
  // TODO: call API once backend is ready
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

const sortOptions = [
  { label: "Title (A–Z)", value: "title" },
  { label: "Type", value: "type" },
  { label: "Year", value: "year" },
  { label: "Recently added", value: "added" },
]

const typeOrder: Record<Exclude<MediaType, "all">, number> = {
  movie: 0,
  show: 1,
  season: 2,
  collection: 3,
}

const filtered = computed(() => {
  const byTab =
    activeTab.value === "all" ? media.value : media.value.filter((m) => m.type === activeTab.value)

  const searched = search.value.trim()
    ? byTab.filter((m) => m.title.toLowerCase().includes(search.value.toLowerCase().trim()))
    : byTab

  return [...searched].sort((a, b) => {
    if (sort.value === "type")
      return typeOrder[a.type] - typeOrder[b.type] || a.title.localeCompare(b.title)
    if (sort.value === "year")
      return (b.year ?? 0) - (a.year ?? 0) || a.title.localeCompare(b.title)
    if (sort.value === "added") return (b.addedAt ?? 0) - (a.addedAt ?? 0)
    return a.title.localeCompare(b.title)
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

async function importFromPlex() {
  importing.value = true
  try {
    await fetch("/api/plex/import", { method: "POST" })
    const res = await fetch("/api/media")
    if (res.ok) media.value = await res.json()
  } finally {
    importing.value = false
  }
}

// DEV ONLY — toggle mock data to preview the grid
const MOCK_MEDIA: MediaItem[] = [
  { id: "1", title: "Inception", type: "movie", year: 2010 },
  { id: "2", title: "The Dark Knight", type: "movie", year: 2008 },
  { id: "3", title: "Interstellar", type: "movie", year: 2014 },
  { id: "4", title: "Dune: Part Two", type: "movie", year: 2024 },
  { id: "5", title: "Oppenheimer", type: "movie", year: 2023 },
  { id: "6", title: "The Godfather", type: "movie", year: 1972 },
  { id: "7", title: "Pulp Fiction", type: "movie", year: 1994 },
  { id: "8", title: "The Matrix", type: "movie", year: 1999 },
  { id: "9", title: "Blade Runner 2049", type: "movie", year: 2017 },
  { id: "10", title: "Parasite", type: "movie", year: 2019 },
  { id: "11", title: "Everything Everywhere", type: "movie", year: 2022 },
  { id: "12", title: "Poor Things", type: "movie", year: 2023 },
  { id: "13", title: "The Brutalist", type: "movie", year: 2024 },
  { id: "14", title: "Breaking Bad", type: "show", year: 2008 },
  { id: "15", title: "The Last of Us", type: "show", year: 2023 },
  { id: "16", title: "Severance", type: "show", year: 2022 },
  { id: "17", title: "The Bear", type: "show", year: 2022 },
  { id: "18", title: "Andor", type: "show", year: 2022 },
  { id: "19", title: "Shogun", type: "show", year: 2024 },
  { id: "20", title: "True Detective", type: "show", year: 2014 },
  { id: "21", title: "Breaking Bad - Season 1", type: "season", year: 2008 },
  { id: "22", title: "Breaking Bad - Season 2", type: "season", year: 2009 },
  { id: "23", title: "Breaking Bad - Season 3", type: "season", year: 2010 },
  { id: "24", title: "Breaking Bad - Season 4", type: "season", year: 2011 },
  { id: "25", title: "Breaking Bad - Season 5", type: "season", year: 2012 },
  { id: "26", title: "The Last of Us - Season 1", type: "season", year: 2023 },
  { id: "27", title: "The Last of Us - Season 2", type: "season", year: 2025 },
  { id: "28", title: "Severance - Season 1", type: "season", year: 2022 },
  { id: "29", title: "Severance - Season 2", type: "season", year: 2025 },
  { id: "30", title: "Christopher Nolan", type: "collection" },
  { id: "31", title: "Marvel Cinematic Universe", type: "collection" },
  { id: "32", title: "Star Wars", type: "collection" },
  { id: "33", title: "The Godfather Trilogy", type: "collection" },
]

function toggleMock() {
  media.value = media.value.length === 0 ? MOCK_MEDIA : []
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
        <!-- DEV ONLY -->
        <UButton
          :icon="media.length === 0 ? 'i-lucide-eye' : 'i-lucide-eye-off'"
          variant="ghost"
          color="neutral"
          size="sm"
          title="Toggle mock data"
          @click="toggleMock"
        />
        <UButton
          :loading="importing"
          icon="i-lucide-refresh-cw"
          variant="outline"
          color="neutral"
          size="sm"
          @click="importFromPlex"
        >
          Import from Plex
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
          <UButton
            :loading="importing"
            icon="i-lucide-refresh-cw"
            size="lg"
            @click="importFromPlex"
          >
            Import from Plex
          </UButton>
          <p class="text-xs text-neutral-600">
            Make sure your Plex server is configured in
            <UButton to="/settings" variant="link" color="primary" size="xs" class="px-0">
              Settings
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
            :thumb="item.thumb"
            @change-poster="openPosterModal(item)"
            @send-to-plex="() => {}"
            @get-from-plex="() => {}"
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

    <ChangePosterModal v-model:open="posterModal" :item="selectedItem" @confirm="onPosterConfirm" />
  </div>
</template>
