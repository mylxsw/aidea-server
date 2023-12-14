package file

import (
	"context"
	"errors"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"github.com/mylxsw/asteria/log"
	"time"
)

type File struct {
	up    *uploader.Uploader
	cache *repo.CacheRepo
}

func New(up *uploader.Uploader, cache *repo.CacheRepo) *File {
	return &File{up: up, cache: cache}
}

func (fi *File) UploadTempFileData(ctx context.Context, data []byte, ext string, expireAfterDays int) (string, error) {
	hash := misc.Sha1(data)
	key := fmt.Sprintf("temp_file:%s.%s", hash, ext)

	res, err := fi.cache.Get(ctx, key)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return "", err
	}

	if res != "" {
		return res, nil
	}

	res, err = fi.up.UploadStream(ctx, 0, expireAfterDays, data, ext)
	if err != nil {
		return "", err
	}

	if err := fi.cache.Set(ctx, key, res, time.Duration(expireAfterDays)*24*time.Hour); err != nil {
		log.Errorf("cache temp file [%s] failed: %s", key, err)
	}

	return res, nil
}
