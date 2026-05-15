import { onMounted, ref } from 'vue'

export function useAsyncData<T>(loader: () => Promise<T>) {
  const data = ref<T | null>(null)
  const isLoading = ref(true)
  const error = ref<string | null>(null)

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

  return {
    data,
    isLoading,
    error,
    reload,
  }
}
