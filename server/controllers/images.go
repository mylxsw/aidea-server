package controllers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
)

type ImageController struct{}

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
		"avatars": []string{
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-20230630-14.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-20230630-17.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-20230630-2.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-20230630-21.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-20230630-22.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-20230630-3.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-20230630-4.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-20230630-5.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-20230630-6.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-20230630-7.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-20230630-8.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-20230630-9.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-1.JPG-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-10.JPG-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-11.JPG-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-12.JPG-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-13.JPG-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-14.JPG-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-15.JPG-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-16.JPG-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-17.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-18.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-19.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-2.JPG-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-22.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-23.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-24.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-25.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-26.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-27.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-28.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-29.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-3.JPG-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-30.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-31.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-32.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-33.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-34.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-35.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-36.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-37.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-38.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-39.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-4.JPG-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-40.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-41.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-42.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-43.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-44.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-45.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-46.png-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-5.JPG-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-6.JPG-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-7.JPG-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-8.JPG-avatar",
			"https://ssl.aicode.cc/ai-server/assets/avatar/ava-9.JPG-avatar",
		},
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
