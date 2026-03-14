import "@testing-library/jest-dom"
import { vi } from "vitest"
import { config } from "@vue/test-utils"
import { createRouter, createMemoryHistory } from "vue-router"

// Nuxt UI fetches SVG icons at runtime — stub fetch to avoid happy-dom
// AbortError noise when the test environment tears down.
vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, json: async () => ({}) }))

// Nuxt UI's UButton renders a Link internally that requires Vue Router.
// Provide a minimal router globally so all tests have the injection available.
const router = createRouter({
  history: createMemoryHistory(),
  routes: [{ path: "/", component: { template: "<div />" } }],
})

config.global.plugins = [router]
