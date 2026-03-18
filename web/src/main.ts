import "./assets/main.css"
import { createApp } from "vue"
import { createPinia } from "pinia"
import { createRouter, createWebHistory } from "vue-router"
import ui from "@nuxt/ui/vue-plugin"
import App from "./App.vue"
import LibraryPage from "./pages/LibraryPage.vue"
import LoginPage from "./pages/LoginPage.vue"
import SettingsPage from "./pages/SettingsPage.vue"
import { useAuthStore } from "./stores/useAuthStore"

const app = createApp(App)

const pinia = createPinia()
app.use(pinia)

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", component: LibraryPage },
    { path: "/login", component: LoginPage },
    { path: "/settings", component: SettingsPage },
  ],
})

router.beforeEach(async (to) => {
  const authStore = useAuthStore()
  await authStore.check()

  if (to.path === "/login") {
    if (!authStore.authEnabled || authStore.authenticated) return { path: "/", replace: true }
    return true
  }

  if (authStore.authEnabled && !authStore.authenticated) return { path: "/login", replace: true }
  return true
})

app.use(router)
app.use(ui)
app.mount("#app")
