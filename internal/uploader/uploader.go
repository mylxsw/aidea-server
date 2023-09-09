package uploader

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/go-utils/must"
	"github.com/mylxsw/go-utils/ternary"
	qiniuAuth "github.com/qiniu/go-sdk/v7/auth"
	"github.com/qiniu/go-sdk/v7/storage"
	"golang.org/x/net/proxy"
)

// DefaultUploadExpireAfterDays 默认上传文件过期时间，0 表示永不过期
const DefaultUploadExpireAfterDays = 0

type Uploader struct {
	conf       *config.Config
	baseURL    string
	httpClient *http.Client
}

func NewUploader(resolver infra.Resolver, conf *config.Config) *Uploader {
	client := &http.Client{Timeout: 120 * time.Second}
	if conf.Socks5Proxy != "" {
		resolver.MustResolve(func(dialer proxy.Dialer) {
			client.Transport = &http.Transport{Dial: dialer.Dial}
		})
	}

	return &Uploader{conf: conf, baseURL: "https://ssl.aicode.cc", httpClient: client}
}

type UploadInit struct {
	Filename string `json:"filename"`
	Token    string `json:"token"`
	Bucket   string `json:"bucket"`
	Key      string `json:"key"`
	URL      string `json:"url"`
}

const (
	UploadUsageAvatar = "avatar"
)

func (u *Uploader) Init(filename string, uid int, usage string, maxSizeInMB int64, expireAfterDays int, enableCallback bool) UploadInit {
	putPolicy := storage.PutPolicy{
		Scope:           u.conf.StorageBucket,
		FsizeLimit:      1024 * 1024 * maxSizeInMB,
		DeleteAfterDays: expireAfterDays,
	}

	if enableCallback {
		putPolicy.CallbackURL = u.conf.StorageCallback
		putPolicy.CallbackBodyType = "application/json"
		putPolicy.CallbackBody = fmt.Sprintf(`{"key":"$(key)","hash":"$(etag)","fsize":$(fsize),"bucket":"$(bucket)","name":"$(x:name)","uid":%d,"usage":"%s"}`, uid, usage)
	}

	mac := qiniuAuth.New(u.conf.StorageAppKey, u.conf.StorageAppSecret)

	var publicUrl, key string
	switch usage {
	case UploadUsageAvatar:
		key = fmt.Sprintf("ai-server/%d/avatar/ugc%s.%s", uid, must.Must(uuid.GenerateUUID()), fileExt(filename))
		publicUrl = fmt.Sprintf("%s/%s-avatar", u.baseURL, key)
	default:
		key = fmt.Sprintf("ai-server/%d/%s/ugc%s.%s", uid, time.Now().Format("20060102"), must.Must(uuid.GenerateUUID()), fileExt(filename))
		publicUrl = fmt.Sprintf("%s/%s", u.baseURL, key)
	}

	return UploadInit{
		Filename: filename,
		Token:    putPolicy.UploadToken(mac),
		Bucket:   u.conf.StorageBucket,
		Key:      key,
		URL:      publicUrl,
	}
}

func (u *Uploader) Upload(ctx context.Context, init UploadInit) (string, error) {
	cfg := storage.Config{}
	cfg.Region = &storage.ZoneHuadong
	cfg.UseHTTPS = true
	cfg.UseCdnDomains = true

	formUploader := storage.NewFormUploader(&cfg)
	ret := storage.PutRet{}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := formUploader.PutFile(ctx, &ret, init.Token, init.Key, init.Filename, nil)
	if err != nil {
		return "", err
	}

	return init.URL, nil
}

func (u *Uploader) UploadRemoteFile(ctx context.Context, url string, uid int, expiredAfterDays int, ext string, breakWall bool) (string, error) {
	res, err := u.uploadRemoteFile(ctx, url, uid, expiredAfterDays, ext, breakWall)
	if err != nil {
		time.Sleep(500 * time.Millisecond)
		return u.uploadRemoteFile(ctx, url, uid, expiredAfterDays, ext, breakWall)
	}

	return res, nil
}

func (u *Uploader) uploadRemoteFile(ctx context.Context, url string, uid int, expiredAfterDays int, ext string, breakWall bool) (string, error) {
	client := ternary.If(breakWall, u.httpClient, &http.Client{Timeout: 120 * time.Second})
	resp, err := client.Get(url)
	if err != nil {
		time.Sleep(500 * time.Millisecond)

		resp, err = client.Get(url)
		if err != nil {
			return "", fmt.Errorf("download remote file failed: %w", err)
		}
	}

	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read remote file failed: %w", err)
	}

	return u.UploadStream(ctx, uid, expiredAfterDays, data, ext)
}

func (u *Uploader) UploadStream(ctx context.Context, uid int, expireAfterDays int, data []byte, ext string) (string, error) {
	res, err := u.uploadStream(ctx, uid, expireAfterDays, data, ext)
	if err != nil {
		time.Sleep(500 * time.Millisecond)
		return u.uploadStream(ctx, uid, expireAfterDays, data, ext)
	}

	return res, nil
}

func (u *Uploader) uploadStream(ctx context.Context, uid int, expireAfterDays int, data []byte, ext string) (string, error) {
	putPolicy := storage.PutPolicy{
		Scope:           u.conf.StorageBucket,
		FsizeLimit:      1024 * 1024 * 20,
		DeleteAfterDays: expireAfterDays,
	}
	mac := qiniuAuth.New(u.conf.StorageAppKey, u.conf.StorageAppSecret)
	upToken := putPolicy.UploadToken(mac)

	cfg := storage.Config{}
	cfg.Region = &storage.ZoneHuadong
	cfg.UseHTTPS = true
	cfg.UseCdnDomains = true

	formUploader := storage.NewFormUploader(&cfg)
	ret := storage.PutRet{}

	key := fmt.Sprintf("ai-server/%d/%s/aigc%s.%s", uid, time.Now().Format("20060102"), must.Must(uuid.GenerateUUID()), ext)

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	err := formUploader.Put(ctx, &ret, upToken, key, bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		return "", fmt.Errorf("upload file failed: %w", err)
	}

	return fmt.Sprintf("%s/%s", u.baseURL, key), nil
}

func fileExt(filename string) string {
	return strings.ToLower(path.Ext(filename))
}
