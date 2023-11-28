package misc_test

import (
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"testing"
	"time"

	"github.com/mylxsw/go-utils/assert"
)

func TestIsChinese(t *testing.T) {
	assert.True(t, misc.IsChinese("中文"))
	assert.False(t, misc.IsChinese("English"))
	assert.True(t, misc.IsChinese("中文数据量大English"))
	assert.False(t, misc.IsChinese("中English"))
	assert.False(t, misc.IsChinese(""))
	assert.True(t, misc.IsChinese("钢铁侠"))
}

func TestParseAppleDateTime(t *testing.T) {
	dt := "2023-06-03 10:04:39 Etc/GMT"
	res, err := misc.ParseAppleDateTime(dt)
	assert.NoError(t, err)
	fmt.Println(res.In(time.Local).Format("2006-01-02 15:04:05"))
}

func TestHashID(t *testing.T) {
	hashIds := make(map[string]int)
	for i := 0; i < 10000; i++ {
		hash := misc.HashID(int64(i))
		if _, ok := hashIds[hash]; ok {
			t.Errorf("duplicate hash: %s, id=%d\n", hash, i)
		} else {
			hashIds[hash] = i
		}
	}
}

func TestVersionCompare(t *testing.T) {
	assert.True(t, misc.VersionNewer("1.0.1", "1.0.0"))
	assert.True(t, misc.VersionNewer("1.1.1", "1.0.1"))
	assert.False(t, misc.VersionNewer("1.0.0", "1.0.1"))

	assert.True(t, misc.VersionOlder("1.0.0", "2.0.0"))
	assert.True(t, misc.VersionOlder("1.0.0", "1.0.1"))
	assert.False(t, misc.VersionOlder("1.0.6", "1.0.6"))
}

func TestResolveAspectRadio(t *testing.T) {
	cases := [][]int{
		{512, 512},
		{512, 768},
		{768, 512},
		{512, 1024},
		{1024, 512},
		{1024, 1408},

		{1152, 896},
		{896, 1152},
		{1216, 832},
		{832, 1216},
		{1344, 768},
		{768, 1344},
		{1536, 640},
		{640, 1536},
		{1024, 1792},
	}

	for _, c := range cases {
		fmt.Printf("%6s => %dx%d\n", misc.ResolveAspectRatio(c[0], c[1]), c[0], c[1])
	}
}

func TestResolveHeightFromAspectRatio(t *testing.T) {
	widths := []int{512, 768, 1024}
	aspectRatios := []string{"1:1", "4:3", "16:9", "16:10", "3:2", "2:1"}

	for _, width := range widths {
		fmt.Printf("#######%d#######\n", width)
		for _, aspectRatio := range aspectRatios {
			if !exactDivision(misc.ResolveHeightFromAspectRatio(width, aspectRatio), 64) {
				continue
			}
			fmt.Printf("%6s  %dx%d\n", aspectRatio, width, misc.ResolveHeightFromAspectRatio(width, aspectRatio))
		}
	}
}

func exactDivision(value, by int) bool {
	return value%by == 0
}

func TestOrderID(t *testing.T) {
	fmt.Println(misc.OrderID(1))
	fmt.Println(misc.OrderID(2))
	fmt.Println(misc.OrderID(3))
	fmt.Println(misc.OrderID(4))
	fmt.Println(misc.OrderID(5))
	fmt.Println(misc.OrderID(6))
	fmt.Println(misc.OrderID(3499494))
	fmt.Println(misc.OrderID(34994954883))
}

func TestSplitText(t *testing.T) {
	for _, line := range misc.TextSplit("这个世界也太方框了啊，123456，abcdefg", 10) {
		fmt.Println(line)
	}
}

func TestTodayRemainTimeSeconds(t *testing.T) {
	fmt.Println(misc.TodayRemainTimeSeconds())
}

func TestWordCount(t *testing.T) {
	assert.EqualValues(t, 5, misc.WordCount("hello"))
	assert.EqualValues(t, 4, misc.WordCount("逍遥神剑"))
	assert.EqualValues(t, 12, len("逍遥神剑"))
}

func TestWordTruncate(t *testing.T) {
	assert.EqualValues(t, "逍遥神剑", misc.WordTruncate("逍遥神剑", 4))
	assert.EqualValues(t, "逍遥神", misc.WordTruncate("逍遥神剑", 3))
	assert.EqualValues(t, "逍", misc.WordTruncate("逍遥神剑", 1))
	assert.EqualValues(t, "", misc.WordTruncate("逍遥神剑", 0))
	assert.EqualValues(t, "逍遥神剑", misc.WordTruncate("逍遥神剑", 5))
}

func TestGenerateAPIToken(t *testing.T) {
	fmt.Println(misc.GenerateAPIToken("default", 11222233))
}

func TestUUID(t *testing.T) {
	fmt.Println(misc.UUID())
	fmt.Println(misc.UUID())
	fmt.Println(misc.UUID())
	fmt.Println(misc.UUID())
}

func TestFileExt(t *testing.T) {
	fmt.Println(misc.FileExt("abc.jpg"))
}
