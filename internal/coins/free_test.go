package coins_test

import (
	"fmt"
	"testing"

	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/go-utils/assert"
	"gopkg.in/yaml.v3"
)

func TestFreeModels(t *testing.T) {
	data, err := yaml.Marshal(coins.FreeModels())
	assert.NoError(t, err)

	fmt.Println(string(data))
}
