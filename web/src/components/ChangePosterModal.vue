<script setup lang="ts">
import { ref, computed, watch, nextTick, onUnmounted } from "vue"

type MediaType = "movie" | "show" | "season" | "collection"

interface MediaItem {
  id: string
  title: string
  type: MediaType
  year?: number
  thumb?: string
}

const props = defineProps<{
  open: boolean
  item: MediaItem | null
}>()

const emit = defineEmits<{
  "update:open": [value: boolean]
  confirm: [file: File | string]
}>()

// -- Tabs --
const activeTab = ref("upload")
const tabs = [
  { label: "Upload", value: "upload", icon: "i-lucide-upload" },
  { label: "Find online", value: "find", icon: "i-lucide-search" },
  { label: "From URL", value: "url", icon: "i-lucide-link" },
]

// -- Upload --
const uploadedFile = ref<File | null>(null)
const uploadedPreview = ref<string | null>(null)
const isDragging = ref(false)

function onFileChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (file) setFile(file)
}

function onDrop(e: DragEvent) {
  isDragging.value = false
  const file = e.dataTransfer?.files?.[0]
  if (file?.type.startsWith("image/")) setFile(file)
}

function setFile(file: File) {
  uploadedFile.value = file
  uploadedPreview.value = URL.createObjectURL(file)
}

function clearUpload() {
  if (uploadedPreview.value) URL.revokeObjectURL(uploadedPreview.value)
  uploadedFile.value = null
  uploadedPreview.value = null
}

// -- From URL --
const pastedUrl = ref("")
const urlPreviewError = ref(false)

watch(pastedUrl, () => {
  urlPreviewError.value = false
})

// -- Find online --
const SOURCES = ["TMDB", "TVDB", "Fanart.tv"]
const activeSource = ref(SOURCES[0])
const selectedPosterUrl = ref<string | null>(null)

// Mock results — replaced by real API calls when backend is ready
const MOCK_TOTAL = 48
const PAGE_SIZE = 12

function mockPage(page: number) {
  return Array.from({ length: PAGE_SIZE }, (_, i) => {
    const n = (page - 1) * PAGE_SIZE + i + 1
    return {
      url: `https://picsum.photos/seed/${n}/300/450`,
      label: `Poster ${n}`,
      author: `artist_${n}`,
      sourceUrl: `https://example.com/poster/${n}`,
    }
  })
}

const posters = ref(mockPage(1))
const currentPage = ref(1)
const loadingMore = ref(false)
const hasMore = computed(() => posters.value.length < MOCK_TOTAL)
const sentinel = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

async function loadMore() {
  if (loadingMore.value || !hasMore.value) return
  loadingMore.value = true
  // Simulate network delay
  await new Promise((r) => setTimeout(r, 600))
  currentPage.value++
  posters.value.push(...mockPage(currentPage.value))
  loadingMore.value = false
}

function setupObserver() {
  observer?.disconnect()
  if (!sentinel.value) return
  observer = new IntersectionObserver(
    ([entry]) => {
      if (entry.isIntersecting) loadMore()
    },
    { threshold: 0.1 }
  )
  observer.observe(sentinel.value)
}

function selectSource(source: string) {
  activeSource.value = source
  resetPosters()
}

function resetPosters() {
  observer?.disconnect()
  posters.value = mockPage(1)
  currentPage.value = 1
  loadingMore.value = false
  selectedPosterUrl.value = null
  nextTick(setupObserver)
}

watch(sentinel, (el) => {
  if (el) setupObserver()
})

onUnmounted(() => observer?.disconnect())

// -- Confirm --
const canConfirm = computed(() => {
  if (activeTab.value === "upload") return uploadedFile.value !== null
  if (activeTab.value === "url") return pastedUrl.value.trim().length > 0 && !urlPreviewError.value
  return selectedPosterUrl.value !== null
})

function confirm() {
  if (activeTab.value === "upload" && uploadedFile.value) {
    emit("confirm", uploadedFile.value)
  } else if (activeTab.value === "find" && selectedPosterUrl.value) {
    emit("confirm", selectedPosterUrl.value)
  } else if (activeTab.value === "url" && pastedUrl.value.trim()) {
    emit("confirm", pastedUrl.value.trim())
  }
  close()
}

function close() {
  emit("update:open", false)
}

// Reset state when modal closes
watch(
  () => props.open,
  (open) => {
    if (!open) {
      activeTab.value = "upload"
      clearUpload()
      resetPosters()
      pastedUrl.value = ""
      urlPreviewError.value = false
    }
  }
)
</script>

<template>
  <UModal
    :open="open"
    :dismissible="false"
    :ui="{ overlay: 'items-start pt-10' }"
    :title="item?.title ?? 'Change poster'"
    :description="item ? [item.year, item.type].filter(Boolean).join(' · ') : undefined"
    @update:open="$emit('update:open', $event)"
  >
    <template #body>
      <UTabs v-model="activeTab" :items="tabs" class="mb-4" />

      <!-- Upload tab -->
      <div v-if="activeTab === 'upload'">
        <!-- Preview -->
        <div v-if="uploadedPreview" class="relative mb-4">
          <img
            :src="uploadedPreview"
            alt="Preview"
            class="w-full max-h-64 object-contain rounded-lg bg-neutral-800"
          />
          <UButton
            icon="i-lucide-x"
            size="xs"
            color="neutral"
            variant="solid"
            class="absolute top-2 right-2"
            @click="clearUpload"
          />
        </div>

        <!-- Drop zone -->
        <label
          v-else
          class="flex flex-col items-center justify-center gap-3 w-full h-48 rounded-xl border-2 border-dashed cursor-pointer transition-colors"
          :class="
            isDragging
              ? 'border-primary-500 bg-primary-500/10'
              : 'border-neutral-700 hover:border-neutral-500 bg-neutral-800/40'
          "
          @dragover.prevent="isDragging = true"
          @dragleave="isDragging = false"
          @drop.prevent="onDrop"
        >
          <UIcon name="i-lucide-image-up" class="w-8 h-8 text-neutral-500" />
          <div class="text-center">
            <p class="text-sm text-neutral-300">
              Drop an image here or <span class="text-primary-400">browse</span>
            </p>
            <p class="text-xs text-neutral-500 mt-1">JPG, PNG, WEBP</p>
          </div>
          <input type="file" accept="image/*" class="sr-only" @change="onFileChange" />
        </label>
      </div>

      <!-- From URL tab -->
      <div v-else-if="activeTab === 'url'" class="flex flex-col gap-4">
        <UInput
          v-model="pastedUrl"
          placeholder="https://image.tmdb.org/t/p/original/..."
          icon="i-lucide-link"
          size="lg"
          autofocus
        />
        <div v-if="pastedUrl.trim()" class="flex justify-center">
          <div class="relative w-40 aspect-[2/3] rounded-lg overflow-hidden bg-neutral-800">
            <img
              v-if="!urlPreviewError"
              :src="pastedUrl.trim()"
              alt="Preview"
              class="w-full h-full object-cover"
              @error="urlPreviewError = true"
            />
            <div
              v-else
              class="w-full h-full flex flex-col items-center justify-center gap-2 text-neutral-500"
            >
              <UIcon name="i-lucide-image-off" class="w-6 h-6" />
              <p class="text-xs text-center px-2">Unable to load image</p>
            </div>
          </div>
        </div>
      </div>

      <!-- Find online tab -->
      <div v-else>
        <!-- Source selector -->

        <div class="flex gap-2 mb-4 flex-wrap">
          <UButton
            v-for="source in SOURCES"
            :key="source"
            :label="source"
            size="xs"
            :variant="activeSource === source ? 'solid' : 'outline'"
            :color="activeSource === source ? 'primary' : 'neutral'"
            @click="selectSource(source)"
          />
        </div>

        <!-- Results grid -->
        <div class="grid grid-cols-3 sm:grid-cols-4 gap-2 max-h-72 overflow-y-auto p-0.5">
          <button
            v-for="poster in posters"
            :key="poster.url"
            class="group relative aspect-[2/3] rounded-lg overflow-hidden bg-neutral-800 ring-2 transition-all w-full"
            :class="
              selectedPosterUrl === poster.url
                ? 'ring-primary-500'
                : 'ring-transparent hover:ring-neutral-500'
            "
            @click="selectedPosterUrl = poster.url"
          >
            <img :src="poster.url" :alt="poster.label" class="w-full h-full object-cover" />

            <!-- Selected check -->
            <div
              v-if="selectedPosterUrl === poster.url"
              class="absolute inset-0 bg-primary-500/20 flex items-center justify-center"
            >
              <UIcon name="i-lucide-check-circle" class="w-6 h-6 text-primary-400" />
            </div>

            <!-- Hover info overlay -->
            <div
              class="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/90 to-transparent px-2 py-2 translate-y-full group-hover:translate-y-0 transition-transform duration-200"
            >
              <p class="text-xs text-neutral-300 truncate">
                by <span class="font-medium text-white">{{ poster.author }}</span>
              </p>
              <a
                :href="poster.sourceUrl"
                target="_blank"
                rel="noopener noreferrer"
                class="text-xs text-primary-400 hover:text-primary-300 underline"
                @click.stop
              >
                View source
              </a>
            </div>
          </button>

          <!-- Sentinel for infinite scroll -->
          <div ref="sentinel" class="col-span-full h-1" />
        </div>

        <!-- Loading indicator -->
        <div v-if="loadingMore" class="flex justify-center py-3">
          <UIcon name="i-lucide-loader-circle" class="w-5 h-5 text-neutral-500 animate-spin" />
        </div>
      </div>
    </template>

    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <UButton label="Cancel" color="neutral" variant="ghost" @click="close" />
        <UButton label="Apply" :disabled="!canConfirm" icon="i-lucide-check" @click="confirm" />
      </div>
    </template>
  </UModal>
</template>
