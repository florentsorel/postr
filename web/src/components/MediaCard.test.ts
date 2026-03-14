import { render, screen } from "@testing-library/vue"
import { userEvent } from "@testing-library/user-event"
import { describe, it, expect } from "vitest"
import MediaCard from "./MediaCard.vue"

const stubs = {
  UIcon: { template: "<span />" },
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

    await userEvent.click(screen.getByText("Change poster"))
    expect(emitted("changePoster")).toHaveLength(1)
  })

  it("emits sendToPlex when Send to Plex is clicked", async () => {
    const { emitted } = render(MediaCard, {
      props: { title: "Inception", type: "movie" },
      global: { stubs },
    })

    await userEvent.click(screen.getByText("Send to Plex"))
    expect(emitted("sendToPlex")).toHaveLength(1)
  })

  it("emits getFromPlex when Get from Plex is clicked", async () => {
    const { emitted } = render(MediaCard, {
      props: { title: "Inception", type: "movie" },
      global: { stubs },
    })

    await userEvent.click(screen.getByText("Get from Plex"))
    expect(emitted("getFromPlex")).toHaveLength(1)
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
})
