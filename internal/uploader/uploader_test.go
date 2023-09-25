package uploader_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/uploader"
	"github.com/mylxsw/asteria/log"
)

func TestUploader_DeleteFile(t *testing.T) {
	client := uploader.New(&config.Config{
		StorageAppKey:    os.Getenv("QINIU_ACCESS_KEY"),
		StorageAppSecret: os.Getenv("QINIU_SECRET_KEY"),
		StorageBucket:    "aicode",
	})

	// 这里填写要删除的文件列表，不要包含 URL 前缀，每行一个文件
	filesToDelete := ``

	for _, f := range strings.Split(filesToDelete, "\n") {
		if f == "" {
			continue
		}

		if err := client.RemoveFile(context.TODO(), f); err != nil {
			log.WithFields(log.Fields{"file": f}).Errorf("delete file failed: %v", err)
		}
	}

}
