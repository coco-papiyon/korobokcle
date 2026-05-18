import { onMounted, onUnmounted, ref, unref, watch } from 'vue'

type UseAsyncDataOptions<T> = {
  pollIntervalMs?: number | { value: number | null | undefined } | (() => number | null | undefined)
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

  function clearPollTimer() {
    if (pollTimer !== null) {
      window.clearInterval(pollTimer)
      pollTimer = null
    }
  }

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

  watch(
    () => {
      if (typeof options.pollIntervalMs === 'function') {
        return options.pollIntervalMs()
      }
      return unref(options.pollIntervalMs)
    },
    (intervalMs) => {
      clearPollTimer()
      if (intervalMs && intervalMs > 0) {
        pollTimer = window.setInterval(() => {
          void reload({ silent: true })
        }, intervalMs)
      }
    },
    { immediate: true },
  )

  onUnmounted(clearPollTimer)

  return {
    data,
    isLoading,
    isRefreshing,
    error,
    reload,
  }
}
