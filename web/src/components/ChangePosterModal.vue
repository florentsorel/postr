<script setup lang="ts">
import { ref, computed, watch, onMounted } from "vue"
import PosterPreview from "./PosterPreview.vue"

type MediaType = "movie" | "show" | "season" | "collection"

interface MediaItem {
  id: number
  ratingKey: string
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
  uploaded: [payload: { ratingKey: string; thumb: string }]
}>()

// -- Settings --
const settings = ref({ autoResize: true, resizeWidth: 1000 })

onMounted(async () => {
  try {
    const res = await fetch("/api/settings")
    if (res.ok) {
      const data = await res.json()
      settings.value.autoResize = data.auto_resize ?? true
      settings.value.resizeWidth = data.resize_width ?? 1000
    }
  } catch {
    // use defaults
  }
})

// -- Tabs --
const activeTab = ref("upload")
const tabs = [
  { label: "Upload", value: "upload", icon: "i-lucide-upload" },
  { label: "From URL", value: "url", icon: "i-lucide-link" },
]

// -- Upload --
const uploadedFile = ref<File | null>(null)
const uploadedPreview = ref<string | null>(null)
const isDragging = ref(false)
const uploading = ref(false)

function onFileChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (file) setFile(file)
}

const ALLOWED_TYPES = ["image/jpeg", "image/png", "image/webp"]

function onDragOver(e: DragEvent) {
  e.preventDefault()
  if (!e.dataTransfer) return
  const fileType = e.dataTransfer.items[0]?.type
  if (ALLOWED_TYPES.includes(fileType)) {
    e.dataTransfer.dropEffect = "copy"
    isDragging.value = true
  } else {
    e.dataTransfer.dropEffect = "none"
  }
}

function onDrop(e: DragEvent) {
  isDragging.value = false
  const file = e.dataTransfer?.files?.[0]
  if (file && ALLOWED_TYPES.includes(file.type)) setFile(file)
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

// -- Resize --
async function resizeIfNeeded(file: File): Promise<Blob> {
  if (!settings.value.autoResize) return file
  return new Promise((resolve) => {
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
    img.src = url
  })
}

// -- From URL --
const pastedUrl = ref("")

// -- Confirm --
const canConfirm = computed(() => {
  if (activeTab.value === "upload") return uploadedFile.value !== null
  return pastedUrl.value.trim().length > 0
})

async function confirm() {
  if (activeTab.value === "upload" && uploadedFile.value && props.item) {
    uploading.value = true
    try {
      const blob = await resizeIfNeeded(uploadedFile.value)
      const formData = new FormData()
      formData.append("file", blob, uploadedFile.value.name)
      const res = await fetch(`/api/media/${props.item.ratingKey}/upload`, {
        method: "POST",
        body: formData,
      })
      if (res.ok) {
        const data = await res.json()
        emit("uploaded", { ratingKey: props.item.ratingKey, thumb: data.thumb })
        close()
      }
    } finally {
      uploading.value = false
    }
  } else if (activeTab.value === "url" && pastedUrl.value.trim() && props.item) {
    uploading.value = true
    try {
      const res = await fetch(`/api/media/${props.item.ratingKey}/upload-url`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url: pastedUrl.value.trim() }),
      })
      if (res.ok) {
        const data = await res.json()
        emit("uploaded", { ratingKey: props.item.ratingKey, thumb: data.thumb })
        close()
      }
    } finally {
      uploading.value = false
    }
  }
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
      pastedUrl.value = ""
      uploading.value = false
    }
  }
)
</script>

<template>
  <UModal
    :open="open"
    class="select-none"
    :dismissible="false"
    :ui="{ overlay: 'items-start pt-10' }"
    :title="item?.title ?? 'Change poster'"
    :description="
      item
        ? [item.year, item.type].filter(Boolean).join(' · ') || 'Change poster artwork'
        : 'Change poster artwork'
    "
    @update:open="$emit('update:open', $event)"
  >
    <template #body>
      <UTabs v-model="activeTab" :items="tabs" class="mb-4" />

      <!-- Upload tab -->
      <div v-if="activeTab === 'upload'">
        <!-- Preview -->
        <PosterPreview
          v-if="uploadedPreview"
          :src="uploadedPreview"
          clearable
          @clear="clearUpload"
        />

        <!-- Drop zone -->
        <label
          v-else
          class="flex flex-col items-center justify-center gap-3 w-full h-48 rounded-xl border-2 border-dashed cursor-pointer transition-colors"
          :class="
            isDragging
              ? 'border-primary-500 bg-primary-500/10'
              : 'border-neutral-700 hover:border-neutral-500 bg-neutral-800/40'
          "
          @dragover="onDragOver"
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
          <input
            type="file"
            accept="image/jpeg,image/png,image/webp"
            class="sr-only"
            @change="onFileChange"
          />
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
        <PosterPreview v-if="pastedUrl.trim()" :src="pastedUrl.trim()" />
      </div>
    </template>

    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <UButton label="Cancel" color="neutral" variant="ghost" @click="close" />
        <UButton
          label="Apply"
          :disabled="!canConfirm"
          :loading="uploading"
          icon="i-lucide-check"
          @click="confirm"
        />
      </div>
    </template>
  </UModal>
</template>
