package dingding_test

import (
	"testing"

	"github.com/mylxsw/aidea-server/pkg/dingding"
	"github.com/mylxsw/go-utils/assert"
)

func TestDingding_Send(t *testing.T) {
	ding := dingding.NewDingding(true, "xxxxxx/xxxxxxx/xxxxxxxx", "", true, "https://apprise.example.com", "token", "tags")
	assert.NoError(t, ding.Send(dingding.NewMarkdownMessage("Test", "Hello, world", []string{"mylxsw"})))
}
