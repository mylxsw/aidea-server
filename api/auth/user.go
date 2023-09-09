package auth

import (
	"time"

	"github.com/mylxsw/aidea-server/internal/repo/model"
	"github.com/mylxsw/go-utils/array"

	"github.com/mylxsw/aidea-server/internal/repo"
)

// User 用户信息
type User struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Email         string    `json:"email"`
	Phone         string    `json:"phone"`
	InviteCode    string    `json:"invite_code,omitempty"`
	InvitedBy     int64     `json:"invited_by,omitempty"`
	Avatar        string    `json:"avatar,omitempty"`
	UserType      int64     `json:"user_type,omitempty"`
	AppleUID      string    `json:"apple_uid,omitempty"`
	IsSetPassword bool      `json:"is_set_password,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	WithLab       bool      `json:"-"`
}

func (u User) InternalUser() bool {
	return u.UserType == repo.UserTypeInternal
}

// UserOptional 用户信息，可选，如果用户未登录，则为 User 为 nil
type UserOptional struct {
	User *User `json:"user"`
}

func CreateAuthUserFromModel(user *model.Users) *User {
	if user == nil {
		return nil
	}

	return &User{
		ID:            user.Id,
		Name:          user.Realname,
		Email:         user.Email,
		Phone:         user.Phone,
		InviteCode:    user.InviteCode,
		InvitedBy:     user.InvitedBy,
		Avatar:        user.Avatar,
		UserType:      user.UserType,
		AppleUID:      user.AppleUid,
		IsSetPassword: user.Password != "",
		CreatedAt:     user.CreatedAt,
		// 仅限实验室用户
		WithLab: array.In(user.Id, []int64{
			5,  /* 18678859721 */
			24, /* 17347870010 */
			10, /* 18888888888 */
			11, /* 18883185443 */
			12, /* 18883184444 */
			14, /* 18746463333 */
			15, /* 18566669988 */
			16, /* 18564647784 */
		}),
	}
}
