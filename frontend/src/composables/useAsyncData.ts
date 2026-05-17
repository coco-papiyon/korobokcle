import { onMounted, onUnmounted, ref } from 'vue'

type UseAsyncDataOptions<T> = {
  pollIntervalMs?: number
  mergeData?: (current: T | null, incoming: T) => T
}

type ReloadOptions = {
  silent?: boolean
}

export function useAsyncData<T>(loader: () => Promise<T>, options: UseAsyncDataOptions<T> = {}) {
  const data = ref<T | null>(null)
  const isLoading = ref(true)
  const isRefreshing = ref(false)
  const error = ref<string | null>(null)
  let pollTimer: number | null = null

  async function reload(reloadOptions: ReloadOptions = {}) {
    const silent = reloadOptions.silent ?? data.value !== null
    if (silent) {
      isRefreshing.value = true
    } else {
      isLoading.value = true
      error.value = null
    }
    try {
      const next = await loader()
      data.value = options.mergeData ? options.mergeData(data.value, next) : next
      if (silent) {
        error.value = null
      }
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Unknown error'
    } finally {
      if (silent) {
        isRefreshing.value = false
      } else {
        isLoading.value = false
      }
    }
  }

  onMounted(reload)

  if (options.pollIntervalMs && options.pollIntervalMs > 0) {
    onMounted(() => {
      pollTimer = window.setInterval(() => {
        void reload({ silent: true })
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
    isRefreshing,
    error,
    reload,
  }
}
