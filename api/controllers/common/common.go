package common

import (
	"github.com/mylxsw/aidea-server/internal/youdao"
	"github.com/mylxsw/glacier/web"
)

const (
	ErrQuotaNotEnough    = "智慧果不足，请充值后再试"
	ErrInvalidModel      = "无效的模型"
	ErrInvalidRequest    = "请求参数错误"
	ErrInternalError     = "服务器故障，请稍后再试"
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
