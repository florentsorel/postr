import "@testing-library/jest-dom"
import { vi } from "vitest"
import { config } from "@vue/test-utils"
import { createRouter, createMemoryHistory } from "vue-router"
import ui from "@nuxt/ui/vue-plugin"

// Nuxt UI fetches SVG icons at runtime — stub fetch to avoid happy-dom
// AbortError noise when the test environment tears down.
vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, json: async () => ({}) }))

// happy-dom does not implement IntersectionObserver — stub it as a class.
vi.stubGlobal(
  "IntersectionObserver",
  class {
    observe = vi.fn()
    unobserve = vi.fn()
    disconnect = vi.fn()
  }
)

// Stub URL static methods used when previewing uploaded files.
// Only patch the static methods — stubbing the whole URL class breaks new URL().
URL.createObjectURL = vi.fn(() => "blob:mock")
URL.revokeObjectURL = vi.fn()

// Nuxt UI's color-mode plugin calls localStorage.getItem via VueUse.
// Provide a minimal stub so it does not throw in happy-dom.
vi.stubGlobal("localStorage", {
  getItem: vi.fn(() => null),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn(),
})

// Provide a minimal router globally so all tests have the injection available.
const router = createRouter({
  history: createMemoryHistory(),
  routes: [{ path: "/", component: { template: "<div />" } }],
})

// Register Nuxt UI components + router globally.
// This avoids individual vi.mock() calls for every component and ensures
// virtual Nuxt modules (#imports, #build/ui/*) are properly resolved.
config.global.plugins = [router, ui]

// UTooltip requires a TooltipProvider context (provided by UApp) which is
// not present in unit tests — stub it globally to render its slot content.
config.global.stubs = {
  UTooltip: { template: "<slot />" },
  Tooltip: { template: "<slot />" },
}
