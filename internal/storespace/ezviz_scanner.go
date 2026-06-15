package storespace

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/ezviz"
)

type EzvizScanner struct {
	client   *ezviz.Client
	accounts map[string]ezviz.Account
}

type ezvizCredential struct {
	Name        string `json:"name"`
	AccountName string `json:"account_name"`
	AppKey      string `json:"app_key"`
	AppSecret   string `json:"app_secret"`
	AccessToken string `json:"access_token"`
}

func NewEzvizScanner(client *ezviz.Client, accounts []ezviz.Account) *EzvizScanner {
	accountMap := map[string]ezviz.Account{}
	for _, account := range accounts {
		if strings.TrimSpace(account.Name) == "" {
			continue
		}
		accountMap[account.Name] = account
	}
	return &EzvizScanner{client: client, accounts: accountMap}
}

func NewEzvizScannerFromEnv() (*EzvizScanner, bool, error) {
	raw := strings.TrimSpace(os.Getenv("EZVIZ_ACCOUNTS_JSON"))
	if raw == "" {
		return nil, false, nil
	}
	var credentials []ezvizCredential
	if err := json.Unmarshal([]byte(raw), &credentials); err != nil {
		return nil, true, err
	}
	accounts := make([]ezviz.Account, 0, len(credentials))
	for _, credential := range credentials {
		name := strings.TrimSpace(credential.Name)
		if name == "" {
			name = strings.TrimSpace(credential.AccountName)
		}
		accounts = append(accounts, ezviz.Account{
			Name:        name,
			AppKey:      strings.TrimSpace(credential.AppKey),
			AppSecret:   strings.TrimSpace(credential.AppSecret),
			AccessToken: strings.TrimSpace(credential.AccessToken),
		})
	}
	return NewEzvizScanner(ezviz.NewClient(ezviz.ClientOptions{}), accounts), true, nil
}

func (s *EzvizScanner) ScanRecorderChannels(ctx context.Context, account EzvizAccount, recorder Recorder) ([]ScannedChannel, error) {
	if s == nil || s.client == nil {
		return nil, ErrNotImplemented
	}
	credentials, ok := s.accounts[account.AccountName]
	if !ok {
		return nil, &ValidationError{Fields: map[string]string{"ezviz_account_id": "找不到萤石云账号配置"}}
	}
	if strings.TrimSpace(credentials.AppKey) == "" {
		return nil, &ValidationError{Fields: map[string]string{"app_key": "缺少萤石云 appKey"}}
	}
	if strings.TrimSpace(credentials.AppSecret) == "" {
		return nil, &ValidationError{Fields: map[string]string{"app_secret": "缺少萤石云 appSecret"}}
	}
	cameras, err := s.client.CameraList(ctx, credentials, recorder.DeviceCode)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, err
		}
		return nil, err
	}
	channels := make([]ScannedChannel, 0, len(cameras))
	for _, camera := range cameras {
		channels = append(channels, ScannedChannel{
			ChannelNo:   camera.ChannelNo,
			ChannelName: strings.TrimSpace(camera.CameraName),
			Active:      camera.Status == 1,
		})
	}
	return channels, nil
}

func (s *EzvizScanner) CaptureChannel(ctx context.Context, account EzvizAccount, recorder Recorder, channel Channel) (ChannelSnapshotInput, error) {
	if s == nil || s.client == nil {
		return ChannelSnapshotInput{}, ErrNotImplemented
	}
	credentials, ok := s.accounts[account.AccountName]
	if !ok {
		return ChannelSnapshotInput{}, &ValidationError{Fields: map[string]string{"ezviz_account_id": "找不到萤石云账号配置"}}
	}
	if strings.TrimSpace(credentials.AppKey) == "" {
		return ChannelSnapshotInput{}, &ValidationError{Fields: map[string]string{"app_key": "缺少萤石云 appKey"}}
	}
	if strings.TrimSpace(credentials.AppSecret) == "" {
		return ChannelSnapshotInput{}, &ValidationError{Fields: map[string]string{"app_secret": "缺少萤石云 appSecret"}}
	}
	result, err := s.client.Capture(ctx, credentials, recorder.DeviceCode, channel.ChannelNo)
	if err != nil {
		return ChannelSnapshotInput{}, err
	}
	expiresAt := time.Now().UTC().Add(7 * 24 * time.Hour)
	return ChannelSnapshotInput{
		ThumbnailPath:      result.PicURL,
		FullImagePath:      result.PicURL,
		FullImageExpiresAt: &expiresAt,
	}, nil
}
