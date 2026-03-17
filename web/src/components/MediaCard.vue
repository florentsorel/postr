<script setup lang="ts">
type MediaType = "movie" | "show" | "season" | "collection"

interface Props {
  title: string
  type: MediaType
  year?: number
  seasonNumber?: number
  thumb?: string
  syncing?: boolean
  pulling?: boolean
  inQueue?: boolean
  locallyModified?: boolean
}

defineProps<Props>()

defineEmits<{
  changePoster: []
  sendToPlex: []
  getFromPlex: []
}>()

const typeLabel: Record<MediaType, string> = {
  movie: "Movie",
  show: "TV Series",
  season: "Season",
  collection: "Collection",
}
</script>

<template>
  <div class="group flex flex-col gap-2">
    <!-- Poster -->
    <div class="relative w-full aspect-[2/3] rounded-xl overflow-hidden bg-neutral-800">
      <img
        v-if="thumb"
        :src="thumb"
        :alt="title"
        class="w-full h-full object-cover transition-transform duration-300 group-hover:scale-105"
      />
      <div
        v-else
        class="w-full h-full flex items-center justify-center"
        data-testid="poster-fallback"
      >
        <UIcon name="i-lucide-image-off" class="w-8 h-8 text-neutral-600" />
      </div>

      <!-- Syncing overlay (push or pull) -->
      <div
        v-if="syncing || pulling"
        class="absolute inset-0 bg-black/60 flex flex-col items-center justify-center gap-2"
      >
        <UIcon name="i-lucide-loader-circle" class="w-7 h-7 text-white animate-spin" />
        <span class="text-xs text-white/80">{{ syncing ? "Pushing…" : "Getting…" }}</span>
      </div>

      <!-- Hover overlay -->
      <div
        v-else
        class="absolute inset-0 bg-black/70 opacity-0 group-hover:opacity-100 transition-opacity duration-200 flex flex-col items-center justify-center gap-2 p-3"
      >
        <UButton
          icon="i-lucide-image"
          size="sm"
          variant="solid"
          block
          @click.stop="$emit('changePoster')"
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
          @click.stop="$emit('sendToPlex')"
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
          @click.stop="$emit('getFromPlex')"
        >
          Get from Plex
        </UButton>
      </div>
    </div>

    <!-- Info -->
    <div class="px-0.5">
      <div class="flex items-center gap-1.5">
        <p class="text-sm font-medium text-white truncate leading-tight flex-1">{{ title }}</p>
        <UTooltip v-if="inQueue" text="Pending push to Plex">
          <UIcon name="i-lucide-upload" class="w-3.5 h-3.5 text-primary-400 shrink-0" />
        </UTooltip>
      </div>
      <div class="flex items-center gap-1.5 mt-1">
        <span
          :class="`badge-${type}`"
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
