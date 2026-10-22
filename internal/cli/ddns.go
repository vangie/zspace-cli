package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/vangie/zspace-cli/internal/api"
)

func cmdDDNS(ctx context.Context, c *api.Client, args []string, o opts) (any, int, error) {
	sub := "list"
	if len(args) > 0 {
		sub = args[0]
		args = args[1:]
	}
	switch sub {
	case "list":
		return call(c.DDNSList(ctx))
	case "refresh":
		if err := confirm(o, "刷新全部 DDNS 记录"); err != nil {
			return nil, 2, err
		}
		if err := c.DDNSRefresh(ctx); err != nil {
			return nil, 1, err
		}
		return map[string]any{"ok": true}, 0, nil
	case "add":
		rec, err := ddnsFromFlags(o, true)
		if err != nil {
			return nil, 2, err
		}
		if err := confirm(o, "新增 DDNS "+rec.Provider); err != nil {
			return nil, 2, err
		}
		if err := c.DDNSCreate(ctx, rec); err != nil {
			return nil, 1, err
		}
		return map[string]any{"provider": rec.Provider, "ok": true}, 0, nil
	case "update":
		rec, err := ddnsFromFlags(o, false)
		if err != nil {
			return nil, 2, err
		}
		if err := confirm(o, "更新 DDNS "+rec.Provider); err != nil {
			return nil, 2, err
		}
		if err := c.DDNSUpdate(ctx, rec); err != nil {
			return nil, 1, err
		}
		return map[string]any{"provider": rec.Provider, "ok": true}, 0, nil
	case "rm":
		if o.provider == "" {
			return nil, 2, errors.New("用法：zspace ddns rm --provider <tencent|aliyun|cloudflare|duckdns> --yes")
		}
		if err := confirm(o, "删除 DDNS "+o.provider); err != nil {
			return nil, 2, err
		}
		if err := c.DDNSDelete(ctx, o.provider); err != nil {
			return nil, 1, err
		}
		return map[string]any{"provider": o.provider, "ok": true}, 0, nil
	default:
		return nil, 2, errors.New("用法：zspace ddns <list|add|update|rm|refresh>")
	}
}

func ddnsFromFlags(o opts, creating bool) (api.DDNSRecord, error) {
	if o.provider == "" {
		return api.DDNSRecord{}, errors.New("需要 --provider tencent|aliyun|cloudflare|duckdns")
	}
	if !api.ValidDDNSProvider(o.provider) {
		return api.DDNSRecord{}, fmt.Errorf("服务商必须是 tencent、aliyun、cloudflare、duckdns")
	}
	rec := api.DDNSRecord{
		Provider:         o.provider,
		Record4:          splitList(o.record4),
		Record6:          splitList(o.record6),
		SetRecord4:       o.record4 != "",
		SetRecord6:       o.record6 != "",
		AccessID:         o.accessID,
		AccessKey:        o.accessKey,
		IPv4Enable:       o.ipv4,
		IPv6Enable:       o.ipv6,
		IPv4DetectMethod: o.ipv4Method,
		IPv6DetectMethod: o.ipv6Method,
		IPv4Interface:    o.ipv4Interface,
		IPv6Interface:    o.ipv6Interface,
	}
	if creating && len(rec.Record4) == 0 && len(rec.Record6) == 0 {
		return api.DDNSRecord{}, errors.New("需要 --record4 或 --record6")
	}
	if creating && rec.AccessKey == "" {
		return api.DDNSRecord{}, errors.New("需要 --access-key")
	}
	if creating && rec.Provider != api.DDNSDuckDNS && rec.AccessID == "" {
		return api.DDNSRecord{}, errors.New("需要 --access-id")
	}
	return rec, nil
}

func splitList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
