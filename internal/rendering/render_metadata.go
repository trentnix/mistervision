package rendering

import (
	"fmt"
	"strings"

	"mistervision/internal/media"
)

func subtitle(i media.Item) (string, uint32) {
	color := uint32(0x585858)
	switch i.Type {
	case "TvChannel", "LiveTvChannel":
		if i.CurrentProgram.Name != "" {
			return i.CurrentProgram.Name, color
		}
		return "No guide information", color
	case "MusicArtist":
		return positiveCount(i.ChildCount, "album"), color
	case "MusicAlbum":
		parts := []string{}
		if i.ProductionYear > 0 {
			parts = append(parts, fmt.Sprint(i.ProductionYear))
		}
		if tracks := positiveCount(i.ChildCount, "track"); tracks != "" {
			parts = append(parts, tracks)
		}
		return strings.Join(parts, " - "), color
	case "Season":
		return positiveCount(i.ChildCount, "episode"), color
	case "Series":
		seasons := positiveCount(i.ChildCount, "season")
		if seasons != "" && i.RecursiveItemCount > 0 {
			seasons += " - " + positiveCount(i.RecursiveItemCount, "episode")
		}
		return seasons, color
	case "Playlist", "BoxSet":
		return positiveCount(i.ChildCount, "item"), color
	case "Audio":
		return runtime(i.RunTimeTicks), color
	}
	s := ""
	if i.RunTimeTicks > 0 {
		s = fmt.Sprintf("%d min", i.RunTimeTicks/600000000)
	}
	if i.UserData.Played {
		s += " - watched"
		color = 0x40cc40
	} else if i.UserData.PlaybackPositionTicks > 0 {
		s += " - resume " + runtime(i.UserData.PlaybackPositionTicks)
		color = 0xffc040
	}
	return strings.TrimPrefix(s, " - "), color
}

func itemTitle(i media.Item) string {
	s := i.Name
	if i.ContinueAction != "" && i.Type == "Episode" && i.SeriesName != "" {
		s = i.SeriesName + " - " + s
	}
	if media.IsLive(i) {
		number := i.Number
		if number == "" {
			number = i.ChannelNumber
		}
		if number != "" {
			return number + "  " + s
		}
		return s
	}

	if i.ProductionYear > 0 && (i.Type == "Movie" || i.Type == "MusicVideo" || i.Type == "Video" || i.Type == "Series") {
		s += fmt.Sprintf(" (%d)", i.ProductionYear)
	}
	return s
}

// positiveCount follows the C list metadata: omit unknown counts and pluralize.
func positiveCount(count int, name string) string {
	if count <= 0 {
		return ""
	}
	if count != 1 {
		name += "s"
	}
	return fmt.Sprintf("%d %s", count, name)
}

// continueSubtitle distinguishes starting an episode from resuming saved progress.
func continueSubtitle(item media.Item) string {
	parts := []string{}
	if item.ContinueAction == "resume" {
		parts = append(parts, "Resume", runtime(item.UserData.PlaybackPositionTicks))
	} else {
		parts = append(parts, "Next")
	}
	if item.Type == "Episode" {
		if item.ParentIndexNumber != nil && item.IndexNumber != nil {
			parts = append(parts, fmt.Sprintf("S%d E%d", *item.ParentIndexNumber, *item.IndexNumber))
		} else if item.IndexNumber != nil {
			parts = append(parts, fmt.Sprintf("Episode %d", *item.IndexNumber))
		}
	}
	return strings.Join(parts, " · ")
}

// libraryCountText names the server's count unit consistently in both home views.
func libraryCountText(item media.Item, count int) string {
	singular, plural := "item", "items"
	switch item.CollectionType {
	case "movies":
		singular, plural = "movie", "movies"
	case "tvshows":
		singular, plural = "series", "series"
	case "music":
		singular, plural = "album", "albums"
		if item.CountType == "MusicArtist" {
			singular, plural = "artist", "artists"
		}
	case "musicvideos":
		singular, plural = "video", "videos"
	case "livetv":
		singular, plural = "channel", "channels"
	case "playlists":
		singular, plural = "playlist", "playlists"
	case "boxsets":
		singular, plural = "collection", "collections"
	}
	if count == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}
