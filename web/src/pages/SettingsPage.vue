<script setup lang="ts">
import { ref, onMounted } from "vue"
import { VueDraggable } from "vue-draggable-plus"

const toast = useToast()
const saving = ref(false)

interface Source {
  id: string
  label: string
  description: string
  enabled: boolean
}

// Read-only from env vars — fetched from backend
const env = ref({
  plexUrl: "",
  plexToken: "",
  authEnabled: false,
  authUser: "",
})

// Editable — stored in SQLite, order matters
const sources = ref<Source[]>([
  { id: "tmdb", label: "TMDB", description: "The Movie Database", enabled: true },
  { id: "tvdb", label: "TVDB", description: "The TV Database", enabled: true },
  { id: "fanart", label: "Fanart.tv", description: "Community artwork", enabled: false },
])

const options = ref({ autoResize: true })

onMounted(async () => {
  try {
    const res = await fetch("/api/settings")
    if (res.ok) {
      const data = await res.json()
      env.value.plexUrl = data.plex_url ?? ""
      env.value.plexToken = data.plex_token ?? ""
      env.value.authEnabled = data.auth_enabled ?? false
      env.value.authUser = data.auth_user ?? ""
      options.value.autoResize = data.auto_resize ?? true

      if (Array.isArray(data.sources)) {
        sources.value = data.sources
      }
    }
  } catch {
    // settings will remain at their defaults
  }
})

async function save() {
  saving.value = true
  try {
    await fetch("/api/settings", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ sources: sources.value, options: options.value }),
    })
    toast.add({ title: "Settings saved", color: "primary", icon: "i-lucide-check-circle" })
  } catch {
    toast.add({ title: "Failed to save settings", color: "error", icon: "i-lucide-circle-x" })
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="min-h-screen bg-[#1f1f1f] text-white">
    <!-- Header -->
    <header class="border-b border-neutral-800 px-6 py-4 flex items-center gap-4">
      <UButton to="/" icon="i-lucide-arrow-left" variant="ghost" color="neutral" size="sm" />
      <div class="flex items-center gap-2">
        <div class="w-7 h-7 rounded-lg bg-primary-500 flex items-center justify-center">
          <UIcon name="i-lucide-image" class="w-4 h-4 text-white" />
        </div>
        <span class="font-semibold text-white">Postr</span>
      </div>
      <USeparator orientation="vertical" class="h-5" />
      <h1 class="text-sm font-medium text-neutral-300">Settings</h1>
      <div class="ml-auto">
        <UButton :loading="saving" icon="i-lucide-save" size="sm" @click="save">
          Save changes
        </UButton>
      </div>
    </header>

    <!-- Content -->
    <div class="max-w-2xl mx-auto px-6 py-10 flex flex-col gap-8">
      <!-- Plex Server (read-only) -->
      <section>
        <div class="mb-4">
          <h2 class="text-base font-semibold text-white flex items-center gap-2">
            <UIcon name="i-lucide-server" class="w-4 h-4 text-primary-500" />
            Plex Server
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
                <span class="text-sm text-neutral-300 font-mono">
                  {{ env.plexUrl || "" }}
                </span>
              </div>
              <p v-if="!env.plexUrl" class="text-xs text-neutral-500">
                Set the <code class="text-neutral-400">PLEX_URL</code> environment variable — e.g.
                <code class="text-neutral-400">http://192.168.1.x:32400</code>
              </p>
            </div>
            <div class="flex flex-col gap-1">
              <span class="text-xs font-medium text-neutral-400">Plex Token</span>
              <div
                class="flex items-center gap-2 px-3 py-2 rounded-lg bg-neutral-800/60 border border-neutral-700/50"
              >
                <UIcon name="i-lucide-key" class="w-4 h-4 text-neutral-500 shrink-0" />
                <span class="text-sm text-neutral-300 font-mono">
                  {{ env.plexToken ? "••••••••••••••••" : "" }}
                </span>
                <UBadge
                  v-if="env.plexToken"
                  label="Set"
                  color="success"
                  variant="soft"
                  size="xs"
                  class="ml-auto"
                />
              </div>
              <p v-if="!env.plexToken" class="text-xs text-neutral-500">
                Set the <code class="text-neutral-400">PLEX_TOKEN</code> environment variable —
                <a
                  href="https://support.plex.tv/articles/204059436-finding-an-authentication-token-x-plex-token/"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="text-primary-400 hover:text-primary-300 underline"
                  >how to find your token</a
                >.
              </p>
            </div>
          </div>
        </UCard>
      </section>

      <!-- Poster Sources -->
      <section>
        <div class="mb-4">
          <h2 class="text-base font-semibold text-white flex items-center gap-2">
            <UIcon name="i-lucide-images" class="w-4 h-4 text-primary-500" />
            Poster Sources
          </h2>
          <p class="text-sm text-neutral-500 mt-0.5">
            Select and reorder sources — the first enabled one is used by default when fetching
            posters
          </p>
        </div>
        <UCard variant="soft" class="bg-[#282828] border-neutral-700/50">
          <VueDraggable
            v-model="sources"
            handle=".drag-handle"
            :animation="150"
            ghost-class="drag-ghost"
            chosen-class="drag-chosen"
            class="flex flex-col divide-y divide-neutral-700/50 -mx-4 sm:-mx-6 -my-4 sm:-my-6 overflow-hidden"
          >
            <div
              v-for="source in sources"
              :key="source.id"
              class="flex items-center gap-3 px-4 sm:px-6 py-3"
            >
              <UIcon
                name="i-lucide-grip-vertical"
                class="drag-handle w-4 h-4 text-neutral-600 cursor-grab active:cursor-grabbing shrink-0"
              />
              <div class="flex-1 min-w-0">
                <p class="text-sm font-medium text-white">{{ source.label }}</p>
                <p class="text-xs text-neutral-500">{{ source.description }}</p>
              </div>
              <USwitch v-model="source.enabled" />
            </div>
          </VueDraggable>
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
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm font-medium text-white">Auto-resize images</p>
              <p class="text-xs text-neutral-500">
                Automatically resize uploaded posters to Plex-compatible dimensions
              </p>
            </div>
            <USwitch v-model="options.autoResize" />
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
                  Set via <code class="text-primary-400">AUTH_ENABLED</code> env var
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
                  <span class="text-sm text-neutral-300 font-mono">
                    {{ env.authUser || "Not set — AUTH_USER" }}
                  </span>
                </div>
              </div>
              <div class="flex flex-col gap-1">
                <span class="text-xs font-medium text-neutral-400">Password</span>
                <div
                  class="flex items-center gap-2 px-3 py-2 rounded-lg bg-neutral-800/60 border border-neutral-700/50"
                >
                  <UIcon name="i-lucide-lock" class="w-4 h-4 text-neutral-500 shrink-0" />
                  <span class="text-sm text-neutral-300 font-mono">••••••••</span>
                  <UBadge label="Set" color="success" variant="soft" size="xs" class="ml-auto" />
                </div>
              </div>
            </template>
          </div>
        </UCard>
      </section>

      <!-- Save -->
      <div class="flex justify-end pt-2 pb-6">
        <UButton :loading="saving" icon="i-lucide-save" size="lg" @click="save">
          Save changes
        </UButton>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Placeholder left behind while dragging */
:deep(.drag-ghost) {
  opacity: 0.3;
  background: transparent;
}

/* The element currently being dragged */
:deep(.drag-chosen) {
  background: color-mix(in srgb, var(--color-plex-500) 8%, #282828);
  border-radius: 0;
  border: 1px solid color-mix(in srgb, var(--color-plex-500) 30%, transparent);
  padding-top: 0.75rem;
  padding-bottom: 0.75rem;
  opacity: 1;
}
</style>
