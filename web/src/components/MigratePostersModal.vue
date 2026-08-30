<script setup lang="ts">
import { ref, computed, watch } from "vue"
import { readSSEStream } from "@/composables/useSSEStream"
import { useServerStore } from "@/stores/useServerStore"

const server = useServerStore()

type Phase = "loading" | "confirm" | "running" | "done"

interface MigrateStatus {
  available: boolean
  sourceName: string
  posterCount: number
  targetImported: number
  reason?: string
}

interface Report {
  title: string
  mediaType: string
  reason: string
}

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{
  "update:open": [value: boolean]
  migrated: []
}>()

const phase = ref<Phase>("loading")
const status = ref<MigrateStatus | null>(null)
const progress = ref({ current: 0, total: 0 })
const stats = ref({ migrated: 0, byId: 0, byTitle: 0, unchanged: 0, unmatched: 0, skipped: 0 })
const unmatched = ref<Report[]>([])
const skipped = ref<Report[]>([])
const unmatchedOpen = ref(false)
const skippedOpen = ref(false)
const streamError = ref("")

const progressPercent = computed(() =>
  progress.value.total > 0 ? Math.round((progress.value.current / progress.value.total) * 100) : 0
)

const sourceName = computed(() => status.value?.sourceName ?? "the other server")

async function loadStatus() {
  phase.value = "loading"
  try {
    const res = await fetch("/api/server/migrate/status")
    status.value = res.ok ? await res.json() : null
  } catch {
    status.value = null
  }
  phase.value = "confirm"
}

watch(
  () => props.open,
  (v) => {
    if (!v) return
    progress.value = { current: 0, total: 0 }
    stats.value = { migrated: 0, byId: 0, byTitle: 0, unchanged: 0, unmatched: 0, skipped: 0 }
    unmatched.value = []
    skipped.value = []
    unmatchedOpen.value = false
    skippedOpen.value = false
    streamError.value = ""
    loadStatus()
  },
  { immediate: true }
)

async function start() {
  phase.value = "running"
  await readSSEStream("/api/server/migrate", { method: "POST" }, handleEvent)
  if (phase.value === "running") phase.value = "done"
}

function handleEvent(event: Record<string, unknown>) {
  if (event.type === "start") {
    progress.value.total = event.total as number
  } else if (event.type === "progress") {
    progress.value.current = event.current as number
    progress.value.total = event.total as number
  } else if (event.type === "unmatched" || event.type === "skipped") {
    const entry = {
      title: event.title as string,
      mediaType: event.mediaType as string,
      reason: event.reason as string,
    }
    if (event.type === "unmatched") unmatched.value.push(entry)
    else skipped.value.push(entry)
  } else if (event.type === "error") {
    streamError.value = event.message as string
    phase.value = "done"
  } else if (event.type === "done") {
    stats.value = {
      migrated: event.migrated as number,
      byId: event.byId as number,
      byTitle: event.byTitle as number,
      unchanged: (event.unchanged as number) ?? 0,
      unmatched: event.unmatched as number,
      skipped: event.skipped as number,
    }
    phase.value = "done"
    if (stats.value.migrated > 0) emit("migrated")
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
    :title="`Migrate posters from ${sourceName}`"
    :close="phase !== 'running'"
    :dismissible="phase !== 'running'"
    @update:open="
      (v) => {
        if (!v) close()
      }
    "
  >
    <template #body>
      <!-- Loading status -->
      <div v-if="phase === 'loading'" class="flex items-center gap-2 text-sm px-1">
        <UIcon name="i-lucide-loader-circle" class="w-4 h-4 text-neutral-400 animate-spin" />
        <span class="text-neutral-400">Checking what can be migrated…</span>
      </div>

      <!-- Confirm -->
      <div v-else-if="phase === 'confirm'" class="flex flex-col gap-4">
        <div v-if="!status?.available" class="flex items-start gap-2 text-sm px-1">
          <UIcon name="i-lucide-info" class="w-4 h-4 text-neutral-500 shrink-0 mt-0.5" />
          <span class="text-neutral-400">
            {{ status?.reason ?? "Poster migration is not available." }}
          </span>
        </div>
        <template v-else>
          <p class="text-sm text-neutral-300">
            Postr will match your
            <span class="text-white font-medium">{{ status.posterCount }}</span>
            {{ sourceName }} posters against the
            <span class="text-white font-medium">{{ status.targetImported }}</span>
            items imported from {{ server.name }}, using their TMDB / IMDB / TVDB identifiers and
            falling back to titles.
          </p>
          <div
            class="flex items-start gap-2 rounded-lg bg-neutral-800/50 border border-neutral-700/50 px-4 py-3"
          >
            <UIcon name="i-lucide-shield-check" class="w-4 h-4 text-primary-400 shrink-0 mt-0.5" />
            <p class="text-xs text-neutral-400">
              Nothing is written to {{ server.name }}. Matched posters land in the queue for you to
              review, and your {{ sourceName }} copies are left untouched. Running this again later
              only picks up what has changed.
            </p>
          </div>
          <div
            class="flex items-start gap-2 rounded-lg bg-yellow-500/5 border border-yellow-500/20 px-4 py-3"
          >
            <UIcon name="i-lucide-triangle-alert" class="w-4 h-4 text-yellow-400 shrink-0 mt-0.5" />
            <p class="text-xs text-neutral-400">
              <span class="text-neutral-300">Collections are matched by title alone</span> — no
              external database tracks them. If your two servers name them differently, in another
              language or with a suffix, they are reported as unmatched rather than guessed at.
              Expect to set those posters by hand.
            </p>
          </div>
        </template>
      </div>

      <!-- Running -->
      <div v-else-if="phase === 'running'" class="flex flex-col gap-5 py-2">
        <div class="flex items-center gap-3">
          <UIcon
            name="i-lucide-loader-circle"
            class="w-5 h-5 text-primary-400 animate-spin shrink-0"
          />
          <span class="text-sm text-neutral-300">
            Matching posters…
            <span class="text-neutral-500 ml-1">{{ progress.current }} / {{ progress.total }}</span>
          </span>
          <span class="ml-auto text-sm font-medium text-white">{{ progressPercent }}%</span>
        </div>
        <UProgress :model-value="progressPercent" :max="100" />
      </div>

      <!-- Done -->
      <div v-else class="flex flex-col gap-4">
        <div v-if="streamError" class="flex items-start gap-2 text-sm px-1">
          <UIcon name="i-lucide-circle-x" class="w-4 h-4 text-red-400 shrink-0 mt-0.5" />
          <span class="text-red-400">{{ streamError }}</span>
        </div>

        <template v-else>
          <div class="grid grid-cols-3 gap-3">
            <div
              class="flex flex-col items-center gap-1 rounded-lg bg-neutral-800/50 border border-neutral-700/50 px-4 py-3"
            >
              <span class="text-xl font-bold text-green-400">{{ stats.migrated }}</span>
              <span class="text-xs text-neutral-500">Queued</span>
            </div>
            <div
              class="flex flex-col items-center gap-1 rounded-lg bg-neutral-800/50 border border-neutral-700/50 px-4 py-3"
            >
              <span class="text-xl font-bold text-yellow-400">{{ stats.unmatched }}</span>
              <span class="text-xs text-neutral-500">Unmatched</span>
            </div>
            <div
              class="flex flex-col items-center gap-1 rounded-lg bg-neutral-800/50 border border-neutral-700/50 px-4 py-3"
            >
              <span class="text-xl font-bold text-neutral-400">{{ stats.skipped }}</span>
              <span class="text-xs text-neutral-500">Skipped</span>
            </div>
          </div>

          <p v-if="stats.migrated > 0" class="text-xs text-neutral-500 px-1">
            {{ stats.byId }} matched on a database identifier, {{ stats.byTitle }} on title alone —
            review those before pushing.
          </p>

          <p v-if="stats.unchanged > 0" class="text-xs text-neutral-500 px-1">
            {{ stats.unchanged }} already carried over by an earlier run, left untouched.
          </p>

          <div
            v-if="unmatched.length > 0"
            class="rounded-lg border border-neutral-700/50 overflow-hidden"
          >
            <button
              class="w-full flex items-center justify-between px-4 py-3 text-sm text-neutral-300 hover:bg-neutral-800/50 transition-colors"
              @click="unmatchedOpen = !unmatchedOpen"
            >
              <span class="flex items-center gap-2">
                <UIcon name="i-lucide-search-x" class="w-4 h-4 text-yellow-400" />
                {{ unmatched.length }} without a match on {{ server.name }}
              </span>
              <UIcon
                :name="unmatchedOpen ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'"
                class="w-4 h-4 text-neutral-500"
              />
            </button>
            <div
              v-if="unmatchedOpen"
              class="divide-y divide-neutral-700/50 max-h-48 overflow-y-auto"
            >
              <div v-for="(item, i) in unmatched" :key="i" class="px-4 py-2.5">
                <p class="text-sm text-neutral-300">{{ item.title }}</p>
                <p class="text-xs text-neutral-500 mt-0.5">{{ item.reason }}</p>
              </div>
            </div>
          </div>

          <div
            v-if="skipped.length > 0"
            class="rounded-lg border border-neutral-700/50 overflow-hidden"
          >
            <button
              class="w-full flex items-center justify-between px-4 py-3 text-sm text-neutral-300 hover:bg-neutral-800/50 transition-colors"
              @click="skippedOpen = !skippedOpen"
            >
              <span class="flex items-center gap-2">
                <UIcon name="i-lucide-minus-circle" class="w-4 h-4 text-neutral-400" />
                {{ skipped.length }} skipped
              </span>
              <UIcon
                :name="skippedOpen ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'"
                class="w-4 h-4 text-neutral-500"
              />
            </button>
            <div v-if="skippedOpen" class="divide-y divide-neutral-700/50 max-h-48 overflow-y-auto">
              <div v-for="(item, i) in skipped" :key="i" class="px-4 py-2.5">
                <p class="text-sm text-neutral-300">{{ item.title }}</p>
                <p class="text-xs text-neutral-500 mt-0.5">{{ item.reason }}</p>
              </div>
            </div>
          </div>
        </template>
      </div>
    </template>

    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <template v-if="phase === 'confirm'">
          <UButton label="Cancel" color="neutral" variant="ghost" @click="close" />
          <UButton
            :label="`Migrate from ${sourceName}`"
            icon="i-lucide-arrow-right-left"
            :disabled="!status?.available"
            @click="start"
          />
        </template>
        <template v-else-if="phase === 'done'">
          <UButton label="Close" @click="close" />
        </template>
      </div>
    </template>
  </UModal>
</template>
