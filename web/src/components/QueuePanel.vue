<script setup lang="ts">
import { ref } from "vue"
import { useQueueStore } from "@/stores/useQueueStore"
import MediaItemRow from "./MediaItemRow.vue"

defineProps<{ open: boolean }>()
const emit = defineEmits<{
  "update:open": [value: boolean]
  restored: [payload: { ratingKey: string; thumb: string | null }]
}>()

const queue = useQueueStore()
const pushingAll = ref(false)
const pushingItem = ref<string | null>(null)
const error = ref<string | null>(null)

async function pushAll() {
  pushingAll.value = true
  error.value = null
  try {
    await queue.pushAll()
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Push failed"
  } finally {
    pushingAll.value = false
  }
}

async function pushOne(ratingKey: string) {
  pushingItem.value = ratingKey
  error.value = null
  try {
    await queue.pushOne(ratingKey)
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Push failed"
  } finally {
    pushingItem.value = null
  }
}
</script>

<template>
  <UModal
    :open="open"
    class="select-none"
    title="Pending posters"
    :description="`${queue.count} item${queue.count !== 1 ? 's' : ''} waiting to be pushed to Plex`"
    @update:open="$emit('update:open', $event)"
  >
    <template #body>
      <div
        v-if="queue.items.length === 0"
        class="flex flex-col items-center justify-center py-10 gap-3 text-center"
      >
        <UIcon name="i-lucide-check-circle" class="w-10 h-10 text-green-500" />
        <p class="text-neutral-400 text-sm">Nothing pending — all posters are in sync.</p>
      </div>
      <div v-else class="flex flex-col divide-y divide-neutral-800 max-h-96 overflow-y-auto">
        <MediaItemRow
          v-for="item in queue.items"
          :key="item.ratingKey"
          :thumb="item.thumb"
          :title="item.title"
          :type="item.type"
          :season-number="item.seasonNumber"
        >
          <div class="flex items-center gap-2 shrink-0">
            <UTooltip text="Send to Plex">
              <UButton
                icon="i-lucide-upload"
                size="xs"
                variant="outline"
                color="neutral"
                :loading="pushingItem === item.ratingKey"
                :disabled="pushingAll"
                @click="pushOne(item.ratingKey)"
              />
            </UTooltip>
            <UTooltip text="Remove from queue and restore poster from Plex">
              <UButton
                icon="i-lucide-x"
                size="xs"
                variant="ghost"
                color="neutral"
                :disabled="pushingAll || pushingItem === item.ratingKey"
                @click="
                  async () => {
                    const { thumb } = await queue.removeItem(item.ratingKey)
                    emit('restored', { ratingKey: item.ratingKey, thumb })
                  }
                "
              />
            </UTooltip>
          </div>
        </MediaItemRow>
      </div>
    </template>
    <template #footer>
      <div class="flex flex-col gap-3 w-full">
        <p v-if="error" class="text-xs text-red-400 px-1">{{ error }}</p>
        <div class="flex justify-between">
          <UButton
            label="Close"
            color="neutral"
            variant="ghost"
            @click="$emit('update:open', false)"
          />
          <UButton
            v-if="queue.count > 0"
            label="Push all to Plex"
            icon="i-lucide-upload"
            :loading="pushingAll"
            @click="pushAll"
          />
        </div>
      </div>
    </template>
  </UModal>
</template>
