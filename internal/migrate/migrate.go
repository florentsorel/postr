// Package migrate pairs the items of two media servers so artwork imported from
// one can be carried over to the other.
//
// The pairing is pure: it takes two item lists and returns the correspondence
// between them, with no network or filesystem access. Everything that can go
// wrong with a match is therefore reproducible in a test.
package migrate

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/florentsorel/postr/internal/mediaserver"
)

// How a pair was established, in descending order of confidence.
const (
	// ByExternalID means both servers agree on a TMDB/IMDB/TVDB identifier.
	// These matches are as good as certain.
	ByExternalID = "external-id"
	// ByTitle means the items were paired on their normalized title (and year,
	// when both sides have one). Used for collections, which no external
	// database tracks, and for items either server failed to identify.
	ByTitle = "title"
)

// Match pairs one source item with the target item it was recognised as.
type Match struct {
	SourceID  string
	TargetID  string
	Title     string
	MediaType string
	By        string
}

// Unmatched is a source item that could not be paired, with the reason.
type Unmatched struct {
	SourceID  string
	Title     string
	MediaType string
	Reason    string
}

// Reasons an item was left unpaired.
const (
	ReasonNoCounterpart = "no counterpart found on the target server"
	ReasonAmbiguous     = "several candidates matched — too risky to guess"
)

// Plan is the full correspondence between two item lists.
type Plan struct {
	Matches   []Match
	Unmatched []Unmatched
}

// BuildPlan pairs source items with target items of the same media type.
//
// External identifiers are tried first and win outright. Whatever is left falls
// back to a normalized title, which is the only option for collections. A
// fallback candidate that is not unique on either side is reported as ambiguous
// rather than guessed at: silently writing the wrong artwork over a title is a
// worse outcome than leaving it for the user to handle.
func BuildPlan(mediaType string, source, target []mediaserver.Item) Plan {
	var plan Plan

	// A target is reachable through every source it carries, so a source item
	// that only knows one of them can still find it.
	targetByExternal := indexUniqueMulti(target, func(i mediaserver.Item) []string {
		return i.MatchKeys(mediaType)
	})
	targetByTitle := indexUniqueMulti(target, func(i mediaserver.Item) []string {
		if k := titleKey(mediaType, i); k != "" {
			return []string{k}
		}
		return nil
	})

	claimed := make(map[string]bool, len(source))

	// Pass 1: external identifiers.
	remaining := make([]mediaserver.Item, 0, len(source))
	for _, item := range source {
		var found *candidate
		for _, key := range item.MatchKeys(mediaType) {
			match, ok := targetByExternal[key]
			if ok && !match.ambiguous && !claimed[match.item.ID] {
				found = &match
				break
			}
		}
		if found == nil {
			remaining = append(remaining, item)
			continue
		}
		claimed[found.item.ID] = true
		plan.Matches = append(plan.Matches, Match{
			SourceID:  item.ID,
			TargetID:  found.item.ID,
			Title:     item.Title,
			MediaType: mediaType,
			By:        ByExternalID,
		})
	}

	// Pass 2: normalized titles, for whatever pass 1 could not identify.
	sourceTitleCount := make(map[string]int, len(remaining))
	for _, item := range remaining {
		sourceTitleCount[titleKey(mediaType, item)]++
	}

	for _, item := range remaining {
		key := titleKey(mediaType, item)
		match, ok := targetByTitle[key]
		switch {
		case !ok:
			plan.Unmatched = append(plan.Unmatched, unmatched(item, mediaType, ReasonNoCounterpart))
		case match.ambiguous || sourceTitleCount[key] > 1:
			// Either several targets answer to this title, or several source
			// items do — both make the pairing a coin flip.
			plan.Unmatched = append(plan.Unmatched, unmatched(item, mediaType, ReasonAmbiguous))
		case claimed[match.item.ID]:
			// An external-id match already took this target; trust that one.
			plan.Unmatched = append(plan.Unmatched, unmatched(item, mediaType, ReasonNoCounterpart))
		default:
			claimed[match.item.ID] = true
			plan.Matches = append(plan.Matches, Match{
				SourceID:  item.ID,
				TargetID:  match.item.ID,
				Title:     item.Title,
				MediaType: mediaType,
				By:        ByTitle,
			})
		}
	}

	return plan
}

func unmatched(item mediaserver.Item, mediaType, reason string) Unmatched {
	return Unmatched{SourceID: item.ID, Title: item.Title, MediaType: mediaType, Reason: reason}
}

// candidate remembers whether a key was claimed by more than one item, so an
// ambiguous key is never silently resolved to whichever came first.
type candidate struct {
	item      mediaserver.Item
	ambiguous bool
}

// indexUniqueMulti indexes each item under every key it answers to. A key two
// different items answer to is flagged ambiguous rather than resolved to
// whichever came first.
func indexUniqueMulti(items []mediaserver.Item, keys func(mediaserver.Item) []string) map[string]candidate {
	index := make(map[string]candidate, len(items))
	for _, item := range items {
		for _, k := range keys(item) {
			if k == "" {
				continue
			}
			if existing, seen := index[k]; seen {
				if existing.item.ID != item.ID {
					existing.ambiguous = true
					index[k] = existing
				}
				continue
			}
			index[k] = candidate{item: item}
		}
	}
	return index
}

// titleKey builds the fallback identity of an item. The year is only part of it
// when present, since collections never carry one; seasons add their number so
// two seasons of the same show stay distinct.
func titleKey(mediaType string, i mediaserver.Item) string {
	title := NormalizeTitle(i.Title)
	if title == "" {
		return ""
	}
	key := mediaType + "|" + title
	if mediaType == mediaserver.TypeSeason {
		return key + "|s" + strconv.Itoa(i.SeasonNumber)
	}
	if i.Year != 0 {
		key += "|" + strconv.Itoa(i.Year)
	}
	return key
}

var (
	// trailingYear matches the "(2021)" some libraries append to disambiguate a
	// title, which the other server usually omits.
	trailingYear = regexp.MustCompile(`\s*\((?:19|20)\d{2}\)\s*$`)
	nonAlphaNum  = regexp.MustCompile(`[^\p{L}\p{N}]+`)
)

// NormalizeTitle reduces a title to a form that survives the cosmetic
// differences between two servers' metadata agents: case, punctuation, spacing
// and a trailing disambiguation year.
func NormalizeTitle(title string) string {
	title = strings.ToLower(strings.TrimSpace(title))
	title = trailingYear.ReplaceAllString(title, "")
	title = nonAlphaNum.ReplaceAllString(title, " ")
	return strings.TrimSpace(title)
}
