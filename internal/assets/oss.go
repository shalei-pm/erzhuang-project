package assets

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type OSSConfig struct {
	Bucket          string
	Endpoint        string
	AccessKeyID     string
	AccessKeySecret string
	HTTPClient      *http.Client
}

type OSSStore struct {
	bucket          string
	endpoint        string
	accessKeyID     string
	accessKeySecret string
	client          *http.Client
}

func NewOSSStore(config OSSConfig) *OSSStore {
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	return &OSSStore{
		bucket:          strings.Trim(strings.TrimSpace(config.Bucket), "/"),
		endpoint:        strings.TrimRight(strings.TrimSpace(config.Endpoint), "/"),
		accessKeyID:     strings.TrimSpace(config.AccessKeyID),
		accessKeySecret: strings.TrimSpace(config.AccessKeySecret),
		client:          client,
	}
}

func (s *OSSStore) Save(ctx context.Context, key string, body io.Reader, contentType string) error {
	clean, err := cleanKey(key)
	if err != nil {
		return err
	}
	payload, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, s.objectURL(clean), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", contentTypeOrDefault(contentType))
	s.authorize(request, clean)
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ossHTTPError("save asset", response)
	}
	return nil
}

func (s *OSSStore) Open(ctx context.Context, key string) (io.ReadCloser, string, error) {
	clean, err := cleanKey(key)
	if err != nil {
		return nil, "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.objectURL(clean), nil)
	if err != nil {
		return nil, "", err
	}
	s.authorize(request, clean)
	response, err := s.client.Do(request)
	if err != nil {
		return nil, "", err
	}
	if response.StatusCode == http.StatusNotFound {
		response.Body.Close()
		return nil, "", ErrNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		return nil, "", ossHTTPError("open asset", response)
	}
	return response.Body, contentTypeOrDefault(response.Header.Get("Content-Type")), nil
}

func (s *OSSStore) DeletePrefix(ctx context.Context, prefix string) error {
	isDirectoryPrefix := strings.HasSuffix(strings.TrimSpace(prefix), "/")
	cleanPrefix, err := cleanKey(prefix)
	if err != nil {
		return err
	}
	keys, err := s.listKeys(ctx, cleanPrefix, isDirectoryPrefix)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err := s.deleteKey(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

func (s *OSSStore) listKeys(ctx context.Context, prefix string, isDirectoryPrefix bool) ([]string, error) {
	listPrefix := ossListPrefix(prefix, isDirectoryPrefix)
	listURL := s.baseURL()
	query := listURL.Query()
	query.Set("list-type", "2")
	query.Set("prefix", listPrefix)
	listURL.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL.String(), nil)
	if err != nil {
		return nil, err
	}
	s.authorize(request, "")
	response, err := s.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, ossHTTPError("list assets", response)
	}
	var listing struct {
		Contents []struct {
			Key string `xml:"Key"`
		} `xml:"Contents"`
	}
	if err := xml.NewDecoder(response.Body).Decode(&listing); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(listing.Contents))
	for _, entry := range listing.Contents {
		key := strings.TrimSpace(entry.Key)
		if key == "" || !ossKeyMatchesPrefix(key, listPrefix, isDirectoryPrefix) {
			continue
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func ossListPrefix(prefix string, isDirectoryPrefix bool) string {
	if isDirectoryPrefix {
		return strings.TrimSuffix(prefix, "/") + "/"
	}
	return strings.TrimSuffix(prefix, "/")
}

func ossKeyMatchesPrefix(key string, listPrefix string, isDirectoryPrefix bool) bool {
	if isDirectoryPrefix {
		return strings.HasPrefix(key, listPrefix)
	}
	return key == listPrefix
}

func (s *OSSStore) deleteKey(ctx context.Context, key string) error {
	clean, err := cleanKey(key)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.objectURL(clean), nil)
	if err != nil {
		return err
	}
	s.authorize(request, clean)
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ossHTTPError("delete asset", response)
	}
	return nil
}

func (s *OSSStore) objectURL(key string) string {
	assetURL := s.baseURL()
	assetURL.Path = "/" + key
	return assetURL.String()
}

func (s *OSSStore) baseURL() *url.URL {
	endpoint := strings.TrimSpace(s.endpoint)
	if endpoint == "" {
		endpoint = s.bucket
	}
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "https://" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return &url.URL{Scheme: "https", Host: s.endpoint}
	}
	if parsed.Host == "" {
		parsed.Host = parsed.Path
		parsed.Path = ""
	}
	return parsed
}

func (s *OSSStore) authorize(request *http.Request, key string) {
	if request.Header.Get("Date") == "" {
		request.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	}
	canonicalResource := s.canonicalResource(request, key)
	contentMD5 := request.Header.Get("Content-MD5")
	contentType := request.Header.Get("Content-Type")
	stringToSign := strings.Join([]string{
		request.Method,
		contentMD5,
		contentType,
		request.Header.Get("Date"),
	}, "\n") + "\n" + canonicalizedOSSHeaders(request.Header) + canonicalResource
	signature := hmacSHA1Base64(s.accessKeySecret, stringToSign)
	request.Header.Set("Authorization", "OSS "+s.accessKeyID+":"+signature)
}

func (s *OSSStore) canonicalResource(request *http.Request, key string) string {
	resource := "/" + strings.Trim(s.bucket, "/")
	if key != "" {
		resource += "/" + strings.TrimLeft(key, "/")
	} else if request.URL.RawQuery != "" {
		resource += "/"
	}
	subresources := make([]string, 0, 2)
	query := request.URL.Query()
	for _, name := range []string{"list-type", "prefix"} {
		if value := query.Get(name); value != "" {
			subresources = append(subresources, name+"="+value)
		}
	}
	if len(subresources) > 0 {
		sort.Strings(subresources)
		resource += "?" + strings.Join(subresources, "&")
	}
	return resource
}

func canonicalizedOSSHeaders(header http.Header) string {
	var names []string
	for name := range header {
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "x-oss-") {
			names = append(names, lower)
		}
	}
	sort.Strings(names)
	lines := make([]string, 0, len(names))
	for _, name := range names {
		lines = append(lines, name+":"+strings.Join(header.Values(name), ","))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func hmacSHA1Base64(secret string, value string) string {
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write([]byte(value))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func ossHTTPError(action string, response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
	code, message := safeOSSErrorMessage(body)
	if code == "" {
		code = http.StatusText(response.StatusCode)
	}
	if message == "" {
		return fmt.Errorf("%s failed: http %d: %s", action, response.StatusCode, code)
	}
	return fmt.Errorf("%s failed: http %d: %s: %s", action, response.StatusCode, code, message)
}

func safeOSSErrorMessage(body []byte) (string, string) {
	var payload struct {
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	}
	if err := xml.Unmarshal(body, &payload); err == nil && (payload.Code != "" || payload.Message != "") {
		return sanitizeOSSErrorText(payload.Code), sanitizeOSSErrorText(payload.Message)
	}
	text := sanitizeOSSErrorText(string(body))
	if text == "" {
		return "", ""
	}
	return text, ""
}

func sanitizeOSSErrorText(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.TrimSpace(value)
	if len(value) > 256 {
		value = value[:256]
	}
	blocked := []string{"AccessKey", "Authorization", "SignatureProvided", "StringToSign"}
	for _, marker := range blocked {
		if strings.Contains(strings.ToLower(value), strings.ToLower(marker)) {
			return "redacted"
		}
	}
	return value
}
