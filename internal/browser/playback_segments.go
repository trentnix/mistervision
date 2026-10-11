package browser

import (
	"time"

	"mistervision/internal/media"
)

const segmentPromptDuration = 8 * time.Second

// segmentPrompt tracks one automatic invitation. Playback controls can reveal
// the same action after this deadline without starting another automatic timer.
type segmentPrompt struct {
	segment media.Segment
	until   time.Time
}

func (c *PlaybackController) setSegments(segments []media.Segment, now time.Time) {
	c.segments = media.ValidSegments(segments, c.item.RunTimeTicks)
	c.updateSegmentPrompt(now)
}

// updateSegmentPrompt starts the timer on entry, including resume and seeking
// back into a segment. Temporary buffering and picker visibility do not reset it.
func (c *PlaybackController) updateSegmentPrompt(now time.Time) {
	segment, ok := c.currentSegment()
	if !ok {
		c.segmentPrompt = segmentPrompt{}
	} else if segment != c.segmentPrompt.segment {
		c.segmentPrompt = segmentPrompt{segment: segment, until: now.Add(segmentPromptDuration)}
	}
}

// currentSegment uses the original video's reported position. End-of-video
// actions use the seek controller's final-second safety margin.
func (c *PlaybackController) currentSegment() (media.Segment, bool) {
	if !c.running || c.stoppedByUser || !c.state.PlayingVideo || !c.state.ProgressSeen || media.IsLive(c.item) {
		return media.Segment{}, false
	}
	for _, segment := range c.segments {
		end := segment.EndTicks
		if c.item.RunTimeTicks > 0 {
			end = min(end, max(0, c.item.RunTimeTicks-10000000))
		}
		if c.state.PositionTicks >= segment.StartTicks && c.state.PositionTicks < end {
			return segment, true
		}
	}
	return media.Segment{}, false
}

// segmentAt exposes an action only while its prompt is visible. Opening controls
// reveals a timed-out prompt, but cannot revive one outside its segment.
func (c *PlaybackController) segmentAt(now time.Time) (media.Segment, bool) {
	if c.picker.visible || c.notice != "" || c.state.SeekTarget != nil ||
		c.seekPhase != seekInactive || c.state.videoWaitLabel(now) != "" {
		return media.Segment{}, false
	}
	segment, ok := c.currentSegment()
	visible := c.state.ControlsVisible(now) || (segment == c.segmentPrompt.segment && now.Before(c.segmentPrompt.until))
	return segment, ok && visible
}

// skipSegment uses the existing seek handoff, including pause restoration and
// failure handling. A discrete skip needs no repeated-press debounce.
func (c *PlaybackController) skipSegment(now time.Time) {
	if segment, ok := c.segmentAt(now); ok {
		c.SeekTo(segment.EndTicks, now)
		c.state.SeekDeadline = now
		c.state.SeekPresses = 1
		c.Tick(now)
	}
}

func segmentLabel(kind media.SegmentKind) string {
	switch kind {
	case media.SegmentIntro:
		return "Skip intro"
	case media.SegmentCredits:
		return "Skip credits"
	case media.SegmentRecap:
		return "Skip recap"
	case media.SegmentPreview:
		return "Skip preview"
	case media.SegmentCommercial:
		return "Skip commercials"
	default:
		return ""
	}
}
