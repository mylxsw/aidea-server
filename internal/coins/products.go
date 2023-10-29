package coins

import (
	"fmt"
	"time"
)

type ExpirePolicy string

const (
	ExpirePolicyNever  ExpirePolicy = "never"
	ExpirePolicyWeek   ExpirePolicy = "week"
	ExpirePolicy2Week  ExpirePolicy = "2week"
	ExpirePolicyMonth  ExpirePolicy = "month"
	ExpirePolicy3Month ExpirePolicy = "3month"
	ExpirePolicy6Month ExpirePolicy = "6month"
	ExpirePolicyYear   ExpirePolicy = "year"
)

type Product struct {
	ID               string       `json:"id,omitempty" yaml:"id,omitempty"`
	Name             string       `json:"name,omitempty" yaml:"name,omitempty"`
	Quota            int64        `json:"quota,omitempty" yaml:"quota,omitempty"`
	RetailPrice      int64        `json:"retail_price,omitempty" yaml:"retail_price,omitempty"`
	ExpirePolicy     ExpirePolicy `json:"expire_policy,omitempty" yaml:"expire_policy,omitempty"`
	ExpirePolicyText string       `json:"expire_policy_text,omitempty" yaml:"expire_policy_text,omitempty"`
	Recommend        bool         `json:"recommend,omitempty" yaml:"recommend,omitempty"`
	Description      string       `json:"description,omitempty" yaml:"description,omitempty"`
	PlatformLimit    Platform     `json:"platform_limits,omitempty" yaml:"platform_limits,omitempty"`
}

type Platform string

const (
	PlatformNoneIOS Platform = "none_ios"
	PlatformIOS     Platform = "ios"
)

func (ap Product) GetExpirePolicyText() string {
	switch ap.ExpirePolicy {
	case ExpirePolicyNever:
		return "永久"
	case ExpirePolicyWeek:
		return "7天"
	case ExpirePolicy2Week:
		return "14天"
	case ExpirePolicyMonth:
		return "30天"
	case ExpirePolicy3Month:
		return "90天"
	case ExpirePolicy6Month:
		return "180天"
	case ExpirePolicyYear:
		return "365天"
	}

	return "永久"
}

func (ap Product) ExpiredAt() time.Time {
	switch ap.ExpirePolicy {
	case ExpirePolicyNever:
		return time.Now().AddDate(100, 0, 0)
	case ExpirePolicyWeek:
		return time.Now().AddDate(0, 0, 7)
	case ExpirePolicy2Week:
		return time.Now().AddDate(0, 0, 14)
	case ExpirePolicyMonth:
		return time.Now().AddDate(0, 0, 30)
	case ExpirePolicy3Month:
		return time.Now().AddDate(0, 0, 90)
	case ExpirePolicy6Month:
		return time.Now().AddDate(0, 0, 180)
	case ExpirePolicyYear:
		return time.Now().AddDate(0, 0, 365)
	}

	return time.Now().AddDate(100, 0, 0)
}

// 可选价格 1, 3, 6, 8, 12, 18, 28, 38, 48, 58, 68, 78, 88, 98, 128, 168, 198, 228, 268, 298, 348, 398, 498, 598, 698

func buildDescription(quota int64) string {
	multiple := float64(quota) / 100.0
	return fmt.Sprintf("预计可与您对话 %.0f 次（GPT-4 约 %.0f 次），或创作 %d 张图片", 30*multiple, 2*multiple, quota/int64(GetUnifiedImageGenCoins()))
}

func GetProduct(productId string) *Product {
	for _, product := range Products {
		if product.ID == productId {
			return &product
		}
	}

	return nil
}

func IsProduct(productId string) bool {
	for _, product := range Products {
		if product.ID == productId {
			return true
		}
	}

	return false
}

var Products = []Product{
	{
		ID:           "cc.aicode.aidea.coins_100",
		Quota:        50,
		RetailPrice:  100,
		Name:         "1元尝鲜", // 1 元
		ExpirePolicy: ExpirePolicyWeek,
		Description:  buildDescription(50),
	},
	//{
	//	ID:           "cc.aicode.aidea.coins_300",
	//	Quota:        200,
	//	RetailPrice:  300,
	//	Name:         "3元200个", // 3 元
	//	ExpirePolicy: ExpirePolicyWeek,
	//	Description:  buildDescription(200),
	//},
	{
		ID:           "cc.aicode.aidea.coins_600_2",
		Quota:        700,
		RetailPrice:  600,
		Name:         "6元700个", // 6 元
		ExpirePolicy: ExpirePolicyMonth,
		Description:  buildDescription(700),
	},
	{
		ID:           "cc.aicode.aidea.coins_1200",
		Quota:        1500,
		RetailPrice:  1200,
		Name:         "12元1500个", // 12 元
		ExpirePolicy: ExpirePolicyMonth,
		Description:  buildDescription(1500),
	},
	{
		ID:           "cc.aicode.aidea.coins_3800",
		Quota:        5000,
		RetailPrice:  3800,
		Name:         "38元5000个", // 38 元
		ExpirePolicy: ExpirePolicy3Month,
		Recommend:    true,
		Description:  buildDescription(5000),
	},
	{
		ID:           "cc.aicode.aidea.coins_6800_2",
		Quota:        10000,
		RetailPrice:  6800,
		Name:         "68元10000个", // 68 元
		ExpirePolicy: ExpirePolicy6Month,
		Description:  buildDescription(10000),
	},
	//{
	//	ID:           "cc.aicode.aidea.coins_12800",
	//	Quota:        22800,
	//	RetailPrice:  12800,
	//	Name:         "128元22800个", // 128 元
	//	ExpirePolicy: ExpirePolicyYear,
	//	Description:  buildDescription(22800),
	//},
	// {
	// 	ID:           "cc.aicode.aidea.coins_19800",
	// 	Quota:        38000,
	// 	RetailPrice:  19800,
	// 	Name:         "198元得18200个", // 198 元
	// 	ExpirePolicy: ExpirePolicyYear,
	// 	Description:  buildDescription(38000),
	// },
}
