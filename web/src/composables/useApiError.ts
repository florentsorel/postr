import { ref } from "vue"

interface ApiError {
  code: number
  message: string
}

const HTTP_MESSAGES: Record<number, string> = {
  502: "Bad Gateway — the backend is unreachable.",
  503: "Service Unavailable — the backend is not ready.",
  504: "Gateway Timeout — the backend took too long to respond.",
}

export function useApiError() {
  const error = ref<ApiError | null>(null)

  function handleResponse(res: Response): boolean {
    if (res.ok) return true
    error.value = {
      code: res.status,
      message: HTTP_MESSAGES[res.status] ?? `Unexpected error (${res.status}).`,
    }
    return false
  }

  function handleException(): void {
    error.value = {
      code: 503,
      message: HTTP_MESSAGES[503],
    }
  }

  return { error, handleResponse, handleException }
}
