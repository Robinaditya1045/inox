package live

import (
	"encoding/hex"
	"net/url"
	"strings"

	"github.com/inox/inox/backend/internal/domain"
)

// Kinds of DRM evidence, as they appear in diagnostics.
const (
	signalManifestProtected = "manifest_protected"
	signalAccessRequested   = "eme_access_requested"
	signalKeysCreated       = "media_keys_created"
	signalServerCertificate = "server_certificate_set"
	signalKeysAttached      = "media_keys_attached"
	signalSessionCreated    = "key_session_created"
	signalLicenseRequested  = "license_request_generated"
	signalLicenseApplied    = "license_applied"
	signalEncryptedMedia    = "encrypted_media"
	signalWaitingForKey     = "waiting_for_key"
	signalLicenseServer     = "license_server_contacted"
)

// confirmingSignals are the ones that mean the stream itself needs DRM. The others
// only show a page asking what the browser supports, which players probing their
// capabilities and fingerprinting scripts both do on pages whose stream is
// unprotected -- on their own they prove nothing about the stream.
var confirmingSignals = map[string]bool{
	signalManifestProtected: true,
	signalKeysAttached:      true,
	signalSessionCreated:    true,
	signalLicenseRequested:  true,
	signalLicenseApplied:    true,
	signalEncryptedMedia:    true,
	signalWaitingForKey:     true,
	signalLicenseServer:     true,
}

// maxEvidence bounds each list of evidence, so a page calling EME in a loop cannot
// grow them without limit.
const maxEvidence = 16

// drmEvidence is what a page, its player and its manifests revealed about DRM,
// reduced to names, counts and hosts -- see domain.ResolveDiagnostics.
type drmEvidence struct {
	signals      []string
	keySystems   []string // asked for through EME
	active       []string // MediaKeys were created for
	systems      []string // DRM systems by name
	schemes      []string
	initData     []string // EME init data types seen
	licenseHosts []string
	renditions   int
	protected    int
}

func (d *drmEvidence) note(signal string) { d.signals = appendBounded(d.signals, signal) }

// usingEME reports that the page did more than probe EME support: it instantiated a
// CDM or reached encrypted playback. A bare requestMediaKeySystemAccess() probe --
// which clear-stream players and fingerprinting scripts both make -- does not count.
// Without this, such a probe would make any later POST to a licence-shaped URL look
// like a licence request and get a clear stream reported as DRM-protected.
func (d *drmEvidence) usingEME() bool {
	for _, s := range d.signals {
		if s != signalAccessRequested {
			return true
		}
	}
	return false
}

func (d *drmEvidence) confirmed() bool {
	for _, s := range d.signals {
		if confirmingSignals[s] {
			return true
		}
	}
	return false
}

// requestedSystems names the DRM systems the page asked the browser for.
func (d *drmEvidence) requestedSystems() []string {
	var names []string
	for _, ks := range d.keySystems {
		if name := drmSystemName(ks); name != "" {
			names = appendUnique(names, name)
		}
	}
	return names
}

func (d drmEvidence) clone() drmEvidence {
	c := d
	for _, list := range []*[]string{&c.signals, &c.keySystems, &c.active, &c.systems, &c.schemes, &c.initData, &c.licenseHosts} {
		*list = append([]string(nil), *list...)
	}
	return c
}

// noteManifest folds in what a manifest said about its own protection.
func (d *drmEvidence) noteManifest(shape manifestShape) {
	if len(shape.drm) == 0 {
		return
	}
	d.note(signalManifestProtected)
	for _, s := range shape.drm {
		d.systems = appendBounded(d.systems, s)
	}
	for _, s := range shape.schemes {
		d.schemes = appendBounded(d.schemes, s)
	}
	d.renditions = max(d.renditions, shape.renditions)
	d.protected = max(d.protected, shape.protected)
}

func (d *drmEvidence) diagnostics() *domain.DRMDiagnostics {
	if len(d.signals) == 0 {
		return nil
	}
	return &domain.DRMDiagnostics{
		Confirmed:           d.confirmed(),
		Systems:             d.systems,
		KeySystems:          d.keySystems,
		ActiveKeySystems:    d.active,
		Schemes:             d.schemes,
		InitDataTypes:       d.initData,
		LicenseHosts:        d.licenseHosts,
		Signals:             d.signals,
		ProtectedRenditions: d.protected,
		Renditions:          d.renditions,
	}
}

// ── what the page reports ───────────────────────────────────────────────────

// probeBinding is the CDP binding mediaProbeScript reports through. The script takes
// the function off the global object before any page script runs, so a page can
// neither see it nor feed it reports.
const probeBinding = "__inoxMediaProbe"

// mediaProbeScript runs in every frame before the page's own scripts and reports how
// the page uses Encrypted Media Extensions. It observes and never interferes: every
// wrapped method calls straight through and returns exactly what the original
// returns, and nothing about the page's DRM is changed, delayed or captured beyond the
// names below. From EME init data it keeps only PSSH system IDs -- public constants
// naming a DRM vendor -- never key IDs, licence challenges or responses.
const mediaProbeScript = `(() => {
  const report = globalThis.` + probeBinding + `;
  if (typeof report !== 'function') return;
  try { delete globalThis.` + probeBinding + `; } catch (e) {}
  let budget = 200;
  const send = (event) => {
    if (budget-- <= 0) return;
    try { report(JSON.stringify(event)); } catch (e) {}
  };
  const systems = (data) => {
    try {
      const u8 = ArrayBuffer.isView(data) ? new Uint8Array(data.buffer, data.byteOffset, data.byteLength) : new Uint8Array(data);
      const ids = [];
      for (let off = 0; off + 28 <= u8.length && ids.length < 8;) {
        const size = ((u8[off] << 24) | (u8[off + 1] << 16) | (u8[off + 2] << 8) | u8[off + 3]) >>> 0;
        if (size < 28 || off + size > u8.length) break;
        if (u8[off + 4] === 0x70 && u8[off + 5] === 0x73 && u8[off + 6] === 0x73 && u8[off + 7] === 0x68) {
          let id = '';
          for (let i = off + 12; i < off + 28; i++) id += (u8[i] < 16 ? '0' : '') + u8[i].toString(16);
          ids.push(id);
        }
        off += size;
      }
      return ids;
    } catch (e) { return []; }
  };
  const wrap = (proto, name, describe) => {
    const original = proto && proto[name];
    if (typeof original !== 'function') return;
    proto[name] = function () {
      try { const event = describe.apply(this, arguments); if (event) send(event); } catch (e) {}
      return original.apply(this, arguments);
    };
  };
  wrap(globalThis.Navigator && Navigator.prototype, 'requestMediaKeySystemAccess', (ks) => ({ t: 'access', ks: String(ks) }));
  wrap(globalThis.MediaKeySystemAccess && MediaKeySystemAccess.prototype, 'createMediaKeys', function () { return { t: 'keys', ks: String(this.keySystem) }; });
  wrap(globalThis.MediaKeys && MediaKeys.prototype, 'setServerCertificate', () => ({ t: 'certificate' }));
  wrap(globalThis.MediaKeys && MediaKeys.prototype, 'createSession', () => ({ t: 'session' }));
  wrap(globalThis.HTMLMediaElement && HTMLMediaElement.prototype, 'setMediaKeys', (keys) => (keys ? { t: 'attach' } : null));
  wrap(globalThis.MediaKeySession && MediaKeySession.prototype, 'generateRequest', (type, data) => ({ t: 'generate', idt: String(type), sys: String(type) === 'cenc' ? systems(data) : [] }));
  wrap(globalThis.MediaKeySession && MediaKeySession.prototype, 'update', () => ({ t: 'update' }));
  addEventListener('encrypted', (e) => send({ t: 'encrypted', idt: String(e.initDataType), sys: e.initDataType === 'cenc' && e.initData ? systems(e.initData) : [] }), true);
  addEventListener('waitingforkey', () => send({ t: 'waiting' }), true);
})()`

// probeEvent is one report from mediaProbeScript. It came from inside the page, so
// every field is validated before any of it is kept.
type probeEvent struct {
	Type         string   `json:"t"`
	KeySystem    string   `json:"ks"`
	InitDataType string   `json:"idt"`
	SystemIDs    []string `json:"sys"`
}

// maxProbePayload bounds a report; real ones are a few dozen bytes.
const maxProbePayload = 2048

// apply folds one probe report into the evidence, and reports whether it was a
// licence request being generated.
func (d *drmEvidence) apply(ev probeEvent) (licenceRequest bool) {
	switch ev.Type {
	case "access":
		d.note(signalAccessRequested)
		if ks := cleanKeySystem(ev.KeySystem); ks != "" {
			d.keySystems = appendBounded(d.keySystems, ks)
		}
	case "keys":
		d.note(signalKeysCreated)
		if ks := cleanKeySystem(ev.KeySystem); ks != "" {
			d.active = appendBounded(d.active, ks)
			if name := drmSystemName(ks); name != "" {
				d.systems = appendBounded(d.systems, name)
			}
		}
	case "certificate":
		d.note(signalServerCertificate)
	case "attach":
		d.note(signalKeysAttached)
	case "session":
		d.note(signalSessionCreated)
	case "generate":
		d.note(signalLicenseRequested)
		d.noteInitData(ev)
		return true
	case "update":
		d.note(signalLicenseApplied)
	case "encrypted":
		d.note(signalEncryptedMedia)
		d.noteInitData(ev)
	case "waiting":
		d.note(signalWaitingForKey)
	}
	return false
}

func (d *drmEvidence) noteInitData(ev probeEvent) {
	switch ev.InitDataType {
	case "cenc", "keyids", "webm", "sinf", "skd":
		d.initData = appendBounded(d.initData, ev.InitDataType)
	}
	for _, id := range ev.SystemIDs {
		raw, err := hex.DecodeString(id)
		if err != nil || len(raw) != 16 {
			continue
		}
		uuid := formatUUID(raw)
		name := drmSystemName(uuid)
		if name == "" {
			name = "DRM system " + uuid
		}
		d.systems = appendBounded(d.systems, name)
	}
}

// noteLicence records a licence request, by host only: licence URLs routinely carry
// tokens in their path and query.
func (d *drmEvidence) noteLicence(rawURL string) {
	d.note(signalLicenseServer)
	if u, err := url.Parse(rawURL); err == nil && u.Hostname() != "" {
		d.licenseHosts = appendBounded(d.licenseHosts, u.Hostname())
	}
}

// cleanKeySystem keeps an EME key system string only if it looks like one
// (com.widevine.alpha, org.w3.clearkey).
func cleanKeySystem(ks string) string {
	ks = strings.ToLower(strings.TrimSpace(ks))
	if ks == "" || len(ks) > 64 {
		return ""
	}
	for _, r := range ks {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-') {
			return ""
		}
	}
	return ks
}

// licenseMarkers are what licence server URLs are made of, across the DRM vendors
// and services in common use. Only POST requests are matched against them.
var licenseMarkers = []string{
	"license", "licence", "widevine", "playready", "fairplay", "clearkey",
	"drmtoday", "ezdrm", "axprod", "keyos", "buydrm", "expressplay", "vualto",
	"verimatrix", "irdeto", "castlabs", "rightsmanager", "/drm",
}

// licenseURL reports whether a POST to this URL looks like a licence request.
func licenseURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	target := strings.ToLower(u.Host + u.Path)
	for _, marker := range licenseMarkers {
		if strings.Contains(target, marker) {
			return true
		}
	}
	return strings.HasPrefix(strings.ToLower(u.Hostname()), "drm.")
}

func appendBounded(list []string, value string) []string {
	if len(list) >= maxEvidence {
		return list
	}
	return appendUnique(list, value)
}
