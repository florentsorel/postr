<script setup lang="ts">
import { ref, computed, watch } from "vue"
import { readSSEStream } from "@/composables/useSSEStream"

type PingStatus = "idle" | "loading" | "ok" | "error"
type Phase = "selecting" | "importing" | "done"

interface Library {
  key: string
  title: string
  type: string
  enabled: boolean
}

interface SkipEntry {
  title: string
  message: string
}

interface ImportStats {
  added: number
  skipped: number
  deleted: number
  skippedItems: SkipEntry[]
  errors: SkipEntry[]
}

const SECTION_TYPE: Record<string, string> = {
  movie: "movie",
  show: "show",
  season: "show",
  collection: "movie",
}

const ALL_MEDIA_TYPES = [
  { value: "movie", label: "Movies" },
  { value: "show", label: "TV Series" },
  { value: "season", label: "Seasons" },
  { value: "collection", label: "Collections" },
]

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  "update:open": [value: boolean]
  imported: []
}>()

const phase = ref<Phase>("selecting")
const selected = ref<string[]>([])
const pingStatus = ref<PingStatus>("idle")
const pingError = ref("")
const libraries = ref<Library[]>([])

const progress = ref({ current: 0, total: 0 })
const stats = ref<ImportStats>({ added: 0, skipped: 0, deleted: 0, skippedItems: [], errors: [] })
const skipsOpen = ref(false)
const errorsOpen = ref(false)

const progressPercent = computed(() =>
  progress.value.total > 0 ? Math.round((progress.value.current / progress.value.total) * 100) : 0
)

function enabledSectionsFor(typeValue: string): Library[] {
  return libraries.value.filter((l) => l.type === SECTION_TYPE[typeValue] && l.enabled)
}

const availableTypes = computed(() =>
  ALL_MEDIA_TYPES.filter((t) => enabledSectionsFor(t.value).length > 0)
)

async function checkConnection() {
  pingStatus.value = "loading"
  libraries.value = []
  selected.value = []
  try {
    const [pingRes, libRes] = await Promise.all([fetch("/api/plex/ping"), fetch("/api/libraries")])
    const pingData = await pingRes.json()
    if (pingData.reachable) {
      pingStatus.value = "ok"
      if (libRes.ok) {
        const data = await libRes.json()
        libraries.value = data.libraries ?? []
        selected.value = ALL_MEDIA_TYPES.filter((t) => enabledSectionsFor(t.value).length > 0).map(
          (t) => t.value
        )
      }
    } else {
      pingStatus.value = "error"
      pingError.value = pingData.error ?? "Unable to reach Plex server."
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
      phase.value = "selecting"
      progress.value = { current: 0, total: 0 }
      stats.value = { added: 0, skipped: 0, deleted: 0, skippedItems: [], errors: [] }
      skipsOpen.value = false
      errorsOpen.value = false
      checkConnection()
    }
  }
)

const canImport = computed(() => selected.value.length > 0 && pingStatus.value === "ok")

async function startImport() {
  phase.value = "importing"
  progress.value = { current: 0, total: 0 }
  stats.value = { added: 0, skipped: 0, deleted: 0, skippedItems: [], errors: [] }

  const targets = selected.value.map((type) => ({
    type,
    sectionKeys: enabledSectionsFor(type).map((l) => l.key),
  }))

  await readSSEStream(
    "/api/plex/import",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ targets }),
    },
    handleEvent
  )

  if (phase.value === "importing") phase.value = "done"
}

function handleEvent(event: Record<string, unknown>) {
  if (event.type === "start") {
    progress.value.total = event.total as number
  } else if (event.type === "progress") {
    progress.value.current = event.current as number
    progress.value.total = event.total as number
  } else if (event.type === "skip") {
    const entry = { title: event.title as string, message: event.message as string }
    if (event.message === "unchanged") {
      stats.value.skippedItems.push(entry)
    } else {
      stats.value.errors.push(entry)
    }
  } else if (event.type === "done") {
    stats.value.added = event.added as number
    stats.value.skipped = event.skipped as number
    stats.value.deleted = event.deleted as number
    phase.value = "done"
    emit("imported")
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
    title="Import from Plex"
    :description="phase === 'selecting' ? 'Select which media types to import.' : undefined"
    :close="phase !== 'importing'"
    :dismissible="phase !== 'importing'"
    @update:open="
      (v) => {
        if (!v) close()
      }
    "
  >
    <template #body>
      <!-- Selecting phase -->
      <div v-if="phase === 'selecting'" class="flex flex-col gap-4">
        <!-- Loading -->
        <div v-if="pingStatus === 'loading'" class="flex items-center gap-2 text-sm px-1">
          <UIcon name="i-lucide-loader-circle" class="w-4 h-4 text-neutral-400 animate-spin" />
          <span class="text-neutral-400">Checking Plex connection…</span>
        </div>

        <!-- Error -->
        <div v-else-if="pingStatus === 'error'" class="flex items-center gap-2 text-sm px-1">
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

        <!-- OK -->
        <template v-else-if="pingStatus === 'ok'">
          <div v-if="availableTypes.length === 0" class="text-sm text-neutral-500 px-1">
            No libraries enabled. Enable libraries in Settings first.
          </div>
          <div v-else class="flex flex-col gap-3">
            <label
              v-for="type in availableTypes"
              :key="type.value"
              class="flex items-center gap-3 px-4 py-3 rounded-lg bg-neutral-800/50 border border-neutral-700/50 cursor-pointer hover:bg-neutral-800 transition-colors"
            >
              <UCheckbox
                :model-value="selected.includes(type.value)"
                @update:model-value="
                  (v) =>
                    v ? selected.push(type.value) : selected.splice(selected.indexOf(type.value), 1)
                "
              />
              <div class="flex-1 min-w-0">
                <p class="text-sm font-medium text-white">{{ type.label }}</p>
                <p class="text-xs text-neutral-500 mt-0.5 truncate">
                  {{
                    enabledSectionsFor(type.value)
                      .map((l) => l.title)
                      .join(", ")
                  }}
                </p>
              </div>
            </label>
          </div>
        </template>
      </div>

      <!-- Importing phase -->
      <div v-else-if="phase === 'importing'" class="flex flex-col gap-5 py-2">
        <div class="flex items-center gap-3">
          <UIcon
            name="i-lucide-loader-circle"
            class="w-5 h-5 text-primary-400 animate-spin shrink-0"
          />
          <span class="text-sm text-neutral-300">
            Importing posters…
            <span class="text-neutral-500 ml-1">{{ progress.current }} / {{ progress.total }}</span>
          </span>
          <span class="ml-auto text-sm font-medium text-white">{{ progressPercent }}%</span>
        </div>
        <UProgress :model-value="progressPercent" :max="100" />
      </div>

      <!-- Done phase -->
      <div v-else-if="phase === 'done'" class="flex flex-col gap-4">
        <div class="grid grid-cols-3 gap-3">
          <div
            class="flex flex-col items-center gap-1 rounded-lg bg-neutral-800/50 border border-neutral-700/50 px-4 py-3"
          >
            <span class="text-xl font-bold text-green-400">{{ stats.added }}</span>
            <span class="text-xs text-neutral-500">Added</span>
          </div>
          <div
            class="flex flex-col items-center gap-1 rounded-lg bg-neutral-800/50 border border-neutral-700/50 px-4 py-3"
          >
            <span class="text-xl font-bold text-yellow-400">{{ stats.skipped }}</span>
            <span class="text-xs text-neutral-500">Skipped</span>
          </div>
          <div
            class="flex flex-col items-center gap-1 rounded-lg bg-neutral-800/50 border border-neutral-700/50 px-4 py-3"
          >
            <span class="text-xl font-bold text-neutral-400">{{ stats.deleted }}</span>
            <span class="text-xs text-neutral-500">Deleted</span>
          </div>
        </div>

        <!-- Skipped accordion -->
        <div
          v-if="stats.skippedItems.length > 0"
          class="rounded-lg border border-neutral-700/50 overflow-hidden"
        >
          <button
            class="w-full flex items-center justify-between px-4 py-3 text-sm text-neutral-300 hover:bg-neutral-800/50 transition-colors"
            @click="skipsOpen = !skipsOpen"
          >
            <span class="flex items-center gap-2">
              <UIcon name="i-lucide-minus-circle" class="w-4 h-4 text-yellow-400" />
              {{ stats.skippedItems.length }} skipped
            </span>
            <UIcon
              :name="skipsOpen ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'"
              class="w-4 h-4 text-neutral-500"
            />
          </button>
          <div v-if="skipsOpen" class="divide-y divide-neutral-700/50 max-h-48 overflow-y-auto">
            <div v-for="(item, i) in stats.skippedItems" :key="i" class="px-4 py-2.5">
              <p class="text-sm text-neutral-300">{{ item.title }}</p>
              <p class="text-xs text-neutral-500 mt-0.5">{{ item.message }}</p>
            </div>
          </div>
        </div>

        <div
          v-if="stats.errors.length > 0"
          class="rounded-lg border border-neutral-700/50 overflow-hidden"
        >
          <button
            class="w-full flex items-center justify-between px-4 py-3 text-sm text-neutral-300 hover:bg-neutral-800/50 transition-colors"
            @click="errorsOpen = !errorsOpen"
          >
            <span class="flex items-center gap-2">
              <UIcon name="i-lucide-triangle-alert" class="w-4 h-4 text-red-400" />
              {{ stats.errors.length }} error{{ stats.errors.length !== 1 ? "s" : "" }}
            </span>
            <UIcon
              :name="errorsOpen ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'"
              class="w-4 h-4 text-neutral-500"
            />
          </button>
          <div v-if="errorsOpen" class="divide-y divide-neutral-700/50 max-h-48 overflow-y-auto">
            <div v-for="(error, i) in stats.errors" :key="i" class="px-4 py-2.5">
              <p class="text-sm text-neutral-300">{{ error.title }}</p>
              <p class="text-xs text-neutral-500 mt-0.5">{{ error.message }}</p>
            </div>
          </div>
        </div>
      </div>
    </template>

    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <template v-if="phase === 'selecting'">
          <UButton label="Cancel" color="neutral" variant="ghost" @click="close" />
          <UButton
            label="Import"
            icon="i-lucide-download"
            :disabled="!canImport"
            :loading="pingStatus === 'loading'"
            @click="startImport"
          />
        </template>
        <template v-else-if="phase === 'done'">
          <UButton label="Close" @click="close" />
        </template>
      </div>
    </template>
  </UModal>
</template>
