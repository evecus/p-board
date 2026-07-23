package builder

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/singa/internal/node"
)

// NodeToOutbound converts a parsed Node to a sing-box outbound map.
func NodeToOutbound(n *node.Node, tag string) (map[string]interface{}, error) {
	switch n.Protocol {
	case node.ProtoVMess:
		return vmessOB(n, tag)
	case node.ProtoVLESS:
		return vlessOB(n, tag)
	case node.ProtoTrojan:
		return trojanOB(n, tag)
	case node.ProtoSS:
		return ssOB(n, tag)
	case node.ProtoTUIC:
		return tuicOB(n, tag)
	case node.ProtoHysteria2:
		return hy2OB(n, tag)
	case node.ProtoHTTP:
		return httpOB(n, tag)
	case node.ProtoHTTPS:
		return httpsOB(n, tag)
	case node.ProtoSOCKS5:
		return socks5OB(n, tag)
	case node.ProtoWireGuard:
		return wireguardOB(n, tag)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", n.Protocol)
	}
}

func vmessOB(n *node.Node, tag string) (map[string]interface{}, error) {
	ob := M{
		"type":        "vmess",
		"tag":         tag,
		"server":      n.Address,
		"server_port": n.Port,
		"uuid":        n.UUID,
		"alter_id":    n.AlterID,
		"security":    nonEmpty(n.Security, "auto"),
	}
	setTransport(ob, n)
	setTLS(ob, n)
	return ob, nil
}

func vlessOB(n *node.Node, tag string) (map[string]interface{}, error) {
	ob := M{
		"type":        "vless",
		"tag":         tag,
		"server":      n.Address,
		"server_port": n.Port,
		"uuid":        n.UUID,
	}
	if n.Flow != "" {
		ob["flow"] = n.Flow
	}
	setTransport(ob, n)
	setTLS(ob, n)
	return ob, nil
}

func trojanOB(n *node.Node, tag string) (map[string]interface{}, error) {
	ob := M{
		"type":        "trojan",
		"tag":         tag,
		"server":      n.Address,
		"server_port": n.Port,
		"password":    n.Password,
	}
	if n.Flow != "" {
		ob["flow"] = n.Flow
	}
	setTransport(ob, n)
	setTLS(ob, n)
	return ob, nil
}

func ssOB(n *node.Node, tag string) (map[string]interface{}, error) {
	return M{
		"type":        "shadowsocks",
		"tag":         tag,
		"server":      n.Address,
		"server_port": n.Port,
		"method":      n.Method,
		"password":    n.Password,
	}, nil
}

func tuicOB(n *node.Node, tag string) (map[string]interface{}, error) {
	ob := M{
		"type":                "tuic",
		"tag":                 tag,
		"server":              n.Address,
		"server_port":         n.Port,
		"uuid":                n.UUID,
		"password":            n.Password,
		"congestion_control":  nonEmpty(n.CongestionControl, "bbr"),
	}
	setTLS(ob, n)
	return ob, nil
}

func hy2OB(n *node.Node, tag string) (map[string]interface{}, error) {
	ob := M{
		"type":     "hysteria2",
		"tag":      tag,
		"server":   n.Address,
		"password": n.Password,
	}
	if n.Ports != "" {
		ob["server_port"] = n.Ports
	} else {
		ob["server_port"] = n.Port
	}
	if n.ObfsType != "" {
		ob["obfs"] = M{
			"type":     n.ObfsType,
			"password": n.ObfsPassword,
		}
	}
	setTLS(ob, n)
	return ob, nil
}

func httpOB(n *node.Node, tag string) (map[string]interface{}, error) {
	ob := M{
		"type":        "http",
		"tag":         tag,
		"server":      n.Address,
		"server_port": n.Port,
	}
	if n.UUID != "" || n.Password != "" {
		ob["username"] = n.UUID
		ob["password"] = n.Password
	}
	return ob, nil
}

func httpsOB(n *node.Node, tag string) (map[string]interface{}, error) {
	ob := M{
		"type":        "http",
		"tag":         tag,
		"server":      n.Address,
		"server_port": n.Port,
	}
	if n.UUID != "" || n.Password != "" {
		ob["username"] = n.UUID
		ob["password"] = n.Password
	}
	setTLS(ob, n)
	return ob, nil
}

func socks5OB(n *node.Node, tag string) (map[string]interface{}, error) {
	ob := M{
		"type":        "socks",
		"tag":         tag,
		"server":      n.Address,
		"server_port": n.Port,
		"version":     "5",
	}
	if n.UUID != "" || n.Password != "" {
		ob["username"] = n.UUID
		ob["password"] = n.Password
	}
	return ob, nil
}

func wireguardOB(n *node.Node, tag string) (map[string]interface{}, error) {
	peer := M{
		"server":      n.Address,
		"server_port": n.Port,
		"public_key":  n.PublicKeyWG,
	}
	if n.PreSharedKey != "" {
		peer["pre_shared_key"] = n.PreSharedKey
	}
	if n.Reserved != "" {
		// reserved can be "0,0,0" (decimal bytes) or base64; pass as-is,
		// sing-box accepts both string and array forms.
		parts := strings.Split(n.Reserved, ",")
		if len(parts) == 3 {
			nums := make([]int, 0, 3)
			ok := true
			for _, p := range parts {
				p = strings.TrimSpace(p)
				v, err := strconv.Atoi(p)
				if err != nil {
					ok = false
					break
				}
				nums = append(nums, v)
			}
			if ok {
				peer["reserved"] = nums
			} else {
				peer["reserved"] = n.Reserved // base64 fallback
			}
		} else {
			peer["reserved"] = n.Reserved
		}
	}

	ob := M{
		"type":        "wireguard",
		"tag":         tag,
		"private_key": n.PrivateKey,
		"peers":       []M{peer},
	}
	if n.LocalAddress != "" {
		ob["local_address"] = strings.Split(n.LocalAddress, ",")
	}
	if n.MTU > 0 {
		ob["mtu"] = n.MTU
	}
	return ob, nil
}

// ── TLS ────────────────────────────────────────────────────────────────────

func setTLS(ob M, n *node.Node) {
	if n.TLS == "" {
		return
	}
	tls := M{"enabled": true}
	if n.SNI != "" {
		tls["server_name"] = n.SNI
	}
	if n.Insecure {
		tls["insecure"] = true
	}
	if n.Fingerprint != "" {
		tls["utls"] = M{"enabled": true, "fingerprint": n.Fingerprint}
	}
	if n.ALPN != "" {
		tls["alpn"] = strings.Split(n.ALPN, ",")
	}
	if n.TLS == "reality" {
		reality := M{"enabled": true, "public_key": n.PublicKey}
		if n.ShortID != "" {
			reality["short_id"] = n.ShortID
		}
		tls["reality"] = reality
		delete(tls, "insecure")
	}
	ob["tls"] = tls
}

// ── Transport ──────────────────────────────────────────────────────────────

func setTransport(ob M, n *node.Node) {
	t := transportObj(n)
	if t != nil {
		ob["transport"] = t
	}
}

func transportObj(n *node.Node) M {
	switch n.Network {
	case "ws":
		t := M{"type": "ws"}
		if n.Path != "" {
			t["path"] = n.Path
		}
		if n.Host != "" {
			t["headers"] = M{"Host": n.Host}
		}
		return t
	case "grpc":
		t := M{"type": "grpc"}
		if n.GrpcSvc != "" {
			t["service_name"] = n.GrpcSvc
		}
		return t
	case "http":
		t := M{"type": "http"}
		if n.Host != "" {
			t["host"] = []string{n.Host}
		}
		if n.Path != "" {
			t["path"] = n.Path
		}
		return t
	case "httpupgrade":
		t := M{"type": "httpupgrade"}
		if n.Host != "" {
			t["host"] = n.Host
		}
		if n.Path != "" {
			t["path"] = n.Path
		}
		return t
	case "xhttp":
		t := M{"type": "splithttp"}
		if n.Host != "" {
			t["host"] = n.Host
		}
		if n.Path != "" {
			t["path"] = n.Path
		}
		return t
	default:
		return nil
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────

type M = map[string]interface{}

func nonEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
