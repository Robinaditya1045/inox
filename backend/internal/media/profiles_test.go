package media

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeTranscodeProfiles(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"none means the full ladder", nil, nil},
		{"blanks are ignored", []string{"", " "}, nil},
		{"every rung means the full ladder", []string{"480p", "1080p", "720p"}, nil},
		{"one rung", []string{"720p"}, []string{"720p"}},
		{"ladder order, deduplicated", []string{"480p", " 1080P ", "480p"}, []string{"1080p", "480p"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeTranscodeProfiles(tt.in)
			if err != nil {
				t.Fatalf("NormalizeTranscodeProfiles(%q) error: %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NormalizeTranscodeProfiles(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeTranscodeProfilesRejectsUnknown(t *testing.T) {
	_, err := NormalizeTranscodeProfiles([]string{"720p", "4k"})
	if !errors.Is(err, ErrUnknownTranscodeProfile) {
		t.Fatalf("error = %v, want ErrUnknownTranscodeProfile", err)
	}
}

func TestResolveTranscodeProfiles(t *testing.T) {
	names := func(ps []TranscodeProfile) []string {
		var out []string
		for _, p := range ps {
			out = append(out, p.Name)
		}
		return out
	}
	if got := names(resolveTranscodeProfiles(nil)); !reflect.DeepEqual(got, TranscodeProfileNames()) {
		t.Errorf("no choice resolved to %q, want the full ladder", got)
	}
	if got := names(resolveTranscodeProfiles([]string{"480p", "720p"})); !reflect.DeepEqual(got, []string{"720p", "480p"}) {
		t.Errorf("resolved to %q, want [720p 480p]", got)
	}
	// A name dropped from the ladder after the upload was queued must not leave
	// the transcode with nothing to encode.
	if got := names(resolveTranscodeProfiles([]string{"240p"})); !reflect.DeepEqual(got, TranscodeProfileNames()) {
		t.Errorf("unknown-only choice resolved to %q, want the full ladder", got)
	}
}

// The full ladder must keep producing exactly the encode every upload got before
// qualities could be chosen.
func TestLadderArgsFullLadderUnchanged(t *testing.T) {
	args, varStreamMap := ladderArgs(transcodeLadder, true)
	want := []string{
		"-filter_complex",
		"[0:v]split=3[v0][v1][v2];[v0]scale=w=1920:h=1080:force_original_aspect_ratio=decrease,pad=ceil(iw/2)*2:ceil(ih/2)*2[v0out];[v1]scale=w=1280:h=720:force_original_aspect_ratio=decrease,pad=ceil(iw/2)*2:ceil(ih/2)*2[v1out];[v2]scale=w=854:h=480:force_original_aspect_ratio=decrease,pad=ceil(iw/2)*2:ceil(ih/2)*2[v2out]",
		"-map", "[v0out]", "-c:v:0", "libx264", "-preset:v:0", "veryfast", "-profile:v:0", "main", "-pix_fmt:v:0", "yuv420p", "-b:v:0", "4500k", "-maxrate:v:0", "5000k", "-bufsize:v:0", "6000k",
		"-map", "[v1out]", "-c:v:1", "libx264", "-preset:v:1", "veryfast", "-profile:v:1", "main", "-pix_fmt:v:1", "yuv420p", "-b:v:1", "2500k", "-maxrate:v:1", "2800k", "-bufsize:v:1", "3500k",
		"-map", "[v2out]", "-c:v:2", "libx264", "-preset:v:2", "veryfast", "-profile:v:2", "main", "-pix_fmt:v:2", "yuv420p", "-b:v:2", "1000k", "-maxrate:v:2", "1200k", "-bufsize:v:2", "1500k",
		"-map", "0:a:0?", "-map", "0:a:0?", "-map", "0:a:0?",
		"-c:a", "aac", "-b:a", "128k", "-ar", "48000",
	}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("args =\n%q\nwant\n%q", args, want)
	}
	if want := "v:0,a:0,name:1080p v:1,a:1,name:720p v:2,a:2,name:480p"; varStreamMap != want {
		t.Errorf("var_stream_map = %q, want %q", varStreamMap, want)
	}
}

func TestLadderArgsSingleProfile(t *testing.T) {
	args, varStreamMap := ladderArgs(resolveTranscodeProfiles([]string{"720p"}), false)
	joined := strings.Join(args, " ")

	if strings.Contains(joined, "split") {
		t.Errorf("a single rung needs no split: %s", joined)
	}
	if !strings.Contains(joined, "[0:v]scale=w=1280:h=720:") || !strings.Contains(joined, "-b:v:0 2500k") {
		t.Errorf("args do not encode 720p as stream 0: %s", joined)
	}
	if strings.Contains(joined, "v:1") || strings.Contains(joined, "0:a:0?") {
		t.Errorf("args encode more than one stream, or audio the source lacks: %s", joined)
	}
	if varStreamMap != "v:0,name:720p" {
		t.Errorf("var_stream_map = %q, want v:0,name:720p", varStreamMap)
	}
}
