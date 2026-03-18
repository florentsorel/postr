<script setup lang="ts">
import { ref, computed, watch } from "vue"
import { readSSEStream } from "@/composables/useSSEStream"
import MediaItemRow from "./MediaItemRow.vue"

type PingStatus = "idle" | "loading" | "ok" | "error"
type Phase = "confirm" | "checking" | "done"

interface ChangedItem {
  ratingKey: string
  title: string
  mediaType: string
  seasonNumber?: number | null
  updatedAt: number
}

interface FailedItem {
  ratingKey: string
  title: string
  reason: string
  orphaned: boolean
}

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{
  "update:open": [value: boolean]
  synced: [items: ChangedItem[]]
  orphaned: [ratingKeys: string[]]
}>()

const phase = ref<Phase>("confirm")
const pingStatus = ref<PingStatus>("idle")
const pingError = ref("")
const progress = ref({ current: 0, total: 0 })
const changedItems = ref<ChangedItem[]>([])
const failedItems = ref<FailedItem[]>([])

const progressPercent = computed(() =>
  progress.value.total > 0 ? Math.round((progress.value.current / progress.value.total) * 100) : 0
)

async function checkConnection() {
  pingStatus.value = "loading"
  try {
    const res = await fetch("/api/plex/ping")
    const data = await res.json()
    if (data.reachable) {
      pingStatus.value = "ok"
    } else {
      pingStatus.value = "error"
      pingError.value = data.error ?? "Unable to reach Plex server."
    }
  } catch {
    pingStatus.value = "error"
    pingError.value = "Unable to reach Plex server."
  }
}

watch(
  () => props.open,
  (v) => {
    if (v) {
      phase.value = "confirm"
      pingStatus.value = "idle"
      pingError.value = ""
      progress.value = { current: 0, total: 0 }
      changedItems.value = []
      failedItems.value = []
      checkConnection()
    }
  },
  { immediate: true }
)

async function confirm() {
  phase.value = "checking"
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
  } else if (event.type === "failed") {
    failedItems.value.push({
      ratingKey: event.ratingKey as string,
      title: event.title as string,
      reason: event.reason as string,
      orphaned: (event.orphaned as boolean) ?? false,
    })
  } else if (event.type === "done") {
    phase.value = "done"
    if (changedItems.value.length > 0) {
      emit("synced", changedItems.value)
    }
    const newOrphans = failedItems.value.filter((i) => i.orphaned).map((i) => i.ratingKey)
    if (newOrphans.length > 0) {
      emit("orphaned", newOrphans)
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
        : phase === 'done'
          ? changedItems.length === 0 && failedItems.length === 0
            ? `All posters are up to date (${progress.total} checked)`
            : [
                changedItems.length > 0
                  ? `${changedItems.length} poster${changedItems.length !== 1 ? 's' : ''} updated`
                  : '',
                failedItems.length > 0 ? `${failedItems.length} failed` : '',
              ]
                .filter(Boolean)
                .join(' · ')
          : 'Compare local posters with Plex and update any that have changed.'
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
      <!-- Confirm phase -->
      <div v-if="phase === 'confirm'" class="flex flex-col gap-4">
        <!-- Loading -->
        <div v-if="pingStatus === 'loading'" class="flex items-center gap-2 text-sm px-1">
          <UIcon name="i-lucide-loader-circle" class="w-4 h-4 text-neutral-400 animate-spin" />
          <span class="text-neutral-400">Checking Plex connection…</span>
        </div>

        <!-- OK -->
        <template v-else-if="pingStatus === 'ok'">
          <p class="text-sm text-neutral-300">
            This will compare all local posters with Plex and update any that have changed directly
            in Plex. Only items not modified locally will be checked.
          </p>
          <p class="text-sm text-neutral-400">Do you want to proceed?</p>
        </template>

        <!-- Error -->
        <template v-else>
          <div class="flex items-center gap-2 text-sm px-1">
            <UIcon name="i-lucide-circle-x" class="w-4 h-4 text-red-400" />
            <span class="text-red-400">{{ pingError }}</span>
            <UButton
              size="xs"
              variant="ghost"
              color="neutral"
              icon="i-lucide-refresh-cw"
              class="ml-auto"
              @click="checkConnection"
            />
          </div>
        </template>
      </div>

      <!-- Checking phase -->
      <div v-else-if="phase === 'checking'" class="flex flex-col gap-5 py-2">
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
      <div v-else-if="phase === 'done'" class="flex flex-col gap-4">
        <!-- All up to date -->
        <div
          v-if="changedItems.length === 0 && failedItems.length === 0"
          class="flex flex-col items-center justify-center py-10 gap-3 text-center"
        >
          <UIcon name="i-lucide-check-circle" class="w-10 h-10 text-green-500" />
          <p class="text-neutral-400 text-sm">All posters are up to date.</p>
        </div>

        <template v-else>
          <!-- Changed items list -->
          <div
            v-if="changedItems.length > 0"
            class="flex flex-col divide-y divide-neutral-800 max-h-48 overflow-y-auto pr-2"
          >
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

          <!-- Failed items list -->
          <div v-if="failedItems.length > 0" class="flex flex-col gap-2">
            <p class="text-xs font-medium text-neutral-500 uppercase tracking-wide px-1">
              Failed ({{ failedItems.length }})
            </p>
            <div class="flex flex-col divide-y divide-neutral-800 max-h-48 overflow-y-auto pr-2">
              <MediaItemRow
                v-for="item in failedItems"
                :key="item.ratingKey"
                :thumb="`/api/media/${item.ratingKey}/thumb`"
                :title="item.title"
                type=""
              >
                <span class="text-xs text-red-400 shrink-0">{{ item.reason }}</span>
              </MediaItemRow>
            </div>
          </div>
        </template>
      </div>
    </template>

    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <UButton
          v-if="phase === 'confirm'"
          label="Cancel"
          color="neutral"
          variant="ghost"
          @click="close"
        />
        <UButton
          v-if="phase === 'confirm'"
          label="Sync"
          icon="i-lucide-scan-search"
          :disabled="pingStatus !== 'ok'"
          @click="confirm"
        />
        <UButton v-if="phase === 'done'" label="Close" @click="close" />
      </div>
    </template>
  </UModal>
</template>
