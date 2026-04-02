package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sagernet/sing-box/option"
	"io"
	"net/http"
	"net/netip"
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
	err = json.Unmarshal(data, &peers)
	if err != nil {
		return nil, err
	}
	return peers, nil
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
	if conf.SubAddr != "" {
		var peers []*Peer
		peers, err = FetchSubscription(conf.SubAddr, 10*time.Second)
		if err != nil {
			return nil, err
		}
		conf.PeerList = MergePeers(conf.PeerList, peers)
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
