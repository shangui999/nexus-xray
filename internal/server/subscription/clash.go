package subscription

import (
	"fmt"

	"github.com/shangui999/nexus-xray/internal/server/model"
	"gopkg.in/yaml.v3"
)

// ClashConfig Clash 完整配置
// 注意：VLESS 协议需要 Clash Meta (mihomo) 内核支持，标准 Clash 不支持
type ClashConfig struct {
	Port               int               `yaml:"port"`
	SocksPort          int               `yaml:"socks-port"`
	AllowLan           bool              `yaml:"allow-lan"`
	Mode               string            `yaml:"mode"`
	LogLevel           string            `yaml:"log-level"`
	ExternalController string            `yaml:"external-controller"`
	DNS                *ClashDNS         `yaml:"dns"`
	Proxies            []ClashProxy      `yaml:"proxies"`
	ProxyGroups        []ClashProxyGroup `yaml:"proxy-groups"`
	Rules              []string          `yaml:"rules"`
}

// ClashDNS Clash DNS 配置
type ClashDNS struct {
	Enable     bool     `yaml:"enable"`
	IPv6       bool     `yaml:"ipv6"`
	NameServer []string `yaml:"nameserver"`
	Fallback   []string `yaml:"fallback"`
}

// ClashProxy Clash 代理节点
type ClashProxy struct {
	Name              string        `yaml:"name"`
	Type              string        `yaml:"type"`                        // vless/trojan
	Server            string        `yaml:"server"`
	Port              int           `yaml:"port"`
	UUID              string        `yaml:"uuid"`
	Password          string        `yaml:"password,omitempty"`          // trojan 使用
	Network           string        `yaml:"network,omitempty"`           // tcp/ws/grpc
	TLS               bool          `yaml:"tls,omitempty"`
	UDP               bool          `yaml:"udp,omitempty"`
	Flow              string        `yaml:"flow,omitempty"`              // xtls-rprx-vision
	Servername        string        `yaml:"servername,omitempty"`        // SNI
	RealityOpts       *RealityOpts  `yaml:"reality-opts,omitempty"`
	ClientFingerprint string        `yaml:"client-fingerprint,omitempty"`
	SkipCertVerify    bool          `yaml:"skip-cert-verify,omitempty"`
}

// RealityOpts REALITY 协议选项
type RealityOpts struct {
	PublicKey string `yaml:"public-key"`
	ShortID   string `yaml:"short-id,omitempty"`
}

// ClashProxyGroup Clash 代理组
type ClashProxyGroup struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"` // select/url-test/fallback
	Proxies  []string `yaml:"proxies"`
	URL      string   `yaml:"url,omitempty"`
	Interval int      `yaml:"interval,omitempty"`
}

// GenerateClashConfig 生成 Clash YAML 配置
func (g *Generator) GenerateClashConfig(user *model.User, nodes []NodeConfig) ([]byte, error) {
	var proxies []ClashProxy
	var proxyNames []string

	for _, node := range nodes {
		// 确定主地址
		primaryAddr := node.Address
		if node.SubscriptionAddr != "" {
			primaryAddr = node.SubscriptionAddr
		}

		// 生成主节点代理
		proxy := g.nodeToClashProxy(node, user, primaryAddr, node.NodeName)
		proxies = append(proxies, proxy)
		proxyNames = append(proxyNames, proxy.Name)

		// IPv6 节点（Clash server 字段直接写 IPv6，不需要方括号）
		if node.IPv6Addr != "" {
			ipv6Proxy := g.nodeToClashProxy(node, user, node.IPv6Addr, node.NodeName+" [IPv6]")
			proxies = append(proxies, ipv6Proxy)
			proxyNames = append(proxyNames, ipv6Proxy.Name)
		}
	}

	config := &ClashConfig{
		Port:               7890,
		SocksPort:          7891,
		AllowLan:           false,
		Mode:               "rule",
		LogLevel:           "info",
		ExternalController: "127.0.0.1:9090",
		DNS: &ClashDNS{
			Enable:     true,
			IPv6:       true,
			NameServer: []string{"8.8.8.8", "1.1.1.1"},
			Fallback:   []string{"tls://8.8.4.4:853", "tls://1.0.0.1:853"},
		},
		Proxies: proxies,
		ProxyGroups: []ClashProxyGroup{
			{
				Name:    "🚀 节点选择",
				Type:    "select",
				Proxies: append([]string{"♻️ 自动选择", "DIRECT"}, proxyNames...),
			},
			{
				Name:     "♻️ 自动选择",
				Type:     "url-test",
				Proxies:  proxyNames,
				URL:      "http://www.gstatic.com/generate_204",
				Interval: 300,
			},
			{
				Name:    "🎯 全球直连",
				Type:    "select",
				Proxies: []string{"DIRECT", "🚀 节点选择"},
			},
			{
				Name:    "🛑 广告拦截",
				Type:    "select",
				Proxies: []string{"REJECT", "DIRECT"},
			},
		},
		Rules: []string{
			"DOMAIN-SUFFIX,local,DIRECT",
			"IP-CIDR,127.0.0.0/8,DIRECT",
			"IP-CIDR,172.16.0.0/12,DIRECT",
			"IP-CIDR,192.168.0.0/16,DIRECT",
			"IP-CIDR,10.0.0.0/8,DIRECT",
			"GEOIP,CN,🎯 全球直连",
			"MATCH,🚀 节点选择",
		},
	}

	return yaml.Marshal(config)
}

// nodeToClashProxy 将节点配置转换为 Clash 代理
func (g *Generator) nodeToClashProxy(node NodeConfig, user *model.User, address, name string) ClashProxy {
	proxy := ClashProxy{
		Name:   name,
		Server: address,
		Port:   node.Port,
		UUID:   node.UUID,
		UDP:    true,
	}

	switch node.Protocol {
	case "vless":
		proxy.Type = "vless"

		// 从 StreamSettings 提取 network
		if ss, ok := node.StreamSettings["network"]; ok {
			proxy.Network = fmt.Sprintf("%v", ss)
		} else {
			proxy.Network = "tcp"
		}

		// 处理 security = reality
		if security, ok := node.StreamSettings["security"]; ok && fmt.Sprintf("%v", security) == "reality" {
			proxy.TLS = true
			proxy.RealityOpts = &RealityOpts{}
			if pbk, ok := node.StreamSettings["pbk"]; ok {
				proxy.RealityOpts.PublicKey = fmt.Sprintf("%v", pbk)
			}
			if sid, ok := node.StreamSettings["sid"]; ok {
				proxy.RealityOpts.ShortID = fmt.Sprintf("%v", sid)
			}
			if sni, ok := node.StreamSettings["sni"]; ok {
				proxy.Servername = fmt.Sprintf("%v", sni)
			}
			if fp, ok := node.StreamSettings["fp"]; ok {
				proxy.ClientFingerprint = fmt.Sprintf("%v", fp)
			}
		}

		// 处理 security = tls
		if security, ok := node.StreamSettings["security"]; ok && fmt.Sprintf("%v", security) == "tls" {
			proxy.TLS = true
			if sni, ok := node.StreamSettings["sni"]; ok {
				proxy.Servername = fmt.Sprintf("%v", sni)
			}
		}

		// flow (xtls-rprx-vision)
		if flow, ok := node.StreamSettings["flow"]; ok {
			proxy.Flow = fmt.Sprintf("%v", flow)
		}

	case "trojan":
		proxy.Type = "trojan"
		proxy.Password = node.UUID
		proxy.TLS = true

		if sni, ok := node.StreamSettings["sni"]; ok {
			proxy.Servername = fmt.Sprintf("%v", sni)
		}
		if network, ok := node.StreamSettings["network"]; ok {
			proxy.Network = fmt.Sprintf("%v", network)
		}
		if security, ok := node.StreamSettings["security"]; ok {
			if fmt.Sprintf("%v", security) == "none" {
				proxy.TLS = false
				proxy.SkipCertVerify = true
			}
		}

	default:
		// 默认作为 vless 处理
		proxy.Type = "vless"
		if ss, ok := node.StreamSettings["network"]; ok {
			proxy.Network = fmt.Sprintf("%v", ss)
		} else {
			proxy.Network = "tcp"
		}
	}

	return proxy
}
