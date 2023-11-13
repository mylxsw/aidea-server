package uploader_test

import (
	"context"
	"fmt"
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

func TestUploader_DownloadFile(t *testing.T) {
	ret, err := uploader.DownloadRemoteFile(context.TODO(), "https://ssl.aicode.cc/ai-server/24/20231113/ugc29bd6ca3-41e0-5977-dbe4-8952e4583059..jpg")
	if err == nil {
		t.Error("should be error")
	}

	fmt.Println(ret, err)
}
