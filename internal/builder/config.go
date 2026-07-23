package builder

import (
	"encoding/json"
	"fmt"

	"github.com/singa/internal/node"
)

type RouteMode string

const (
	RouteModeWhitelist RouteMode = "whitelist"
	RouteModeGFWList   RouteMode = "gfwlist"
	RouteModeGlobal    RouteMode = "global"
)

// BuildConfig generates a complete sing-box config for node mode.
// All inbounds (dns-in, mixed-in, tproxy-in, redirect-in, tun-in) are
// intentionally omitted here; patchConfig injects them from SingaSettings
// at start time based on the user's proxy mode selection.
func BuildConfig(
	routeMode RouteMode,
	n *node.Node,
	ports Ports,
	lanProxy bool,
	ipv6 bool,
	srsDir string,
	isReF1nd bool,
	blockAds bool,
	fakeip bool,
) ([]byte, error) {
	proxyOB, err := NodeToOutbound(n, "proxy")
	if err != nil {
		return nil, fmt.Errorf("outbound: %w", err)
	}

	cfg := M{
		"dns":      buildDNS(routeMode, ipv6, fakeip),
		"inbounds": []interface{}{},
		"outbounds": []interface{}{
			proxyOB,
			M{"type": "direct", "tag": "direct"},
			M{"type": "block", "tag": "block"},
		},
		"route": buildRoute(routeMode, srsDir, isReF1nd, blockAds),
	}

	return json.MarshalIndent(cfg, "", "  ")
}

// ── DNS ────────────────────────────────────────────────────────────────────

func buildDNS(routeMode RouteMode, ipv6 bool, fakeip bool) M {
	strategy := "ipv4_only"
	if ipv6 {
		strategy = "prefer_ipv4"
	}

	servers := []interface{}{
		M{
			"type":   "tls",
			"tag":    "remote-dns",
			"server": "1.1.1.1",
			"detour": "proxy",
		},
		M{
			"type":   "udp",
			"tag":    "direct-dns",
			"server": "223.5.5.5",
		},
	}

	if fakeip {
		servers = append(servers, M{
			"type":        "fakeip",
			"tag":         "fakeip-dns",
			"inet4_range": "198.18.0.0/15",
			"inet6_range": "fc00::/18",
		})
	}

	var rules []interface{}
	var finalDNS string
	switch routeMode {
	case RouteModeWhitelist:
		rules = append(rules, M{
			"rule_set": []string{"geosite-cn"},
			"action":   "route",
			"server":   "direct-dns",
		})
		// fakeip cannot be the default (final) server; route A/AAAA queries to
		// fakeip-dns explicitly and fall back to remote-dns for everything else.
		if fakeip {
			rules = append(rules, M{
				"query_type": []string{"A", "AAAA"},
				"action":     "route",
				"server":     "fakeip-dns",
				"strategy":   strategy,
			})
		}
		finalDNS = "remote-dns"

	case RouteModeGFWList:
		// final="direct-dns", so fakeip is never the default server here.
		// Route GFW-listed domains to remote-dns (or fakeip-dns when enabled).
		target := "remote-dns"
		if fakeip {
			target = "fakeip-dns"
		}
		rules = append(rules, M{
			"rule_set": []string{"geosite-gfw", "geosite-geolocation-!cn"},
			"action":   "route",
			"server":   target,
		})
		finalDNS = "direct-dns"

	case RouteModeGlobal:
		// fakeip cannot be the default server either.
		if fakeip {
			rules = append(rules, M{
				"query_type": []string{"A", "AAAA"},
				"action":     "route",
				"server":     "fakeip-dns",
				"strategy":   strategy,
			})
		}
		finalDNS = "remote-dns"
	}

	dns := M{
		"servers":  servers,
		"rules":    rules,
		"final":    finalDNS,
		"strategy": strategy,
	}
	if fakeip {
		dns["independent_cache"] = true
	}
	return dns
}

// ── Route ──────────────────────────────────────────────────────────────────

func buildRoute(routeMode RouteMode, srsDir string, isReF1nd bool, blockAds bool) M {
	defaultResolver := "remote-dns"
	if routeMode == RouteModeGFWList {
		defaultResolver = "direct-dns"
	}

	return M{
		"rules":                   buildRouteRules(routeMode, isReF1nd, blockAds),
		"rule_set":                buildRuleSets(routeMode, srsDir, blockAds),
		"final":                   routeFinal(routeMode),
		"auto_detect_interface":   true,
		"default_domain_resolver": defaultResolver,
	}
}

func routeFinal(mode RouteMode) string {
	if mode == RouteModeGFWList {
		return "direct"
	}
	return "proxy"
}

func buildRouteRules(routeMode RouteMode, isReF1nd bool, blockAds bool) []interface{} {
	rules := []interface{}{
		M{"action": "sniff", "timeout": "500ms"},
		M{"inbound": []string{"dns-in"}, "action": "hijack-dns"},
	}

	if blockAds {
		rules = append(rules, M{"action": "reject", "rule_set": []string{"ads"}})
	}

	switch routeMode {
	case RouteModeWhitelist:
		rules = append(rules,
			M{"rule_set": []string{"geosite-cn"}, "outbound": "direct"},
		)
		if isReF1nd {
			rules = append(rules, M{"action": "resolve", "match_only": true})
		}
		rules = append(rules,
			M{"rule_set": []string{"geoip-cn"}, "outbound": "direct"},
		)

	case RouteModeGFWList:
		rules = append(rules,
			M{"rule_set": []string{"geosite-gfw", "geosite-geolocation-!cn"}, "outbound": "proxy"},
			M{"rule_set": []string{"geoip-telegram"}, "outbound": "proxy"},
		)

	case RouteModeGlobal:
		// final="proxy" routes everything
	}

	return rules
}

func buildRuleSets(routeMode RouteMode, srsDir string, blockAds bool) []interface{} {
	var tags []string

	switch routeMode {
	case RouteModeWhitelist:
		tags = append(tags, "geosite-cn", "geoip-cn")
	case RouteModeGFWList:
		tags = append(tags, "geosite-gfw", "geosite-geolocation-!cn", "geoip-telegram")
	}

	out := make([]interface{}, 0, len(tags)+1)
	for _, tag := range tags {
		out = append(out, M{
			"type":   "local",
			"tag":    tag,
			"format": "binary",
			"path":   srsDir + "/" + tag + ".srs",
		})
	}

	if blockAds {
		out = append(out, M{
			"type":   "local",
			"tag":    "ads",
			"format": "binary",
			"path":   srsDir + "/ads.srs",
		})
	}

	return out
}
