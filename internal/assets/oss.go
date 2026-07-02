package assets

import (
	"context"
	"errors"
	"io"
	"net/http"
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
	return errors.New("oss save not implemented")
}

func (s *OSSStore) Open(ctx context.Context, key string) (io.ReadCloser, string, error) {
	return nil, "", errors.New("oss open not implemented")
}

func (s *OSSStore) DeletePrefix(ctx context.Context, prefix string) error {
	return errors.New("oss delete prefix not implemented")
}
