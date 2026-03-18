import { defineStore } from "pinia"
import { ref } from "vue"

export const useAuthStore = defineStore("auth", () => {
  const authEnabled = ref(false)
  const authenticated = ref(false)
  const checked = ref(false)

  async function check(): Promise<void> {
    if (checked.value) return
    checked.value = true
    try {
      const res = await fetch("/api/auth/check")
      if (res.ok) {
        const data = await res.json()
        authEnabled.value = data.authEnabled
        authenticated.value = data.authenticated
      }
    } catch {
      authenticated.value = false
    }
  }

  async function logout(): Promise<void> {
    await fetch("/api/auth/logout", { method: "POST" })
    authenticated.value = false
    checked.value = false
  }

  return { authEnabled, authenticated, checked, check, logout }
})
