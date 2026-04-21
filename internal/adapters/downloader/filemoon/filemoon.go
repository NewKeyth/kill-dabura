package filemoon

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

var (
	rxDomain = regexp.MustCompile(`(https?://[^/]+)`)
	rxId     = regexp.MustCompile(`/(e|d)/([a-zA-Z0-9]+)`)
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/129.0.0.0 Safari/537.36"

type detailsResp struct {
	EmbedFrameURL string `json:"embed_frame_url"`
}

type playbackResp struct {
	Playback *playbackData `json:"playback"`
}

type playbackData struct {
	IV       string   `json:"iv"`
	Payload  string   `json:"payload"`
	KeyParts []string `json:"key_parts"`
}

func IsFilemoonURL(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "filemoon.site") ||
		strings.Contains(lower, "bf0skv.org") ||
		strings.Contains(lower, "filemoon.sx") ||
		strings.Contains(lower, "byse") ||
		strings.Contains(lower, "moflix-stream")
}

func Extract(link string) (string, error) {
	match := rxId.FindStringSubmatch(link)
	if len(match) < 3 {
		return "", fmt.Errorf("could not extract video ID or type from filemoon link")
	}
	linkType := match[1]
	videoId := match[2]

	domainMatch := rxDomain.FindStringSubmatch(link)
	if len(domainMatch) < 2 {
		return "", fmt.Errorf("could not extract base url")
	}
	currentDomain := domainMatch[1]

	// 1. Get Details
	detailsURL := fmt.Sprintf("%s/api/videos/%s/embed/details", currentDomain, videoId)
	
	reqD, _ := http.NewRequest("GET", detailsURL, nil)
	reqD.Header.Set("User-Agent", userAgent)
	
	client := &http.Client{}
	respD, err := client.Do(reqD)
	if err != nil {
		return "", err
	}
	defer respD.Body.Close()
	
	bodyD, _ := io.ReadAll(respD.Body)
	var dRes detailsResp
	if err := json.Unmarshal(bodyD, &dRes); err != nil {
		return "", fmt.Errorf("error parsing details response")
	}
	if dRes.EmbedFrameURL == "" {
		return "", fmt.Errorf("embed_frame_url not found")
	}

	// 2. Get Playback Data
	playbackDomain := currentDomain
	referer := link
	xparent := ""

	if linkType != "d" {
		pbMatch := rxDomain.FindStringSubmatch(dRes.EmbedFrameURL)
		if len(pbMatch) >= 2 {
			playbackDomain = pbMatch[1]
		}
		referer = dRes.EmbedFrameURL
		xparent = link
	}

	playbackURL := fmt.Sprintf("%s/api/videos/%s/embed/playback", playbackDomain, videoId)
	
	reqP, _ := http.NewRequest("GET", playbackURL, nil)
	reqP.Header.Set("User-Agent", userAgent)
	reqP.Header.Set("Accept", "application/json")
	reqP.Header.Set("Referer", referer)
	if xparent != "" {
		reqP.Header.Set("X-Embed-Parent", xparent)
	}

	respP, err := client.Do(reqP)
	if err != nil {
		return "", err
	}
	defer respP.Body.Close()
	
	bodyP, _ := io.ReadAll(respP.Body)
	var pRes playbackResp
	if err := json.Unmarshal(bodyP, &pRes); err != nil {
		return "", fmt.Errorf("error parsing playback response")
	}
	
	if pRes.Playback == nil {
		return "", fmt.Errorf("no playback data found in filemoon api")
	}
	
	// 3. Decrypt Payload
	decryptedJson, err := decryptAESGCM(pRes.Playback)
	if err != nil {
		return "", err
	}

	var jsonMap map[string]interface{}
	if err := json.Unmarshal([]byte(decryptedJson), &jsonMap); err != nil {
		return "", fmt.Errorf("error decodificando json desencriptado")
	}

	sourcesAny, ok := jsonMap["sources"].([]interface{})
	if !ok || len(sourcesAny) == 0 {
		return "", fmt.Errorf("no sources found inside decrypted payload")
	}

	firstSource, ok := sourcesAny[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("source item is not a map")
	}
	
	urlItem, ok := firstSource["url"].(string)
	if !ok || urlItem == "" {
		return "", fmt.Errorf("no url found inside source item")
	}

	return urlItem, nil
}

func decryptAESGCM(data *playbackData) (string, error) {
	iv, err := base64.RawURLEncoding.DecodeString(data.IV)
	if err != nil {
		// Fallback to std / url safe with padding
		iv, _ = base64.URLEncoding.DecodeString(data.IV)
	}
	
	payload, err := base64.RawURLEncoding.DecodeString(data.Payload)
	if err != nil {
		payload, _ = base64.URLEncoding.DecodeString(data.Payload)
	}
	
	if len(data.KeyParts) < 2 {
		return "", fmt.Errorf("missing key parts")
	}
	
	p1, err := base64.RawURLEncoding.DecodeString(data.KeyParts[0])
	if err != nil {
		p1, _ = base64.URLEncoding.DecodeString(data.KeyParts[0])
	}
	
	p2, err := base64.RawURLEncoding.DecodeString(data.KeyParts[1])
	if err != nil {
		p2, _ = base64.URLEncoding.DecodeString(data.KeyParts[1])
	}

	key := append(p1, p2...)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	plaintext, err := aesgcm.Open(nil, iv, payload, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
