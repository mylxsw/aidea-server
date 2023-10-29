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
	withLab       bool      `json:"-"`
}

func (u User) InternalUser() bool {
	return u.UserType == repo.UserTypeInternal || u.withLab
}

func (u User) ExtraPermissionUser() bool {
	return u.UserType == repo.UserTypeExtraPermission
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
		withLab: array.In(user.Id, []int64{1}),
	}
}
