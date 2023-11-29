package common

import (
	"github.com/mylxsw/aidea-server/pkg/youdao"
	"github.com/mylxsw/glacier/web"
)

const (
	ErrQuotaNotEnough    = "智慧果不足，请充值后再试"
	ErrInvalidModel      = "无效的模型"
	ErrInvalidRequest    = "请求参数错误"
	ErrInternalError     = "很抱歉，我们的服务暂时出现了点问题，但我们正在全力修复。请您稍后再试，感谢您的耐心等待。"
	ErrInvalidCredential = "无效的凭证"
	ErrNotFound          = "资源不存在"
	ErrFileTooLarge      = "文件太大"
)

func GetLanguage(webCtx web.Context) string {
	language := webCtx.Header("X-LANGUAGE")
	if language == "" {
		language = "zh-CHS"
	}

	return language
}

func Text(webCtx web.Context, translater youdao.Translater, text string) string {
	language := GetLanguage(webCtx)
	if language == "en" {
		return translater.TranslateToEnglish(text)
	}

	return text
}
