<script setup lang="ts">
defineProps<{
  thumb: string
  title: string
  type: string
  seasonNumber?: number | null
}>()

function typeLabel(type: string, seasonNumber?: number | null): string {
  if (type === "season" && seasonNumber) return `Season · S${seasonNumber}`
  const labels: Record<string, string> = {
    movie: "Movie",
    show: "TV Series",
    season: "Season",
    collection: "Collection",
  }
  return labels[type] ?? type
}
</script>

<template>
  <div class="flex items-center gap-3 py-3">
    <div class="w-10 h-14 rounded-md overflow-hidden bg-neutral-800 shrink-0">
      <img :src="thumb" :alt="title" class="w-full h-full object-cover" />
    </div>
    <div class="flex-1 min-w-0">
      <p class="text-sm font-medium text-white truncate">{{ title }}</p>
      <p class="text-xs text-neutral-500">{{ typeLabel(type, seasonNumber) }}</p>
    </div>
    <slot />
  </div>
</template>
