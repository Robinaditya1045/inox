package media

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeCaptionsKeepsWebVTT(t *testing.T) {
	in := "\xef\xbb\xbfWEBVTT - English\r\n\r\nSTYLE\r\n::cue { color: yellow }\r\n\r\nNOTE a comment\r\n\r\nintro\r\n00:01.000 --> 00:04.500 line:90%\r\nHello\r\n\r\n01:00:00.000 --> 01:00:02.000\r\n<i>World</i>\r\n"
	out, count, err := NormalizeCaptions([]byte(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
	want := "WEBVTT - English\n\nSTYLE\n::cue { color: yellow }\n\nNOTE a comment\n\nintro\n00:01.000 --> 00:04.500 line:90%\nHello\n\n01:00:00.000 --> 01:00:02.000\n<i>World</i>\n"
	if out != want {
		t.Errorf("output mismatch:\n got %q\nwant %q", out, want)
	}
}

func TestNormalizeCaptionsConvertsSubRip(t *testing.T) {
	in := "1\r\n00:00:01,000 --> 00:00:04,000\r\n{\\an8}Hello there\r\n\r\n2\r\n0:00:05,5 --> 0:00:07,25 X1:100 X2:200\r\nTwo\r\nlines --> here\r\n"
	out, count, err := NormalizeCaptions([]byte(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
	want := "WEBVTT\n\n1\n00:00:01.000 --> 00:00:04.000\nHello there\n\n2\n00:00:05.500 --> 00:00:07.250\nTwo\nlines -> here\n"
	if out != want {
		t.Errorf("output mismatch:\n got %q\nwant %q", out, want)
	}
	// The conversion has to produce something the WebVTT path accepts.
	if _, again, err := NormalizeCaptions([]byte(out)); err != nil || again != 2 {
		t.Errorf("converted output does not validate as WebVTT: count=%d err=%v", again, err)
	}
}

func TestNormalizeCaptionsRejects(t *testing.T) {
	cases := map[string]string{
		"not captions":     "hello world\nthis is a text file\n",
		"empty vtt":        "WEBVTT\n\nNOTE nothing here\n",
		"ends before":      "WEBVTT\n\n00:05.000 --> 00:01.000\nBackwards\n",
		"bad timestamp":    "WEBVTT\n\n00:05,000 --> 00:06.000\nComma in VTT\n",
		"no timing":        "WEBVTT\n\nid\njust text\n",
		"bad srt in body":  "1\n00:00:01,000 --> 00:00:02,000\nok\n\n2\nnot a timing line\nnope\n",
		"latin-1":          "1\n00:00:01,000 --> 00:00:02,000\ncaf\xe9\n",
		"zero-length cue":  "WEBVTT\n\n00:01.000 --> 00:01.000\nInstant\n",
		"minutes overflow": "WEBVTT\n\n00:75.000 --> 00:76.000\nNo\n",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := NormalizeCaptions([]byte(in)); !errors.Is(err, ErrInvalidCaptions) {
				t.Errorf("err = %v, want ErrInvalidCaptions", err)
			}
		})
	}
}

func TestNormalizeCaptionsNamesTheLine(t *testing.T) {
	_, _, err := NormalizeCaptions([]byte("WEBVTT\n\n00:01.000 --> 00:02.000\nok\n\n00:09.000 --> 00:03.000\nbad\n"))
	if err == nil || !strings.Contains(err.Error(), "line 6") {
		t.Errorf("err = %v, want it to point at line 6", err)
	}
}

func TestCanonicalLanguageTag(t *testing.T) {
	good := map[string]string{
		"en":         "en",
		"EN":         "en",
		"pt-br":      "pt-BR",
		"zh_hant_tw": "zh-Hant-TW",
		"es-419":     "es-419",
		" fil ":      "fil",
	}
	for in, want := range good {
		if got, err := CanonicalLanguageTag(in); err != nil || got != want {
			t.Errorf("CanonicalLanguageTag(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "e", "english!", "en--us", "1en", strings.Repeat("a", 40)} {
		if _, err := CanonicalLanguageTag(in); !errors.Is(err, ErrInvalidLanguage) {
			t.Errorf("CanonicalLanguageTag(%q) err = %v, want ErrInvalidLanguage", in, err)
		}
	}
}
