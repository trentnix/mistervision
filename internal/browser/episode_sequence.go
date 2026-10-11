package browser

import (
	"context"
	"errors"

	"mistervision/internal/media"
)

// episodeSequence keeps the completed episode's list and its chronological
// successor. A nil next means the server has no later episode in this series.
// Views own their navigation state and borrow immutable catalog item slices.
type episodeSequence struct {
	episodes View
	next     *mediaSelection
}

// findEpisodeSequence follows the same season and episode order as browsing.
// It reuses a loaded episode page, crosses page and season boundaries, and never
// substitutes an unrelated Continue Watching recommendation or skips watched items.
func findEpisodeSequence(ctx context.Context, catalog itemLister, item media.Item, parent View, rows int) (episodeSequence, error) {
	var result episodeSequence
	series := item.SeriesID
	if series == "" && parent.Location.Kind == "episodes" {
		series = parent.Location.SeriesID
	}
	currentSeason := ""
	if parent.Location.Kind == "episodes" && parent.Item() != nil && parent.Item().ID == item.ID {
		result.episodes = parent
		currentSeason = parent.Location.ParentID
		next, episode, err := adjacentMedia(ctx, catalog, parent, "Episode", 1, rows)
		if err != nil {
			return result, err
		}
		if episode != nil {
			result.next = &mediaSelection{parent: next, item: *episode}
			return result, nil
		}
	}
	if series == "" {
		return result, errors.New("episode has no series identifier")
	}
	seasons := View{Location: media.Location{Kind: "seasons", SeriesID: series}}
	found := false
	for {
		if err := loadEpisodePage(ctx, catalog, &seasons); err != nil {
			return result, err
		}
		for _, season := range seasons.Page.Items {
			if season.Type != "Season" || season.ID == "" {
				continue
			}
			if currentSeason != "" && !found {
				found = season.ID == currentSeason
				continue
			}
			// From Continue Watching, the season number can narrow the search.
			// The episode ID still has to match before accepting a successor.
			if !found && item.ParentIndexNumber != nil && season.IndexNumber != nil && *item.ParentIndexNumber != *season.IndexNumber {
				continue
			}
			title := season.Name
			if item.SeriesName != "" {
				title = item.SeriesName + " / " + title
			}
			episodes := View{Title: title, Location: media.Location{Kind: "episodes", ParentID: season.ID, SeriesID: series, Collection: parent.Location.Collection}}
			for {
				if err := loadEpisodePage(ctx, catalog, &episodes); err != nil {
					return result, err
				}
				for i, episode := range episodes.Page.Items {
					if episode.Type != "Episode" || episode.ID == "" {
						continue
					}
					episodes.Selected = i
					episodes.Target = episodes.Start + i
					episodes.centerSelection(rows)
					if episode.ID == item.ID {
						result.episodes = episodes
						found = true
						continue
					}
					if found {
						result.next = &mediaSelection{parent: episodes, item: episode}
						return result, nil
					}
				}
				if !episodes.More() {
					break
				}
			}
		}
		if !seasons.More() {
			break
		}
	}
	if !found {
		return result, errors.New("completed episode not found in series")
	}
	return result, nil
}

// loadEpisodePage advances a private cursor. Reject a repeated page so a server
// that ignores pagination cannot keep an optional lookup running indefinitely.
func loadEpisodePage(ctx context.Context, catalog itemLister, view *View) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	start := view.Start + len(view.Page.Items)
	page, err := catalog.List(ctx, view.Location, start, PageSize)
	if err != nil {
		return err
	}
	if len(page.Items) > 0 && len(view.Page.Items) > 0 && page.Items[0].ID == view.Page.Items[0].ID {
		return errors.New("episode listing did not advance")
	}
	view.Start, view.Page = start, page
	// An empty response ends a listing even when its total is stale.
	if len(page.Items) == 0 {
		view.Page.TotalRecordCount = &start
	}
	return nil
}
