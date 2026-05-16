import { onMounted, onUnmounted, ref } from 'vue'

type UseAsyncDataOptions = {
  pollIntervalMs?: number
}

export function useAsyncData<T>(loader: () => Promise<T>, options: UseAsyncDataOptions = {}) {
  const data = ref<T | null>(null)
  const isLoading = ref(true)
  const error = ref<string | null>(null)
  let pollTimer: number | null = null

  async function reload() {
    isLoading.value = true
    error.value = null
    try {
      data.value = await loader()
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Unknown error'
    } finally {
      isLoading.value = false
    }
  }

  onMounted(reload)

  if (options.pollIntervalMs && options.pollIntervalMs > 0) {
    onMounted(() => {
      pollTimer = window.setInterval(() => {
        void reload()
      }, options.pollIntervalMs)
    })
    onUnmounted(() => {
      if (pollTimer !== null) {
        window.clearInterval(pollTimer)
      }
    })
  }

  return {
    data,
    isLoading,
    error,
    reload,
  }
}
