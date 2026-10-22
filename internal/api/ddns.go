package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// DDNS 服务商。页面按一个服务商一行来增删改，接口的更新和删除也只提交 provider。
const (
	DDNSTencent    = "tencent"
	DDNSAliyun     = "aliyun"
	DDNSCloudflare = "cloudflare"
	DDNSDuckDNS    = "duckdns"
)

// DDNSRecord 是一行 DDNS 配置。主机名在 record4、record6 里，不是独立资源。
// AccessID 和 AccessKey 只用于提交，列出时由调用方去掉。
type DDNSRecord struct {
	Provider         string   `json:"provider"`
	Record4          []string `json:"record4"`
	Record6          []string `json:"record6"`
	AccessID         string   `json:"access_id,omitempty"`
	AccessKey        string   `json:"access_key,omitempty"`
	IPv4Enable       bool     `json:"ipv4_enable"`
	IPv6Enable       bool     `json:"ipv6_enable"`
	IPv4DetectMethod string   `json:"ipv4_detect_method,omitempty"`
	IPv6DetectMethod string   `json:"ipv6_detect_method,omitempty"`
	IPv4Interface    string   `json:"ipv4_interface,omitempty"`
	IPv6Interface    string   `json:"ipv6_interface,omitempty"`
	IPv6AddressIndex int      `json:"ipv6_address_index,omitempty"`
	SetRecord4       bool     `json:"-"`
	SetRecord6       bool     `json:"-"`
	Public4          string   `json:"public4,omitempty"`
	Public6          string   `json:"public6,omitempty"`
	UpdateTime       any      `json:"update_time,omitempty"`
}

// DDNSPublic 是可以打印的一行。不含 access_id 和 access_key。
type DDNSPublic struct {
	Provider         string   `json:"provider"`
	Record4          []string `json:"record4"`
	Record6          []string `json:"record6"`
	IPv4Enable       bool     `json:"ipv4_enable"`
	IPv6Enable       bool     `json:"ipv6_enable"`
	IPv4DetectMethod string   `json:"ipv4_detect_method,omitempty"`
	IPv6DetectMethod string   `json:"ipv6_detect_method,omitempty"`
	IPv4Interface    string   `json:"ipv4_interface,omitempty"`
	IPv6Interface    string   `json:"ipv6_interface,omitempty"`
	Public4          string   `json:"public4,omitempty"`
	Public6          string   `json:"public6,omitempty"`
	UpdateTime       any      `json:"update_time,omitempty"`
}

func (r DDNSRecord) Public() DDNSPublic {
	return DDNSPublic{
		Provider: r.Provider, Record4: r.Record4, Record6: r.Record6,
		IPv4Enable: r.IPv4Enable, IPv6Enable: r.IPv6Enable,
		IPv4DetectMethod: r.IPv4DetectMethod, IPv6DetectMethod: r.IPv6DetectMethod,
		IPv4Interface: r.IPv4Interface, IPv6Interface: r.IPv6Interface,
		Public4: r.Public4, Public6: r.Public6, UpdateTime: r.UpdateTime,
	}
}

// ValidDDNSProvider 报告服务商是否是页面提供的四个之一。
func ValidDDNSProvider(name string) bool {
	switch name {
	case DDNSTencent, DDNSAliyun, DDNSCloudflare, DDNSDuckDNS:
		return true
	default:
		return false
	}
}

// DDNSList 返回全部配置。密钥已去掉。
func (c *Client) DDNSList(ctx context.Context) ([]DDNSPublic, error) {
	rows, err := c.ddnsRaw(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]DDNSPublic, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Public())
	}
	return out, nil
}

func (c *Client) ddnsRaw(ctx context.Context) ([]DDNSRecord, error) {
	env, err := c.Get(ctx, "/zddns/v1/ddns/list", nil)
	if err != nil {
		return nil, err
	}
	var data struct {
		List []DDNSRecord `json:"list"`
	}
	if err := env.DecodeData(&data); err != nil {
		return nil, err
	}
	return data.List, nil
}

// DDNSCreate 新增一行。record4、record6 至少要有一个主机名。
func (c *Client) DDNSCreate(ctx context.Context, rec DDNSRecord) error {
	body, err := prepareDDNS(rec, true)
	if err != nil {
		return err
	}
	_, err = c.PostJSON(ctx, "/zddns/v1/ddns/create", nil, body)
	return err
}

// DDNSUpdate 按服务商覆盖一行。没给出的密钥和检测方式沿用当前记录。
func (c *Client) DDNSUpdate(ctx context.Context, rec DDNSRecord) error {
	if !ValidDDNSProvider(rec.Provider) {
		return fmt.Errorf("服务商必须是 tencent、aliyun、cloudflare、duckdns")
	}
	rows, err := c.ddnsRaw(ctx)
	if err != nil {
		return err
	}
	var current *DDNSRecord
	for i := range rows {
		if rows[i].Provider == rec.Provider {
			current = &rows[i]
			break
		}
	}
	if current == nil {
		return fmt.Errorf("没有 %s 的 DDNS 配置", rec.Provider)
	}
	merged := *current
	if rec.SetRecord4 {
		merged.Record4 = rec.Record4
	}
	if rec.SetRecord6 {
		merged.Record6 = rec.Record6
	}
	if rec.AccessID != "" {
		merged.AccessID = rec.AccessID
	}
	if rec.AccessKey != "" {
		merged.AccessKey = rec.AccessKey
	}
	merged.IPv4Enable = rec.IPv4Enable
	merged.IPv6Enable = rec.IPv6Enable
	if rec.IPv4DetectMethod != "" {
		merged.IPv4DetectMethod = rec.IPv4DetectMethod
	}
	if rec.IPv6DetectMethod != "" {
		merged.IPv6DetectMethod = rec.IPv6DetectMethod
	}
	if rec.IPv4Interface != "" {
		merged.IPv4Interface = rec.IPv4Interface
	}
	if rec.IPv6Interface != "" {
		merged.IPv6Interface = rec.IPv6Interface
	}
	body, err := prepareDDNS(merged, false)
	if err != nil {
		return err
	}
	_, err = c.PostJSON(ctx, "/zddns/v1/ddns/update", nil, body)
	return err
}

// DDNSDelete 按服务商删除整行。
func (c *Client) DDNSDelete(ctx context.Context, provider string) error {
	if !ValidDDNSProvider(provider) {
		return fmt.Errorf("服务商必须是 tencent、aliyun、cloudflare、duckdns")
	}
	_, err := c.PostJSON(ctx, "/zddns/v1/ddns/delete", nil, map[string]string{"provider": provider})
	return err
}

// DDNSRefresh 让极空间按当前公网 IP 更新全部配置。
func (c *Client) DDNSRefresh(ctx context.Context) error {
	_, err := c.PostJSON(ctx, "/zddns/v1/ddns/refresh", nil, map[string]any{})
	return err
}

func prepareDDNS(rec DDNSRecord, creating bool) (map[string]any, error) {
	if !ValidDDNSProvider(rec.Provider) {
		return nil, fmt.Errorf("服务商必须是 tencent、aliyun、cloudflare、duckdns")
	}
	rec.Record4 = cleanRecords(rec.Record4)
	rec.Record6 = cleanRecords(rec.Record6)
	if creating && len(rec.Record4) == 0 && len(rec.Record6) == 0 {
		return nil, fmt.Errorf("至少要一个 IPv4 或 IPv6 主机记录")
	}
	if len(rec.Record4) > 9 || len(rec.Record6) > 9 {
		return nil, fmt.Errorf("同一协议最多 9 个主机记录")
	}
	if rec.AccessKey == "" {
		return nil, fmt.Errorf("需要 access_key")
	}
	if rec.Provider != DDNSDuckDNS && rec.AccessID == "" {
		return nil, fmt.Errorf("需要 access_id")
	}
	if rec.IPv4DetectMethod == "" {
		rec.IPv4DetectMethod = "zspace"
	}
	if rec.IPv6DetectMethod == "" {
		rec.IPv6DetectMethod = "zspace"
	}
	for _, method := range []string{rec.IPv4DetectMethod, rec.IPv6DetectMethod} {
		if method != "zspace" && method != "interface" {
			return nil, fmt.Errorf("IP 检测方式必须是 zspace 或 interface")
		}
	}
	if rec.IPv4Enable && rec.IPv4DetectMethod == "interface" && rec.IPv4Interface == "" {
		return nil, fmt.Errorf("IPv4 使用网卡检测时需要网卡名")
	}
	if rec.IPv6Enable && rec.IPv6DetectMethod == "interface" && rec.IPv6Interface == "" {
		return nil, fmt.Errorf("IPv6 使用网卡检测时需要网卡名")
	}
	if rec.Provider == DDNSDuckDNS {
		rec.Record4 = suffixDuck(rec.Record4)
		rec.Record6 = suffixDuck(rec.Record6)
	}
	buf, err := json.Marshal(rec)
	if err != nil {
		return nil, err
	}
	var body map[string]any
	if err := json.Unmarshal(buf, &body); err != nil {
		return nil, err
	}
	return body, nil
}

func cleanRecords(in []string) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func suffixDuck(in []string) []string {
	out := make([]string, len(in))
	for i, item := range in {
		if strings.HasSuffix(item, ".duckdns.org") {
			out[i] = item
			continue
		}
		out[i] = item + ".duckdns.org"
	}
	return out
}
