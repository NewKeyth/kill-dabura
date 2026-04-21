package voe

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var voeAliases = []string{
	"voe.sx",
	"jilliandescribecompany.com",
	"mikaylaarealike.com",
	"christopheruntilpoint.com",
	"walterprettytheir.com",
	"crystaltreatmenteast.com",
	"lauradaydo.com",
	"lancewhosedifficult.com",
}

func IsVoeURL(u string) bool {
	for _, alias := range voeAliases {
		if strings.Contains(u, alias) {
			return true
		}
	}
	return false
}

func Extract(urlStr string) (string, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	html, err := fetchHTML(client, urlStr, urlStr)
	if err != nil {
		return "", fmt.Errorf("error fetching voe: %w", err)
	}

	if strings.Contains(html, "window.location.href") {
		re := regexp.MustCompile(`window\.location\.href\s*=\s*'([^']+)'`)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 {
			redirectURL := matches[1]
			html, err = fetchHTML(client, redirectURL, urlStr)
			if err != nil {
				return "", fmt.Errorf("error on voe redirect: %w", err)
			}
		}
	}

	var encodedStr string
	reEncoded := regexp.MustCompile(`(?s)<script\s+type="application/json">(.*?)</script>`)
	matches := reEncoded.FindStringSubmatch(html)
	if len(matches) > 1 {
		encodedStr = strings.TrimSpace(matches[1])
	}

	if encodedStr == "" {
		return urlStr, nil // fallback to what we have
	}

	var obfuscatedPayload string
	if strings.HasPrefix(encodedStr, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(encodedStr), &arr); err == nil && len(arr) > 0 {
			obfuscatedPayload = arr[0]
		}
	} else if strings.HasPrefix(encodedStr, `"`) {
		var s string
		if err := json.Unmarshal([]byte(encodedStr), &s); err == nil {
			obfuscatedPayload = s
		}
	}

	if obfuscatedPayload == "" {
		obfuscatedPayload = encodedStr
	}

	vF := rot13(obfuscatedPayload)
	vF = replacePatterns(vF)
	vF = removeUnderscores(vF)

	vF4Bytes, err := base64.StdEncoding.DecodeString(vF)
	if err != nil {
		return "", err
	}

	vF5 := charShift(string(vF4Bytes), 3)
	vF6 := reverse(vF5)

	vAtobBytes, err := base64.StdEncoding.DecodeString(vF6)
	if err != nil {
		return "", err
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(vAtobBytes, &payload); err != nil {
		return "", err
	}

	source, ok := payload["source"].(string)
	if !ok || source == "" {
		return urlStr, nil
	}

	return source, nil
}

func fetchHTML(client *http.Client, target, referer string) (string, error) {
	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", referer)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	return string(bodyBytes), nil
}

func rot13(input string) string {
	var sb strings.Builder
	for _, c := range input {
		switch {
		case 'A' <= c && c <= 'Z':
			sb.WriteByte(byte((c-'A'+13)%26 + 'A'))
		case 'a' <= c && c <= 'z':
			sb.WriteByte(byte((c-'a'+13)%26 + 'a'))
		default:
			sb.WriteRune(c)
		}
	}
	return sb.String()
}

func replacePatterns(input string) string {
	patterns := []string{"@$", "^^", "~@", "%?", "*~", "!!", "#&"}
	res := input
	for _, p := range patterns {
		res = strings.ReplaceAll(res, p, "_")
	}
	return res
}

func removeUnderscores(input string) string {
	return strings.ReplaceAll(input, "_", "")
}

func charShift(input string, shift int) string {
	var sb strings.Builder
	for _, c := range input {
		sb.WriteRune(c - rune(shift))
	}
	return sb.String()
}

func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < len(r)/2; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}
