package plex

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mistervision/internal/media"
)

var _ media.ProgramGuide = (*Client)(nil)

// Programs maps Plex airing identifiers back to the opaque channel catalog IDs.
// DVRs without a guide return no listings. Requests never allocate a tuner.
func (c *Client) Programs(ctx context.Context, channels []string, start, end time.Time) ([]media.Program, error) {
	if err := media.ValidateGuideRequest(channels, start, end); err != nil {
		return nil, err
	}
	wanted := make(map[string]map[string]string)
	for _, id := range channels {
		dvrID, key, err := parseChannelID(id)
		if err != nil {
			return nil, err
		}
		if wanted[dvrID] == nil {
			wanted[dvrID] = make(map[string]string)
		}
		wanted[dvrID][key] = id
	}
	dvrs, err := c.dvrs(ctx)
	if err != nil {
		return nil, err
	}
	var result []media.Program
	for _, d := range dvrs {
		requested := wanted[string(d.ID)]
		if len(requested) == 0 || d.EPGIdentifier == "" {
			continue
		}
		aliases := d.programAliases(requested, c.channelGuide(ctx, d.Lineup))
		programs, err := c.programGrid(ctx, d.EPGIdentifier, start, end)
		if err != nil {
			return nil, err
		}
		for _, p := range programs {
			if id := aliases[p.ChannelID]; id != "" {
				p.ChannelID = id
				result = append(result, p)
			}
		}
	}
	return result, ctx.Err()
}

// programAliases joins tuner and guide identifiers to the requested catalog IDs.
// Disabled mappings and other channels cannot contribute listings.
func (d dvr) programAliases(requested map[string]string, guide map[string]guideChannel) map[string]string {
	aliases := make(map[string]string)
	for _, device := range d.Devices {
		if device.State == "disabled" {
			continue
		}
		for _, mapping := range device.Mappings {
			id := requested[mapping.key()]
			if !mapping.enabled() || id == "" {
				continue
			}
			for _, key := range []string{mapping.ChannelKey, mapping.LineupIdentifier, mapping.DeviceIdentifier} {
				if key == "" {
					continue
				}
				detail := guide[key]
				for _, alias := range []string{key, detail.Key, detail.Identifier} {
					if alias != "" {
						aliases[alias] = id
					}
				}
			}
		}
	}
	return aliases
}

// programGrid reads one bounded Plex EPG window. ChannelID is the guide's
// identifier here; Programs translates it before returning records to the UI.
func (c *Client) programGrid(ctx context.Context, provider string, start, end time.Time) ([]media.Program, error) {
	name, id, ok := strings.Cut(provider, ":")
	if !ok || !validID(id) || (name != "tv.plex.providers.epg.cloud" && name != "tv.plex.providers.epg.xmltv" && name != "tv.plex.providers.epg.custom") {
		return nil, errors.New("unsupported Plex guide provider")
	}
	q := url.Values{
		"type":                  {"1,4"},
		"beginsAt<":             {strconv.FormatInt(end.Unix(), 10)},
		"endsAt>":               {strconv.FormatInt(start.Unix(), 10)},
		"X-Plex-Container-Size": {"2000"},
	}
	var response struct {
		Container *struct {
			Total    int `json:"totalSize"`
			Metadata []struct {
				Title, GrandparentTitle string
				Media                   []struct {
					ChannelIdentifier string
					BeginsAt, EndsAt  int64
				}
			}
		} `json:"MediaContainer"`
	}
	if err := c.json(ctx, "/"+provider+"/grid", q, &response); err != nil {
		return nil, err
	}
	if response.Container == nil {
		return nil, errors.New("missing Plex guide container")
	}
	if response.Container.Total > 2000 || len(response.Container.Metadata) > 2000 {
		return nil, errors.New("program guide exceeds window limit")
	}
	var result []media.Program
	for _, item := range response.Container.Metadata {
		title := item.GrandparentTitle
		if title == "" {
			title = item.Title
		}
		for _, airing := range item.Media {
			begin, finish := time.Unix(airing.BeginsAt, 0), time.Unix(airing.EndsAt, 0)
			if title != "" && airing.BeginsAt > 0 && finish.After(begin) && begin.Before(end) && finish.After(start) {
				if len(result) == 2000 {
					return nil, errors.New("program guide exceeds airing limit")
				}
				result = append(result, media.Program{ChannelID: airing.ChannelIdentifier, Title: title, Start: begin, End: finish})
			}
		}
	}
	return result, nil
}
