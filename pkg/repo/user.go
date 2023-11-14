package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
	model2 "github.com/mylxsw/aidea-server/pkg/repo/model"
	"strings"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/eloquent"
	"github.com/mylxsw/eloquent/query"
	"github.com/mylxsw/go-utils/array"
	"github.com/mylxsw/go-utils/must"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/guregu/null.v3"
)

var (
	ErrUserInvalidCredentials = errors.New("用户名或密码错误")
	ErrUserAccountDisabled    = errors.New("账号已被注销")
	ErrUserExists             = errors.New("用户已存在")
)

const (
	UserStatusActive  = "active"
	UserStatusDeleted = "deleted"
)

const (
	// UserTypeNormal 普通用户
	UserTypeNormal = 0
	// UserTypeInternal 内部用户
	UserTypeInternal = 1
	// UserTypeTester 测试用户
	UserTypeTester = 2
	// UserTypeExtraPermission 例外用户
	UserTypeExtraPermission = 3
)

type UserRepo struct {
	db   *sql.DB
	conf *config.Config
}

func NewUserRepo(db *sql.DB, conf *config.Config) *UserRepo {
	return &UserRepo{db: db, conf: conf}
}

// GetUserByInviteCode 根据邀请码获取用户信息
func (repo *UserRepo) GetUserByInviteCode(ctx context.Context, code string) (*model2.Users, error) {
	user, err := model2.NewUsersModel(repo.db).First(ctx, query.Builder().Where(model2.FieldUsersInviteCode, code))
	if err != nil {
		if errors.Is(err, query.ErrNoResult) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	if user.Status.ValueOrZero() == UserStatusDeleted {
		return nil, ErrUserAccountDisabled
	}

	ret := user.ToUsers()
	return &ret, nil
}

// UpdateUserInviteBy 更新用户的邀请人信息
func (repo *UserRepo) UpdateUserInviteBy(ctx context.Context, userId int64, invitedByUserId int64) error {
	_, err := model2.NewUsersModel(repo.db).Update(ctx, query.Builder().Where(model2.FieldUsersId, userId), model2.UsersN{
		InvitedBy: null.IntFrom(invitedByUserId),
	})

	return err
}

// GenerateInviteCode 为用户生成邀请码
func (repo *UserRepo) GenerateInviteCode(ctx context.Context, userId int64) error {
	_, err := model2.NewUsersModel(repo.db).Update(ctx, query.Builder().Where(model2.FieldUsersId, userId), model2.UsersN{
		InviteCode: null.StringFrom(misc.HashID(userId)),
	})

	return err
}

// GetUserByID 根据用户ID获取用户信息
func (repo *UserRepo) GetUserByID(ctx context.Context, userID int64) (*model2.Users, error) {
	user, err := model2.NewUsersModel(repo.db).First(ctx, query.Builder().Where(model2.FieldUsersId, userID))
	if err != nil {
		if errors.Is(err, query.ErrNoResult) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	if user.Status.ValueOrZero() == UserStatusDeleted {
		return nil, ErrUserAccountDisabled
	}

	ret := user.ToUsers()
	return &ret, nil
}

// GetUserByPhone 根据用户手机号获取用户信息
func (repo *UserRepo) GetUserByPhone(ctx context.Context, phone string) (*model2.Users, error) {
	user, err := model2.NewUsersModel(repo.db).First(ctx, query.Builder().Where(model2.FieldUsersPhone, phone))
	if err != nil {
		if errors.Is(err, query.ErrNoResult) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	if user.Status.ValueOrZero() == UserStatusDeleted {
		return nil, ErrUserAccountDisabled
	}

	ret := user.ToUsers()
	return &ret, nil
}

// GetUserByEmail 根据用户邮箱地址获取用户信息
func (repo *UserRepo) GetUserByEmail(ctx context.Context, username string) (*model2.Users, error) {
	user, err := model2.NewUsersModel(repo.db).First(ctx, query.Builder().Where(model2.FieldUsersEmail, username))
	if err != nil {
		if errors.Is(err, query.ErrNoResult) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	if user.Status.ValueOrZero() == UserStatusDeleted {
		return nil, ErrUserAccountDisabled
	}

	ret := user.ToUsers()
	return &ret, nil
}

// VerifyPassword 验证用户密码
func (repo *UserRepo) VerifyPassword(ctx context.Context, userID int64, password string) error {
	user, err := model2.NewUsersModel(repo.db).First(ctx, query.Builder().Where(model2.FieldUsersId, userID))
	if err != nil {
		if errors.Is(err, query.ErrNoResult) {
			return ErrNotFound
		}

		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password.ValueOrZero()), []byte(password)); err != nil {
		return ErrUserInvalidCredentials
	}

	return nil
}

// UpdateStatus 更新用户状态
func (repo *UserRepo) UpdateStatus(ctx context.Context, userID int64, status string) error {
	_, err := model2.NewUsersModel(repo.db).Update(ctx, query.Builder().Where(model2.FieldUsersId, userID), model2.UsersN{
		Status: null.StringFrom(status),
	})

	return err
}

// UpdatePassword 更新用户密码
func (repo *UserRepo) UpdatePassword(ctx context.Context, userID int64, password string) error {
	encryptedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = model2.NewUsersModel(repo.db).Update(ctx, query.Builder().Where(model2.FieldUsersId, userID), model2.UsersN{
		Password: null.StringFrom(string(encryptedPassword)),
	})

	return err
}

// UpdateAvatarURL 更新用户头像
func (repo *UserRepo) UpdateAvatarURL(ctx context.Context, userID int64, avatarURL string) error {
	_, err := model2.NewUsersModel(repo.db).Update(ctx, query.Builder().Where(model2.FieldUsersId, userID), model2.UsersN{
		Avatar: null.StringFrom(avatarURL),
	})

	return err
}

// UpdateRealname 更新用户真实姓名
func (repo *UserRepo) UpdateRealname(ctx context.Context, userID int64, realname string) error {
	_, err := model2.NewUsersModel(repo.db).Update(ctx, query.Builder().Where(model2.FieldUsersId, userID), model2.UsersN{
		Realname: null.StringFrom(realname),
	})

	return err
}

// SignUpPhone 使用手机号注册用户
func (repo *UserRepo) SignUpPhone(ctx context.Context, username string, password string, realname string) (user *model2.Users, eventID int64, err error) {
	if err = eloquent.Transaction(repo.db, func(tx query.Database) error {
		q := query.Builder().Where(model2.FieldUsersPhone, username)
		matchedCount, err := model2.NewUsersModel(tx).Count(ctx, q)
		if err != nil {
			return err
		}

		if matchedCount > 0 {
			return ErrUserExists
		}

		user = &model2.Users{
			Phone:    username,
			Realname: realname,
			Status:   UserStatusActive,
		}

		if password != "" {
			encryptedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				return err
			}

			user.Password = string(encryptedPassword)
		}

		id, err := model2.NewUsersModel(tx).Save(ctx, user.ToUsersN(
			model2.FieldUsersPhone,
			model2.FieldUsersPassword,
			model2.FieldUsersRealname,
			model2.FieldUsersStatus,
		))
		if err != nil {
			return err
		}
		user.Id = id

		if eventID, err = model2.NewEventsModel(tx).Save(ctx, model2.EventsN{
			EventType: null.StringFrom(EventTypeUserCreated),
			Payload:   null.StringFrom(string(must.Must(json.Marshal(UserCreatedEvent{UserID: user.Id, From: UserCreatedEventSourcePhone})))),
			Status:    null.StringFrom(EventStatusWaiting),
		}); err != nil {
			log.With(user).Errorf("create event failed: %s", err)
			return err
		}

		return nil
	}); err != nil {
		return nil, 0, err
	}

	return user, eventID, nil
}

// SignUpEmail 使用邮箱注册用户
func (repo *UserRepo) SignUpEmail(ctx context.Context, username string, password string, realname string) (user *model2.Users, eventID int64, err error) {
	if err = eloquent.Transaction(repo.db, func(tx query.Database) error {
		q := query.Builder().Where(model2.FieldUsersEmail, username)
		matchedCount, err := model2.NewUsersModel(tx).Count(ctx, q)
		if err != nil {
			return err
		}

		if matchedCount > 0 {
			return ErrUserExists
		}

		user = &model2.Users{
			Email:    username,
			Realname: realname,
			Status:   UserStatusActive,
		}

		if password != "" {
			encryptedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			user.Password = string(encryptedPassword)
		}

		id, err := model2.NewUsersModel(tx).Save(ctx, user.ToUsersN(
			model2.FieldUsersEmail,
			model2.FieldUsersPassword,
			model2.FieldUsersRealname,
			model2.FieldUsersStatus,
		))
		if err != nil {
			return err
		}
		user.Id = id

		if eventID, err = model2.NewEventsModel(tx).Save(ctx, model2.EventsN{
			EventType: null.StringFrom(EventTypeUserCreated),
			Payload:   null.StringFrom(string(must.Must(json.Marshal(UserCreatedEvent{UserID: user.Id, From: UserCreatedEventSourceEmail})))),
			Status:    null.StringFrom(EventStatusWaiting),
		}); err != nil {
			log.With(user).Errorf("create event failed: %s", err)
			return err
		}

		return nil
	}); err != nil {
		return nil, 0, err
	}

	return user, eventID, nil
}

// SignIn 用户登录
func (repo *UserRepo) SignIn(ctx context.Context, emailOrPhone, password string) (*model2.Users, error) {
	q := query.Builder().Where(model2.FieldUsersEmail, emailOrPhone).OrWhere(model2.FieldUsersPhone, emailOrPhone)
	user, err := model2.NewUsersModel(repo.db).First(ctx, q)
	if err != nil {
		if errors.Is(err, query.ErrNoResult) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	if user.Status.ValueOrZero() == UserStatusDeleted {
		return nil, ErrUserAccountDisabled
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password.ValueOrZero()), []byte(password)); err != nil {
		return nil, ErrUserInvalidCredentials
	}

	u := user.ToUsers()
	return &u, nil
}

// AppleSignIn Apple 登录
func (repo *UserRepo) AppleSignIn(
	ctx context.Context,
	appleUID string,
	email string,
	isPrivateEmail bool,
	familyName, givenName string,
) (user *model2.Users, eventID int64, err error) {
	err = eloquent.Transaction(repo.db, func(tx query.Database) error {
		q := query.Builder().
			Where(model2.FieldUsersAppleUid, appleUID).
			OrWhere(model2.FieldUsersEmail, email)
		matched, err := model2.NewUsersModel(tx).Get(ctx, q)
		if err != nil {
			return err
		}

		// 如果没有匹配的用户，那么创建一个新用户
		if len(matched) == 0 {
			user = &model2.Users{
				AppleUid: appleUID,
				Email:    email,
				Realname: strings.TrimSpace(givenName + " " + familyName),
				Status:   UserStatusActive,
			}

			id, err := model2.NewUsersModel(tx).Save(ctx, user.ToUsersN(
				model2.FieldUsersAppleUid,
				model2.FieldUsersEmail,
				model2.FieldUsersRealname,
				model2.FieldUsersStatus,
			))
			if err != nil {
				return err
			}
			user.Id = id

			if eventID, err = model2.NewEventsModel(tx).Save(ctx, model2.EventsN{
				EventType: null.StringFrom(EventTypeUserCreated),
				Payload:   null.StringFrom(string(must.Must(json.Marshal(UserCreatedEvent{UserID: user.Id, From: "apple"})))),
				Status:    null.StringFrom(EventStatusWaiting),
			}); err != nil {
				log.With(user).Errorf("create event failed: %s", err)
				return err
			}

			return nil
		}

		// 如果只有一个匹配的用户，那么直接返回
		if len(matched) == 1 {
			matched[0].AppleUid = null.StringFrom(appleUID)
			if err := matched[0].Save(ctx, model2.FieldUsersAppleUid); err != nil {
				log.With(matched[0]).Errorf("update user failed: %s", err)
			}

			matchedUser := matched[0].ToUsers()
			user = &matchedUser
			return nil
		}

		// 如果有多个匹配的用户
		appleLoginUser := array.Filter(matched, func(user model2.UsersN, _ int) bool { return user.AppleUid.ValueOrZero() == appleUID })
		if len(appleLoginUser) != 1 {
			return errors.New("apple login failed: multiple users matched")
		}

		matchedUser := appleLoginUser[0].ToUsers()
		user = &matchedUser
		return nil
	})

	if user != nil && user.Status == UserStatusDeleted {
		return nil, 0, ErrUserAccountDisabled
	}

	return user, eventID, err
}

func (repo *UserRepo) BindPhone(ctx context.Context, userID int64, phone string, sendEvent bool) (eventID int64, err error) {
	q := query.Builder().Where(model2.FieldUsersId, userID)
	err = eloquent.Transaction(repo.db, func(tx query.Database) error {
		if _, err := model2.NewUsersModel(tx).Update(
			ctx,
			q,
			model2.UsersN{
				Phone: null.StringFrom(phone),
			},
			model2.FieldUsersPhone,
		); err != nil {
			return fmt.Errorf("update user's phone failed: %w", err)
		}

		if !sendEvent {
			return nil
		}

		if eventID, err = model2.NewEventsModel(tx).Save(ctx, model2.EventsN{
			EventType: null.StringFrom(EventTypeUserPhoneBound),
			Payload:   null.StringFrom(string(must.Must(json.Marshal(UserBindEvent{UserID: userID, Phone: phone})))),
			Status:    null.StringFrom(EventStatusWaiting),
		}); err != nil {
			log.WithFields(log.Fields{
				"user_id": userID,
				"phone":   phone,
			}).Errorf("create event failed: %s", err)
			return err
		}

		return nil
	})

	return
}

// UserCustomConfig 用户自定义配置
type UserCustomConfig struct {
	// HomeModels 主页显示的模型
	HomeModels []string `json:"home_models,omitempty"`
}

// CustomConfig 查询用户自定义配置
func (repo *UserRepo) CustomConfig(ctx context.Context, userID int64) (*UserCustomConfig, error) {
	user, err := model2.NewUserCustomModel(repo.db).First(ctx, query.Builder().Where(model2.FieldUserCustomUserId, userID))
	if err != nil && !errors.Is(err, query.ErrNoResult) {
		return nil, fmt.Errorf("查询用户自定义配置失败：%w", err)
	}

	var configData string
	if errors.Is(err, query.ErrNoResult) {
		configData = "{}"
	} else {
		configData = user.Config.ValueOrZero()
	}

	var customConfig UserCustomConfig
	if err := json.Unmarshal([]byte(configData), &customConfig); err != nil {
		return nil, fmt.Errorf("解析用户 %d 自定义配置失败：%w", userID, err)
	}

	return &customConfig, nil
}

// UpdateCustomConfig 更新用户自定义配置
func (repo *UserRepo) UpdateCustomConfig(ctx context.Context, userID int64, conf UserCustomConfig) error {
	configData, err := json.Marshal(conf)
	if err != nil {
		return fmt.Errorf("序列化用户 %d 自定义配置失败：%w", userID, err)
	}

	return eloquent.Transaction(repo.db, func(tx query.Database) error {
		q := query.Builder().Where(model2.FieldUserCustomUserId, userID)

		cus, err := model2.NewUserCustomModel(tx).First(ctx, q)
		if err != nil && !errors.Is(err, query.ErrNoResult) {
			return fmt.Errorf("查询用户自定义配置失败：%w", err)
		}

		if errors.Is(err, query.ErrNoResult) {
			_, err = model2.NewUserCustomModel(tx).Save(ctx, model2.UserCustomN{
				UserId: null.IntFrom(userID),
				Config: null.StringFrom(string(configData)),
			})
			return err
		}

		_, err = model2.NewUserCustomModel(tx).UpdateById(
			ctx,
			cus.Id.ValueOrZero(),
			model2.UserCustomN{Config: null.StringFrom(string(configData))},
			model2.FieldUserCustomConfig,
		)

		return err
	})
}

const (
	UserApiKeyStatusDisabled = 2
	UserAPiKeyStatusActive   = 1
)

// GetUserByAPIKey 根据 API Token 获取用户信息
func (repo *UserRepo) GetUserByAPIKey(ctx context.Context, token string) (*model2.Users, error) {
	key, err := model2.NewUserApiKeyModel(repo.db).First(ctx, query.Builder().Where(model2.FieldUserApiKeyToken, token))
	if err != nil {
		if errors.Is(err, query.ErrNoResult) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	apiKey := key.ToUserApiKey()
	if apiKey.Status == UserApiKeyStatusDisabled {
		return nil, ErrNotFound
	}

	if !apiKey.ValidBefore.IsZero() && apiKey.ValidBefore.Before(time.Now()) {
		return nil, ErrNotFound
	}

	return repo.GetUserByID(ctx, apiKey.UserId)
}

// GetAPIKeys 获取用户的 API Keys
func (repo *UserRepo) GetAPIKeys(ctx context.Context, userID int64) ([]model2.UserApiKey, error) {
	q := query.Builder().Where(model2.FieldUserApiKeyUserId, userID).
		Where(model2.FieldUserApiKeyStatus, UserAPiKeyStatusActive)
	keys, err := model2.NewUserApiKeyModel(repo.db).Get(ctx, q)
	if err != nil {
		return nil, err
	}

	return array.Map(keys, func(key model2.UserApiKeyN, _ int) model2.UserApiKey {
		item := key.ToUserApiKey()
		item.Token = misc.MaskStr(item.Token, 6)
		return item
	}), nil
}

// GetAPIKey 获取用户的 API Key
func (repo *UserRepo) GetAPIKey(ctx context.Context, userID int64, keyID int64) (*model2.UserApiKey, error) {
	key, err := model2.NewUserApiKeyModel(repo.db).First(ctx, query.Builder().
		Where(model2.FieldUserApiKeyUserId, userID).
		Where(model2.FieldUserApiKeyId, keyID).
		Where(model2.FieldUserApiKeyStatus, UserAPiKeyStatusActive),
	)
	if err != nil {
		if errors.Is(err, query.ErrNoResult) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	ret := key.ToUserApiKey()
	return &ret, nil
}

// CreateAPIKey 创建一个 API Token
func (repo *UserRepo) CreateAPIKey(ctx context.Context, userID int64, name string, validBefore time.Time) (string, error) {
	key := model2.UserApiKey{
		UserId:      userID,
		Name:        name,
		ValidBefore: validBefore,
		Status:      UserAPiKeyStatusActive,
		Token:       fmt.Sprintf("sk-%s", misc.GenerateAPIToken(name, userID)),
	}

	allows := []string{
		model2.FieldUserApiKeyUserId,
		model2.FieldUserApiKeyName,
		model2.FieldUserApiKeyToken,
		model2.FieldUserApiKeyStatus,
	}

	if !validBefore.IsZero() {
		allows = append(allows, model2.FieldUserApiKeyValidBefore)
	}

	id, err := model2.NewUserApiKeyModel(repo.db).Save(ctx, key.ToUserApiKeyN(allows...))
	if err != nil {
		return "", err
	}

	key.Id = id
	return key.Token, nil
}

// DeleteAPIKey 删除一个 API Key
func (repo *UserRepo) DeleteAPIKey(ctx context.Context, userID int64, keyID int64) error {
	//_, err := model.NewUserApiKeyModel(repo.db).Delete(ctx, query.Builder().
	//	Where(model.FieldUserApiKeyUserId, userID).
	//	Where(model.FieldUserApiKeyId, keyID),
	//)

	q := query.Builder().Where(model2.FieldUserApiKeyUserId, userID).Where(model2.FieldUserApiKeyId, keyID)
	update := query.KV{model2.FieldUserApiKeyStatus: UserApiKeyStatusDisabled}

	_, err := model2.NewUserApiKeyModel(repo.db).UpdateFields(ctx, update, q)
	return err
}
