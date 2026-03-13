<script setup lang="ts">
import { reactive, ref } from "vue"

const state = reactive({
  username: "",
  password: "",
})

const error = ref("")
const loading = ref(false)
const showPassword = ref(false)

async function onSubmit() {
  error.value = ""
  loading.value = true

  try {
    const res = await fetch("/api/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(state),
    })

    if (!res.ok) {
      error.value = "Invalid username or password"
      return
    }

    window.location.href = "/"
  } catch {
    error.value = "Unable to connect to server"
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen bg-[#1f1f1f] flex items-center justify-center px-4">
    <div class="w-full max-w-sm">
      <!-- Logo -->
      <div class="flex flex-col items-center mb-8 gap-3">
        <div
          class="w-14 h-14 rounded-2xl bg-primary-500 flex items-center justify-center shadow-lg"
        >
          <UIcon name="i-lucide-image" class="w-8 h-8 text-white" />
        </div>
        <div class="text-center">
          <h1 class="text-2xl font-bold text-white tracking-wide">Postr</h1>
          <p class="text-sm text-neutral-400 mt-1">Plex Poster Manager</p>
        </div>
      </div>

      <!-- Card -->
      <UCard variant="soft" class="bg-[#282828] border-neutral-700/50">
        <form class="flex flex-col gap-5" @submit.prevent="onSubmit">
          <UFormField label="Username" name="username">
            <UInput
              v-model="state.username"
              placeholder="Enter your username"
              icon="i-lucide-user"
              size="lg"
              class="w-full"
              autocomplete="username"
              :disabled="loading"
            />
          </UFormField>

          <UFormField label="Password" name="password">
            <UInput
              v-model="state.password"
              :type="showPassword ? 'text' : 'password'"
              placeholder="Enter your password"
              icon="i-lucide-lock"
              size="lg"
              class="w-full"
              autocomplete="current-password"
              :disabled="loading"
            >
              <template #trailing>
                <UButton
                  :icon="showPassword ? 'i-lucide-eye-off' : 'i-lucide-eye'"
                  variant="ghost"
                  color="neutral"
                  size="sm"
                  tabindex="-1"
                  @click="showPassword = !showPassword"
                />
              </template>
            </UInput>
          </UFormField>

          <!-- Error -->
          <p v-if="error" class="text-sm text-red-400 flex items-center gap-2">
            <UIcon name="i-lucide-circle-alert" class="w-4 h-4 shrink-0" />
            {{ error }}
          </p>

          <UButton
            type="submit"
            size="lg"
            block
            :loading="loading"
            :disabled="!state.username || !state.password"
            class="mt-1"
          >
            Sign in
          </UButton>
        </form>
      </UCard>
    </div>
  </div>
</template>
