package dingding_test

import (
	"github.com/mylxsw/aidea-server/pkg/dingding"
	"github.com/mylxsw/go-utils/assert"
	"testing"
)

func TestDingding_Send(t *testing.T) {
	ding := dingding.NewDingding(true, "xxxxxx/xxxxxxx/xxxxxxxx", "")
	assert.NoError(t, ding.Send(dingding.NewMarkdownMessage("Test", "Hello, world", []string{"mylxsw"})))
}
