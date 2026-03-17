<script setup lang="ts">
import { ref, watch } from "vue"

const props = defineProps<{
  src: string
  clearable?: boolean
}>()

defineEmits<{ clear: [] }>()

const hasError = ref(false)

watch(
  () => props.src,
  () => {
    hasError.value = false
  }
)
</script>

<template>
  <div class="flex justify-center mb-4">
    <div class="relative w-40 aspect-[2/3] rounded-lg overflow-hidden bg-neutral-800">
      <img
        v-if="!hasError"
        :src="src"
        alt="Preview"
        class="w-full h-full object-cover"
        @error="hasError = true"
      />
      <div
        v-else
        class="w-full h-full flex flex-col items-center justify-center gap-2 text-neutral-500"
      >
        <UIcon name="i-lucide-image-off" class="w-6 h-6" />
        <p class="text-xs text-center px-2">Unable to load image</p>
      </div>
      <UButton
        v-if="clearable"
        icon="i-lucide-x"
        size="xs"
        color="neutral"
        variant="solid"
        class="absolute top-2 right-2 transition-transform hover:scale-110"
        @click="$emit('clear')"
      />
    </div>
  </div>
</template>
