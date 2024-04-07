package controllers

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/service"
	"net/http"
	"strconv"

	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
)

type ImageController struct {
	svc *service.SettingService `autowire:"@"`
}

func NewImageController(resolver infra.Resolver) web.Controller {
	ctl := ImageController{}
	resolver.MustAutoWire(&ctl)
	return &ctl
}

func (ctl *ImageController) Register(router web.Router) {
	router.Group("/images", func(router web.Router) {
		router.Get("/background", ctl.Background)
		router.Get("/avatar", ctl.Avatar)
		router.Get("/random-avatar/{usage}/{seed}/{size}", ctl.RandomAvatar)
	})
}

// RandomAvatar 随机头像
func (ctl *ImageController) RandomAvatar(ctx context.Context, webCtx web.Context) web.Response {
	size, err := strconv.Atoi(webCtx.PathVar("size"))
	if err != nil || size < 10 || size > 1000 {
		return webCtx.Error("invalid size", http.StatusBadRequest)
	}

	usage := webCtx.PathVar("usage")
	seed := webCtx.PathVar("seed")

	if seed == "0" || seed == "匿名" {
		return webCtx.Redirect(
			"https://ssl.aicode.cc/ai-server/assets/app-icon-1024.png-avatar",
			http.StatusPermanentRedirect,
		)
	}

	// TODO 替换为预定义的头像池
	return webCtx.Redirect(
		fmt.Sprintf("https://picsum.photos/%d/%d?random=%s-%s", size, size, usage, seed),
		http.StatusPermanentRedirect,
	)
}

type ImagePreset struct {
	Preview string `json:"preview"`
	URL     string `json:"url"`
}

func (ctl *ImageController) Avatar(ctx context.Context, webCtx web.Context) web.Response {
	return webCtx.JSON(web.M{
		"avatars": ctl.svc.Avatars(ctx),
	})
}

func (ctl *ImageController) Background(ctx context.Context, webCtx web.Context) web.Response {
	return webCtx.JSON(web.M{
		"preset": []ImagePreset{
			{
				Preview: "https://ssl.aicode.cc/ai-server/assets/background/background-preset-13.jpg-square_500",
				URL:     "https://ssl.aicode.cc/ai-server/assets/background/background-preset-13.jpg",
			},

			{
				Preview: "https://ssl.aicode.cc/ai-server/assets/background/background-preset-1.jpeg-square_500",
				URL:     "https://ssl.aicode.cc/ai-server/assets/background/background-preset-1.jpeg",
			},

			{
				Preview: "https://ssl.aicode.cc/ai-server/assets/background/background-preset-12.jpg-square_500",
				URL:     "https://ssl.aicode.cc/ai-server/assets/background/background-preset-12.jpg",
			},

			{
				Preview: "https://ssl.aicode.cc/ai-server/assets/background/background-preset-11.jpg-square_500",
				URL:     "https://ssl.aicode.cc/ai-server/assets/background/background-preset-11.jpg",
			},

			{
				Preview: "https://ssl.aicode.cc/ai-server/assets/background/background-preset-10.jpg-square_500",
				URL:     "https://ssl.aicode.cc/ai-server/assets/background/background-preset-10.jpg",
			},

			{
				Preview: "https://ssl.aicode.cc/ai-server/assets/background/background-preset-9.jpg-square_500",
				URL:     "https://ssl.aicode.cc/ai-server/assets/background/background-preset-9.jpg",
			},

			{
				Preview: "https://ssl.aicode.cc/ai-server/assets/background/background-preset-8.jpg-square_500",
				URL:     "https://ssl.aicode.cc/ai-server/assets/background/background-preset-8.jpg",
			},

			{
				Preview: "https://ssl.aicode.cc/ai-server/assets/background/background-preset-7.jpeg-square_500",
				URL:     "https://ssl.aicode.cc/ai-server/assets/background/background-preset-7.jpeg",
			},

			{
				Preview: "https://ssl.aicode.cc/ai-server/assets/background/background-preset-6.jpeg-square_500",
				URL:     "https://ssl.aicode.cc/ai-server/assets/background/background-preset-6.jpeg",
			},

			{
				Preview: "https://ssl.aicode.cc/ai-server/assets/background/background-preset-5.jpeg-square_500",
				URL:     "https://ssl.aicode.cc/ai-server/assets/background/background-preset-5.jpeg",
			},

			{
				Preview: "https://ssl.aicode.cc/ai-server/assets/background/background-preset-4.jpeg-square_500",
				URL:     "https://ssl.aicode.cc/ai-server/assets/background/background-preset-4.jpeg",
			},

			{
				Preview: "https://ssl.aicode.cc/ai-server/assets/background/background-preset-3.jpeg-square_500",
				URL:     "https://ssl.aicode.cc/ai-server/assets/background/background-preset-3.jpeg",
			},

			{
				Preview: "https://ssl.aicode.cc/ai-server/assets/background/background-preset-2.jpeg-square_500",
				URL:     "https://ssl.aicode.cc/ai-server/assets/background/background-preset-2.jpeg",
			},
		},
	})
}
