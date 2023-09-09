package coins

import (
	"fmt"
	"testing"
)

func TestBuildDescription(t *testing.T) {
	fmt.Println(buildDescription(50))
	fmt.Println(buildDescription(700))
	fmt.Println(buildDescription(1500))
	fmt.Println(buildDescription(5000))
	fmt.Println(buildDescription(10000))
}
