<script setup lang="ts">
import { ref, computed, onMounted } from "vue"
import { readSSEStream } from "@/composables/useSSEStream"
import MediaItemRow from "./MediaItemRow.vue"

type Phase = "checking" | "done"

interface ChangedItem {
  ratingKey: string
  title: string
  mediaType: string
  seasonNumber?: number | null
  updatedAt: number
}

defineProps<{ open: boolean }>()
const emit = defineEmits<{
  "update:open": [value: boolean]
  synced: [items: ChangedItem[]]
}>()

const phase = ref<Phase>("checking")
const progress = ref({ current: 0, total: 0 })
const changedItems = ref<ChangedItem[]>([])

const progressPercent = computed(() =>
  progress.value.total > 0 ? Math.round((progress.value.current / progress.value.total) * 100) : 0
)

onMounted(() => {
  startSync()
})

async function startSync() {
  await readSSEStream("/api/plex/sync", { method: "POST" }, handleEvent)
  if (phase.value === "checking") phase.value = "done"
}

function handleEvent(event: Record<string, unknown>) {
  if (event.type === "start") {
    progress.value.total = event.total as number
  } else if (event.type === "progress") {
    progress.value.current = event.current as number
    progress.value.total = event.total as number
  } else if (event.type === "changed") {
    changedItems.value.push({
      ratingKey: event.ratingKey as string,
      title: event.title as string,
      mediaType: event.mediaType as string,
      seasonNumber: event.seasonNumber as number | null | undefined,
      updatedAt: event.updatedAt as number,
    })
  } else if (event.type === "done") {
    phase.value = "done"
    if (changedItems.value.length > 0) {
      emit("synced", changedItems.value)
    }
  }
}

function close() {
  emit("update:open", false)
}
</script>

<template>
  <UModal
    :open="open"
    class="select-none"
    title="Sync from Plex"
    :description="
      phase === 'checking'
        ? 'Comparing local posters with Plex…'
        : changedItems.length === 0
          ? `All posters are up to date (${progress.total} checked)`
          : `${changedItems.length} poster${changedItems.length !== 1 ? 's' : ''} updated from Plex`
    "
    :close="phase !== 'checking'"
    :dismissible="phase !== 'checking'"
    @update:open="
      (v) => {
        if (!v) close()
      }
    "
  >
    <template #body>
      <!-- Checking phase -->
      <div v-if="phase === 'checking'" class="flex flex-col gap-5 py-2">
        <div class="flex items-center gap-3">
          <UIcon
            name="i-lucide-loader-circle"
            class="w-5 h-5 text-primary-400 animate-spin shrink-0"
          />
          <span class="text-sm text-neutral-300">
            Checking posters…
            <span class="text-neutral-500 ml-1">{{ progress.current }} / {{ progress.total }}</span>
          </span>
          <span class="ml-auto text-sm font-medium text-white">{{ progressPercent }}%</span>
        </div>
        <UProgress :model-value="progressPercent" :max="100" />
      </div>

      <!-- Done phase -->
      <div v-else>
        <!-- No changes -->
        <div
          v-if="changedItems.length === 0"
          class="flex flex-col items-center justify-center py-10 gap-3 text-center"
        >
          <UIcon name="i-lucide-check-circle" class="w-10 h-10 text-green-500" />
          <p class="text-neutral-400 text-sm">All posters are up to date.</p>
        </div>

        <!-- Changed items list -->
        <div v-else class="flex flex-col divide-y divide-neutral-800 max-h-96 overflow-y-auto pr-2">
          <MediaItemRow
            v-for="item in changedItems"
            :key="item.ratingKey"
            :thumb="`/api/media/${item.ratingKey}/thumb?v=${item.updatedAt}`"
            :title="item.title"
            :type="item.mediaType"
            :season-number="item.seasonNumber"
          >
            <UBadge label="Updated" color="success" variant="subtle" size="xs" />
          </MediaItemRow>
        </div>
      </div>
    </template>

    <template #footer>
      <div class="flex justify-end w-full">
        <UButton v-if="phase === 'done'" label="Close" @click="close" />
      </div>
    </template>
  </UModal>
</template>
