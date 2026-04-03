package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cloverstd/tcping/ping"
	"github.com/danbai225/gpp/backend/client"
	"github.com/danbai225/gpp/backend/config"
	"github.com/danbai225/gpp/backend/data"
	"github.com/sagernet/sing-box/option"
	"github.com/danbai225/gpp/systray"
	box "github.com/sagernet/sing-box"
	netutils "github.com/shirou/gopsutil/v3/net"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// App struct
type App struct {
	ctx      context.Context
	conf     *config.Config
	gamePeer *config.Peer
	httpPeer *config.Peer
	box      *box.Box
	lock     sync.Mutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	conf := config.Config{}
	app := App{
		conf: &conf,
	}
	return &app
}
func (a *App) systemTray() {
	systray.SetIcon(logo) // read the icon from a file
	show := systray.AddMenuItem("显示窗口", "显示窗口")
	systray.AddSeparator()
	exit := systray.AddMenuItem("退出加速器", "退出加速器")
	show.Click(func() { runtime.WindowShow(a.ctx) })
	exit.Click(func() {
		a.Stop()
		runtime.Quit(a.ctx)
		systray.Quit()
		time.Sleep(time.Second)
		os.Exit(0)
	})
	systray.SetOnClick(func(menu systray.IMenu) { runtime.WindowShow(a.ctx) })
	go func() {
		listener, err := net.Listen("tcp", "127.0.0.1:54713")
		if err != nil {
			_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
				Type:    runtime.ErrorDialog,
				Title:   "监听错误",
				Message: fmt.Sprintln("Error listening0:", err),
			})
		}
		var conn net.Conn
		for {
			conn, err = listener.Accept()
			if err != nil {
				_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
					Type:    runtime.ErrorDialog,
					Title:   "监听错误",
					Message: fmt.Sprintln("Error listening1:", err),
				})
				continue
			}
			// 读取指令
			buffer := make([]byte, 1024)
			n, err := conn.Read(buffer)
			if err != nil {
				_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
					Type:    runtime.ErrorDialog,
					Title:   "监听错误",
					Message: fmt.Sprintln("Error read:", err),
				})
				continue
			}
			command := string(buffer[:n])
			// 如果收到显示窗口的命令，则显示窗口
			if command == "SHOW_WINDOW" {
				// 展示窗口的代码
				runtime.WindowShow(a.ctx)
			}
			_ = conn.Close()
		}
	}()
}

func (a *App) testPing() {
	for {
		a.PingAll()
		time.Sleep(time.Second * 5)
	}
}
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	go systray.Run(a.systemTray, func() {})
	loadConfig, err := config.LoadConfig()
	if err != nil {
		_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.WarningDialog,
			Title:   "配置加载错误",
			Message: err.Error(),
		})
	} else {
		a.conf = loadConfig
	}
	if len(a.conf.PeerList) > 0 {
		if a.conf.GamePeer == "" {
			a.conf.GamePeer = a.conf.PeerList[0].Name
			a.gamePeer = a.conf.PeerList[0]
		} else {
			for _, peer := range a.conf.PeerList {
				if peer.Name == a.conf.GamePeer {
					a.gamePeer = peer
				}
			}
			if a.gamePeer == nil {
				a.gamePeer = a.conf.PeerList[0]
				a.conf.GamePeer = a.gamePeer.Name
			}
		}
		if a.conf.HTTPPeer == "" {
			a.conf.HTTPPeer = a.conf.PeerList[0].Name
			a.httpPeer = a.conf.PeerList[0]
		} else {
			for _, peer := range a.conf.PeerList {
				if peer.Name == a.conf.HTTPPeer {
					a.httpPeer = peer
				}
			}
			if a.httpPeer == nil {
				a.httpPeer = a.conf.PeerList[0]
				a.conf.HTTPPeer = a.httpPeer.Name
			}
		}
		_ = config.SaveConfig(a.conf)
	}
	go a.testPing()
}
func (a *App) PingAll() {
	a.lock.Lock()
	if a.box != nil {
		a.lock.Unlock()
		return
	}
	peers := make([]*config.Peer, 0, len(a.conf.PeerList))
	for _, p := range a.conf.PeerList {
		if p != nil {
			peers = append(peers, p)
		}
	}
	a.lock.Unlock()
	group := sync.WaitGroup{}
	for i := range peers {
		if peers[i].Protocol == "direct" {
			continue
		}
		group.Add(1)
		peer := peers[i]
		go func() {
			defer group.Done()
			pingVal := pingPort(peer.Addr, peer.Port)
			a.lock.Lock()
			peer.Ping = pingVal
			a.lock.Unlock()
		}()
	}
	group.Wait()
}

func (a *App) Status() *data.Status {
	a.lock.Lock()
	defer a.lock.Unlock()
	status := data.Status{
		Running:  a.box != nil,
		GamePeer: a.gamePeer,
		HttpPeer: a.httpPeer,
	}

	counters, _ := netutils.IOCounters(true)
	for _, counter := range counters {
		if counter.Name == "utun225" {
			status.Up = counter.BytesSent
			status.Down = counter.BytesRecv
		}
	}
	return &status
}

func (a *App) List() []*config.Peer {
	a.lock.Lock()
	config.EnsureDirectPeer(a.conf)
	list := make([]*config.Peer, 0, len(a.conf.PeerList))
	list = append(list, a.conf.PeerList...)
	a.lock.Unlock()
	sort.Slice(list, func(i, j int) bool { return list[i].Ping < list[j].Ping })
	return list
}
func (a *App) Add(token string) string {
	if a.conf.PeerList == nil {
		a.conf.PeerList = make([]*config.Peer, 0)
	}
	if strings.HasPrefix(token, "http") {
		a.conf.SubAddr = token
		config.NormalizeSubAddrs(a.conf)
		found := false
		for _, addr := range a.conf.SubAddrs {
			if addr == token {
				found = true
				break
			}
		}
		if !found {
			a.conf.SubAddrs = append(a.conf.SubAddrs, token)
		}
		if len(a.conf.SubAddrs) > 0 {
			a.conf.SubAddr = a.conf.SubAddrs[0]
		}
	} else {
		err, peer := config.ParsePeer(token)
		if err != nil {
			_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
				Type:    runtime.ErrorDialog,
				Title:   "导入错误",
				Message: err.Error(),
			})
			return err.Error()
		}
		for _, p := range a.conf.PeerList {
			if p.Name == peer.Name {
				_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
					Type:    runtime.ErrorDialog,
					Title:   "导入错误",
					Message: fmt.Sprintf("节点 %s 已存在", peer.Name),
				})
				return fmt.Sprintf("peer %s already exists", peer.Name)
			}
		}
		a.conf.PeerList = append(a.conf.PeerList, peer)
	}
	config.EnsureDirectPeer(a.conf)
	err := config.SaveConfig(a.conf)
	if err != nil {
		_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "导入错误",
			Message: err.Error(),
		})
		return err.Error()
	}
	return "ok"
}

func (a *App) ListSubscriptions() []string {
	a.lock.Lock()
	defer a.lock.Unlock()
	config.NormalizeSubAddrs(a.conf)
	out := make([]string, len(a.conf.SubAddrs))
	copy(out, a.conf.SubAddrs)
	return out
}

func (a *App) DeleteSubscription(addr string) string {
	a.lock.Lock()
	defer a.lock.Unlock()
	config.NormalizeSubAddrs(a.conf)
	out := make([]string, 0, len(a.conf.SubAddrs))
	for _, sub := range a.conf.SubAddrs {
		if sub != addr {
			out = append(out, sub)
		}
	}
	a.conf.SubAddrs = out
	if len(a.conf.SubAddrs) > 0 {
		a.conf.SubAddr = a.conf.SubAddrs[0]
	} else {
		a.conf.SubAddr = ""
	}
	if err := config.SaveConfig(a.conf); err != nil {
		return err.Error()
	}
	return "ok"
}

func (a *App) GetRuleText() string {
	a.lock.Lock()
	defer a.lock.Unlock()
	lines := make([]string, 0)
	for _, r := range a.conf.Rules {
		if r.Type != "default" {
			continue
		}
		outbound := strings.ToLower(r.DefaultOptions.Outbound)
		action := "PROXY"
		if outbound == "direct" {
			action = "DIRECT"
		}
		if len(r.DefaultOptions.DomainSuffix) > 0 {
			lines = append(lines, fmt.Sprintf("%s domain_suffix %s", action, strings.Join(r.DefaultOptions.DomainSuffix, ",")))
		}
	}
	if len(lines) == 0 {
		return "# 规则格式：\n# DIRECT domain_suffix steamcontent.com,cm.steampowered.com\n# PROXY domain_suffix steampowered.com,steamcommunity.com\n# 上面示例：下载走直连，商店走代理"
	}
	return strings.Join(lines, "\n")
}

func (a *App) SaveRuleText(text string) string {
	lines := strings.Split(text, "\n")
	parsed := make([]option.Rule, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			return fmt.Sprintf("规则格式错误: %s", line)
		}
		action := strings.ToUpper(parts[0])
		kind := strings.ToLower(parts[1])
		vals := strings.Split(strings.Join(parts[2:], " "), ",")
		clean := make([]string, 0)
		for _, v := range vals {
			v = strings.TrimSpace(v)
			if v != "" {
				clean = append(clean, v)
			}
		}
		if len(clean) == 0 {
			continue
		}
		outbound := "proxy"
		if action == "DIRECT" {
			outbound = "direct"
		} else if action != "PROXY" {
			return fmt.Sprintf("不支持的动作: %s", action)
		}
		if kind != "domain_suffix" {
			return fmt.Sprintf("暂只支持 domain_suffix: %s", kind)
		}
		parsed = append(parsed, option.Rule{
			Type: "default",
			DefaultOptions: option.DefaultRule{
				DomainSuffix: clean,
				Outbound:     outbound,
			},
		})
	}

	a.lock.Lock()
	a.conf.Rules = parsed
	config.EnsureDirectPeer(a.conf)
	err := config.SaveConfig(a.conf)
	a.lock.Unlock()
	if err != nil {
		return err.Error()
	}
	return "ok"
}
func (a *App) Del(Name string) string {
	for i, peer := range a.conf.PeerList {
		if peer.Name == Name {
			a.conf.PeerList = append(a.conf.PeerList[:i], a.conf.PeerList[i+1:]...)
			break
		}
	}
	err := config.SaveConfig(a.conf)
	if err != nil {
		return err.Error()
	}
	return "ok"
}
func (a *App) SetPeer(game, http string) string {
	for _, peer := range a.conf.PeerList {
		if peer.Name == game {
			a.gamePeer = peer
			a.conf.GamePeer = peer.Name
			break
		}
	}
	for _, peer := range a.conf.PeerList {
		if peer.Name == http {
			a.httpPeer = peer
			a.conf.HTTPPeer = peer.Name
			break
		}
	}
	err := config.SaveConfig(a.conf)
	if err != nil {
		_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "保存错误",
			Message: err.Error(),
		})
		return err.Error()
	}
	return "ok"
}

func (a *App) RefreshSubscription() string {
	a.lock.Lock()
	config.NormalizeSubAddrs(a.conf)
	if len(a.conf.SubAddrs) == 0 {
		a.lock.Unlock()
		return "订阅地址为空"
	}
	addrs := make([]string, len(a.conf.SubAddrs))
	copy(addrs, a.conf.SubAddrs)
	a.lock.Unlock()

	mergedPeers := make([]*config.Peer, 0)
	failed := 0
	for _, addr := range addrs {
		peers, err := config.FetchSubscription(addr, 10*time.Second)
		if err != nil {
			failed++
			continue
		}
		mergedPeers = config.MergePeers(mergedPeers, peers)
	}
	if len(mergedPeers) == 0 {
		if failed > 0 {
			return "全部订阅更新失败，请检查订阅链接是否有效"
		}
		return "未解析到可用节点"
	}

	a.lock.Lock()
	a.conf.PeerList = config.MergePeers(a.conf.PeerList, mergedPeers)
	config.EnsureDirectPeer(a.conf)
	if a.gamePeer == nil && len(a.conf.PeerList) > 0 {
		a.gamePeer = a.conf.PeerList[0]
		a.conf.GamePeer = a.gamePeer.Name
	}
	if a.httpPeer == nil && len(a.conf.PeerList) > 0 {
		a.httpPeer = a.conf.PeerList[0]
		a.conf.HTTPPeer = a.httpPeer.Name
	}
	saveErr := config.SaveConfig(a.conf)
	a.lock.Unlock()
	if saveErr != nil {
		return saveErr.Error()
	}
	return "ok"
}

func (a *App) BatchAdd(tokens string) string {
	lines := strings.Split(tokens, "\n")
	success := 0
	fail := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if a.Add(line) == "ok" {
			success++
		} else {
			fail++
		}
	}
	return fmt.Sprintf("导入完成: 成功 %d 条, 失败 %d 条", success, fail)
}

func (a *App) ExportConfig() string {
	a.lock.Lock()
	data, err := json.MarshalIndent(a.conf, "", "  ")
	a.lock.Unlock()
	if err != nil {
		return err.Error()
	}

	defaultName := fmt.Sprintf("gpp-config-%s.json", time.Now().Format("20060102-150405"))
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{DefaultFilename: defaultName})
	if err != nil {
		return err.Error()
	}
	if path == "" {
		return "cancel"
	}
	if filepath.Ext(path) == "" {
		path += ".json"
	}
	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		return err.Error()
	}
	return path
}

func (a *App) ImportConfig(merge bool) string {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Filters: []runtime.FileFilter{{DisplayName: "JSON Config", Pattern: "*.json"}}})
	if err != nil {
		return err.Error()
	}
	if path == "" {
		return "cancel"
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err.Error()
	}
	in := &config.Config{}
	err = json.Unmarshal(content, in)
	if err != nil {
		return err.Error()
	}

	a.lock.Lock()
	if merge {
		a.conf.PeerList = config.MergePeers(a.conf.PeerList, in.PeerList)
		if in.SubAddr != "" {
			a.conf.SubAddr = in.SubAddr
		}
		if len(in.Rules) > 0 {
			a.conf.Rules = in.Rules
		}
		if in.ProxyDNS != "" {
			a.conf.ProxyDNS = in.ProxyDNS
		}
		if in.LocalDNS != "" {
			a.conf.LocalDNS = in.LocalDNS
		}
	} else {
		a.conf = in
	}
	config.EnsureDirectPeer(a.conf)

	if len(a.conf.PeerList) > 0 {
		if a.conf.GamePeer == "" {
			a.conf.GamePeer = a.conf.PeerList[0].Name
		}
		if a.conf.HTTPPeer == "" {
			a.conf.HTTPPeer = a.conf.PeerList[0].Name
		}
	}
	a.gamePeer = nil
	a.httpPeer = nil
	for _, p := range a.conf.PeerList {
		if p.Name == a.conf.GamePeer {
			a.gamePeer = p
		}
		if p.Name == a.conf.HTTPPeer {
			a.httpPeer = p
		}
	}
	err = config.SaveConfig(a.conf)
	a.lock.Unlock()
	if err != nil {
		return err.Error()
	}
	return "ok"
}

// Start 启动加速
func (a *App) Start() string {
	a.lock.Lock()
	defer a.lock.Unlock()
	if a.box != nil {
		return "running"
	}
	var err error
	a.box, err = client.Client(a.gamePeer, a.httpPeer, a.conf.ProxyDNS, a.conf.LocalDNS, a.conf.Rules)
	if err != nil {
		_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "加速失败",
			Message: err.Error(),
		})
		a.box = nil
		return err.Error()
	}
	err = a.box.Start()
	if err != nil {
		_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "加速失败",
			Message: err.Error(),
		})
		a.box = nil
		return err.Error()
	}
	return "ok"
}

// Stop 停止加速
func (a *App) Stop() string {
	a.lock.Lock()
	defer a.lock.Unlock()
	if a.box == nil {
		return "not running"
	}
	err := a.box.Close()
	if err != nil {
		_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "停止失败",
			Message: err.Error(),
		})
		return err.Error()
	}
	a.box = nil
	return "ok"
}
func pingPort(host string, port uint16) uint {
	tcPing := ping.NewTCPing()
	tcPing.SetTarget(&ping.Target{
		Host:     host,
		Port:     int(port),
		Counter:  1,
		Interval: time.Millisecond * 200,
		Timeout:  time.Second * 3,
	})
	start := tcPing.Start()
	<-start
	result := tcPing.Result()
	return uint(result.Avg().Milliseconds())
}
func httpGet(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(resp.Body)
}
