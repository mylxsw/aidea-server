package controllers

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"net/http"
)

type NotificationController struct {
	repo *repo.Repository `autowire:"@"`
}

func NewNotificationController(resolver infra.Resolver) web.Controller {
	ctl := NotificationController{}
	resolver.MustAutoWire(&ctl)
	return &ctl
}

func (ctl *NotificationController) Register(router web.Router) {
	router.Group("/notifications", func(router web.Router) {
		router.Get("/", ctl.Notifications)
		router.Get("/promotions", ctl.Promotion)
	})
}

type ClickButtonType string

const (
	// ClickButtonNone 不可点击
	ClickButtonNone ClickButtonType = ""
	// ClickButtonURL 可点击，跳转到指定的URL
	ClickButtonURL ClickButtonType = "url"
	// ClickButtonInAppRoute 可点击，跳转到 APP 内部路由
	ClickButtonInAppRoute ClickButtonType = "in_app_route"
)

type PromotionEvent struct {
	// ID 事件 ID，用于标识事件展示的位置
	ID string `json:"id,omitempty"`
	// Title 事件标题
	Title string `json:"title,omitempty"`
	// Content 事件内容
	Content string `json:"content,omitempty"`
	// ClickButtonType 点击按钮类型
	ClickButtonType ClickButtonType `json:"click_button_type,omitempty"`
	// ClickValue 点击按钮的值，如果 ClickButtonType 为 ClickButtonURL，则为 URL 地址，如果为 ClickButtonInAppRoute，则为 APP 内部路由
	ClickValue string `json:"click_value,omitempty"`
	// ClickButtonColor 点击按钮的颜色
	ClickButtonColor string `json:"click_button_color,omitempty"`
	// BackgroundImage 背景图片
	BackgroundImage string `json:"background_image,omitempty"`
	// TextColor 文本颜色
	TextColor string `json:"text_color,omitempty"`
	// MaxCloseDurationInDays 最大关闭天数，如果用户关闭了该事件，则在该天数内不再显示，默认 7 天
	MaxCloseDurationInDays int `json:"max_close_duration_in_days,omitempty"`
	// Closeable 是否可关闭
	Closeable bool `json:"closeable,omitempty"`
}

func (ctl *NotificationController) Promotion(ctx context.Context, webCtx web.Context) web.Response {
	return webCtx.JSON(web.M{
		"data": []PromotionEvent{
			{
				ID:               "chat_page",
				Title:            "免费畅享",
				Content:          "现推出系列福利模型，每日免费畅享！",
				ClickButtonType:  ClickButtonInAppRoute,
				ClickButtonColor: "FF9e5652",
				ClickValue:       "/free-statistics",
				BackgroundImage:  "https://ssl.aicode.cc/ai-server/assets/ad/free-notify-ad-20231004.jpeg",
				TextColor:        "FF9e5652",
				Closeable:        true,
			},
		},
	})
}

// Notifications 获取通知列表
func (ctl *NotificationController) Notifications(ctx context.Context, webCtx web.Context) web.Response {
	startID := webCtx.Int64Input("start_id", 0)
	perPage := webCtx.Int64Input("per_page", 100)
	if perPage < 1 || perPage > 300 {
		perPage = 100
	}

	messages, lastID, err := ctl.repo.Notification.NotifyMessages(ctx, startID, perPage)
	if err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{
		"data":     messages,
		"start_id": startID,
		"last_id":  lastID,
		"per_page": perPage,
	})
}
