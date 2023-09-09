package token

import (
	"errors"
	"fmt"
	"time"

	jwtlib "github.com/dgrijalva/jwt-go"
)

// ErrTokenInvalid token不合法错误
var ErrTokenInvalid = errors.New("jwt token invalid")

// Token token对象
type Token struct {
	key string
}

// Claims payload内容
type Claims map[string]interface{}

func (c Claims) StringValue(key string) string {
	if v, ok := c[key]; ok {
		return fmt.Sprintf("%v", v)
	}

	return ""
}

func (c Claims) IntValue(key string) int {
	if v, ok := c[key]; ok {
		if v, ok := v.(float64); ok {
			return int(v)
		}
	}

	return 0
}

func (c Claims) Int64Value(key string) int64 {
	if v, ok := c[key]; ok {
		if v, ok := v.(float64); ok {
			return int64(v)
		}
	}

	return 0
}

func (c Claims) Float64Value(key string) float64 {
	if v, ok := c[key]; ok {
		if v, ok := v.(float64); ok {
			return v
		}
	}

	return 0
}

// New 创建一个Token
func New(key string) *Token {

	return &Token{
		key: key,
	}
}

// CreateToken 创建Token
func (jt *Token) CreateToken(payloads Claims, expire time.Duration) string {

	// 设置标准payload，指定有效时间范围
	// 签署时间允许误差60秒
	if expire != 0 {
		payloads["exp"] = time.Now().Add(expire).Unix()
	}
	payloads["iat"] = time.Now().Add(-60 * time.Second).Unix()

	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, jwtlib.MapClaims(payloads))
	str, err := token.SignedString([]byte(jt.key))
	if err != nil {
		return ""
	}

	return str
}

// ParseToken 解析Token
func (jt *Token) ParseToken(tokenString string) (Claims, error) {
	token, err := jwtlib.Parse(tokenString, func(token *jwtlib.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(jt.key), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwtlib.MapClaims); ok && token.Valid {
		return Claims(claims), nil
	}

	return nil, ErrTokenInvalid
}
