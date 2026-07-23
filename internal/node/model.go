package node

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type Protocol string

const (
	ProtoVMess     Protocol = "vmess"
	ProtoVLESS     Protocol = "vless"
	ProtoTrojan    Protocol = "trojan"
	ProtoSS        Protocol = "ss"
	ProtoTUIC      Protocol = "tuic"
	ProtoHysteria2 Protocol = "hysteria2"
	ProtoHTTP      Protocol = "http"
	ProtoHTTPS     Protocol = "https"
	ProtoSOCKS5    Protocol = "socks5"
	ProtoWireGuard Protocol = "wireguard"
)

type Node struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Protocol Protocol `json:"protocol"`
	Address  string   `json:"address"`
	Port     int      `json:"port"`

	// Auth
	UUID     string `json:"uuid,omitempty"`
	Password string `json:"password,omitempty"`
	Method   string `json:"method,omitempty"`

	// VMess
	AlterID  int    `json:"alter_id,omitempty"`
	Security string `json:"security,omitempty"`

	// VLESS / Trojan
	Flow       string `json:"flow,omitempty"`
	Encryption string `json:"encryption,omitempty"`

	// Transport
	Network  string `json:"network,omitempty"`
	Path     string `json:"path,omitempty"`
	Host     string `json:"host,omitempty"`
	GrpcSvc  string `json:"grpc_svc,omitempty"`
	GrpcMode string `json:"grpc_mode,omitempty"`

	// TLS
	TLS         string `json:"tls,omitempty"`
	SNI         string `json:"sni,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	ALPN        string `json:"alpn,omitempty"`
	Insecure    bool   `json:"insecure,omitempty"`

	// Reality
	PublicKey string `json:"public_key,omitempty"`
	ShortID   string `json:"short_id,omitempty"`
	SpiderX   string `json:"spider_x,omitempty"`

	// TUIC
	CongestionControl string `json:"congestion_control,omitempty"`

	// Hysteria2
	ObfsType     string `json:"obfs_type,omitempty"`
	ObfsPassword string `json:"obfs_password,omitempty"`
	Ports        string `json:"ports,omitempty"`
	PinSHA256    string `json:"pin_sha256,omitempty"`

	// WireGuard
	PrivateKey   string `json:"private_key,omitempty"`
	PublicKeyWG  string `json:"public_key_wg,omitempty"` // peer public key (WG)
	PreSharedKey string `json:"pre_shared_key,omitempty"`
	Reserved     string `json:"reserved,omitempty"`   // comma-sep 3 bytes or base64
	LocalAddress string `json:"local_address,omitempty"` // comma-sep CIDRs
	MTU          int    `json:"mtu,omitempty"`
}

func NewID() string {
	return uuid.New().String()
}

// FromMap constructs a Node from a sing-box proxy map (as stored in subscription cache).
// Only the fields needed by BuildConfig are extracted.
func FromMap(m map[string]any) (*Node, error) {
	str := func(key string) string {
		v, _ := m[key].(string)
		return v
	}
	intVal := func(key string) int {
		switch v := m[key].(type) {
		case float64:
			return int(v)
		case int:
			return v
		}
		return 0
	}
	boolVal := func(key string) bool {
		v, _ := m[key].(bool)
		return v
	}

	typ := str("type")
	var proto Protocol
	switch typ {
	case "vmess":
		proto = ProtoVMess
	case "vless":
		proto = ProtoVLESS
	case "trojan":
		proto = ProtoTrojan
	case "shadowsocks":
		proto = ProtoSS
	case "tuic":
		proto = ProtoTUIC
	case "hysteria2":
		proto = ProtoHysteria2
	case "http":
		proto = ProtoHTTP
	case "https":
		proto = ProtoHTTPS
	case "socks":
		proto = ProtoSOCKS5
	case "wireguard":
		proto = ProtoWireGuard
	default:
		return nil, fmt.Errorf("unsupported proxy type %q", typ)
	}

	n := &Node{
		Name:     str("tag"),
		Protocol: proto,
		Address:  str("server"),
		Port:     intVal("server_port"),
		UUID:     str("uuid"),
		Password: str("password"),
		Method:   str("method"),
		AlterID:  intVal("alter_id"),
		Security: str("security"),
		Flow:     str("flow"),
	}
	if n.Name == "" {
		n.Name = str("name")
	}

	// TLS
	if tls, ok := m["tls"].(map[string]any); ok {
		tlsStr := func(k string) string { v, _ := tls[k].(string); return v }
		tlsBool := func(k string) bool { v, _ := tls[k].(bool); return v }
		n.TLS = "tls"
		n.SNI = tlsStr("server_name")
		n.Fingerprint = tlsStr("utls")
		n.Insecure = tlsBool("insecure")
		if alpn, ok := tls["alpn"].([]interface{}); ok && len(alpn) > 0 {
			parts := make([]string, 0, len(alpn))
			for _, a := range alpn {
				if s, ok := a.(string); ok {
					parts = append(parts, s)
				}
			}
			n.ALPN = strings.Join(parts, ",")
		}
		if reality, ok := tls["reality"].(map[string]any); ok {
			realStr := func(k string) string { v, _ := reality[k].(string); return v }
			n.PublicKey = realStr("public_key")
			n.ShortID = realStr("short_id")
			n.TLS = "reality"
		}
	}

	// Transport
	if transport, ok := m["transport"].(map[string]any); ok {
		tStr := func(k string) string { v, _ := transport[k].(string); return v }
		n.Network = tStr("type")
		n.Path = tStr("path")
		if host, ok := transport["headers"].(map[string]any); ok {
			if h, ok := host["Host"].(string); ok {
				n.Host = h
			}
		}
		n.GrpcSvc = tStr("service_name")
	}

	// Hysteria2
	if obfs, ok := m["obfs"].(map[string]any); ok {
		n.ObfsType, _ = obfs["type"].(string)
		n.ObfsPassword, _ = obfs["password"].(string)
	}
	n.PinSHA256 = str("tls_insecure_skip_verify") // placeholder; actual field differs
	n.Ports = str("brutal_bitrate")
	_ = boolVal("tcp_fast_open")

	// TUIC
	n.CongestionControl = str("congestion_control")

	// WireGuard
	n.PrivateKey   = str("private_key")
	n.PreSharedKey = str("pre_shared_key")
	n.Reserved     = str("reserved")
	if mtu, ok := m["mtu"].(float64); ok {
		n.MTU = int(mtu)
	}
	if peer, ok := m["peers"].([]interface{}); ok && len(peer) > 0 {
		if p, ok := peer[0].(map[string]any); ok {
			if pk, ok := p["public_key"].(string); ok {
				n.PublicKeyWG = pk
			}
			if psk, ok := p["pre_shared_key"].(string); ok && n.PreSharedKey == "" {
				n.PreSharedKey = psk
			}
		}
	}
	if la, ok := m["local_address"].([]interface{}); ok {
		parts := make([]string, 0, len(la))
		for _, v := range la {
			if s, ok := v.(string); ok {
				parts = append(parts, s)
			}
		}
		n.LocalAddress = strings.Join(parts, ",")
	}

	return n, nil
}
