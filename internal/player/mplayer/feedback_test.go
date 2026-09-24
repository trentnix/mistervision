package mplayer

import (
	"fmt"
	"mistervision/internal/media"
	"mistervision/internal/player"
	"testing"
)

func TestDecoderInputFormat(t *testing.T) {
	var got []player.Feedback
	w := (Decoder{}).Feedback(func(f player.Feedback) { got = append(got, f) })
	fmt.Fprint(w, "ID_VIDEO_WID")
	fmt.Fprint(w, "TH=640\nID_VIDEO_HEIGHT=480\nID_VIDEO_FPS=23.976\nID_VIDEO_CODEC=ffh264\nID_VIDEO_BITRATE=1234567\n")
	want := media.VideoFormat{Width: 640, Height: 480, FrameRate: 23.976, Codec: "h264", BitRate: 1234567}
	if len(got) != 5 || got[4].Kind != player.FeedbackVideoFormat || got[4].VideoFormat != want {
		t.Fatalf("feedback: %+v", got)
	}
	for _, line := range []string{"ID_VIDEO_WIDTH=Infinity", "ID_VIDEO_HEIGHT=-1", "ID_VIDEO_FPS=NaN", "ID_VIDEO_FPS=1001", "ID_VIDEO_CODEC=https://private/?token=secret", "ID_VIDEO_BITRATE=1e99", "ID_FILENAME=private", "ID_VIDEO_WIDTH=1.5", "ID_VIDEO_WIDTH=640"} {
		fmt.Fprintln(w, line)
	}
	if len(got) != 5 {
		t.Fatalf("accepted invalid or duplicate metadata: %+v", got)
	}
	var next player.Feedback
	w = (Decoder{}).Feedback(func(f player.Feedback) { next = f })
	fmt.Fprintln(w, "ID_VIDEO_FPS=30")
	if next.VideoFormat.Width != 0 || next.VideoFormat.FrameRate != 30 {
		t.Fatal("metadata leaked across decoder instances")
	}
}
