package waf

import (
	"encoding/json"
	"hash/fnv"
	"strings"
)

// Transformer applies WAF-specific filtering and enrichment.
type Transformer struct {
	ACLAllowlist          map[string]bool
	ActionAllowlist       map[string]bool
	SampleAllowPercent    int
	EnableGeoIP           bool
	EnableCountryCentroid bool
}

// WAFLog documents expected WAF line fields when decoding JSON.
type WAFLog struct {
	Timestamp         int64  `json:"timestamp"`
	Action            string `json:"action"`
	TerminatingRuleID string `json:"terminatingRuleId,omitempty"`
	HTTPRequest       struct {
		ClientIP string `json:"clientIp"`
		Country  string `json:"country"`
	} `json:"httpRequest"`
}

// WafACLNameFromS3Key extracts the ACL segment from standard WAF S3 key layout.
func WafACLNameFromS3Key(key string) string {
	parts := strings.Split(key, "/")
	idx := -1
	for i, p := range parts {
		if p == "WAFLogs" {
			idx = i
			break
		}
	}
	if idx < 0 || idx+2 >= len(parts) {
		return "unknown"
	}
	acl := strings.TrimSpace(parts[idx+2])
	if acl == "" {
		return "unknown"
	}
	return acl
}

// Transform filters and enriches one WAF line then returns output and keep flag.
func (t Transformer) Transform(line, acl string) (string, bool) {
	var obj map[string]any
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return line, true
	}

	if len(t.ACLAllowlist) > 0 && !t.ACLAllowlist[acl] {
		return "", false
	}

	action := str(obj, "action")
	if len(t.ActionAllowlist) > 0 && !t.ActionAllowlist[action] {
		return "", false
	}

	// Sample only ALLOW records when configured.
	if t.SampleAllowPercent < 100 && strings.EqualFold(action, "ALLOW") {
		if !keepSample(line, t.SampleAllowPercent) {
			return "", false
		}
	}

	obj["waf_acl"] = acl
	enrichHTTPRequest(obj, t)

	out, err := json.Marshal(obj)
	if err != nil {
		return line, true
	}
	return string(out), true
}

func enrichHTTPRequest(obj map[string]any, t Transformer) {
	hr, ok := obj["httpRequest"].(map[string]any)
	if !ok {
		return
	}

	country := strings.ToUpper(strings.TrimSpace(str(hr, "country")))
	if country != "" {
		obj["country"] = country
		if t.EnableCountryCentroid {
			if c, ok := countryCentroids[country]; ok {
				obj["country_lat"] = c[0]
				obj["country_lon"] = c[1]
			}
		}
	}

	ip := strings.TrimSpace(str(hr, "clientIp"))
	if ip != "" {
		obj["clientIp"] = ip
		// GeoIP resolution intentionally disabled by default in this worker image because
		// embedding a full IP DB increases image size and update complexity.
		_ = t.EnableGeoIP
	}

	if u := buildRequestURL(hr); u != "" {
		obj["request_url"] = u
	}
}

// headerValue returns the first matching WAF httpRequest.headers[].value (name match is case-insensitive).
func headerValue(headers any, wantName string) string {
	arr, ok := headers.([]any)
	if !ok {
		return ""
	}
	want := strings.ToLower(wantName)
	for _, raw := range arr {
		hm, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		n, _ := hm["name"].(string)
		if strings.ToLower(strings.TrimSpace(n)) != want {
			continue
		}
		v, _ := hm["value"].(string)
		return strings.TrimSpace(v)
	}
	return ""
}

// buildRequestURL builds scheme://host/path?args for Loki dashboards; path-only if Host is missing.
func buildRequestURL(hr map[string]any) string {
	uri := strings.TrimSpace(str(hr, "uri"))
	if uri == "" {
		return ""
	}
	path := uri
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	args := strings.TrimSpace(str(hr, "args"))
	qs := ""
	if args != "" {
		qs = "?" + args
	}
	host := headerValue(hr["headers"], "host")
	scheme := strings.ToLower(headerValue(hr["headers"], "x-forwarded-proto"))
	if scheme != "http" && scheme != "https" {
		scheme = "https"
	}
	if host == "" {
		return path + qs
	}
	return scheme + "://" + host + path + qs
}

func str(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func keepSample(seed string, percent int) bool {
	if percent <= 0 {
		return false
	}
	if percent >= 100 {
		return true
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	return int(h.Sum32()%100) < percent
}

var countryCentroids = map[string][2]float64{
	"AU": {-25.2744, 133.7751},
	"NZ": {-40.9006, 174.8860},
	"US": {37.0902, -95.7129},
	"GB": {55.3781, -3.4360},
}
