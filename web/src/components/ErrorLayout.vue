<script setup lang="ts">
defineProps<{
  code: number
  message?: string
}>()
</script>

<template>
  <div class="min-h-screen bg-[#1f1f1f] text-white flex flex-col items-center justify-center gap-4">
    <UIcon name="i-lucide-server-off" class="w-12 h-12 text-neutral-600" />
    <div class="text-center">
      <p class="text-5xl font-bold text-neutral-500">{{ code }}</p>
      <p class="text-base text-neutral-400 mt-2">
        {{ message ?? "An unexpected error occurred." }}
      </p>
    </div>
    <UButton variant="ghost" color="neutral" icon="i-lucide-refresh-cw" @click="$router.go(0)">
      Retry
    </UButton>
  </div>
</template>
