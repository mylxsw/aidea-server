package wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
)

type WeChat struct {
	appID  string
	secret string
}

func NewWeChat(appID, secret string) *WeChat {
	return &WeChat{appID: appID, secret: secret}
}

type AccessToken struct {
	// AccessToken 接口调用凭证
	AccessToken string `json:"access_token"`
	// ExpiresIn access_token 接口调用凭证超时时间，单位（秒）
	ExpiresIn int64 `json:"expires_in,omitempty"`
	// RefreshToken 用户刷新 access_token
	RefreshToken string `json:"refresh_token,omitempty"`
	// OpenID 授权用户唯一标识
	OpenID string `json:"openid,omitempty"`
	// Scope 用户授权的作用域（snsapi_userinfo）
	Scope string `json:"scope,omitempty"`
	// UnionID 当且仅当该移动应用已获得该用户的 userinfo 授权时，才会出现该字段
	UnionID string `json:"unionid,omitempty"`

	ErrCode int64  `json:"errcode,omitempty"`
	ErrMsg  string `json:"errmsg,omitempty"`
}

// OAuthAccessToken 获取微信 access token
func (wc *WeChat) OAuthAccessToken(ctx context.Context, code string) (*AccessToken, error) {
	resp, err := misc.RestyClient(2).R().
		SetContext(ctx).
		SetQueryParam("appid", wc.appID).
		SetQueryParam("secret", wc.secret).
		SetQueryParam("grant_type", "authorization_code").
		SetQueryParam("code", code).
		Post("https://api.weixin.qq.com/sns/oauth2/access_token")
	if err != nil {
		return nil, err
	}

	if resp.IsError() {
		return nil, fmt.Errorf("request wechat access token failed: %s", resp.String())
	}

	var accessToken AccessToken
	if err := json.Unmarshal(resp.Body(), &accessToken); err != nil {
		return nil, err
	}

	if accessToken.ErrCode != 0 {
		return nil, fmt.Errorf("request wechat access token failed: %s", accessToken.ErrMsg)
	}

	return &accessToken, nil
}

type UserInfo struct {
	// OpenID 普通用户的标识，对当前开发者账号唯一
	OpenID string `json:"openid"`
	// NickName 普通用户昵称
	NickName string `json:"nickname"`
	// Sex 普通用户性别，1 为男性，2 为女性
	Sex int64 `json:"sex,omitempty"`
	// Province 普通用户个人资料填写的省份
	Province string `json:"province,omitempty"`
	// City 普通用户个人资料填写的城市
	City string `json:"city,omitempty"`
	// Country 国家，如中国为 CN
	Country string `json:"country,omitempty"`
	// HeadImgURL 用户头像，最后一个数值代表正方形头像大小（有 0、46、64、96、132 数值可选，0 代表 640*640 正方形头像），用户没有头像时该项为空
	HeadImgURL string `json:"headimgurl,omitempty"`
	// UnionID 用户统一标识。针对一个微信开放平台账号下的应用，同一用户的 unionid 是唯一的
	UnionID string `json:"unionid,omitempty"`

	ErrCode int64  `json:"errcode,omitempty"`
	ErrMsg  string `json:"errmsg,omitempty"`
}

// QueryUserInfo 查询用户信息
func (wc *WeChat) QueryUserInfo(ctx context.Context, accToken string, openID string) (*UserInfo, error) {
	resp, err := misc.RestyClient(2).R().
		SetContext(ctx).
		SetQueryParam("access_token", accToken).
		SetQueryParam("openid", openID).
		Post("https://api.weixin.qq.com/sns/userinfo")
	if err != nil {
		return nil, err
	}

	if resp.IsError() {
		return nil, fmt.Errorf("request wechat userinfo failed: %s", resp.String())
	}

	var userInfo UserInfo
	if err := json.Unmarshal(resp.Body(), &userInfo); err != nil {
		return nil, err
	}

	if userInfo.ErrCode != 0 {
		return nil, fmt.Errorf("request wechat userinfo failed: %s", userInfo.ErrMsg)
	}

	return &userInfo, nil
}
