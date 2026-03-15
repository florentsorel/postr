<script setup lang="ts">
import { ref, computed, watch } from "vue"

type PingStatus = "idle" | "loading" | "ok" | "error"

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  "update:open": [value: boolean]
  confirm: [types: string[]]
}>()

const MEDIA_TYPES = [
  { value: "movie", label: "Movies" },
  { value: "show", label: "TV Series" },
  { value: "season", label: "Seasons" },
  { value: "collection", label: "Collections" },
]

const selected = ref<string[]>(["movie", "show", "season", "collection"])
const pingStatus = ref<PingStatus>("idle")
const pingError = ref("")

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
    if (v) checkConnection()
    else pingStatus.value = "idle"
  }
)

const canConfirm = computed(() => selected.value.length > 0 && pingStatus.value === "ok")

function confirm() {
  emit("confirm", [...selected.value])
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

        <div class="flex flex-col gap-3">
          <label
            v-for="type in MEDIA_TYPES"
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
            <span class="text-sm font-medium text-white">{{ type.label }}</span>
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
