/* eslint-disable vue/require-default-prop */
import { render, screen, cleanup, fireEvent, createEvent } from "@testing-library/vue"
import { userEvent } from "@testing-library/user-event"
import { describe, it, expect, vi, afterEach } from "vitest"
import { defineComponent } from "vue"
import ChangePosterModal from "./ChangePosterModal.vue"

// UModal uses Teleport + Reka UI Dialog which breaks in happy-dom.
// Replace it with a simple stub that renders its named slots inline.
// All other Nuxt UI components (UTabs, UButton, UInput…) render for real via
// the vue-plugin registered in src/test/setup.ts — their virtual module
// dependencies (#imports, #build/ui/*) are resolved there.
vi.mock("@nuxt/ui/runtime/components/Modal.vue", () => ({
  default: defineComponent({
    name: "UModal",
    props: {
      open: Boolean,
      title: String,
      description: String,
      dismissible: Boolean,
      ui: Object,
    },
    template: `<div><p v-if="title">{{ title }}</p><p v-if="description">{{ description }}</p><slot name="body" /><slot name="footer" /></div>`,
  }),
}))

afterEach(cleanup)

const defaultItem = {
  id: 1,
  ratingKey: "1",
  title: "Inception",
  type: "movie" as const,
  year: 2010,
}

function renderModal(props: Record<string, unknown> = {}) {
  return render(ChangePosterModal, {
    props: { open: true, item: defaultItem, ...props },
  })
}

describe("ChangePosterModal", () => {
  it("renders item title", () => {
    renderModal()
    expect(screen.getByText("Inception")).toBeInTheDocument()
  })

  it("renders default title when item is null", () => {
    renderModal({ item: null })
    expect(screen.getByText("Change poster")).toBeInTheDocument()
  })

  it("renders year and type in description", () => {
    renderModal()
    expect(screen.getByText("2010 · movie")).toBeInTheDocument()
  })

  it("omits year separator when item has no year", () => {
    renderModal({ item: { id: 1, ratingKey: "1", title: "Marvel", type: "collection" as const } })
    expect(screen.queryByText(/·/)).not.toBeInTheDocument()
    expect(screen.getByText("collection")).toBeInTheDocument()
  })

  it("Apply button is disabled initially", () => {
    renderModal()
    expect(screen.getByRole("button", { name: "Apply" })).toBeDisabled()
  })

  it("Cancel emits update:open false", async () => {
    const { emitted } = renderModal()
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }))
    expect(emitted("update:open")).toEqual([[false]])
  })

  describe("Upload tab — drag and drop", () => {
    function dropOnZone(file: File) {
      const label = screen.getByText(/Drop an image here/).closest("label")!
      const event = createEvent.drop(label)
      Object.defineProperty(event, "dataTransfer", {
        value: { files: [file], items: [{ type: file.type, kind: "file" }] },
      })
      return fireEvent(label, event)
    }

    it("accepts jpeg files on drop and enables Apply", async () => {
      renderModal()
      await dropOnZone(new File([""], "poster.jpg", { type: "image/jpeg" }))
      expect(screen.getByRole("button", { name: "Apply" })).not.toBeDisabled()
    })

    it("rejects gif files on drop and keeps Apply disabled", async () => {
      renderModal()
      await dropOnZone(new File([""], "anim.gif", { type: "image/gif" }))
      expect(screen.getByRole("button", { name: "Apply" })).toBeDisabled()
    })
  })

  describe("From URL tab", () => {
    it("Apply is enabled when a URL is entered", async () => {
      renderModal()
      await userEvent.click(screen.getByRole("tab", { name: "From URL" }))
      await userEvent.type(
        screen.getByPlaceholderText(/https:\/\//),
        "https://example.com/poster.jpg"
      )
      expect(screen.getByRole("button", { name: "Apply" })).not.toBeDisabled()
    })

    it("Apply uploads from URL and closes the modal", async () => {
      vi.mocked(fetch).mockImplementation((url) => {
        if (typeof url === "string" && url.includes("upload-url")) {
          return Promise.resolve({
            ok: true,
            json: async () => ({ ext: "jpg", thumb: "/api/media/1/thumb" }),
          } as Response)
        }
        return Promise.resolve({ ok: false, json: async () => ({}) } as Response)
      })

      const { emitted } = renderModal()
      await userEvent.click(screen.getByRole("tab", { name: "From URL" }))
      await userEvent.type(
        screen.getByPlaceholderText(/https:\/\//),
        "https://example.com/poster.jpg"
      )
      await userEvent.click(screen.getByRole("button", { name: "Apply" }))
      expect(emitted("update:open")).toEqual([[false]])
    })
  })
})
