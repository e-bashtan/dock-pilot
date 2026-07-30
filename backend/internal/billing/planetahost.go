package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type VDSInfo struct {
	ID         string
	IP         string
	IPs        []string
	Name       string
	ExpireDate string // YYYY-MM-DD
	Status     string
	Cost       string
}

type billmgrDoc struct {
	Doc struct {
		Elem json.RawMessage `json:"elem"`
		Auth json.RawMessage `json:"auth"`
	} `json:"doc"`
	Auth  json.RawMessage `json:"auth"`
	Error json.RawMessage `json:"error"`
}

// billmgrField accepts {"$":"x"}, {"$":123}, or bare string/number.
type billmgrField struct {
	Value string
}

func (f *billmgrField) UnmarshalJSON(b []byte) error {
	b = bytesTrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		f.Value = ""
		return nil
	}
	if b[0] == '{' {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(b, &obj); err != nil {
			return err
		}
		if raw, ok := obj["$"]; ok {
			f.Value = jsonScalarString(raw)
			return nil
		}
		f.Value = ""
		return nil
	}
	f.Value = jsonScalarString(b)
	return nil
}

func jsonScalarString(raw json.RawMessage) string {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String()
	}
	return strings.Trim(string(raw), `"`)
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func billmgrErrorMessage(raw json.RawMessage) string {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	if raw[0] == '"' {
		return jsonScalarString(raw)
	}
	if raw[0] == '{' {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return string(raw)
		}
		parts := []string{}
		for _, k := range []string{"obj", "msg", "code", "$"} {
			if v, ok := obj[k]; ok {
				if s := strings.TrimSpace(jsonScalarString(v)); s != "" {
					parts = append(parts, s)
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, ": ")
		}
	}
	return string(raw)
}

func extractSessionID(raw json.RawMessage) string {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	if raw[0] == '"' || (raw[0] >= '0' && raw[0] <= '9') {
		return jsonScalarString(raw)
	}
	if raw[0] == '{' {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return ""
		}
		for _, k := range []string{"$", "id", "auth"} {
			if v, ok := obj[k]; ok {
				if s := strings.TrimSpace(jsonScalarString(v)); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

type billmgrElem struct {
	ID         billmgrField `json:"id"`
	IP         billmgrField `json:"ip"`
	Name       billmgrField `json:"name"`
	Domain     billmgrField `json:"domain"`
	ExpireDate billmgrField `json:"expiredate"`
	RealExpire billmgrField `json:"real_expiredate"`
	ItemStatus billmgrField `json:"item_status"`
	Cost       billmgrField `json:"cost"`
	ItemCost   billmgrField `json:"item_cost"`
}

var ipv4InTextRe = regexp.MustCompile(`\b(\d{1,3}(?:\.\d{1,3}){3})\b`)

func fetchVDSList(ctx context.Context, billmgrURL, login, password string) ([]VDSInfo, error) {
	base := strings.TrimSpace(billmgrURL)
	if base == "" {
		base = "https://bill.planetahost.ru/billmgr"
	}
	client, err := newBillmgrClient()
	if err != nil {
		return nil, err
	}

	session, sessErr := billmgrLogin(ctx, client, base, login, password)
	if sessErr == nil && session != "" {
		list, err := fetchVDSWithAuth(ctx, client, base, "auth", session, "JSONdata")
		if err == nil {
			return list, nil
		}
		list, err2 := fetchVDSWithAuth(ctx, client, base, "auth", session, "json")
		if err2 == nil {
			return list, nil
		}
		if err != nil {
			return nil, err
		}
		return nil, err2
	}

	// Fallback for panels that still allow authinfo (Planetahost etc.).
	list, err := fetchVDSWithAuth(ctx, client, base, "authinfo", login+":"+password, "JSONdata")
	if err == nil && len(list) > 0 {
		return list, nil
	}
	fallback, err2 := fetchVDSWithAuth(ctx, client, base, "authinfo", login+":"+password, "json")
	if err2 != nil {
		if sessErr != nil {
			return nil, fmt.Errorf("billmgr session auth failed (%v); authinfo failed (%w)", sessErr, err2)
		}
		if err != nil {
			return nil, err
		}
		return nil, err2
	}
	if len(fallback) == 0 && sessErr != nil {
		return nil, sessErr
	}
	return fallback, nil
}

func newBillmgrClient() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
	}, nil
}

func billmgrLogin(ctx context.Context, client *http.Client, base, login, password string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid billmgr url: %w", err)
	}
	q := u.Query()
	q.Set("func", "auth")
	q.Set("username", login)
	q.Set("password", password)
	q.Set("out", "JSONdata")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("billmgr auth: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("billmgr auth HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}

	var envelope billmgrDoc
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", fmt.Errorf("billmgr auth json: %w", err)
	}
	if msg := billmgrErrorMessage(envelope.Error); msg != "" {
		return "", fmt.Errorf("billmgr auth: %s", msg)
	}
	session := extractSessionID(envelope.Doc.Auth)
	if session == "" {
		session = extractSessionID(envelope.Auth)
	}
	if session == "" {
		// Some panels only set cookies; still usable via jar.
		return "cookie", nil
	}
	return session, nil
}

func fetchVDSWithAuth(ctx context.Context, client *http.Client, base, authParam, authValue, outFmt string) ([]VDSInfo, error) {
	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("invalid billmgr url: %w", err)
	}
	q := u.Query()
	q.Set("func", "vds")
	q.Set("out", outFmt)
	if authParam != "" && authValue != "" && authValue != "cookie" {
		q.Set(authParam, authValue)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("billmgr request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("billmgr HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}

	var envelope billmgrDoc
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("billmgr json: %w", err)
	}
	if msg := billmgrErrorMessage(envelope.Error); msg != "" {
		return nil, fmt.Errorf("billmgr: %s", msg)
	}

	elems, err := parseElems(envelope.Doc.Elem)
	if err != nil {
		return nil, err
	}
	out := make([]VDSInfo, 0, len(elems))
	for _, e := range elems {
		expire := strings.TrimSpace(e.RealExpire.Value)
		if expire == "" {
			expire = strings.TrimSpace(e.ExpireDate.Value)
		}
		cost := strings.TrimSpace(e.Cost.Value)
		if cost == "" {
			cost = strings.TrimSpace(e.ItemCost.Value)
		}
		name := strings.TrimSpace(e.Name.Value)
		if name == "" {
			name = strings.TrimSpace(e.Domain.Value)
		}
		ips := collectIPs(e.IP.Value, name, e.Domain.Value)
		primary := ""
		if len(ips) > 0 {
			primary = ips[0]
		} else {
			primary = strings.TrimSpace(e.IP.Value)
		}
		out = append(out, VDSInfo{
			ID:         strings.TrimSpace(e.ID.Value),
			IP:         primary,
			IPs:        ips,
			Name:       name,
			ExpireDate: expire,
			Status:     strings.TrimSpace(e.ItemStatus.Value),
			Cost:       cost,
		})
	}
	return out, nil
}

func parseElems(raw json.RawMessage) ([]billmgrElem, error) {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if raw[0] == '[' {
		var list []billmgrElem
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, err
		}
		return list, nil
	}
	var one billmgrElem
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, err
	}
	return []billmgrElem{one}, nil
}

func collectIPs(parts ...string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(ip string) {
		ip = strings.TrimSpace(ip)
		if ip == "" || seen[ip] {
			return
		}
		if net.ParseIP(ip) == nil {
			return
		}
		seen[ip] = true
		out = append(out, ip)
	}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		for _, chunk := range strings.FieldsFunc(p, func(r rune) bool {
			return r == ',' || r == ';' || r == ' ' || r == '/' || r == '\n' || r == '\t'
		}) {
			add(chunk)
		}
		for _, m := range ipv4InTextRe.FindAllString(p, -1) {
			add(m)
		}
	}
	return out
}

func findByIP(list []VDSInfo, ip string) (VDSInfo, bool) {
	want := strings.TrimSpace(ip)
	if want == "" {
		return VDSInfo{}, false
	}
	for _, v := range list {
		if v.IP == want {
			return v, true
		}
		for _, candidate := range v.IPs {
			if candidate == want {
				return v, true
			}
		}
		if strings.Contains(v.Name, want) {
			return v, true
		}
	}
	return VDSInfo{}, false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
