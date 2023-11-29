package controllers

import (
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/web"
)

// ProxiesController 代理服务器查询控制器
type ProxiesController struct {
	conf *config.Config
}

// NewProxiesController 创建代理服务器查询控制器
func NewProxiesController(conf *config.Config) web.Controller {
	return &ProxiesController{conf: conf}
}

func (ctl *ProxiesController) Register(router web.Router) {
	router.Group("/proxy", func(router web.Router) {
		router.Get("/servers", ctl.Proxies)
	})
}

// Proxies 获取代理服务器列表
func (ctl *ProxiesController) Proxies(ctx web.Context) web.Response {
	return ctx.JSON(web.M{
		"servers": map[string][]string{
			"openai": {
				"https://api.openai.com",
				"https://openai-proxy.aicode.cc",
				"https://ai-api.aicode.cc",
			},
			"deepai": {
				"https://api.deepai.org",
			},
			"stabilityai": {
				"https://api.stability.ai",
			},
		},
	})
}
