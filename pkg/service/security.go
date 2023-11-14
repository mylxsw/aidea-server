package service

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/aliyun"
	"time"

	"github.com/mylxsw/aidea-server/config"

	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/redis/go-redis/v9"
)

type SecurityService struct {
	aliClient *aliyun.Aliyun `autowire:"@"`
	rds       *redis.Client  `autowire:"@"`
	conf      *config.Config `autowire:"@"`
}

func NewSecurityService(resolver infra.Resolver) *SecurityService {
	srv := &SecurityService{}
	resolver.MustAutoWire(srv)
	return srv
}

func (s *SecurityService) NicknameDetect(nickname string) *aliyun.CheckResult {
	return s.contentDetect(aliyun.CheckTypeNickname, nickname)
}

func (s *SecurityService) PromptDetect(prompt string) *aliyun.CheckResult {
	return s.contentDetect(aliyun.CheckTypeAIGCPrompt, prompt)
}

func (s *SecurityService) ChatDetect(message string) *aliyun.CheckResult {
	return s.contentDetect(aliyun.CheckTypeChat, message)
}

func (s *SecurityService) contentDetect(typ aliyun.CheckType, content string) *aliyun.CheckResult {
	if !s.conf.EnableContentDetect {
		return &aliyun.CheckResult{Safe: true}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cacheKey := fmt.Sprintf("detect:%s:%x", typ, md5.Sum([]byte(content)))
	if cacheValue, err := s.rds.Get(ctx, cacheKey).Result(); err == nil {
		var res aliyun.CheckResult
		if err := json.Unmarshal([]byte(cacheValue), &res); err != nil {
			log.WithFields(log.Fields{
				"cache_key": cacheKey,
			}).Errorf("unmarshal cache value failed: %s", err)
		} else {
			return &res
		}
	}

	res, err := s.aliClient.ContentDetect(typ, content)
	if err != nil {
		log.WithFields(log.Fields{"prompt": content, "type": typ}).Errorf("prompt detect failed: %v", err)
		return nil
	}

	cacheValue, err := json.Marshal(res)
	if err != nil {
		log.WithFields(log.Fields{
			"cache_key": cacheKey,
		}).Errorf("marshal content detect result failed: %s", err)
	} else {
		if err := s.rds.Set(ctx, cacheKey, string(cacheValue), 24*time.Hour).Err(); err != nil {
			log.WithFields(log.Fields{
				"cache_key": cacheKey,
			}).Errorf("cache content detect result failed: %s", err)
		}
	}

	return res
}
