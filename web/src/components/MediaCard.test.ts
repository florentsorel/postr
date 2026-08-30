import { render, screen, fireEvent, createEvent } from "@testing-library/vue"
import { userEvent } from "@testing-library/user-event"
import { describe, it, expect } from "vitest"
import MediaCard from "./MediaCard.vue"
import { useServerStore } from "@/stores/useServerStore"

function drop(element: Element, file: File) {
  const event = createEvent.drop(element)
  Object.defineProperty(event, "dataTransfer", {
    value: { files: [file], items: [{ type: file.type, kind: "file" }] },
  })
  return fireEvent(element, event)
}

const stubs = {
  UIcon: { template: "<span />" },
  UTooltip: { template: "<slot />" },
}

describe("MediaCard", () => {
  it("renders title, badge and year", () => {
    render(MediaCard, {
      props: { title: "Inception", type: "movie", year: 2010 },
      global: { stubs },
    })

    expect(screen.getByText("Inception")).toBeInTheDocument()
    expect(screen.getByText("Movie")).toBeInTheDocument()
    expect(screen.getByText("2010")).toBeInTheDocument()
  })

  it("omits year when not provided", () => {
    render(MediaCard, {
      props: { title: "Christopher Nolan", type: "collection" },
      global: { stubs },
    })

    expect(screen.queryByText(/\d{4}/)).not.toBeInTheDocument()
  })

  it("shows poster image when thumb is provided", () => {
    render(MediaCard, {
      props: { title: "Inception", type: "movie", thumb: "/posters/1.jpg" },
      global: { stubs },
    })

    const img = screen.getByRole("img", { name: "Inception" })
    expect(img).toHaveAttribute("src", "/posters/1.jpg")
  })

  it("shows fallback icon when no thumb", () => {
    render(MediaCard, {
      props: { title: "Inception", type: "movie" },
      global: { stubs },
    })

    expect(screen.queryByRole("img")).not.toBeInTheDocument()
    expect(screen.getByTestId("poster-fallback")).toBeInTheDocument()
  })

  it("emits changePoster when Change poster is clicked", async () => {
    const { emitted } = render(MediaCard, {
      props: { title: "Inception", type: "movie" },
      global: { stubs },
    })

    await userEvent.click(screen.getAllByText("Change poster")[0])
    expect(emitted("changePoster")).toHaveLength(1)
  })

  it("hides Send to server when not in queue", () => {
    render(MediaCard, {
      props: { title: "Inception", type: "movie", inQueue: false },
      global: { stubs },
    })

    expect(screen.queryByText("Send to Plex")).not.toBeInTheDocument()
  })

  it("emits sendToServer when Send to server is clicked and inQueue", async () => {
    const { emitted } = render(MediaCard, {
      props: { title: "Inception", type: "movie", inQueue: true },
      global: { stubs },
    })

    await userEvent.click(screen.getAllByText("Send to Plex")[0])
    expect(emitted("sendToServer")).toHaveLength(1)
  })

  it("hides Get from server when not locally modified", () => {
    render(MediaCard, {
      props: { title: "Inception", type: "movie", locallyModified: false },
      global: { stubs },
    })

    expect(screen.queryByText("Get from Plex")).not.toBeInTheDocument()
  })

  it("emits getFromServer when Get from server is clicked and locallyModified", async () => {
    const { emitted } = render(MediaCard, {
      props: { title: "Inception", type: "movie", locallyModified: true },
      global: { stubs },
    })

    await userEvent.click(screen.getAllByText("Get from Plex")[0])
    expect(emitted("getFromServer")).toHaveLength(1)
  })

  it.each([
    { type: "movie" as const, label: "Movie" },
    { type: "show" as const, label: "TV Series" },
    { type: "season" as const, label: "Season" },
    { type: "collection" as const, label: "Collection" },
  ])("renders correct badge label for type $type", ({ type, label }) => {
    render(MediaCard, {
      props: { title: "Test", type },
      global: { stubs },
    })

    expect(screen.getByText(label)).toBeInTheDocument()
  })

  it("shows Uploading overlay when uploading prop is true", () => {
    render(MediaCard, {
      props: { title: "Inception", type: "movie", thumb: "/posters/1.jpg", uploading: true },
      global: { stubs },
    })

    expect(screen.getByText("Uploading…")).toBeInTheDocument()
  })

  describe("drag and drop", () => {
    it("emits dropFile when a valid image is dropped", async () => {
      const { emitted } = render(MediaCard, {
        props: { title: "Inception", type: "movie", thumb: "/posters/1.jpg" },
        global: { stubs },
      })

      const posterDiv = screen.getByRole("img", { name: "Inception" }).parentElement!
      await drop(posterDiv, new File([""], "poster.jpg", { type: "image/jpeg" }))

      expect(emitted("dropFile")).toHaveLength(1)
    })

    it.each(["image/jpeg", "image/png", "image/webp"])(
      "emits dropFile for allowed type %s",
      async (mimeType) => {
        const { emitted } = render(MediaCard, {
          props: { title: "Inception", type: "movie", thumb: "/posters/1.jpg" },
          global: { stubs },
        })

        const posterDiv = screen.getByRole("img", { name: "Inception" }).parentElement!
        await drop(posterDiv, new File([""], "poster", { type: mimeType }))

        expect(emitted("dropFile")).toHaveLength(1)
      }
    )

    it("does not emit dropFile for disallowed file types", async () => {
      const { emitted } = render(MediaCard, {
        props: { title: "Inception", type: "movie", thumb: "/posters/1.jpg" },
        global: { stubs },
      })

      const posterDiv = screen.getByRole("img", { name: "Inception" }).parentElement!
      await drop(posterDiv, new File([""], "anim.gif", { type: "image/gif" }))

      expect(emitted("dropFile")).toBeUndefined()
    })

    it("does not emit dropFile when card is orphaned", async () => {
      const { emitted } = render(MediaCard, {
        props: { title: "Inception", type: "movie", thumb: "/posters/1.jpg", isOrphan: true },
        global: { stubs },
      })

      const posterDiv = screen.getByRole("img", { name: "Inception" }).parentElement!
      await drop(posterDiv, new File([""], "poster.jpg", { type: "image/jpeg" }))

      expect(emitted("dropFile")).toBeUndefined()
    })
  })

  it("labels the server actions with the configured server name", () => {
    useServerStore().name = "Jellyfin"

    render(MediaCard, {
      props: { title: "Inception", type: "movie", inQueue: true, locallyModified: true },
      global: { stubs },
    })

    expect(screen.getAllByText("Send to Jellyfin")[0]).toBeInTheDocument()
    expect(screen.getAllByText("Get from Jellyfin")[0]).toBeInTheDocument()
    expect(screen.queryByText("Send to Plex")).not.toBeInTheDocument()
  })
})
