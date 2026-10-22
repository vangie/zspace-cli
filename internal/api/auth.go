package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Credentials 是桌面客户端 vuex.json 里的登录态。
// Token 只留在内存里，命令和 MCP 都不会把它写进输出。
type Credentials struct {
	Token      string
	NasID      string
	DeviceID   string
	Username   string
	Device     string
	AppVersion string
}

// LocateVuex 寻找 vuex.json。
// 顺序：显式目录、ZS_CONFIG_DIR、平台默认位置。
func LocateVuex(configDir string) (string, error) {
	var dirs []string
	switch {
	case configDir != "":
		dirs = []string{configDir}
	case os.Getenv("ZS_CONFIG_DIR") != "":
		dirs = []string{os.Getenv("ZS_CONFIG_DIR")}
	default:
		dirs = defaultConfigDirs()
	}
	var tried []string
	for _, d := range dirs {
		p := filepath.Join(d, "vuex.json")
		tried = append(tried, p)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("未找到极空间登录态 vuex.json，已查找：%s\n请先打开并登录极空间客户端，或用 --config-dir / ZS_CONFIG_DIR 指定目录",
		joinLines(tried))
}

func defaultConfigDirs() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return []string{filepath.Join(home, "Library", "Application Support", "zspace")}
	case "windows":
		var dirs []string
		for _, env := range []string{"APPDATA", "LOCALAPPDATA", "USERPROFILE"} {
			if v := os.Getenv(env); v != "" {
				dirs = append(dirs, filepath.Join(v, "zspace"))
			}
		}
		if len(dirs) == 0 {
			dirs = []string{filepath.Join(home, "zspace")}
		}
		return dirs
	default:
		return []string{
			filepath.Join(home, ".zspace"),
			filepath.Join(home, ".config", "zspace"),
			filepath.Join(home, "zspace"),
		}
	}
}

func joinLines(ss []string) string {
	out := ""
	for _, s := range ss {
		out += "\n  " + s
	}
	return out
}

// LoadCredentials 从 vuex.json 读取登录态。文件不存在或缺少 token 时返回错误。
func LoadCredentials(configDir string) (Credentials, error) {
	p, err := LocateVuex(configDir)
	if err != nil {
		return Credentials{}, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return Credentials{}, err
	}
	var root struct {
		State struct {
			User struct {
				Token    string `json:"token"`
				Username string `json:"username"`
			} `json:"user"`
			Nas struct {
				NasID string `json:"nasId"`
			} `json:"nas"`
			App struct {
				DeviceID string `json:"deviceId"`
				Device   string `json:"device"`
				Version  string `json:"version"`
			} `json:"app"`
		} `json:"state"`
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		return Credentials{}, fmt.Errorf("解析 %s 失败：%w", p, err)
	}
	c := Credentials{
		Token:      root.State.User.Token,
		NasID:      root.State.Nas.NasID,
		DeviceID:   root.State.App.DeviceID,
		Username:   root.State.User.Username,
		Device:     root.State.App.Device,
		AppVersion: root.State.App.Version,
	}
	if c.Token == "" {
		return Credentials{}, errors.New("vuex.json 里没有 user.token，请在极空间客户端重新登录")
	}
	if c.AppVersion == "" {
		c.AppVersion = "1.0"
	}
	return c, nil
}
