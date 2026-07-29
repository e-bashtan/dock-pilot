package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type VDSInfo struct {
	ID         string
	IP         string
	Name       string
	ExpireDate string // YYYY-MM-DD
	Status     string
	Cost       string
}

type billmgrDoc struct {
	Doc struct {
		Elem json.RawMessage `json:"elem"`
	} `json:"doc"`
	Error string `json:"error"`
}

type billmgrField struct {
	Value string `json:"$"`
}

type billmgrElem struct {
	ID           billmgrField `json:"id"`
	IP           billmgrField `json:"ip"`
	Name         billmgrField `json:"name"`
	ExpireDate   billmgrField `json:"expiredate"`
	RealExpire   billmgrField `json:"real_expiredate"`
	ItemStatus   struct {
		Value string `json:"$"`
	} `json:"item_status"`
	Cost billmgrField `json:"cost"`
}

func fetchVDSList(ctx context.Context, billmgrURL, login, password string) ([]VDSInfo, error) {
	base := strings.TrimSpace(billmgrURL)
	if base == "" {
		base = "https://bill.planetahost.ru/billmgr"
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("invalid billmgr url: %w", err)
	}
	q := u.Query()
	q.Set("authinfo", login+":"+password)
	q.Set("func", "vds")
	q.Set("out", "json")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 30 * time.Second}
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
	if strings.TrimSpace(envelope.Error) != "" {
		return nil, fmt.Errorf("billmgr: %s", envelope.Error)
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
		out = append(out, VDSInfo{
			ID:         strings.TrimSpace(e.ID.Value),
			IP:         strings.TrimSpace(e.IP.Value),
			Name:       strings.TrimSpace(e.Name.Value),
			ExpireDate: expire,
			Status:     strings.TrimSpace(e.ItemStatus.Value),
			Cost:       strings.TrimSpace(e.Cost.Value),
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

func findByIP(list []VDSInfo, ip string) (VDSInfo, bool) {
	want := strings.TrimSpace(ip)
	for _, v := range list {
		if v.IP == want {
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
