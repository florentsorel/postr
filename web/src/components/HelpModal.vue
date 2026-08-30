<script setup lang="ts">
import { computed } from "vue"
import { useServerStore } from "@/stores/useServerStore"

defineProps<{ open: boolean }>()
defineEmits<{ "update:open": [value: boolean] }>()

const server = useServerStore()

const sections = computed(() => [
  {
    icon: "i-lucide-refresh-cw",
    title: `Import from ${server.name}`,
    description: `Syncs your ${server.name} library into Postr. Downloads posters for all selected media types. Reports how many items were added, skipped (poster unchanged), or deleted. Only visible when a ${server.name} server is configured.`,
  },
  {
    icon: "i-lucide-scan-search",
    title: `Sync from ${server.name}`,
    description: `Checks whether posters have been updated directly in ${server.name} since your last import. Compares each local poster byte-for-byte and updates any that have changed. Does not add or remove items. Only visible once at least one item has been imported.`,
  },
  {
    icon: "i-lucide-upload-cloud",
    title: "Queue",
    description: `Lists all posters modified locally that are pending push to ${server.name}. Use the upload icon to push one at a time, or 'Push all to ${server.name}' to sync everything at once. Removing an item from the queue restores the original ${server.name} poster. Only visible when there are pending posters.`,
  },
  {
    icon: "i-lucide-image",
    title: "Change poster",
    description: `Replace a poster by uploading a file (drag & drop or browse), pasting a direct image URL, or searching online sources (TMDB, TVDB, Fanart.tv). The new poster is saved locally and queued for push to ${server.name}.`,
  },
  {
    icon: "i-lucide-upload",
    title: `Send to ${server.name}`,
    description: `Pushes your locally modified poster directly to ${server.name}. Only visible on cards that have a pending change (upload icon in the title row).`,
  },
  {
    icon: "i-lucide-download",
    title: `Get from ${server.name}`,
    description: `Re-downloads the poster currently set in ${server.name} and overwrites your local copy. Only visible on cards where your local poster differs from ${server.name}.`,
  },
])
</script>

<template>
  <UModal
    :open="open"
    title="Help"
    description="How the main features work."
    class="select-none"
    @update:open="$emit('update:open', $event)"
  >
    <template #body>
      <div class="flex flex-col gap-4">
        <div
          v-for="section in sections"
          :key="section.title"
          class="flex gap-3 rounded-lg bg-neutral-800/40 border border-neutral-700/40 px-4 py-3"
        >
          <div class="mt-0.5 shrink-0">
            <UIcon :name="section.icon" class="w-4 h-4 text-primary-400" />
          </div>
          <div>
            <p class="text-sm font-medium text-white">{{ section.title }}</p>
            <p class="text-xs text-neutral-400 mt-1 leading-relaxed">{{ section.description }}</p>
          </div>
        </div>
      </div>
    </template>

    <template #footer>
      <div class="flex justify-end w-full">
        <UButton
          label="Close"
          color="neutral"
          variant="ghost"
          @click="$emit('update:open', false)"
        />
      </div>
    </template>
  </UModal>
</template>
