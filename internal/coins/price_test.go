package coins_test

import (
	"fmt"
	"testing"

	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/go-utils/assert"
	"gopkg.in/yaml.v3"
)

func TestGetTokensForCoins(t *testing.T) {
	fmt.Println(coins.GetOpenAITextCoins("gpt-3.5-turbo", 1000))
	fmt.Println(coins.GetOpenAITokensForCoins("gpt-3.5-turbo", 5))
}

func TestPriceToCoins(t *testing.T) {
	testcases := map[float64]int64{
		100:        10000,
		10:         1000,
		100.123:    10013,
		100.126:    10013,
		100.5:      10050,
		0.5:        50,
		0.01:       1,
		0.00001:    1,
		0.00000001: 1,
		0.00125:    1,
		0.012503:   2,
		0.02:       2,
		0.023:      3,
		0.3:        30,
		0.35:       35,
		0.351:      36,
	}

	fmt.Printf("%6s => %7s %7s %7s %7s\n", "消费金额", "消费金币", "扣除金币", "收费", "利润")
	serviceRate := 1.0
	for price, c := range testcases {
		assert.Equal(t, coins.PriceToCoins(price, 0), c)
		fmt.Printf(
			"%10f => %10d %10d %10f %10f\n",
			price,
			c,
			coins.PriceToCoins(price, serviceRate),
			float64(coins.PriceToCoins(price, serviceRate))/100.0,
			float64(coins.PriceToCoins(price, serviceRate))/100.0-price,
		)
	}
}

type TokenUsage struct {
	Product string
	Count   int64
}

func TestTokenUsage(t *testing.T) {
	testcases := []TokenUsage{
		// {"gpt3.5-1", coins.GetOpenAITextCoins("gpt-3.5-turbo", 1)},
		// {"gpt3.5-16k-1", coins.GetOpenAITextCoins("gpt-3.5-turbo-16k", 1)},
		// {"gpt3.5-10", coins.GetOpenAITextCoins("gpt-3.5-turbo", 10)},
		// {"gpt3.5-16k-10", coins.GetOpenAITextCoins("gpt-3.5-turbo-16k", 10)},
		// {"gpt3.5-50", coins.GetOpenAITextCoins("gpt-3.5-turbo", 50)},
		// {"gpt3.5-16k-50", coins.GetOpenAITextCoins("gpt-3.5-turbo-16k", 50)},
		// {"gpt3.5-100", coins.GetOpenAITextCoins("gpt-3.5-turbo", 100)},
		// {"gpt3.5-16k-100", coins.GetOpenAITextCoins("gpt-3.5-turbo-16k", 100)},
		// {"gpt3.5-200", coins.GetOpenAITextCoins("gpt-3.5-turbo", 200)},
		// {"gpt3.5-16k-200", coins.GetOpenAITextCoins("gpt-3.5-turbo-16k", 200)},
		// {"gpt3.5-500", coins.GetOpenAITextCoins("gpt-3.5-turbo", 500)},
		// {"gpt3.5-16k-500", coins.GetOpenAITextCoins("gpt-3.5-turbo-16k", 500)},
		{"gpt3.5-1000", coins.GetOpenAITextCoins("gpt-3.5-turbo", 1000)},
		{"gpt3.5-16k-1000", coins.GetOpenAITextCoins("gpt-3.5-turbo-16k", 1000)},
		// {"gpt3.5-2000", coins.GetOpenAITextCoins("gpt-3.5-turbo", 2000)},
		// {"gpt3.5-16k-2000", coins.GetOpenAITextCoins("gpt-3.5-turbo-16k", 2000)},
		// {"gpt3.5-4000", coins.GetOpenAITextCoins("gpt-3.5-turbo", 4000)},
		// {"gpt3.5-16k-4000", coins.GetOpenAITextCoins("gpt-3.5-turbo-16k", 4000)},
		{"gpt-4-8k-1000", coins.GetOpenAITextCoins("gpt-4-8k", 1000)},
		{"gpt-4-32k-1000", coins.GetOpenAITextCoins("gpt-4-32k", 1000)},
		{"voice-tencent", coins.GetVoiceCoins("tencent")},
		{"upload-qiniu", coins.GetUploadCoins()},
	}

	for _, tu := range testcases {
		fmt.Printf("%30s => %10d\n", tu.Product, tu.Count)
	}
	tokenCount := int64(100)
	fmt.Printf("============= 测试 %d 个智慧果可以做哪些事儿 ================\n", tokenCount)
	for _, tu := range testcases {
		fmt.Printf("%30s => %10d\n", tu.Product, tokenCount/tu.Count)
	}
}

func TestLoadCoinsTable(t *testing.T) {
	assert.NoError(t, coins.LoadPriceInfo("../../coins-table.yaml"))

	res, err := yaml.Marshal(coins.GetCoinsTable())
	assert.NoError(t, err)

	fmt.Println(string(res))
}

func TestSpeechCoins(t *testing.T) {
	fmt.Println(coins.GetTextToVoiceCoins("tts-1", 100))
}
