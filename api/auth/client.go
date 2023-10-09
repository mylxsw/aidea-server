package auth

import (
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/helper"
)

type ClientInfo struct {
	Version         string `json:"version"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	Language        string `json:"language"`
	IP              string `json:"ip"`
}

// IsIOS 返回客户端是否是 IOS 平台
func (inf ClientInfo) IsIOS() bool {
	return inf.Platform == "ios"
}

// IsCNLocalMode 是否启用国产化模式
func (inf ClientInfo) IsCNLocalMode(conf *config.Config) bool {
	return inf.isCNLocalMode(conf) && conf.EnableVirtualModel
}

func (inf ClientInfo) isCNLocalMode(conf *config.Config) bool {
	if !conf.CNLocalMode {
		return false
	}

	if !conf.CNLocalOnlyIOS {
		return true
	}

	return inf.IsIOS() && helper.VersionNewer(inf.Version, "1.0.4")
}
