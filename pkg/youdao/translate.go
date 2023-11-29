package youdao

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"time"

	"github.com/mylxsw/asteria/log"
)

type Translater interface {
	TranslateToEnglish(text string) string
	Translate(ctx context.Context, from, target string, text string) (*TranslateResult, error)
}

type TranslaterImpl struct {
	cacheRepo *repo.CacheRepo
	client    *Client
}

func NewTranslater(cacheRepo *repo.CacheRepo, client *Client) *TranslaterImpl {
	return &TranslaterImpl{cacheRepo: cacheRepo, client: client}
}

// TranslateToEnglish 翻译为英文, 如果翻译失败，则返回原文
func (tr *TranslaterImpl) TranslateToEnglish(text string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res, err := tr.Translate(ctx, LanguageAuto, LanguageEnglish, text)
	if err != nil {
		log.Errorf("translate to english failed: %s", err)
		return text
	}

	return res.Result
}

func (tr *TranslaterImpl) Translate(ctx context.Context, from, target string, text string) (*TranslateResult, error) {
	cacheKey := fmt.Sprintf("translate:%s:%s:%x", from, target, md5.Sum([]byte(text)))
	if cacheValue, err := tr.cacheRepo.Get(ctx, cacheKey); err == nil {
		var res TranslateResult
		if err := json.Unmarshal([]byte(cacheValue), &res); err != nil {
			log.WithFields(log.Fields{
				"cache_key": cacheKey,
			}).Errorf("unmarshal cache value failed: %s", err)
		} else {
			return &res, nil
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	res, err := tr.client.Translate(ctx, text, from, target)
	if err != nil {
		return nil, err
	}

	cacheValue, err := json.Marshal(res)
	if err != nil {
		log.WithFields(log.Fields{
			"cache_key": cacheKey,
		}).Errorf("marshal translate result failed: %s", err)
	} else {
		if err := tr.cacheRepo.Set(ctx, cacheKey, string(cacheValue), 6*30*24*time.Hour); err != nil {
			log.WithFields(log.Fields{
				"cache_key": cacheKey,
			}).Errorf("cache translate result failed: %s", err)
		}
	}

	return res, nil
}
