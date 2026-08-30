<script setup lang="ts">
import { ref, computed, onMounted } from "vue"
import { useRouter } from "vue-router"
import { useApiError } from "@/composables/useApiError"
import { useAuthStore } from "@/stores/useAuthStore"
import { useServerStore } from "@/stores/useServerStore"
import MigratePostersModal from "@/components/MigratePostersModal.vue"
const version = import.meta.env.VITE_APP_VERSION ?? "unknown"

const router = useRouter()
const authStore = useAuthStore()
const server = useServerStore()

// Everything about the media server section is derived from the active
// provider, so the page reads correctly whichever server is configured.
const isJellyfin = computed(() => server.provider === "jellyfin")
const urlEnvVar = computed(() => (isJellyfin.value ? "JELLYFIN_URL" : "PLEX_URL"))
const tokenEnvVar = computed(() => (isJellyfin.value ? "JELLYFIN_API_KEY" : "PLEX_TOKEN"))
const urlExample = computed(() =>
  isJellyfin.value ? "http://192.168.1.x:8096" : "http://192.168.1.x:32400"
)

async function logout() {
  await authStore.logout()
  router.push("/login")
}

type PingStatus = "idle" | "loading" | "ok" | "error"
const pingStatus = ref<PingStatus>("idle")
const pingError = ref("")

async function testConnection() {
  pingStatus.value = "loading"
  try {
    const res = await fetch("/api/server/ping")
    const data = await res.json()
    if (data.reachable) {
      pingStatus.value = "ok"
    } else {
      pingStatus.value = "error"
      pingError.value = data.error ?? `Unable to reach ${server.name} server.`
    }
  } catch {
    pingStatus.value = "error"
    pingError.value = `Unable to reach ${server.name} server.`
  }
}

const toast = useToast()
const saving = ref(false)
const { error, handleResponse, handleException } = useApiError()
const loading = ref(true)

// Read-only from env vars — fetched from backend
const env = ref({
  serverUrl: "",
  serverTokenSet: false,
  serverTokenLabel: "Token",
  authEnabled: false,
  authUser: "",
  authPassSet: false,
})

const options = ref({ autoResize: true, resizeWidth: 1000 })

// Poster migration is only offered when a second server is configured and holds
// artwork worth carrying over.
const migrateModalOpen = ref(false)
const migrateStatus = ref<{
  available: boolean
  sourceName: string
  posterCount: number
  reason?: string
} | null>(null)

// After a migration the posters sit in the queue, so the counts shown here are
// stale until we ask again.
async function onMigrated() {
  await loadMigrateStatus()
  toast.add({
    title: "Posters migrated to the queue",
    description: `Open the queue from the library to review them before pushing to ${server.name}.`,
    color: "success",
    icon: "i-lucide-check",
  })
}

async function loadMigrateStatus() {
  try {
    const res = await fetch("/api/server/migrate/status")
    migrateStatus.value = res.ok ? await res.json() : null
  } catch {
    migrateStatus.value = null
  }
}
const validationErrors = ref<Record<string, string>>({})

type LibraryStatus = "loading" | "ok" | "not_configured" | "error"

interface Library {
  key: string
  title: string
  type: string
  enabled: boolean
}

const libraryStatus = ref<LibraryStatus>("loading")
const libraryError = ref("")
const libraries = ref<Library[]>([])

onMounted(async () => {
  try {
    const [settingsRes, librariesRes] = await Promise.all([
      fetch("/api/settings"),
      fetch("/api/libraries"),
      loadMigrateStatus(),
    ])
    if (!handleResponse(settingsRes)) return
    const data = await settingsRes.json()
    env.value.serverUrl = data.server_url ?? ""
    env.value.serverTokenSet = data.server_token_set ?? false
    env.value.serverTokenLabel = data.server_token_label ?? "Token"
    env.value.authEnabled = data.auth_enabled ?? false
    env.value.authUser = data.auth_user ?? ""
    env.value.authPassSet = data.auth_pass_set ?? false
    options.value.autoResize = data.auto_resize ?? true
    options.value.resizeWidth = data.resize_width ?? 1000

    if (librariesRes.ok) {
      const libData = await librariesRes.json()
      if (!libData.configured) {
        libraryStatus.value = "not_configured"
      } else if (!libData.reachable) {
        libraryStatus.value = "error"
        libraryError.value = libData.error ?? `Unable to reach ${server.name} server.`
      } else {
        libraryStatus.value = "ok"
        libraries.value = libData.libraries ?? []
      }
    }
  } catch {
    handleException()
  } finally {
    loading.value = false
  }
})

async function save() {
  validationErrors.value = {}
  if (options.value.autoResize && options.value.resizeWidth < 500) {
    validationErrors.value.resizeWidth = "Target width must be at least 500px"
    return
  }
  saving.value = true
  try {
    const requests: Promise<Response>[] = [
      fetch("/api/settings", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ options: options.value }),
      }),
    ]
    if (libraryStatus.value === "ok") {
      requests.push(
        fetch("/api/libraries", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            libraries: libraries.value.map((l) => ({ key: l.key, enabled: l.enabled })),
          }),
        })
      )
    }
    const responses = await Promise.all(requests)
    const failed = responses.find((r) => !r.ok)
    if (failed) {
      const data = await failed.json().catch(() => ({}))
      throw new Error(data.error ?? `Server error ${failed.status}`)
    }
    toast.add({ title: "Settings saved", color: "primary", icon: "i-lucide-check-circle" })
  } catch (e) {
    toast.add({
      title: "Failed to save settings",
      description: e instanceof Error ? e.message : undefined,
      color: "error",
      icon: "i-lucide-circle-x",
    })
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div v-if="loading" class="min-h-screen bg-[#1f1f1f]" />
  <ErrorLayout v-else-if="error" :code="error.code" :message="error.message" />
  <div v-else class="min-h-screen bg-[#1f1f1f] text-white select-none">
    <!-- Header -->
    <header
      class="border-b border-neutral-800 px-6 py-4 flex items-center gap-4 sm:sticky sm:top-0 sm:z-10 bg-[#1f1f1f]"
    >
      <UButton to="/" icon="i-lucide-arrow-left" variant="ghost" color="neutral" size="sm" />
      <div class="flex items-center gap-2">
        <div class="w-7 h-7 rounded-lg bg-primary-500 flex items-center justify-center">
          <UIcon name="i-lucide-image" class="w-4 h-4 text-white" />
        </div>
        <span class="font-bold text-white text-lg">Postr</span>
      </div>
      <USeparator orientation="vertical" class="h-5" />
      <h1 class="text-sm font-medium text-neutral-300">Settings</h1>
      <div class="ml-auto flex items-center gap-2">
        <UButton
          v-if="authStore.authEnabled"
          icon="i-lucide-log-out"
          variant="ghost"
          color="neutral"
          size="sm"
          @click="logout"
        >
          Logout
        </UButton>
      </div>
    </header>

    <!-- Content -->
    <div class="max-w-2xl mx-auto px-6 py-10 flex flex-col gap-8">
      <!-- Media server (read-only) -->
      <section>
        <div class="mb-4">
          <h2 class="text-base font-semibold text-white flex items-center gap-2">
            <UIcon name="i-lucide-server" class="w-4 h-4 text-primary-500" />
            {{ server.name }} Server
          </h2>
          <p class="text-sm text-neutral-500 mt-0.5">
            Configured via environment variables
            <UBadge label="Read-only" color="neutral" variant="soft" size="xs" class="ml-2" />
          </p>
        </div>
        <UCard variant="soft" class="bg-[#282828] border-neutral-700/50">
          <div class="flex flex-col gap-4">
            <div class="flex flex-col gap-1">
              <span class="text-xs font-medium text-neutral-400">Server URL</span>
              <div
                class="flex items-center gap-2 px-3 py-2 rounded-lg bg-neutral-800/60 border border-neutral-700/50"
              >
                <UIcon name="i-lucide-globe" class="w-4 h-4 text-neutral-500 shrink-0" />
                <span class="text-sm text-neutral-300 font-mono select-text">
                  {{ env.serverUrl || "" }}
                </span>
              </div>
              <p v-if="!env.serverUrl" class="text-xs text-neutral-500">
                Set the <code class="text-neutral-400">{{ urlEnvVar }}</code> environment variable —
                e.g. <code class="text-neutral-400">{{ urlExample }}</code>
              </p>
            </div>
            <div class="flex flex-col gap-1">
              <span class="text-xs font-medium text-neutral-400">
                {{ server.name }} {{ env.serverTokenLabel }}
              </span>
              <div
                class="flex items-center gap-2 px-3 py-2 rounded-lg bg-neutral-800/60 border border-neutral-700/50"
              >
                <UIcon name="i-lucide-key" class="w-4 h-4 text-neutral-500 shrink-0" />
                <span class="text-sm text-neutral-300 font-mono">
                  {{ env.serverTokenSet ? "••••••••••••••••" : "" }}
                </span>
                <UBadge
                  v-if="env.serverTokenSet"
                  label="Set"
                  color="success"
                  variant="soft"
                  size="xs"
                  class="ml-auto"
                />
              </div>
              <p v-if="!env.serverTokenSet" class="text-xs text-neutral-500">
                Set the <code class="text-neutral-400">{{ tokenEnvVar }}</code> environment variable
                —
                <template v-if="isJellyfin">
                  create one in Jellyfin under Dashboard → Advanced → API Keys.
                </template>
                <a
                  v-else
                  href="https://support.plex.tv/articles/204059436-finding-an-authentication-token-x-plex-token/"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="text-primary-400 hover:text-primary-300 underline"
                  >how to find your token</a
                >.
              </p>
            </div>
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2 text-sm">
                <template v-if="pingStatus === 'ok'">
                  <UIcon name="i-lucide-circle-check" class="w-4 h-4 text-green-400" />
                  <span class="text-green-400">Connected</span>
                </template>
                <template v-else-if="pingStatus === 'error'">
                  <UIcon name="i-lucide-circle-x" class="w-4 h-4 text-red-400" />
                  <span class="text-red-400">{{ pingError }}</span>
                </template>
              </div>
              <UButton
                v-if="env.serverUrl && env.serverTokenSet"
                size="sm"
                variant="outline"
                color="neutral"
                icon="i-lucide-plug"
                :loading="pingStatus === 'loading'"
                @click="testConnection"
              >
                Test connection
              </UButton>
            </div>
          </div>
        </UCard>
      </section>

      <!-- Poster migration (only when a second server is configured) -->
      <section v-if="migrateStatus && migrateStatus.posterCount > 0">
        <div class="mb-4">
          <h2 class="text-base font-semibold text-white flex items-center gap-2">
            <UIcon name="i-lucide-arrow-right-left" class="w-4 h-4 text-primary-500" />
            Migrate posters
          </h2>
          <p class="text-sm text-neutral-500 mt-0.5">
            Carry artwork imported from {{ migrateStatus.sourceName }} over to {{ server.name }}
          </p>
        </div>
        <UCard variant="soft" class="bg-[#282828] border-neutral-700/50">
          <div class="flex items-center gap-4">
            <div class="flex-1 min-w-0">
              <p class="text-sm text-neutral-300">
                {{ migrateStatus.posterCount }} posters are still stored under
                {{ migrateStatus.sourceName }}.
              </p>
              <p class="text-xs text-neutral-500 mt-1">
                {{
                  migrateStatus.available
                    ? `Matched posters land in the queue for review — nothing is written to ${server.name} without your confirmation.`
                    : migrateStatus.reason
                }}
              </p>
            </div>
            <UButton
              size="sm"
              variant="outline"
              color="neutral"
              icon="i-lucide-arrow-right-left"
              :disabled="!migrateStatus.available"
              @click="migrateModalOpen = true"
            >
              Migrate
            </UButton>
          </div>
        </UCard>
      </section>

      <!-- Libraries -->
      <section>
        <div class="mb-4">
          <h2 class="text-base font-semibold text-white flex items-center gap-2">
            <UIcon name="i-lucide-library" class="w-4 h-4 text-primary-500" />
            Libraries
          </h2>
          <p class="text-sm text-neutral-500 mt-0.5">
            Choose which {{ server.name }} libraries to include when importing
          </p>
        </div>
        <UCard variant="soft" class="bg-[#282828] border-neutral-700/50">
          <!-- Not configured -->
          <div
            v-if="libraryStatus === 'not_configured'"
            class="flex items-center gap-2 text-sm text-neutral-500"
          >
            <UIcon name="i-lucide-info" class="w-4 h-4 shrink-0" />
            Configure your {{ server.name }} server URL and {{ env.serverTokenLabel.toLowerCase() }}
            above to manage libraries.
          </div>

          <!-- Loading -->
          <div
            v-else-if="libraryStatus === 'loading'"
            class="flex items-center gap-2 text-sm text-neutral-500"
          >
            <UIcon name="i-lucide-loader-circle" class="w-4 h-4 animate-spin shrink-0" />
            Loading libraries…
          </div>

          <!-- Error -->
          <div
            v-else-if="libraryStatus === 'error'"
            class="flex items-center gap-2 text-sm text-red-400"
          >
            <UIcon name="i-lucide-circle-x" class="w-4 h-4 shrink-0" />
            {{ libraryError }}
          </div>

          <!-- List -->
          <div
            v-else
            class="flex flex-col divide-y divide-neutral-700/50 -mx-4 sm:-mx-6 -my-4 sm:-my-6 overflow-hidden"
          >
            <div
              v-for="lib in libraries"
              :key="lib.key"
              class="flex items-center justify-between px-4 sm:px-6 py-3"
            >
              <div>
                <p class="text-sm font-medium text-white">{{ lib.title }}</p>
                <p class="text-xs text-neutral-500 capitalize">{{ lib.type }}</p>
              </div>
              <USwitch v-model="lib.enabled" class="ml-2 shrink-0" />
            </div>
          </div>
        </UCard>
      </section>

      <!-- Options -->

      <section>
        <div class="mb-4">
          <h2 class="text-base font-semibold text-white flex items-center gap-2">
            <UIcon name="i-lucide-settings-2" class="w-4 h-4 text-primary-500" />
            Options
          </h2>
          <p class="text-sm text-neutral-500 mt-0.5">General application settings</p>
        </div>
        <UCard variant="soft" class="bg-[#282828] border-neutral-700/50">
          <div class="flex flex-col gap-4">
            <div class="flex items-center justify-between">
              <div>
                <p class="text-sm font-medium text-white">Auto-resize images</p>
                <p class="text-xs text-neutral-500">
                  Automatically resize uploaded posters to server-compatible dimensions
                </p>
              </div>
              <USwitch v-model="options.autoResize" class="ml-2 shrink-0" />
            </div>
            <div
              v-if="options.autoResize"
              class="flex items-center justify-between pt-3 border-t border-neutral-700/50"
            >
              <div>
                <p class="text-sm font-medium text-white">Target width</p>
                <p class="text-xs text-neutral-500">
                  Images wider than this will be downscaled (height auto-calculated at 2:3 ratio)
                </p>
                <p v-if="validationErrors.resizeWidth" class="text-xs text-red-400 mt-1">
                  {{ validationErrors.resizeWidth }}
                </p>
              </div>
              <UInput
                v-model="options.resizeWidth"
                type="number"
                :min="500"
                class="w-26 ml-2 shrink-0"
                size="sm"
                :color="validationErrors.resizeWidth ? 'error' : undefined"
              />
            </div>
          </div>
        </UCard>
      </section>

      <!-- Authentication (read-only) -->
      <section>
        <div class="mb-4">
          <h2 class="text-base font-semibold text-white flex items-center gap-2">
            <UIcon name="i-lucide-shield" class="w-4 h-4 text-primary-500" />
            Authentication
          </h2>
          <p class="text-sm text-neutral-500 mt-0.5">
            Configured via environment variables
            <UBadge label="Read-only" color="neutral" variant="soft" size="xs" class="ml-2" />
          </p>
        </div>
        <UCard variant="soft" class="bg-[#282828] border-neutral-700/50">
          <div class="flex flex-col gap-4">
            <div class="flex items-center justify-between">
              <div>
                <p class="text-sm font-medium text-white">Login protection</p>
                <p class="text-xs text-neutral-500">
                  Set via <code class="text-primary-400">AUTH_ENABLED</code> environment variable
                </p>
              </div>
              <UBadge
                :label="env.authEnabled ? 'Enabled' : 'Disabled'"
                :color="env.authEnabled ? 'success' : 'neutral'"
                variant="soft"
              />
            </div>
            <template v-if="env.authEnabled">
              <USeparator />
              <div class="flex flex-col gap-1">
                <span class="text-xs font-medium text-neutral-400">Username</span>
                <div
                  class="flex items-center gap-2 px-3 py-2 rounded-lg bg-neutral-800/60 border border-neutral-700/50"
                >
                  <UIcon name="i-lucide-user" class="w-4 h-4 text-neutral-500 shrink-0" />
                  <span class="text-sm text-neutral-300 font-mono select-text">
                    {{ env.authUser || "" }}
                  </span>
                </div>
              </div>
              <div class="flex flex-col gap-1">
                <span class="text-xs font-medium text-neutral-400">Password</span>
                <div
                  class="flex items-center gap-2 px-3 py-2 rounded-lg bg-neutral-800/60 border border-neutral-700/50"
                >
                  <UIcon name="i-lucide-lock" class="w-4 h-4 text-neutral-500 shrink-0" />
                  <span class="text-sm text-neutral-300 font-mono">{{
                    env.authPassSet ? "••••••••" : ""
                  }}</span>
                  <UBadge
                    v-if="env.authPassSet"
                    label="Set"
                    color="success"
                    variant="soft"
                    size="xs"
                    class="ml-auto"
                  />
                </div>
              </div>
            </template>
          </div>
        </UCard>
      </section>

      <!-- Save -->
      <div class="flex justify-end pt-2">
        <UButton :loading="saving" icon="i-lucide-save" size="lg" @click="save">
          Save changes
        </UButton>
      </div>

      <p class="text-center text-xs text-neutral-600 pb-6">v{{ version }}</p>
    </div>

    <MigratePostersModal v-model:open="migrateModalOpen" @migrated="onMigrated" />
  </div>
</template>
