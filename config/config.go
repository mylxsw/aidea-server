package config

import (
	"fmt"
	"os"

	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/starter/app"
)

type Config struct {
	// Listen 监听地址
	Listen string `json:"listen" yaml:"listen"`
	// 会话加密密钥
	SessionSecret string `json:"session_secret" yaml:"session_secret"`
	// Prometheus 监控访问密钥
	PrometheusToken string `json:"-" yaml:"prometheus_token"`
	// 记录聊天历史记录（可方便后期支持聊天消息多端同步，目前仅仅是做了记录，同步功能暂未实现）
	EnableRecordChat bool `json:"enable_record_chat" yaml:"enable_record_chat"`

	// OpenAIKey 配置
	EnableOpenAI       bool     `json:"enable_openai" yaml:"enable_openai"`
	OpenAIAzure        bool     `json:"openai_azure" yaml:"openai_azure"`
	OpenAIAPIVersion   string   `json:"openai_api_version" yaml:"openai_api_version"`
	OpenAIAutoProxy    bool     `json:"openai_auto_proxy" yaml:"openai_auto_proxy"`
	OpenAIOrganization string   `json:"openai_organization" yaml:"openai_organization"`
	OpenAIServers      []string `json:"openai_servers" yaml:"openai_servers"`
	OpenAIKeys         []string `json:"openai_keys" yaml:"openai_keys"`

	// 百度文心大模型配置
	EnableBaiduWXAI bool   `json:"enable_baiduwx_ai" yaml:"enable_baiduwx_ai"`
	BaiduWXKey      string `json:"baidu_ai_key" yaml:"baidu_ai_key"`
	BaiduWXSecret   string `json:"baidu_ai_secret" yaml:"baidu_ai_secret"`

	// 阿里灵积平台配置
	EnableDashScopeAI bool   `json:"enable_dashscope_ai" yaml:"enable_dashscope_ai"`
	DashScopeKey      string `json:"dashscope_key" yaml:"dashscope_key"`

	// 讯飞星火大模型配置
	EnableXFYunAI  bool   `json:"enable_xfyun_ai" yaml:"enable_xfyun_ai"`
	XFYunAppID     string `json:"xfyun_appid" yaml:"xfyun_appid"`
	XFYunAPIKey    string `json:"-" yaml:"-"`
	XFYunAPISecret string `json:"-" yaml:"-"`

	// Proxy
	Socks5Proxy string `json:"socks5_proxy" yaml:"socks5_proxy"`

	// DeepAIKey 配置
	EnableDeepAI    bool     `json:"enable_deepai" yaml:"enable_deepai"`
	DeepAIAutoProxy bool     `json:"deepai_auto_proxy" yaml:"deepai_auto_proxy"`
	DeepAIKey       string   `json:"deepai_key" yaml:"deepai_key"`
	DeepAIServer    []string `json:"deepai_servers" yaml:"deepai_servers"`

	// StabilityAIKey 配置
	EnableStabilityAI       bool     `json:"enable_stabilityai" yaml:"enable_stabilityai"`
	StabilityAIAutoProxy    bool     `json:"stabilityai_auto_proxy" yaml:"stabilityai_auto_proxy"`
	StabilityAIOrganization string   `json:"stabilityai_organization" yaml:"stabilityai_organization"`
	StabilityAIKey          string   `json:"stabilityai_key" yaml:"stabilityai_key"`
	StabilityAIServer       []string `json:"stabilityai_servers" yaml:"stabilityai_servers"`

	// Leap
	EnableLeapAI    bool   `json:"enable_leapai" yaml:"enable_leapai"`
	LeapAIAutoProxy bool   `json:"leapai_auto_proxy" yaml:"leapai_auto_proxy"`
	LeapAIKey       string `json:"leapai_key" yaml:"leapai_key"`
	// https://api.tryleap.ai
	LeapAIServers []string `json:"leapai_servers" yaml:"leapai_servers"`

	// fromston.6pen.art
	EnableFromstonAI bool   `json:"enable_fromstonai" yaml:"enable_fromstonai"`
	FromstonServer   string `json:"fromston_server" yaml:"fromston_server"`
	FromstonKey      string `json:"fromston_key" yaml:"fromston_key"`

	// getimg.ai
	EnableGetimgAI    bool   `json:"enable_getimgai" yaml:"enable_getimgai"`
	GetimgAIAutoProxy bool   `json:"getimgai_auto_proxy" yaml:"getimgai_auto_proxy"`
	GetimgAIServer    string `json:"getimgai_server" yaml:"getimgai_server"`
	GetimgAIKey       string `json:"getimgai_key" yaml:"getimgai_key"`

	// DBURI 数据库连接地址
	DBURI string `json:"db_uri" yaml:"db_uri"`
	// Redis
	RedisHost     string `json:"redis_host" yaml:"redis_host"`
	RedisPort     int    `json:"redis_port" yaml:"redis_port"`
	RedisPassword string `json:"-" yaml:"redis_password"`

	// 任务队列 worker 数量
	QueueWorkers int `json:"queue_workers" yaml:"queue_workers"`
	// 是否启用定时任务执行器
	EnableScheduler bool `json:"enable_scheduler" yaml:"enable_scheduler"`

	// 网易有道词典翻译 API 配置
	EnableTranslate bool   `json:"enable_translate" yaml:"enable_translate"`
	TranslateServer string `json:"translate_server" yaml:"translate_server"`
	TranslateAPPID  string `json:"translate_appid" yaml:"translate_appid"`
	TranslateAPPKey string `json:"-" yaml:"translate_app_key"`

	// 七牛云存储
	StorageAppKey    string `json:"storage_appkey" yaml:"storage_appkey"`
	StorageAppSecret string `json:"-" yaml:"storage_secret"`
	StorageBucket    string `json:"storage_bucket" yaml:"storage_bucket"`
	StorageCallback  string `json:"storage_callback" yaml:"storage_callback"`
	StorageDomain    string `json:"storage_domain" yaml:"storage_domain"`

	// Apple Sign In
	AppleSignIn AppleSignIn `json:"apple_sign_in" yaml:"apple_sign_in"`

	// 邮件配置
	EnableMail bool `json:"enable_mail" yaml:"enable_mail"`
	Mail       Mail `json:"mail" yaml:"mail"`

	// Tencent
	UseTencentVoiceToText bool   `json:"use_tencent_voice_to_text" yaml:"use_tencent_voice_to_text"`
	TencentSecretID       string `json:"tencent_secret_id" yaml:"tencent_secret_id"`
	TencentSecretKey      string `json:"-" yaml:"tencent_secret_key"`
	TencentSMSSDKAppID    string `json:"tencent_sms_sdk_appid" yaml:"tencent_sms_sdk_appid"`

	// Aliyun
	AliyunAccessKeyID   string `json:"aliyun_access_key_id" yaml:"aliyun_access_key_id"`
	AliyunAccessSecret  string `json:"-" yaml:"aliyun_access_secret"`
	EnableContentDetect bool   `json:"enable_content_detect" yaml:"enable_content_detect"`

	// Apple 应用内支付
	EnableApplePay bool `json:"enable_apple_pay" yaml:"enable_apple_pay"`

	// 支付宝
	EnableAlipay            bool   `json:"enable_alipay" yaml:"enable_alipay"`
	AliPayAppID             string `json:"alipay_appid" yaml:"alipay_appid"`
	AliPayAppPrivateKeyPath string `json:"alipay_app_private_key_path" yaml:"alipay_app_private_key_path"`
	AliPayAppPublicKeyPath  string `json:"alipay_app_public_key_path" yaml:"alipay_app_public_key_path"`
	AliPayRootCertPath      string `json:"alipay_root_cert_path" yaml:"alipay_root_cert_path"`
	AliPayPublicKeyPath     string `json:"alipay_public_key_path" yaml:"alipay_public_key_path"`

	// 短信通道
	SMSChannels []string `json:"sms_channels" yaml:"sms_channels"`

	// 钉钉通知设置
	DingDingToken  string `json:"-" yaml:"dingding_token"`
	DingDingSecret string `json:"-" yaml:"dingding_secret"`
}

type Mail struct {
	From         string `json:"from" yaml:"from"`
	SMTPHost     string `json:"smtp_host" yaml:"smtp_host"`
	SMTPPort     int    `json:"smtp_port" yaml:"smtp_port"`
	SMTPUsername string `json:"smtp_username" yaml:"smtp_username"`
	SMTPPassword string `json:"-" yaml:"smtp_password"`
	UseSSL       bool   `json:"use_ssl" yaml:"use_ssl"`
}

type AppleSignIn struct {
	TeamID string `json:"team_id" yaml:"team_id"`
	KeyID  string `json:"key_id" yaml:"key_id"`
	Secret string `json:"secret" yaml:"secret"`
}

func (conf *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%d", conf.RedisHost, conf.RedisPort)
}

func Register(ins *app.App) {
	ins.Singleton(func(ctx infra.FlagContext) *Config {
		var appleSecret string
		appleSecretFile := ctx.String("apple-secret")
		if appleSecretFile != "" {
			data, err := os.ReadFile(appleSecretFile)
			if err != nil {
				panic(err)
			}

			appleSecret = string(data)
		}

		return &Config{
			Listen:           ctx.String("listen"),
			DBURI:            ctx.String("db-uri"),
			SessionSecret:    ctx.String("session-secret"),
			PrometheusToken:  ctx.String("prometheus-token"),
			EnableRecordChat: ctx.Bool("enable-recordchat"),

			RedisHost:     ctx.String("redis-host"),
			RedisPort:     ctx.Int("redis-port"),
			RedisPassword: ctx.String("redis-password"),

			QueueWorkers:    ctx.Int("queue-workers"),
			EnableScheduler: ctx.Bool("enable-scheduler"),

			EnableOpenAI:       ctx.Bool("enable-openai"),
			OpenAIAzure:        ctx.Bool("openai-azure"),
			OpenAIAPIVersion:   ctx.String("openai-apiversion"),
			OpenAIAutoProxy:    ctx.Bool("openai-autoproxy"),
			OpenAIOrganization: ctx.String("openai-organization"),
			OpenAIServers:      ctx.StringSlice("openai-servers"),
			OpenAIKeys:         ctx.StringSlice("openai-keys"),

			EnableBaiduWXAI: ctx.Bool("enable-baiduwxai"),
			BaiduWXKey:      ctx.String("baiduwx-key"),
			BaiduWXSecret:   ctx.String("baiduwx-secret"),

			EnableDashScopeAI: ctx.Bool("enable-dashscopeai"),
			DashScopeKey:      ctx.String("dashscope-key"),

			EnableXFYunAI:  ctx.Bool("enable-xfyunai"),
			XFYunAppID:     ctx.String("xfyun-appid"),
			XFYunAPIKey:    ctx.String("xfyun-apikey"),
			XFYunAPISecret: ctx.String("xfyun-apisecret"),

			Socks5Proxy: ctx.String("socks5-proxy"),

			EnableDeepAI:    ctx.Bool("enable-deepai"),
			DeepAIAutoProxy: ctx.Bool("deepai-autoproxy"),
			DeepAIKey:       ctx.String("deepai-key"),
			DeepAIServer:    ctx.StringSlice("deepai-servers"),

			EnableStabilityAI:       ctx.Bool("enable-stabilityai"),
			StabilityAIAutoProxy:    ctx.Bool("stabilityai-autoproxy"),
			StabilityAIKey:          ctx.String("stabilityai-key"),
			StabilityAIOrganization: ctx.String("stabilityai-organization"),
			StabilityAIServer:       ctx.StringSlice("stabilityai-servers"),

			EnableLeapAI:    ctx.Bool("enable-leapai"),
			LeapAIAutoProxy: ctx.Bool("leapai-autoproxy"),
			LeapAIKey:       ctx.String("leapai-key"),
			LeapAIServers:   ctx.StringSlice("leapai-servers"),

			EnableGetimgAI:    ctx.Bool("enable-getimgai"),
			GetimgAIAutoProxy: ctx.Bool("getimgai-autoproxy"),
			GetimgAIServer:    ctx.String("getimgai-server"),
			GetimgAIKey:       ctx.String("getimgai-key"),

			EnableFromstonAI: ctx.Bool("enable-fromstonai"),
			FromstonServer:   ctx.String("fromston-server"),
			FromstonKey:      ctx.String("fromston-key"),

			EnableTranslate: ctx.Bool("enable-translate"),
			TranslateServer: ctx.String("translate-server"),
			TranslateAPPID:  ctx.String("translate-appid"),
			TranslateAPPKey: ctx.String("translate-appkey"),

			StorageAppKey:    ctx.String("storage-appkey"),
			StorageAppSecret: ctx.String("storage-secret"),
			StorageBucket:    ctx.String("storage-bucket"),
			StorageCallback:  ctx.String("storage-callback"),
			StorageDomain:    ctx.String("storage-domain"),

			AppleSignIn: AppleSignIn{
				TeamID: ctx.String("apple-teamid"),
				KeyID:  ctx.String("apple-keyid"),
				Secret: appleSecret,
			},

			EnableMail: ctx.Bool("enable-mail"),
			Mail: Mail{
				From:         ctx.String("mail-from"),
				SMTPHost:     ctx.String("mail-host"),
				SMTPPort:     ctx.Int("mail-port"),
				SMTPUsername: ctx.String("mail-username"),
				SMTPPassword: ctx.String("mail-password"),
				UseSSL:       ctx.Bool("mail-ssl"),
			},

			UseTencentVoiceToText: ctx.Bool("tencent-voice"),
			TencentSecretID:       ctx.String("tencent-id"),
			TencentSecretKey:      ctx.String("tencent-key"),
			TencentSMSSDKAppID:    ctx.String("tencent-smssdkappid"),

			AliyunAccessKeyID:   ctx.String("aliyun-key"),
			AliyunAccessSecret:  ctx.String("aliyun-secret"),
			EnableContentDetect: ctx.Bool("enable-contentdetect"),

			EnableApplePay: ctx.Bool("enable-applepay"),

			EnableAlipay:            ctx.Bool("enable-alipay"),
			AliPayAppID:             ctx.String("alipay-appid"),
			AliPayAppPrivateKeyPath: ctx.String("alipay-app-private-key"),
			AliPayAppPublicKeyPath:  ctx.String("alipay-app-public-key"),
			AliPayRootCertPath:      ctx.String("alipay-root-cert"),
			AliPayPublicKeyPath:     ctx.String("alipay-public-key"),

			SMSChannels: ctx.StringSlice("sms-channels"),

			DingDingToken:  ctx.String("dingding-token"),
			DingDingSecret: ctx.String("dingding-secret"),
		}
	})
}
