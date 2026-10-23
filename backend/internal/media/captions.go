package media

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// MaxCaptionBytes caps one caption file. A feature film's subtitles are well
// under 200 KB, so anything near this is not a caption file.
const MaxCaptionBytes = 2 << 20 // 2 MB

// ErrInvalidCaptions rejects a caption file that is neither WebVTT nor SubRip,
// or whose cues cannot be read. The wrapped message says which line is wrong.
var ErrInvalidCaptions = errors.New("invalid caption file")

// ErrInvalidLanguage rejects a caption language that is not a BCP 47 tag.
var ErrInvalidLanguage = errors.New("language must be a BCP 47 tag such as en or pt-BR")

var (
	// WebVTT timestamps: hours are optional, everything else is fixed width.
	vttTimestamp = regexp.MustCompile(`^(?:(\d{2,}):)?(\d{2}):(\d{2})\.(\d{3})$`)
	// SubRip in the wild: a comma before the milliseconds by the book, but also
	// a dot, single-digit fields, and milliseconds short of three digits.
	srtTimestamp = regexp.MustCompile(`^(\d{1,3}):(\d{1,2}):(\d{1,2})[,.](\d{1,3})$`)
	// SubStation override blocks ({\an8}, {\i1}) that SubRip files exported from
	// .ass keep. They mean nothing to WebVTT and would show up as text.
	assOverride   = regexp.MustCompile(`\{\\[^}]*\}`)
	languageShape = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{1,8})*$`)
)

// NormalizeCaptions turns an uploaded caption file into the WebVTT the player
// reads. WebVTT is checked and kept as written, including any STYLE or NOTE
// blocks; SubRip (.srt) is converted. It returns the document and its cue count.
func NormalizeCaptions(data []byte) (string, int, error) {
	if len(data) > MaxCaptionBytes {
		return "", 0, fmt.Errorf("%w: larger than %d MB", ErrInvalidCaptions, MaxCaptionBytes>>20)
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if !utf8.Valid(data) {
		return "", 0, fmt.Errorf("%w: the file must be UTF-8 text", ErrInvalidCaptions)
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimLeft(text, " \t\n")

	if isWebVTT(text) {
		count, err := validateWebVTT(text)
		if err != nil {
			return "", 0, err
		}
		return strings.TrimRight(text, " \t\n") + "\n", count, nil
	}
	return convertSubRip(text)
}

func isWebVTT(text string) bool {
	rest, ok := strings.CutPrefix(text, "WEBVTT")
	return ok && (rest == "" || rest[0] == ' ' || rest[0] == '\t' || rest[0] == '\n')
}

// block is a run of non-blank lines; line is the 1-based line number of its first.
type block struct {
	line  int
	lines []string
}

func splitBlocks(text string) []block {
	var blocks []block
	var cur *block
	for i, l := range strings.Split(text, "\n") {
		if strings.TrimSpace(l) == "" {
			cur = nil
			continue
		}
		if cur == nil {
			blocks = append(blocks, block{line: i + 1})
			cur = &blocks[len(blocks)-1]
		}
		cur.lines = append(cur.lines, l)
	}
	return blocks
}

func validateWebVTT(text string) (int, error) {
	blocks := splitBlocks(text)
	count := 0
	// blocks[0] is the WEBVTT header and whatever header lines follow it.
	for _, b := range blocks[1:] {
		first := b.lines[0]
		if hasKeyword(first, "NOTE") || hasKeyword(first, "STYLE") || hasKeyword(first, "REGION") {
			continue
		}
		timing, at := first, b.line
		if !strings.Contains(first, "-->") {
			if len(b.lines) < 2 || !strings.Contains(b.lines[1], "-->") {
				return 0, fmt.Errorf("%w: line %d: expected a cue timing line like 00:00:01.000 --> 00:00:04.000", ErrInvalidCaptions, b.line)
			}
			timing, at = b.lines[1], b.line+1
		}
		if _, _, _, err := parseTiming(timing, vttTimestamp); err != nil {
			return 0, fmt.Errorf("%w: line %d: %v", ErrInvalidCaptions, at, err)
		}
		count++
	}
	if count == 0 {
		return 0, fmt.Errorf("%w: it has no captions", ErrInvalidCaptions)
	}
	return count, nil
}

func hasKeyword(line, keyword string) bool {
	rest, ok := strings.CutPrefix(line, keyword)
	return ok && (rest == "" || rest[0] == ' ' || rest[0] == '\t')
}

func convertSubRip(text string) (string, int, error) {
	blocks := splitBlocks(text)
	if len(blocks) == 0 {
		return "", 0, fmt.Errorf("%w: it has no captions", ErrInvalidCaptions)
	}

	var out strings.Builder
	out.WriteString("WEBVTT\n")
	for i, b := range blocks {
		lines, at := b.lines, b.line
		id := ""
		if !strings.Contains(lines[0], "-->") {
			if len(lines) < 2 || !strings.Contains(lines[1], "-->") {
				if i == 0 {
					return "", 0, fmt.Errorf("%w: expected a WebVTT (.vtt) or SubRip (.srt) file", ErrInvalidCaptions)
				}
				return "", 0, fmt.Errorf("%w: line %d: expected a cue timing line like 00:00:01,000 --> 00:00:04,000", ErrInvalidCaptions, at)
			}
			id, lines, at = strings.TrimSpace(lines[0]), lines[1:], at+1
		}
		start, end, _, err := parseTiming(lines[0], srtTimestamp)
		if err != nil {
			return "", 0, fmt.Errorf("%w: line %d: %v", ErrInvalidCaptions, at, err)
		}

		out.WriteString("\n")
		// The SubRip counter is kept as the cue identifier, which WebVTT allows,
		// so error messages and editors still line up with the original file.
		if id != "" && !strings.Contains(id, "-->") {
			out.WriteString(id + "\n")
		}
		// SubRip's trailing X1/Y1 coordinates have no WebVTT equivalent and are dropped.
		fmt.Fprintf(&out, "%s --> %s\n", formatTimestamp(start), formatTimestamp(end))
		for _, l := range lines[1:] {
			l = assOverride.ReplaceAllString(l, "")
			// "-->" in a payload would read as a timing line.
			out.WriteString(strings.ReplaceAll(l, "-->", "->") + "\n")
		}
	}
	return out.String(), len(blocks), nil
}

// parseTiming reads "start --> end [settings]", returning milliseconds.
func parseTiming(line string, shape *regexp.Regexp) (int64, int64, string, error) {
	left, right, ok := strings.Cut(line, "-->")
	if !ok {
		return 0, 0, "", errors.New("missing -->")
	}
	fields := strings.Fields(right)
	if len(fields) == 0 {
		return 0, 0, "", errors.New("missing end time")
	}
	start, err := parseTimestamp(strings.TrimSpace(left), shape)
	if err != nil {
		return 0, 0, "", err
	}
	end, err := parseTimestamp(fields[0], shape)
	if err != nil {
		return 0, 0, "", err
	}
	if end <= start {
		return 0, 0, "", fmt.Errorf("caption ends (%s) before it starts (%s)", fields[0], strings.TrimSpace(left))
	}
	return start, end, strings.Join(fields[1:], " "), nil
}

func parseTimestamp(s string, shape *regexp.Regexp) (int64, error) {
	m := shape.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("%q is not a timestamp", s)
	}
	var h int64
	if m[1] != "" {
		h, _ = strconv.ParseInt(m[1], 10, 64)
	}
	mins, _ := strconv.ParseInt(m[2], 10, 64)
	secs, _ := strconv.ParseInt(m[3], 10, 64)
	if mins > 59 || secs > 59 {
		return 0, fmt.Errorf("%q is not a timestamp", s)
	}
	// SubRip's short milliseconds are a fraction: ",5" is half a second.
	frac := m[4] + strings.Repeat("0", 3-len(m[4]))
	ms, _ := strconv.ParseInt(frac, 10, 64)
	return ((h*60+mins)*60+secs)*1000 + ms, nil
}

func formatTimestamp(ms int64) string {
	return fmt.Sprintf("%02d:%02d:%02d.%03d", ms/3_600_000, ms/60_000%60, ms/1000%60, ms%1000)
}

// CanonicalLanguageTag checks a BCP 47 tag and writes it the conventional way:
// "PT-br" becomes "pt-BR", "zh-hant" becomes "zh-Hant".
func CanonicalLanguageTag(tag string) (string, error) {
	tag = strings.TrimSpace(strings.ReplaceAll(tag, "_", "-"))
	if len(tag) > 35 || !languageShape.MatchString(tag) {
		return "", ErrInvalidLanguage
	}
	parts := strings.Split(tag, "-")
	parts[0] = strings.ToLower(parts[0])
	for i := 1; i < len(parts); i++ {
		switch p := parts[i]; {
		case len(p) == 2 && isLetters(p):
			parts[i] = strings.ToUpper(p)
		case len(p) == 4 && isLetters(p):
			parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
		default:
			parts[i] = strings.ToLower(p)
		}
	}
	return strings.Join(parts, "-"), nil
}

func isLetters(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}
