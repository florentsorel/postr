import { defineStore } from "pinia"
import { ref } from "vue"

type MediaTab = "all" | "movie" | "show" | "season" | "collection" | "orphan"
type SortKey = "title" | "year" | "added"

const SORT_UNAVAILABLE: Record<MediaTab, SortKey[]> = {
  all: ["year"],
  movie: [],
  show: ["year"],
  season: ["year"],
  collection: ["year"],
  orphan: ["title", "year", "added"],
}

const DEFAULT_SORT: SortKey = "added"

export const useLibraryUiStore = defineStore("libraryUi", () => {
  const sortByTab = ref<Partial<Record<MediaTab, SortKey>>>({})

  function getSortForTab(tab: MediaTab): SortKey {
    return sortByTab.value[tab] ?? DEFAULT_SORT
  }

  function setSortForTab(tab: MediaTab, sort: SortKey) {
    sortByTab.value[tab] = sort
  }

  function isSortAvailable(tab: MediaTab, sort: SortKey): boolean {
    return !SORT_UNAVAILABLE[tab].includes(sort)
  }

  return { sortByTab, getSortForTab, setSortForTab, isSortAvailable }
})
