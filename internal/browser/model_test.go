package browser

import (
	"errors"
	"testing"

	"mistervision/internal/input/control"
	"mistervision/internal/media"
)

func TestNavigationCancelsStaleResultsAndRestoresSelection(t *testing.T) {
	m := New()
	req := m.Load(0)
	m.Apply(*req, media.Page{Items: []media.Item{{ID: "movies", Name: "Movies", CollectionType: "movies"}, {ID: "music", Name: "Music", CollectionType: "music"}}}, nil)
	m.Key(control.Next)
	load := m.Key(control.Open)
	if load.Location.ParentID != "music" || load.Location.Collection != "music" {
		t.Fatal(load)
	}
	m.Key(control.Back)
	if m.Apply(*load, media.Page{Items: []media.Item{{ID: "stale"}}}, nil) {
		t.Fatal("accepted stale response")
	}
	if m.Current().Selected != 1 || m.Current().Title != "Libraries" {
		t.Fatal("lost parent selection")
	}
}

func TestPagingFailurePreservesRowsAndRetriesOffset(t *testing.T) {
	m := New()
	m.Current().Location = media.Location{Kind: "items", Collection: "movies"}
	m.ListMode = true
	total := 503
	items := make([]media.Item, 64)
	for i := range items {
		items[i].ID = "movie"
	}
	m.Apply(*m.Load(0), media.Page{Items: items, TotalRecordCount: &total}, nil)
	m.Current().Selected = 63
	r := m.Key(control.Down)
	if r.Start != 64 {
		t.Fatal(r)
	}
	m.Apply(*r, media.Page{}, errors.New("server unavailable"))
	if len(m.Current().Page.Items) != 64 || m.Current().Start != 0 || m.Current().Error == "" {
		t.Fatal("failed page replaced existing rows")
	}
	r = m.Key(control.Retry)
	if r.Start != 64 {
		t.Fatal("retry changed page")
	}
	m.Apply(*r, media.Page{Items: items, TotalRecordCount: &total}, nil)
	if m.Current().Start != 0 || m.Current().Selected != 64 || len(m.Current().Page.Items) != 128 {
		t.Fatal("page did not advance")
	}
	m.Current().Selected = 127
	r = m.Key(control.Down)
	m.Apply(*r, media.Page{Items: []media.Item{}, TotalRecordCount: &total}, nil)
	if m.Current().Start != 0 || len(m.Current().Page.Items) != 128 || m.Current().Error == "" {
		t.Fatal("empty page hid previous data")
	}
}

func TestSeriesAndUnsupportedItems(t *testing.T) {
	m := New()
	m.Current().Location = media.Location{Kind: "items", Collection: "mixed"}
	m.Current().Page.Items = []media.Item{{ID: "series", Type: "Series"}}
	r := m.Key(control.Open)
	if r.Location.Kind != "seasons" || r.Location.SeriesID != "series" {
		t.Fatal(r)
	}
	m.Apply(*r, media.Page{Items: []media.Item{{ID: "season", Type: "Season"}}}, nil)
	r = m.Key(control.Open)
	if r.Location.Kind != "episodes" || r.Location.SeriesID != "series" || r.Location.ParentID != "season" {
		t.Fatal(r)
	}
	m.Apply(*r, media.Page{Items: []media.Item{{ID: "book", Type: "Book"}}}, nil)
	if r = m.Key(control.Open); r != nil || m.Current().Detail == nil {
		t.Fatal("unsupported item did not produce details")
	}
}

func TestCarouselListToggleAndExit(t *testing.T) {
	m := New()
	m.Current().Page.Items = make([]media.Item, 4)
	m.Key(control.Down)
	if m.Current().Selected != 0 {
		t.Fatal("carousel moved vertically")
	}
	m.Key(control.Next)
	m.Key(control.Select)
	m.Key(control.Down)
	if !m.ListMode || m.Current().Selected != 2 {
		t.Fatal("list toggle lost selection")
	}
	m.Key(control.Back)
	if !m.ExitConfirm || m.Quit {
		t.Fatal("root back must confirm")
	}
	m.Key(control.Back)
	if m.ExitConfirm {
		t.Fatal("cancel did not close dialog")
	}
	m.Key(control.Back)
	m.Key(control.Open)
	if !m.Quit {
		t.Fatal("confirm did not quit")
	}
}
func TestScreenJumpAndBackwardPageBoundary(t *testing.T) {
	m := New()
	m.Rows = 6
	v := m.Current()
	v.Location.Kind = "items"
	m.ListMode = true
	total := 130
	items := make([]media.Item, 64)
	m.Apply(*m.Load(0), media.Page{Items: items, TotalRecordCount: &total}, nil)
	if m.Key(control.Next) != nil || v.Selected != 6 || v.Scroll != 3 {
		t.Fatal("jump must move one screen")
	}
	v.Selected = 63
	r := m.Key(control.Down)
	m.Apply(*r, media.Page{Items: items, TotalRecordCount: &total}, nil)
	r = m.Key(control.Up)
	if r != nil {
		t.Fatal("cached previous page caused a request")
	}
	if v.Selected != 63 || v.Scroll != 60 {
		t.Fatalf("backward crossing: %+v", v)
	}
}

func TestSingleSeasonOpensEpisodesAndBackRestoresShows(t *testing.T) {
	m := New()
	m.Current().Location = media.Location{Kind: "items", Collection: "tvshows"}
	m.Current().Page.Items = []media.Item{{ID: "other", Type: "Series"}, {ID: "show", Name: "Show", Type: "Series"}}
	m.Current().Selected = 1
	seasons := m.Key(control.Open)
	total := 1
	m.Apply(*seasons, media.Page{Items: []media.Item{{ID: "season", Name: "Season 1", Type: "Season", SeriesID: "show"}}, TotalRecordCount: &total}, nil)
	episodes := m.skipSingleSeason()
	if episodes == nil || episodes.Location.Kind != "episodes" || episodes.Location.ParentID != "season" || episodes.Location.SeriesID != "show" || len(m.Stack) != 2 {
		t.Fatalf("single season did not replace its view: request=%+v stack=%+v", episodes, m.Stack)
	}
	m.Apply(*episodes, media.Page{}, errors.New("offline"))
	retry := m.Key(control.Retry)
	if retry.Location != episodes.Location {
		t.Fatal("retry returned to the hidden season list")
	}
	m.Key(control.Back)
	if len(m.Stack) != 1 || m.Current().Selected != 1 || m.Current().Item().ID != "show" {
		t.Fatal("Back lost the original show selection")
	}
	if m.Apply(*retry, media.Page{Items: []media.Item{{ID: "late", Type: "Episode"}}}, nil) {
		t.Fatal("late episodes changed the show list")
	}
}

func TestSingleSeasonRequiresCompleteSuccessfulSeasonListing(t *testing.T) {
	two := 2
	season := media.Item{ID: "season", Name: "Season 1", Type: "Season"}
	for _, tc := range []struct {
		name string
		view View
		want bool
	}{
		{"one season", View{Page: media.Page{Items: []media.Item{season}}}, true},
		{"no seasons", View{}, false},
		{"two seasons", View{Page: media.Page{Items: []media.Item{season, season}}}, false},
		{"partial listing", View{Page: media.Page{Items: []media.Item{season}, TotalRecordCount: &two}}, false},
		{"failed refresh", View{Page: media.Page{Items: []media.Item{season}}, Error: "failed"}, false},
		{"still loading", View{Page: media.Page{Items: []media.Item{season}}, Loading: true}, false},
		{"later page", View{Page: media.Page{Items: []media.Item{season}}, Start: 64}, false},
		{"not a season", View{Page: media.Page{Items: []media.Item{{ID: "episode", Type: "Episode"}}}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New()
			tc.view.Location = media.Location{Kind: "seasons", SeriesID: "show"}
			m.Stack = append(m.Stack, tc.view)
			if got := m.skipSingleSeason(); (got != nil) != tc.want {
				t.Fatalf("request=%+v want redirect=%v", got, tc.want)
			}
		})
	}
}

func TestArtistNavigationRequestsAlbumsThenTracks(t *testing.T) {
	m := New()
	m.Current().Location = media.Location{Kind: "items", ParentID: "music", Collection: "music"}
	m.Current().Page.Items = []media.Item{{ID: "artist", Name: "Artist", Type: "MusicArtist", IsFolder: true}}
	albums := m.Key(control.Open)
	if albums == nil || albums.Location.Kind != "albums" || albums.Location.ParentID != "artist" || albums.Location.Collection != "music" {
		t.Fatalf("artist navigation: %+v", albums)
	}
	m.Apply(*albums, media.Page{Items: []media.Item{{ID: "album", Name: "Album", Type: "MusicAlbum", IsFolder: true}}}, nil)
	tracks := m.Key(control.Open)
	if tracks == nil || tracks.Location.Kind != "items" || tracks.Location.ParentID != "album" {
		t.Fatalf("album navigation: %+v", tracks)
	}
	m.Key(control.Back)
	if m.Current().Location != albums.Location {
		t.Fatal("Back did not restore artist albums")
	}
}
