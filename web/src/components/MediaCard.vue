<script lang="ts">
import { ref } from "vue"
const activeCardId = ref<symbol | null>(null)
</script>

<script setup lang="ts">
import { computed, watch } from "vue"
import { useToast } from "@nuxt/ui/composables/useToast"
import { isAllowedPosterMimeType } from "@/utils/poster"

type MediaType = "movie" | "show" | "season" | "collection"

interface Props {
  title: string
  type: MediaType
  year?: number
  seasonNumber?: number
  thumb?: string
  syncing?: boolean
  pulling?: boolean
  uploading?: boolean
  fileDragging?: boolean
  inQueue?: boolean
  locallyModified?: boolean
  isOrphan?: boolean
}

const props = defineProps<Props>()
const displayedThumb = ref(props.thumb)
const imageLoaded = ref(!props.thumb)
const pendingThumb = ref<string | undefined>(undefined)
const pendingVisible = ref(false)
const cardId = Symbol()
const tapped = computed(() => activeCardId.value === cardId)
const isDragOver = ref(false)
const toast = useToast()
const suppressOverlay = computed(() => props.uploading || pendingThumb.value !== undefined)

function onImageLoad() {
  imageLoaded.value = true
}

function onPendingLoad() {
  pendingVisible.value = true
  setTimeout(() => {
    displayedThumb.value = pendingThumb.value
    pendingThumb.value = undefined
    pendingVisible.value = false
  }, 300)
}

watch(
  () => props.thumb,
  (newThumb) => {
    if (activeCardId.value === cardId) activeCardId.value = null
    if (!imageLoaded.value || !displayedThumb.value) {
      displayedThumb.value = newThumb
      imageLoaded.value = !newThumb
      pendingThumb.value = undefined
      pendingVisible.value = false
      return
    }
    pendingThumb.value = newThumb
    pendingVisible.value = false
  }
)

type EmitName = "changePoster" | "sendToPlex" | "getFromPlex" | "deleteOrphan"

const emit = defineEmits<{
  changePoster: []
  sendToPlex: []
  getFromPlex: []
  deleteOrphan: []
  dropFile: [file: File]
}>()

function onPosterClick() {
  if (!imageLoaded.value || props.syncing || props.pulling || props.uploading) return
  activeCardId.value = activeCardId.value === cardId ? null : cardId
}

function getDragFileType(e: DragEvent): string {
  return Array.from(e.dataTransfer?.items ?? []).find((i) => i.kind === "file")?.type ?? ""
}

function onDragOver(e: DragEvent) {
  if (props.syncing || props.pulling || props.uploading) return
  e.preventDefault()
  if (!e.dataTransfer) return
  if (props.isOrphan || !isAllowedPosterMimeType(getDragFileType(e))) {
    e.dataTransfer.dropEffect = "none"
    return
  }
  e.dataTransfer.dropEffect = "copy"
  isDragOver.value = true
}

function onDragLeave() {
  isDragOver.value = false
}

function onDrop(e: DragEvent) {
  e.preventDefault()
  isDragOver.value = false
  if (props.isOrphan) {
    toast.add({
      title: "Item is orphaned",
      description: "This item no longer exists in Plex and cannot be updated.",
      color: "warning",
      icon: "i-lucide-unlink",
    })
    return
  }
  const file = e.dataTransfer?.files?.[0]
  if (file && isAllowedPosterMimeType(file.type)) emit("dropFile", file)
}

function closeOverlay() {
  activeCardId.value = null
}

function action(name: EmitName) {
  ;(emit as (name: EmitName) => void)(name)
}

const typeLabel: Record<MediaType, string> = {
  movie: "Movie",
  show: "TV Series",
  season: "Season",
  collection: "Collection",
}
</script>

<template>
  <div class="group flex flex-col gap-2 mb-4 sm:mb-0">
    <!-- Poster -->
    <div
      :class="[
        'relative w-full aspect-[2/3] rounded-xl overflow-hidden bg-neutral-800 transition-[box-shadow,opacity] duration-200',
        isDragOver
          ? 'ring-2 ring-primary-500'
          : fileDragging &&
            !isOrphan &&
            !syncing &&
            !pulling &&
            !uploading &&
            'ring-1 ring-primary-500/40',
        fileDragging && isOrphan && 'opacity-40',
      ]"
      @click="onPosterClick"
      @dragover="onDragOver"
      @dragleave="onDragLeave"
      @drop="onDrop"
    >
      <div
        v-if="displayedThumb && !imageLoaded"
        class="absolute inset-0 bg-neutral-700 animate-pulse flex items-center justify-center"
      >
        <UIcon name="i-lucide-loader-circle" class="w-6 h-6 text-neutral-500 animate-spin" />
      </div>
      <img
        v-if="displayedThumb"
        :src="displayedThumb"
        :alt="title"
        :class="[
          'w-full h-full object-cover transition-transform duration-300 group-hover:scale-105',
          isOrphan && 'opacity-40',
          !imageLoaded && 'opacity-0',
        ]"
        @load="onImageLoad"
      />
      <div
        v-else
        class="w-full h-full flex items-center justify-center"
        data-testid="poster-fallback"
      >
        <UIcon name="i-lucide-image-off" class="w-8 h-8 text-neutral-600" />
      </div>

      <!-- Crossfade: incoming image -->
      <img
        v-if="pendingThumb"
        :src="pendingThumb"
        :alt="title"
        :class="[
          'absolute inset-0 w-full h-full object-cover transition-opacity duration-300',
          isOrphan && 'opacity-40',
          pendingVisible ? 'opacity-100' : 'opacity-0',
        ]"
        @load="onPendingLoad"
      />

      <!-- Syncing overlay (push, pull, or upload) -->
      <div
        v-if="syncing || pulling || uploading"
        class="absolute inset-0 bg-black/60 flex flex-col items-center justify-center gap-2"
      >
        <UIcon name="i-lucide-loader-circle" class="w-7 h-7 text-white animate-spin" />
        <span class="text-xs text-white/80">{{
          syncing ? "Pushing…" : uploading ? "Uploading…" : "Getting…"
        }}</span>
      </div>

      <!-- Drag over highlight -->
      <div
        v-if="isDragOver"
        class="absolute inset-0 bg-primary-500/20 flex items-center justify-center pointer-events-none"
      >
        <UIcon name="i-lucide-image-up" class="w-8 h-8 text-primary-400" />
      </div>

      <!-- Actions overlay (hover on sm+, tap on xs) -->
      <div
        v-else-if="imageLoaded"
        :class="[
          'absolute inset-0 bg-black/70 flex flex-col items-center justify-center gap-2 p-3 transition-opacity duration-200',
          tapped
            ? 'opacity-100 pointer-events-auto'
            : suppressOverlay
              ? 'opacity-0 pointer-events-none'
              : 'opacity-0 pointer-events-none sm:group-hover:opacity-100 sm:group-hover:pointer-events-auto',
        ]"
        @click.stop="closeOverlay"
      >
        <template v-if="isOrphan">
          <UButton
            icon="i-lucide-trash-2"
            size="sm"
            color="error"
            variant="solid"
            block
            @click.stop="action('deleteOrphan')"
          >
            Delete
          </UButton>
        </template>
        <template v-else>
          <UButton
            icon="i-lucide-image"
            size="sm"
            variant="solid"
            block
            @click.stop="action('changePoster')"
          >
            Change poster
          </UButton>
          <UButton
            v-if="inQueue"
            icon="i-lucide-upload"
            size="sm"
            variant="outline"
            color="neutral"
            block
            @click.stop="action('sendToPlex')"
          >
            Send to Plex
          </UButton>
          <UButton
            v-if="locallyModified"
            icon="i-lucide-download"
            size="sm"
            variant="outline"
            color="neutral"
            block
            @click.stop="action('getFromPlex')"
          >
            Get from Plex
          </UButton>
        </template>
      </div>
    </div>

    <!-- Info -->
    <div class="px-0.5">
      <div class="flex items-center gap-1.5">
        <p
          class="text-sm font-medium truncate leading-tight flex-1"
          :class="isOrphan ? 'text-neutral-500' : 'text-white'"
        >
          {{ title }}
        </p>
        <UTooltip v-if="inQueue && !isOrphan" text="Pending push to Plex">
          <UIcon name="i-lucide-upload" class="w-3.5 h-3.5 text-primary-400 shrink-0" />
        </UTooltip>
        <UTooltip v-if="isOrphan" text="No longer in Plex">
          <UIcon name="i-lucide-unlink" class="w-3.5 h-3.5 text-neutral-600 shrink-0" />
        </UTooltip>
      </div>
      <div class="flex items-center gap-1.5 mt-1">
        <span
          :class="isOrphan ? 'badge-neutral' : `badge-${type}`"
          class="inline-flex items-center rounded-md px-1.5 py-0.5 text-xs font-medium"
        >
          {{ typeLabel[type] }}
        </span>
        <span v-if="type === 'season' && seasonNumber" class="text-xs text-neutral-500"
          >S{{ seasonNumber }}</span
        >
        <span v-if="year" class="text-xs text-neutral-500">{{ year }}</span>
      </div>
    </div>
  </div>
</template>
