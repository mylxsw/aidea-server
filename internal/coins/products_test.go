package coins

import (
	"fmt"
	"testing"

	"github.com/mylxsw/go-utils/assert"
	"gopkg.in/yaml.v3"
)

func TestBuildDescription(t *testing.T) {
	fmt.Println(buildDescription(50))
	fmt.Println(buildDescription(700))
	fmt.Println(buildDescription(1500))
	fmt.Println(buildDescription(5000))
	fmt.Println(buildDescription(10000))
}

func TestProducts(t *testing.T) {
	data, err := yaml.Marshal(Products)
	assert.NoError(t, err)

	fmt.Println(string(data))
}
