package uploader

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-uuid"
	"github.com/mylxsw/go-utils/str"
)

var supportFilters = []string{"-1024_square", "-512_square", "-avatar", "-maxsize700", "-maxsize800", "-square_500", "-thumb", "-thumb1000", "-thumb_500", "-fix_square_1024"}
var supportImages = []string{".jpg", ".jpeg", ".png", ".webp", ".gif"}

// BuildImageURLWithFilter build image url with filter
func BuildImageURLWithFilter(remoteURL string, filter, storageDomain string) string {
	if !strings.HasPrefix(remoteURL, storageDomain) {
		return remoteURL
	}

	if str.HasSuffixes(strings.ToLower(remoteURL), supportImages) {
		remoteURL = remoteURL + "-" + filter
	} else if str.HasSuffixes(strings.ToLower(remoteURL), supportFilters) {
		segs := strings.Split(remoteURL, "-")
		segs[len(segs)-1] = filter

		remoteURL = strings.Join(segs, "-")
	}

	return remoteURL
}

// DownloadRemoteFile download remote file to local
func DownloadRemoteFile(ctx context.Context, remoteURL string) (string, error) {
	if str.HasSuffixes(strings.ToLower(remoteURL), supportImages) {
		remoteURL = remoteURL + "-thumb"
	}

	resp, err := http.Get(remoteURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	prefix, _ := uuid.GenerateUUID()
	savePath := filepath.Join(os.TempDir(), prefix+"-"+filepath.Base(remoteURL))
	f, err := os.Create(savePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", err
	}

	return savePath, nil
}
