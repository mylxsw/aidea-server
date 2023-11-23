package voice_test

import (
	"fmt"
	"math"
	"testing"
)

func TestPayCount(t *testing.T) {
	fmt.Println(int64(math.Ceil(float64(1) / 2.0)))
	fmt.Println(int64(math.Ceil(float64(2) / 2.0)))
	fmt.Println(int64(math.Ceil(float64(3) / 2.0)))
}
