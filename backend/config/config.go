package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sagernet/sing-box/option"
	"gopkg.in/yaml.v3"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var Debug atomic.Bool

type Peer struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Port     uint16 `json:"port"`
	Addr     string `json:"addr"`
	UUID     string `json:"uuid"`
	Cipher   string `json:"cipher,omitempty"`
	Ping     uint   `json:"ping"`
}

func (p *Peer) Domain() string {
	host := strings.Split(p.Addr, ":")[0]
	_, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	return "placeholder.com"
}

type Config struct {
	PeerList []*Peer       `json:"peer_list"`
	SubAddr  string        `json:"sub_addr"`
	SubAddrs []string      `json:"sub_addrs"`
	Rules    []option.Rule `json:"rules"`
	GamePeer string        `json:"game_peer"`
	HTTPPeer string        `json:"http_peer"`
	ProxyDNS string        `json:"proxy_dns"`
	LocalDNS string        `json:"local_dns"`
	Debug    bool          `json:"debug"`
}

func ensureDirectPeer(conf *Config) {
	if conf.PeerList == nil {
		conf.PeerList = make([]*Peer, 0)
	}
	for _, peer := range conf.PeerList {
		if peer != nil && peer.Name == "直连" {
			return
		}
	}
	conf.PeerList = append(conf.PeerList, &Peer{Name: "直连", Protocol: "direct", Port: 0, Addr: "127.0.0.1", UUID: "", Ping: 0})
}

func ensureDefaults(conf *Config) {
	ensureDirectPeer(conf)
	if conf.ProxyDNS == "" {
		conf.ProxyDNS = "https://1.1.1.1/dns-query"
	}
	if conf.LocalDNS == "" {
		conf.LocalDNS = "https://223.5.5.5/dns-query"
	}
}

func MergePeers(existing, incoming []*Peer) []*Peer {
	set := make(map[string]*Peer)
	for _, peer := range existing {
		if peer == nil || peer.Name == "" {
			continue
		}
		set[peer.Name] = peer
	}
	for _, peer := range incoming {
		if peer == nil || peer.Name == "" {
			continue
		}
		set[peer.Name] = peer
	}
	out := make([]*Peer, 0, len(set))
	for _, peer := range set {
		out = append(out, peer)
	}
	return out
}

func FetchSubscription(subAddr string, timeout time.Duration) ([]*Peer, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(subAddr)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	peers := make([]*Peer, 0)
	if err = json.Unmarshal(data, &peers); err == nil {
		return peers, nil
	}
	if peers, err = ParseSSLinks(string(data)); err == nil {
		return peers, nil
	}
	trimmed := strings.TrimSpace(string(data))
	if decoded, decErr := decodeBase64Text(trimmed); decErr == nil {
		if peers, err = ParseSSLinks(decoded); err == nil {
			return peers, nil
		}
	}
	return ParseClashSubscription(data)
}

func decodeBase64Text(s string) (string, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", ""))
	if s == "" {
		return "", errors.New("empty")
	}
	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		b, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "="))
		if err != nil {
			return "", err
		}
	}
	return string(b), nil
}

func ParseSSLinks(text string) ([]*Peer, error) {
	lines := strings.Split(text, "\n")
	out := make([]*Peer, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "ss://") {
			continue
		}
		peer, err := parseSSLink(line)
		if err != nil {
			continue
		}
		// 过滤机场套餐提示行
		if strings.Contains(peer.Name, "剩余流量") || strings.Contains(peer.Name, "套餐到期") || strings.Contains(peer.Name, "重置") {
			continue
		}
		out = append(out, peer)
	}
	if len(out) == 0 {
		return nil, errors.New("no ss links")
	}
	return out, nil
}

func parseSSLink(link string) (*Peer, error) {
	body := strings.TrimPrefix(strings.TrimSpace(link), "ss://")
	name := ""
	if idx := strings.Index(body, "#"); idx >= 0 {
		namePart := body[idx+1:]
		body = body[:idx]
		if dec, err := url.QueryUnescape(namePart); err == nil {
			name = dec
		} else {
			name = namePart
		}
	}
	// strip query like ?plugin=
	if idx := strings.Index(body, "?"); idx >= 0 {
		body = body[:idx]
	}

	userInfo := ""
	hostPort := ""
	if strings.Contains(body, "@") {
		parts := strings.SplitN(body, "@", 2)
		userInfo = parts[0]
		hostPort = parts[1]
	} else {
		decoded, err := decodeBase64Text(body)
		if err != nil {
			return nil, err
		}
		parts := strings.SplitN(decoded, "@", 2)
		if len(parts) != 2 {
			return nil, errors.New("invalid ss decoded format")
		}
		userInfo = parts[0]
		hostPort = parts[1]
	}

	decodedUserInfo, err := decodeBase64Text(userInfo)
	if err == nil && strings.Contains(decodedUserInfo, ":") {
		userInfo = decodedUserInfo
	}
	creds := strings.SplitN(userInfo, ":", 2)
	if len(creds) != 2 {
		return nil, errors.New("invalid ss userinfo")
	}
	cipher := creds[0]
	password := creds[1]

	addrPort, err := netip.ParseAddrPort(hostPort)
	if err != nil {
		if strings.Count(hostPort, ":") == 1 {
			arr := strings.SplitN(hostPort, ":", 2)
			port, pErr := strconv.Atoi(arr[1])
			if pErr != nil {
				return nil, pErr
			}
			if name == "" {
				name = fmt.Sprintf("%s:%d", arr[0], port)
			}
			return &Peer{Name: name, Protocol: "shadowsocks", Addr: arr[0], Port: uint16(port), UUID: password, Cipher: cipher}, nil
		}
		return nil, err
	}
	if name == "" {
		name = addrPort.Addr().String()
	}
	return &Peer{Name: name, Protocol: "shadowsocks", Addr: addrPort.Addr().String(), Port: addrPort.Port(), UUID: password, Cipher: cipher}, nil
}

type clashProxy struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Server   string `yaml:"server"`
	Port     int    `yaml:"port"`
	UUID     string `yaml:"uuid"`
	Password string `yaml:"password"`
	Cipher   string `yaml:"cipher"`
}

type clashConfig struct {
	Proxies []clashProxy `yaml:"proxies"`
}

func ParseClashSubscription(data []byte) ([]*Peer, error) {
	var cfg clashConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	peers := make([]*Peer, 0, len(cfg.Proxies))
	for _, p := range cfg.Proxies {
		protocol := ""
		switch strings.ToLower(p.Type) {
		case "vless":
			protocol = "vless"
		case "hysteria2", "hy2":
			protocol = "hysteria2"
		case "ss", "shadowsocks":
			protocol = "shadowsocks"
		case "socks5", "socks":
			protocol = "socks"
		default:
			continue
		}
		if p.Name == "" || p.Server == "" || p.Port <= 0 || p.Port > 65535 {
			continue
		}
		uuid := p.UUID
		if uuid == "" {
			uuid = p.Password
		}
		peers = append(peers, &Peer{
			Name:     p.Name,
			Protocol: protocol,
			Port:     uint16(p.Port),
			Addr:     p.Server,
			UUID:     uuid,
			Cipher:   p.Cipher,
		})
	}
	if len(peers) == 0 {
		return nil, errors.New("未解析到可用节点（仅支持 vless/hysteria2/ss/socks）")
	}
	return peers, nil
}

func NormalizeSubAddrs(conf *Config) {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, addr := range conf.SubAddrs {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		out = append(out, addr)
	}
	if conf.SubAddr != "" {
		if _, ok := seen[conf.SubAddr]; !ok {
			out = append(out, conf.SubAddr)
		}
	}
	conf.SubAddrs = out
	if len(conf.SubAddrs) > 0 {
		conf.SubAddr = conf.SubAddrs[0]
	}
}

func InitConfig() {
	home, _ := os.UserHomeDir()
	_path := "config.json"
	_, err := os.Stat(_path)
	if err != nil {
		_path = fmt.Sprintf("%s%c%s%c%s", home, os.PathSeparator, ".gpp", os.PathSeparator, "config.json")
	}
	_ = os.MkdirAll(filepath.Dir(_path), 0o755)
	_, err = os.Stat(_path)
	if err != nil {
		file, _ := json.Marshal(Config{PeerList: make([]*Peer, 0)})
		err = os.WriteFile(_path, file, 0o644)
	}
}
func LoadConfig() (*Config, error) {
	home, _ := os.UserHomeDir()
	_path := "config.json"
	_, err := os.Stat(_path)
	if err != nil {
		_path = fmt.Sprintf("%s%c%s%c%s", home, os.PathSeparator, ".gpp", os.PathSeparator, "config.json")
	}
	file, _ := os.ReadFile(_path)
	conf := &Config{PeerList: make([]*Peer, 0)}
	err = json.Unmarshal(file, &conf)
	ensureDefaults(conf)
	NormalizeSubAddrs(conf)
	if len(conf.SubAddrs) > 0 {
		for _, addr := range conf.SubAddrs {
			var peers []*Peer
			peers, err = FetchSubscription(addr, 10*time.Second)
			if err != nil {
				continue
			}
			conf.PeerList = MergePeers(conf.PeerList, peers)
		}
		ensureDirectPeer(conf)
	}
	if conf.Debug {
		Debug.Swap(true)
	}
	return conf, err
}
func SaveConfig(config *Config) error {
	home, _ := os.UserHomeDir()
	_path := "config.json"
	_, err := os.Stat(_path)
	if err != nil {
		_path = fmt.Sprintf("%s%c%s%c%s", home, os.PathSeparator, ".gpp", os.PathSeparator, "config.json")
	}
	file, _ := json.MarshalIndent(config, "", " ")
	return os.WriteFile(_path, file, 0o600)
}
func ParsePeer(token string) (error, *Peer) {
	split := strings.Split(token, "#")
	name := ""
	if len(split) == 2 {
		token = split[0]
		name = split[1]
	}
	tokenBytes, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return err, nil
	}
	token = string(tokenBytes)
	split = strings.Split(token, "@")
	protocol := strings.ReplaceAll(split[0], "gpp://", "")
	switch protocol {
	case "vless", "shadowsocks", "socks", "hysteria2":
	default:
		return fmt.Errorf("unknown protocol: %s", protocol), nil
	}
	if len(split) != 2 {
		return fmt.Errorf("invalid token: %s", token), nil
	}
	split = strings.Split(split[1], "/")
	addr := strings.Split(split[0], ":")
	if len(addr) != 2 {
		return errors.New("invalid addr: " + split[0]), nil
	}
	if len(split) != 2 {
		return fmt.Errorf("invalid token: %s", token), nil
	}
	uuid := split[1]
	if name == "" {
		name = fmt.Sprintf("%s:%s", addr[0], addr[1])
	}
	port, _ := strconv.ParseInt(addr[1], 10, 64)
	return nil, &Peer{
		Name:     name,
		Protocol: protocol,
		Port:     uint16(port),
		Addr:     addr[0],
		UUID:     uuid,
	}
}
