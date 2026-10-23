package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDDNSCreateAndListRedactsSecrets(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery+" "+string(b))
		switch r.URL.Path {
		case "/zddns/v1/ddns/list":
			_, _ = w.Write([]byte(`{"code":"200","data":{"list":[{"provider":"duckdns","record4":["home.duckdns.org"],"access_id":"ID","access_key":"SECRET","ipv4_enable":true}]}}`))
		default:
			_, _ = w.Write([]byte(`{"code":"200","data":{}}`))
		}
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Creds: Credentials{Token: "T", NasID: "N", DeviceID: "D"}, HTTP: srv.Client()}
	ctx := context.Background()

	rows, err := c.DDNSList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	blob, _ := io.ReadAll(strings.NewReader(mustJSON(t, rows)))
	if strings.Contains(string(blob), "SECRET") || strings.Contains(string(blob), "access_key") {
		t.Fatalf("列表不应含密钥：%s", blob)
	}

	err = c.DDNSCreate(ctx, DDNSRecord{
		Provider: DDNSDuckDNS, Record4: []string{"home"}, AccessKey: "TOKEN", IPv4Enable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = c.DDNSDelete(ctx, DDNSDuckDNS)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{
		"POST /zddns/v1/ddns/create", `"record4":["home.duckdns.org"]`, `"access_key":"TOKEN"`,
		"POST /zddns/v1/ddns/delete", `"provider":"duckdns"`,
		"GET /zddns/v1/ddns/list", "token=T",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("缺少 %s\n%s", want, joined)
		}
	}
	if err := c.DDNSCreate(ctx, DDNSRecord{Provider: "other", Record4: []string{"a"}, AccessKey: "k"}); err == nil {
		t.Fatal("非法服务商应被拒绝")
	}
}

func TestDDNSUpdateKeepsExistingKey(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if r.URL.Path == "/zddns/v1/ddns/update" {
			body = string(b)
		}
		if r.URL.Path == "/zddns/v1/ddns/list" {
			_, _ = w.Write([]byte(`{"code":"200","data":{"list":[{"provider":"cloudflare","record4":["nas.example"],"record6":[],"access_id":"OLDID","access_key":"OLDKEY","ipv4_enable":true,"ipv4_detect_method":"zspace"}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":"200","data":{}}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Creds: Credentials{Token: "T", NasID: "N", DeviceID: "D"}, HTTP: srv.Client()}
	err := c.DDNSUpdate(context.Background(), DDNSRecord{
		Provider: DDNSCloudflare, Record4: []string{"home.example"}, SetRecord4: true, IPv4Enable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"record4":["home.example"]`, `"access_id":"OLDID"`, `"access_key":"OLDKEY"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("更新应保留原密钥并替换主机记录：%s", body)
		}
	}
}

func TestCloudflareCreateAllowsEmptyZoneID(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		_, _ = w.Write([]byte(`{"code":"200","data":{}}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Creds: Credentials{Token: "T", NasID: "N", DeviceID: "D"}, HTTP: srv.Client()}
	err := c.DDNSCreate(context.Background(), DDNSRecord{
		Provider: DDNSCloudflare, Record4: []string{"nas.example.com"}, AccessKey: "cf-token", IPv4Enable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, `"access_id"`) {
		t.Fatalf("Cloudflare 未填 ZONE ID 时不应提交 access_id：%s", body)
	}
	if !strings.Contains(body, `"access_key":"cf-token"`) {
		t.Fatalf("应提交 API Token：%s", body)
	}
	err = c.DDNSCreate(context.Background(), DDNSRecord{
		Provider: DDNSAliyun, Record4: []string{"nas.example.com"}, AccessKey: "secret", IPv4Enable: true,
	})
	if err == nil {
		t.Fatal("阿里云缺少 access_id 应被拒绝")
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
