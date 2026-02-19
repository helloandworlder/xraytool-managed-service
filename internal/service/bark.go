package service

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"xraytool/internal/model"

	"gorm.io/gorm"
)

type BarkService struct {
	db     *gorm.DB
	client *http.Client
}

func NewBarkService(db *gorm.DB) *BarkService {
	return &BarkService{
		db:     db,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (b *BarkService) Notify(title, body string) error {
	settings, err := b.settings()
	if err != nil {
		return err
	}
	if strings.ToLower(settings["bark_enabled"]) != "true" {
		return nil
	}
	base := strings.TrimSuffix(settings["bark_base_url"], "/")
	device := strings.TrimSpace(settings["bark_device_key"])
	if base == "" || device == "" {
		return nil
	}
	group := settings["bark_group"]
	u := fmt.Sprintf("%s/%s/%s/%s?group=%s", base, url.PathEscape(device), url.PathEscape(title), url.PathEscape(body), url.QueryEscape(group))
	resp, err := b.client.Get(u)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

func (b *BarkService) settings() (map[string]string, error) {
	var rows []model.Setting
	if err := b.db.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, row := range rows {
		out[row.Key] = row.Value
	}
	return out, nil
}
