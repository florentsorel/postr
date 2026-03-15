<script setup lang="ts">
import { ref, computed, watch } from "vue"

type PingStatus = "idle" | "loading" | "ok" | "error"

interface Library {
  key: string
  title: string
  type: string
  enabled: boolean
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
  confirm: [targets: { type: string; sectionKeys: string[] }[]]
}>()

const selected = ref<string[]>([])
const pingStatus = ref<PingStatus>("idle")
const pingError = ref("")
const libraries = ref<Library[]>([])

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
    if (v) checkConnection()
  }
)

const canConfirm = computed(() => selected.value.length > 0 && pingStatus.value === "ok")

function confirm() {
  const targets = selected.value.map((type) => ({
    type,
    sectionKeys: enabledSectionsFor(type).map((l) => l.key),
  }))
  emit("confirm", targets)
  emit("update:open", false)
}

function close() {
  emit("update:open", false)
}
</script>

<template>
  <UModal
    :open="open"
    title="Import from Plex"
    description="Select which media types to import."
    :dismissible="false"
    @update:open="$emit('update:open', $event)"
  >
    <template #body>
      <div class="flex flex-col gap-4">
        <!-- Connection status -->
        <div class="flex items-center gap-2 text-sm px-1">
          <template v-if="pingStatus === 'loading'">
            <UIcon name="i-lucide-loader-circle" class="w-4 h-4 text-neutral-400 animate-spin" />
            <span class="text-neutral-400">Checking Plex connection…</span>
          </template>
          <template v-else-if="pingStatus === 'ok'">
            <UIcon name="i-lucide-circle-check" class="w-4 h-4 text-green-400" />
            <span class="text-green-400">Plex server is reachable</span>
          </template>
          <template v-else-if="pingStatus === 'error'">
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
          </template>
        </div>

        <USeparator />

        <div
          v-if="availableTypes.length === 0 && pingStatus === 'ok'"
          class="text-sm text-neutral-500 px-1"
        >
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
      </div>
    </template>

    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <UButton label="Cancel" color="neutral" variant="ghost" @click="close" />
        <UButton
          label="Import"
          icon="i-lucide-download"
          :disabled="!canConfirm"
          :loading="pingStatus === 'loading'"
          @click="confirm"
        />
      </div>
    </template>
  </UModal>
</template>
