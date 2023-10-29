package coins

import (
	"github.com/mylxsw/asteria/log"
	"os"

	"github.com/mylxsw/go-utils/array"
	"gopkg.in/yaml.v3"
)

type PriceInfo struct {
	// CoinTable 价格表
	CoinTables map[string]CoinTable `json:"coin_tables" yaml:"coin_tables"`
	// Products 在线支付产品列表
	Products []Product `json:"products,omitempty" yaml:"products,omitempty"`
	// FreeModels 免费模型列表
	FreeModels []ModelWithName `json:"free_models,omitempty" yaml:"free_models,omitempty"`

	// SignupGiftCoins 注册账号赠币数量
	SignupGiftCoins int `json:"signup_gift_coins,omitempty" yaml:"signup_gift_coins,omitempty"`
	// BindPhoneGiftCoins 绑定手机赠币数量
	BindPhoneGiftCoins int `json:"bind_phone_gift_coins,omitempty" yaml:"bind_phone_gift_coins,omitempty"`
	// InviteGiftCoins 邀请赠币数量
	InviteGiftCoins int `json:"invite_gift_coins,omitempty" yaml:"invite_gift_coins,omitempty"`
	// InvitedGiftCoins 被邀请赠币数量
	InvitedGiftCoins int `json:"invited_gift_coins,omitempty" yaml:"invited_gift_coins,omitempty"`
	// InvitePaymentGiftRate 被引荐人充值，引荐人获得的奖励比例
	InvitePaymentGiftRate float64 `json:"invite_payment_gift_rate,omitempty" yaml:"invite_payment_gift_rate,omitempty"`
}

type CoinTable map[string]int64

// LoadPriceInfo 加载智慧果计费表
// 注意：该方法为非线程安全的，一旦应用启动时加载完毕，不应该再进行修改
func LoadPriceInfo(tableFile string) error {
	data, err := os.ReadFile(tableFile)
	if err != nil {
		return err
	}

	var priceInfo PriceInfo
	if err = yaml.Unmarshal(data, &priceInfo); err != nil {
		return err
	}

	// 加载模型价格表
	for k, v := range priceInfo.CoinTables {
		if _, ok := coinTables[k]; !ok {
			coinTables[k] = make(CoinTable)
		}

		for kk, vv := range v {
			coinTables[k][kk] = vv
		}
	}

	// 加载在线支付产品
	// 如果配置了产品列表，则使用配置文件为主，否则使用默认产品列表
	if len(priceInfo.Products) > 0 {
		Products = array.Map(priceInfo.Products, func(item Product, _ int) Product {
			if item.Description == "" {
				item.Description = buildDescription(item.Quota)
			}

			return item
		})
	}

	// 免费模型列表
	freeModels = priceInfo.FreeModels

	// 加载基础增币信息等
	SignupGiftCoins = priceInfo.SignupGiftCoins
	BindPhoneGiftCoins = priceInfo.BindPhoneGiftCoins
	InviteGiftCoins = priceInfo.InviteGiftCoins
	InvitedGiftCoins = priceInfo.InvitedGiftCoins
	InvitePaymentGiftRate = priceInfo.InvitePaymentGiftRate

	return nil
}

func DebugPrintPriceInfo() {
	log.WithFields(log.Fields{
		"products":                 Products,
		"free":                     freeModels,
		"coins":                    coinTables,
		"signup_gift_coins":        SignupGiftCoins,
		"bind_phone_gift_coins":    BindPhoneGiftCoins,
		"invite_gift_coins":        InviteGiftCoins,
		"invited_gift_coins":       InvitedGiftCoins,
		"invite_payment_gift_rate": InvitePaymentGiftRate,
	}).Debug("coins table loaded")
}
