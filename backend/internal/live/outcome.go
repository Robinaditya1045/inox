package live

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/inox/inox/backend/internal/domain"
)

// ResolveError ends a resolve without a stream, saying which kind of failure it was
// and, where the resolver knows, what it saw.
//
// Message is shown to operators and written to logs and to the channel's
// last_error, so like Diagnostics it names hosts and never URLs, header values or
// DRM data.
type ResolveError struct {
	Outcome     domain.ResolveOutcome
	Reason      string
	Message     string
	Diagnostics *domain.ResolveDiagnostics
	cause       error
}

func (e *ResolveError) Error() string { return e.Message }
func (e *ResolveError) Unwrap() error { return e.cause }

func resolveFailure(outcome domain.ResolveOutcome, reason, message string, cause error) *ResolveError {
	return &ResolveError{Outcome: outcome, Reason: reason, Message: message, cause: cause}
}

// failureOutcome places any resolve error in an outcome, including errors from
// resolvers that do not produce a ResolveError of their own.
func failureOutcome(err error) (domain.ResolveOutcome, string) {
	var re *ResolveError
	switch {
	case errors.As(err, &re):
		return re.Outcome, re.Reason
	case errors.Is(err, ErrDRMProtected):
		return domain.OutcomeDRMProtected, "manifest_protected"
	case errors.Is(err, context.DeadlineExceeded):
		return domain.OutcomeTimeout, "deadline_exceeded"
	case errors.Is(err, ErrHostNotAllowed):
		return domain.OutcomeFailed, "host_not_allowed"
	case errors.Is(err, ErrBlockedAddress):
		return domain.OutcomeFailed, "address_blocked"
	}
	var status *UpstreamStatusError
	if errors.As(err, &status) {
		return domain.OutcomeFailed, "upstream_refused"
	}
	return domain.OutcomeFailed, "resolver_error"
}

// failureDiagnostics is what gets recorded for a failed resolve: the resolver's own
// diagnostics where it gave some, the outcome alone otherwise.
func failureDiagnostics(resolverID string, err error, elapsed time.Duration) *domain.ResolveDiagnostics {
	outcome, reason := failureOutcome(err)
	diag := &domain.ResolveDiagnostics{}
	var re *ResolveError
	if errors.As(err, &re) && re.Diagnostics != nil {
		copied := *re.Diagnostics
		diag = &copied
	}
	diag.Outcome, diag.Reason = outcome, reason
	diag.Resolver = resolverID
	diag.ElapsedMS = elapsed.Milliseconds()
	return diag
}

// retryAfter is how long a failed resolve is handed back to later callers before the
// source is tried again. DRM protection does not lapse between retries, and finding
// it again costs another browser run, so that outcome is remembered far longer.
func retryAfter(err error) time.Duration {
	// A DRM or browser-bound source does not become restreamable by asking again
	// soon, so both back off far longer than a transient failure.
	switch outcome, _ := failureOutcome(err); outcome {
	case domain.OutcomeDRMProtected, domain.OutcomeServerReplayFailed:
		return drmRetryAfter
	}
	return resolveRetryAfter
}

// viewerNotice is what a room is told when its channel cannot be resolved. Generic by
// design: viewers get the kind of problem, operators get the details.
func viewerNotice(err error) string {
	switch outcome, _ := failureOutcome(err); outcome {
	case domain.OutcomeDRMProtected:
		return "This channel's stream is DRM-protected and can't be played here."
	case domain.OutcomeServerReplayFailed:
		return "This channel's source only plays in its own site and can't be restreamed here."
	case domain.OutcomeNoStreamFound:
		return "The channel's source isn't showing a stream right now."
	case domain.OutcomeTimeout:
		return "The stream source is taking too long to respond."
	}
	return "The stream source could not be reached."
}

// redactURL reduces a URL to what is safe to log or show: scheme and host. Signed
// stream URLs carry their credentials in the path as often as in the query.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "<url>"
	}
	return u.Scheme + "://" + u.Host + "/..."
}

// redactedError takes the URL out of a transport error. net/http puts the full
// request URL, query string and all, into every error it returns, which is how a
// signed manifest's token would otherwise end up in logs, in last_error, and in front
// of an operator. Everything errors.Is looks for survives.
func redactedError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return &url.Error{Op: ue.Op, URL: redactURL(ue.URL), Err: ue.Err}
	}
	return err
}

// successDiagnostics is what gets recorded for a resolve that found a stream.
func successDiagnostics(resolverID string, res *Resolution, elapsed time.Duration) *domain.ResolveDiagnostics {
	diag := &domain.ResolveDiagnostics{}
	if res.Diagnostics != nil {
		copied := *res.Diagnostics
		diag = &copied
	}
	diag.Outcome = domain.OutcomeStreamFound
	diag.Resolver = resolverID
	if diag.ElapsedMS == 0 {
		diag.ElapsedMS = elapsed.Milliseconds()
	}
	if len(diag.Manifests) == 0 {
		diag.Manifests = []domain.ManifestDiagnostic{{Host: hostOf(res.ManifestURL), Protocol: res.Protocol, Verdict: "chosen"}}
	}
	return diag
}

// logResolve writes one line per resolve attempt, built from its diagnostics --
// which hold nothing that may not be logged -- and the error message, which names
// hosts only. Per-manifest detail follows at debug level.
func logResolve(slug string, sourceIndex int, diag *domain.ResolveDiagnostics, err error) {
	attrs := []any{"slug", slug, "resolver", diag.Resolver, "outcome", diag.Outcome, "reason", diag.Reason, "elapsed_ms", diag.ElapsedMS}
	if sourceIndex > 0 {
		attrs = append(attrs, "source_index", sourceIndex)
	}
	if p := diag.Page; p != nil {
		attrs = append(attrs, slog.Group("page", "host", p.Host, "status", p.Status, "loaded", p.Loaded,
			"frames", p.Frames, "nudges", p.Nudges, "popups_closed", p.PopupsClosed,
			"cookies", p.Cookies, "cookies_withheld", p.CookiesWithheld, "withheld_names", p.CookiesWithheldNames,
			"cookie_domains", p.CookieDomains, "frame_hosts", p.FrameHosts, "redirects", p.Redirects))
	}
	if d := diag.DRM; d != nil {
		attrs = append(attrs, slog.Group("drm", "confirmed", d.Confirmed, "systems", d.Systems,
			"key_systems", d.KeySystems, "schemes", d.Schemes, "license_hosts", d.LicenseHosts, "signals", d.Signals))
	}
	if len(diag.Manifests) > 0 {
		attrs = append(attrs, "manifests", manifestSummary(diag.Manifests))
	}
	if err != nil {
		slog.Warn("live channel could not be resolved", append(attrs, "error", err.Error())...)
	} else {
		slog.Info("live channel resolved", attrs...)
	}
	logManifests(slug, diag)
}

// manifestSummary is one short line per manifest: host, format and verdict.
func manifestSummary(manifests []domain.ManifestDiagnostic) []string {
	out := make([]string, 0, len(manifests))
	for _, m := range manifests {
		parts := []string{m.Host}
		for _, p := range []string{m.Protocol, m.Kind} {
			if p != "" {
				parts = append(parts, p)
			}
		}
		out = append(out, fmt.Sprintf("%s: %s", strings.Join(parts, " "), m.Verdict))
	}
	return out
}

// logManifests writes what became of each manifest a resolve considered, at debug
// level. Diagnostics hold hosts and verdicts only, so this is safe to log in full.
func logManifests(slug string, diag *domain.ResolveDiagnostics) {
	for _, m := range diag.Manifests {
		slog.Debug("live channel manifest considered", "slug", slug, "host", m.Host,
			"protocol", m.Protocol, "kind", m.Kind, "live", m.Live, "verdict", m.Verdict, "detail", m.Detail, "drm", m.DRM,
			"browser_cookies", m.BrowserCookies)
	}
}
