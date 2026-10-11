package browser

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"mistervision/internal/media"
)

type episodeCatalog struct {
	seasons  []media.Item
	episodes map[string][]media.Item
	calls    []media.Location
}

func (c *episodeCatalog) List(ctx context.Context, loc media.Location, start, limit int) (media.Page, error) {
	if err := ctx.Err(); err != nil {
		return media.Page{}, err
	}
	c.calls = append(c.calls, loc)
	items := c.episodes[loc.ParentID]
	if loc.Kind == "seasons" {
		items = c.seasons
	}
	total := len(items)
	return media.Page{Items: items[min(start, total):min(start+limit, total)], TotalRecordCount: &total}, nil
}

func episodeItem(id string, season, number int) media.Item {
	return media.Item{ID: id, Name: "Episode " + id, Type: "Episode", SeriesID: "series", SeriesName: "Example show", ParentIndexNumber: &season, IndexNumber: &number}
}

func seasonItem(id string, number int) media.Item {
	return media.Item{ID: id, Type: "Season", Name: fmt.Sprintf("Season %d", number), SeriesID: "series", IndexNumber: &number}
}

func episodeView(season string, items []media.Item, selected int) View {
	total := len(items)
	return View{Title: "Example show / " + season, Location: media.Location{Kind: "episodes", SeriesID: "series", ParentID: season}, Page: media.Page{Items: items, TotalRecordCount: &total}, Selected: selected, Target: selected}
}

func TestEpisodeSequenceUsesLoadedOrderIncludingWatchedEpisodes(t *testing.T) {
	items := []media.Item{episodeItem("first", 1, 1), episodeItem("next", 1, 3)}
	items[1].UserData.Played = true
	catalog := &episodeCatalog{}
	parent := episodeView("one", items, 0)
	got, err := findEpisodeSequence(t.Context(), catalog, items[0], parent, 6)
	if err != nil || got.next == nil || got.next.item.ID != "next" || len(catalog.calls) != 0 {
		t.Fatalf("loaded sequence: %+v, %v, calls=%v", got, err, catalog.calls)
	}
	if parent.Selected != 0 || got.episodes.Selected != 0 || got.next.parent.Selected != 1 {
		t.Fatal("lookup moved the completed episode's selection")
	}
}

func TestEpisodeSequenceCrossesPageAndSeasonBoundaries(t *testing.T) {
	items := make([]media.Item, 65)
	for i := range items {
		items[i] = episodeItem(fmt.Sprint(i), 1, i+1)
	}
	catalog := &episodeCatalog{seasons: []media.Item{seasonItem("one", 1), seasonItem("empty", 2), seasonItem("three", 3)}, episodes: map[string][]media.Item{"one": items, "three": {episodeItem("last", 3, 1)}}}
	for _, tc := range []struct {
		selected     int
		want, season string
	}{{63, "64", "one"}, {64, "last", "three"}} {
		parent := episodeView("one", items, tc.selected)
		if tc.selected == 63 {
			parent.Page.Items = items[:64]
		}
		got, err := findEpisodeSequence(t.Context(), catalog, items[tc.selected], parent, 6)
		if err != nil || got.next == nil || got.next.item.ID != tc.want || got.next.parent.Location.ParentID != tc.season || got.episodes.Item().ID != items[tc.selected].ID {
			t.Fatalf("index %d: %+v %v", tc.selected, got, err)
		}
	}
	last := catalog.episodes["three"][0]
	got, err := findEpisodeSequence(t.Context(), catalog, last, episodeView("three", []media.Item{last}, 0), 6)
	if err != nil || got.next != nil || got.episodes.Item().ID != "last" {
		t.Fatalf("finale: %+v %v", got, err)
	}
}

func TestEpisodeSequenceResolvesContinueWatchingAndMissingSeasonNumbers(t *testing.T) {
	catalog := &episodeCatalog{seasons: []media.Item{seasonItem("one", 1), seasonItem("two", 2)}, episodes: map[string][]media.Item{
		"one": {episodeItem("first", 1, 1)}, "two": {episodeItem("second", 2, 1), episodeItem("third", 2, 2)},
	}}
	for _, numbered := range []bool{true, false} {
		item := catalog.episodes["two"][0]
		if !numbered {
			item.ParentIndexNumber = nil
		}
		got, err := findEpisodeSequence(t.Context(), catalog, item, View{Location: media.Location{Kind: "continue"}}, 6)
		if err != nil || got.next == nil || got.next.item.ID != "third" || got.episodes.Location.ParentID != "two" || got.episodes.Item().ID != "second" {
			t.Fatalf("continue numbered=%v: %+v %v", numbered, got, err)
		}
	}
}

type episodeListerFunc func(context.Context, media.Location, int, int) (media.Page, error)

func (f episodeListerFunc) List(ctx context.Context, l media.Location, s, n int) (media.Page, error) {
	return f(ctx, l, s, n)
}

func TestEpisodeSequenceRejectsMissingItemsFailuresAndRepeatedPages(t *testing.T) {
	item := episodeItem("missing", 1, 1)
	catalog := &episodeCatalog{seasons: []media.Item{seasonItem("one", 1)}, episodes: map[string][]media.Item{"one": {episodeItem("different", 1, 1)}}}
	if got, err := findEpisodeSequence(t.Context(), catalog, item, View{}, 6); err == nil || got.next != nil {
		t.Fatal("missing episode offered an unrelated successor")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := findEpisodeSequence(ctx, catalog, item, View{}, 6); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	boom := errors.New("server unavailable")
	failed := episodeListerFunc(func(context.Context, media.Location, int, int) (media.Page, error) { return media.Page{}, boom })
	if _, err := findEpisodeSequence(t.Context(), failed, item, View{}, 6); !errors.Is(err, boom) {
		t.Fatalf("failure: %v", err)
	}
	repeated := episodeListerFunc(func(context.Context, media.Location, int, int) (media.Page, error) {
		total := 128
		return media.Page{Items: []media.Item{seasonItem("one", 1)}, TotalRecordCount: &total}, nil
	})
	view := View{Location: media.Location{Kind: "seasons"}}
	if err := loadEpisodePage(t.Context(), repeated, &view); err != nil {
		t.Fatal(err)
	}
	if err := loadEpisodePage(t.Context(), repeated, &view); err == nil {
		t.Fatal("repeated page accepted")
	}
}
