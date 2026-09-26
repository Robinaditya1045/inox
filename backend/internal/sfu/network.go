package sfu

import (
	"fmt"
	"strings"

	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v3"
)

// NetworkConfig describes how the SFU is reachable from the public internet.
//
// On a NAT'd cloud VM (Oracle, EC2, GCE) the host only ever sees its private
// address, so Pion gathers host candidates like 10.0.0.x that no browser can
// route to, and binds them to a random ephemeral UDP port that no firewall rule
// can anticipate. PublicIP and the port range fix both halves of that.
type NetworkConfig struct {
	// ICEServers is the comma-separated STUN/TURN URL list from WEBRTC_ICE_SERVERS.
	ICEServers string
	// PortMin and PortMax bound the UDP ports Pion binds media to, so the same
	// narrow range can be opened in the firewall. Zero on both disables pinning.
	PortMin uint16
	PortMax uint16
	// PublicIP, when set, replaces the IP of gathered host candidates. Leave it
	// empty when the server is directly addressable or running locally.
	PublicIP string
}

// DefaultICEServers is the fallback used when no ICE servers are configured.
func DefaultICEServers() []webrtc.ICEServer {
	return []webrtc.ICEServer{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
	}
}

// ParseICEServers turns a comma-separated URL list into Pion's ICE server form.
// Blank entries are skipped; an empty result falls back to DefaultICEServers.
func ParseICEServers(raw string) []webrtc.ICEServer {
	urls := make([]string, 0, 2)
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			urls = append(urls, trimmed)
		}
	}
	if len(urls) == 0 {
		return DefaultICEServers()
	}
	return []webrtc.ICEServer{{URLs: urls}}
}

// buildAPI assembles a webrtc.API carrying cfg's NAT and port settings.
//
// Building an API by hand means opting out of the defaults the package-level
// constructor applies, so the media engine and interceptor registry have to be
// populated explicitly — otherwise the SFU negotiates zero codecs and loses
// NACK/PLI handling.
func buildAPI(cfg NetworkConfig) (*webrtc.API, error) {
	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		return nil, fmt.Errorf("failed to register default webrtc codecs: %w", err)
	}

	registry := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(mediaEngine, registry); err != nil {
		return nil, fmt.Errorf("failed to register default webrtc interceptors: %w", err)
	}

	settingEngine := webrtc.SettingEngine{}
	if cfg.PortMin > 0 && cfg.PortMax >= cfg.PortMin {
		if err := settingEngine.SetEphemeralUDPPortRange(cfg.PortMin, cfg.PortMax); err != nil {
			return nil, fmt.Errorf("failed to pin webrtc udp port range %d-%d: %w", cfg.PortMin, cfg.PortMax, err)
		}
	}
	if cfg.PublicIP != "" {
		settingEngine.SetNAT1To1IPs([]string{cfg.PublicIP}, webrtc.ICECandidateTypeHost)
	}

	return webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithInterceptorRegistry(registry),
		webrtc.WithSettingEngine(settingEngine),
	), nil
}
