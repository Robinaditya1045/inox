package media

import "testing"

// The poster frame comes from a fifth of the way in -- past a black open or a
// title card -- but never so deep that the seek costs more than it is worth.
func TestThumbnailOffset(t *testing.T) {
	cases := []struct {
		duration float64
		want     float64
	}{
		{0, 5},      // unknown length: a few seconds in
		{3, 0},      // too short for a fifth to clear the first second
		{30, 6},     // a fifth of the way in
		{75, 15},    // a fifth of the way in
		{7200, 120}, // capped
	}
	for _, c := range cases {
		if got := thumbnailOffset(c.duration); got != c.want {
			t.Errorf("thumbnailOffset(%v) = %v, want %v", c.duration, got, c.want)
		}
	}
}
