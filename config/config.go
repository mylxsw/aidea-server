package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"

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
	// 是否启用跨域支持
	EnableCORS bool `json:"enable_cors" yaml:"enable_cors"`
	// EnableWebsocket 是否启用 Websocket 支持
	EnableWebsocket bool `json:"enable_websocket" yaml:"enable_websocket"`
	// 是否启用 SQL 调试
	DebugWithSQL bool `json:"debug_with_sql" yaml:"debug_with_sql"`
	// 是否启用 API Keys 功能
	EnableAPIKeys bool `json:"enable_api_keys" yaml:"enable_api_keys"`
	// 是否是生产环境
	IsProduction bool `json:"is_production" yaml:"is_production"`

	// 临时文件存储路径
	TempDir string `json:"temp_dir" yaml:"temp_dir"`

	// 是否提示用户绑定手机
	ShouldBindPhone bool `json:"should_bind_phone" yaml:"should_bind_phone"`

	// BaseURL 服务的基础 URL
	BaseURL string `json:"base_url" yaml:"base_url"`

	// UniversalLinkConfig 通用链接配置
	UniversalLinkConfig string `json:"universal_link_config" yaml:"universal_link_config"`

	// EnableModelRateLimit 是否启用模型访问限流
	// 当前流控策略为：每个模型每分钟最多访问 5 次
	EnableModelRateLimit bool `json:"enable_model_rate_limit" yaml:"enable_model_rate_limit"`

	// EnableCustomHomeModels 是否启用自定义首页模型
	EnableCustomHomeModels bool `json:"enable_custom_home_models" yaml:"enable_custom_home_models"`

	// OpenAIKey 配置
	EnableOpenAI       bool     `json:"enable_openai" yaml:"enable_openai"`
	OpenAIAzure        bool     `json:"openai_azure" yaml:"openai_azure"`
	OpenAIAPIVersion   string   `json:"openai_api_version" yaml:"openai_api_version"`
	OpenAIAutoProxy    bool     `json:"openai_auto_proxy" yaml:"openai_auto_proxy"`
	OpenAIOrganization string   `json:"openai_organization" yaml:"openai_organization"`
	OpenAIServers      []string `json:"openai_servers" yaml:"openai_servers"`
	OpenAIKeys         []string `json:"openai_keys" yaml:"openai_keys"`

	EnableOpenAIDalle       bool     `json:"enable_openai_dalle" yaml:"enable_openai_dalle"`
	DalleUsingOpenAISetting bool     `json:"dalle_using_openai_setting" yaml:"dalle_using_openai_setting"`
	OpenAIDalleAzure        bool     `json:"openai_dalle_azure" yaml:"openai_dalle_azure"`
	OpenAIDalleAPIVersion   string   `json:"openai_dalle_api_version" yaml:"openai_dalle_api_version"`
	OpenAIDalleAutoProxy    bool     `json:"openai_dalle_auto_proxy" yaml:"openai_dalle_auto_proxy"`
	OpenAIDalleOrganization string   `json:"openai_dalle_organization" yaml:"openai_dalle_organization"`
	OpenAIDalleServers      []string `json:"openai_dalle_servers" yaml:"openai_dalle_servers"`
	OpenAIDalleKeys         []string `json:"openai_dalle_keys" yaml:"openai_dalle_keys"`

	// OpenAI Fallback 配置
	EnableFallbackOpenAI       bool     `json:"enable_fallback_openai" yaml:"enable_fallback_openai"`
	FallbackOpenAIAzure        bool     `json:"fallback_openai_azure" yaml:"fallback_openai_azure"`
	FallbackOpenAIServers      []string `json:"fallback_openai_servers" yaml:"fallback_openai_servers"`
	FallbackOpenAIKeys         []string `json:"fallback_openai_keys" yaml:"fallback_openai_keys"`
	FallbackOpenAIOrganization string   `json:"fallback_openai_organization" yaml:"fallback_openai_organization"`
	FallbackOpenAIAPIVersion   string   `json:"fallback_openai_api_version" yaml:"fallback_openai_api_version"`
	FallbackOpenAIAutoProxy    bool     `json:"fallback_openai_auto_proxy" yaml:"fallback_openai_auto_proxy"`

	// Anthropic 配置
	EnableAnthropic    bool   `json:"enable_anthropic" yaml:"enable_anthropic"`
	AnthropicAutoProxy bool   `json:"anthropic_auto_proxy" yaml:"anthropic_auto_proxy"`
	AnthropicServer    string `json:"anthropic_server" yaml:"anthropic_server"`
	AnthropicAPIKey    string `json:"anthropic_api_key" yaml:"anthropic_api_key"`

	// Google Gemini 配置
	EnableGoogleAI    bool   `json:"enable_googleai" yaml:"enable_googleai"`
	GoogleAIAutoProxy bool   `json:"googleai_auto_proxy" yaml:"googleai_auto_proxy"`
	GoogleAIServer    string `json:"googleai_server" yaml:"googleai_server"`
	GoogleAIKey       string `json:"googleai_key" yaml:"googleai_key"`

	// 百度文心大模型配置
	EnableBaiduWXAI bool   `json:"enable_baiduwx_ai" yaml:"enable_baiduwx_ai"`
	BaiduWXKey      string `json:"baidu_ai_key" yaml:"baidu_ai_key"`
	BaiduWXSecret   string `json:"baidu_ai_secret" yaml:"baidu_ai_secret"`

	// 阿里灵积平台配置
	EnableDashScopeAI bool     `json:"enable_dashscope_ai" yaml:"enable_dashscope_ai"`
	DashScopeKey      string   `json:"dashscope_key" yaml:"dashscope_key"`
	DashScopeKeys     []string `json:"dashscope_keys" yaml:"dashscope_keys"`

	// 讯飞星火大模型配置
	EnableXFYunAI  bool   `json:"enable_xfyun_ai" yaml:"enable_xfyun_ai"`
	XFYunAppID     string `json:"xfyun_appid" yaml:"xfyun_appid"`
	XFYunAPIKey    string `json:"-" yaml:"-"`
	XFYunAPISecret string `json:"-" yaml:"-"`

	// 商汤日日新
	EnableSenseNovaAI  bool   `json:"enable_sensenova_ai" yaml:"enable_sensenova_ai"`
	SenseNovaKeyID     string `json:"sensenova_keyid" yaml:"sensenova_keyid"`
	SenseNovaKeySecret string `json:"-" yaml:"-"`

	// 百川大模型
	EnableBaichuan bool   `json:"enable_baichuan" yaml:"enable_baichuan"`
	BaichuanAPIKey string `json:"baichuan_api_key" yaml:"baichuan_api_key"`
	BaichuanSecret string `json:"-" yaml:"-"`

	// 天工大模型
	EnableSky    bool   `json:"enable_sky" yaml:"enable_sky"`
	SkyAppKey    string `json:"sky_app_key" yaml:"sky_app_key"`
	SkyAppSecret string `json:"-" yaml:"-"`

	// 360 智脑
	EnableGPT360 bool   `json:"enable_gpt360" yaml:"enable_gpt360"`
	GPT360APIKey string `json:"gpt360_api_key" yaml:"gpt360_api_key"`

	// 智谱
	EnableZhipuAI bool   `json:"enable_zhipuai" yaml:"enable_zhipuai"`
	ZhipuAIKey    string `json:"zhipuai_key" yaml:"zhipuai_key"`

	// 月之暗面
	EnableMoonshot bool   `json:"enable_moonshot" yaml:"enable_moonshot"`
	MoonshotAPIKey string `json:"moonshot_api_key" yaml:"moonshot_api_key"`

	// OneAPI 支持的模型列表
	// one-server: https://github.com/songquanpeng/one-api
	OneAPISupportModels []string `json:"oneapi_support_models" yaml:"oneapi_support_models"`
	EnableOneAPI        bool     `json:"enable_oneapi" yaml:"enable_oneapi"`
	OneAPIServer        string   `json:"oneapi_server" yaml:"oneapi_server"`
	OneAPIKey           string   `json:"one_api_key" yaml:"one_api_key"`

	// OpenRouter 支持的模型列表
	// open-router: https://openrouter.ai
	OpenRouterSupportModels []string `json:"openrouter_support_models" yaml:"openrouter_support_models"`
	EnableOpenRouter        bool     `json:"enable_openrouter" yaml:"enable_openrouter"`
	OpenRouterAutoProxy     bool     `json:"openrouter_auto_proxy" yaml:"openrouter_auto_proxy"`
	OpenRouterServer        string   `json:"openrouter_server" yaml:"openrouter_server"`
	OpenRouterKey           string   `json:"openrouter_key" yaml:"openrouter_key"`

	// Proxy
	Socks5Proxy string `json:"socks5_proxy" yaml:"socks5_proxy"`
	// ProxyURL 代理地址，该值会覆盖 Socks5Proxy 配置
	ProxyURL string `json:"proxy_url" yaml:"proxy_url"`

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

	// EnableLeptonAI 是否启用 Lepton AI
	EnableLeptonAI    bool     `json:"enable_leptonai" yaml:"enable_leptonai"`
	LeptonAIAutoProxy bool     `json:"leptonai_auto_proxy" yaml:"leptonai_auto_proxy"`
	LeptonAIQRServers []string `json:"leptonai_qr_servers" yaml:"leptonai_qr_servers"`
	LeptonAIKeys      []string `json:"leptonai_keys" yaml:"leptonai_keys"`

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
	StorageAppKey       string `json:"storage_appkey" yaml:"storage_appkey"`
	StorageAppSecret    string `json:"-" yaml:"storage_secret"`
	StorageBucket       string `json:"storage_bucket" yaml:"storage_bucket"`
	StorageCallback     string `json:"storage_callback" yaml:"storage_callback"`
	StorageCallbackHost string `json:"storage_callback_host" yaml:"storage_callback_host"`
	StorageDomain       string `json:"storage_domain" yaml:"storage_domain"`
	StorageRegion       string `json:"storage_region" yaml:"storage_region"`

	// Apple Sign In
	AppleSignIn AppleSignIn `json:"apple_sign_in" yaml:"apple_sign_in"`

	// 邮件配置
	EnableMail bool `json:"enable_mail" yaml:"enable_mail"`
	Mail       Mail `json:"mail" yaml:"mail"`

	// Tencent
	UseTencentVoiceToText bool   `json:"use_tencent_voice_to_text" yaml:"use_tencent_voice_to_text"`
	TencentSecretID       string `json:"tencent_secret_id" yaml:"tencent_secret_id"`
	TencentSecretKey      string `json:"-" yaml:"tencent_secret_key"`
	EnableTencentAI       bool   `json:"enable_tencent_ai" yaml:"enable_tencent_ai"`
	TencentSMSSDKAppID    string `json:"tencent_sms_sdk_appid" yaml:"tencent_sms_sdk_appid"`
	TencentSMSTemplateID  string `json:"tencent_sms_template_id" yaml:"tencent_sms_template_id"`
	TencentSMSSign        string `json:"tencent_sms_sign" yaml:"tencent_sms_sign"`

	// Aliyun
	AliyunAccessKeyID   string `json:"aliyun_access_key_id" yaml:"aliyun_access_key_id"`
	AliyunAccessSecret  string `json:"-" yaml:"aliyun_access_secret"`
	EnableContentDetect bool   `json:"enable_content_detect" yaml:"enable_content_detect"`
	AliyunSMSTemplateID string `json:"aliyun_sms_template_id" yaml:"aliyun_sms_template_id"`
	AliyunSMSSign       string `json:"aliyun_sms_sign" yaml:"aliyun_sms_sign"`

	// Apple 应用内支付
	EnableApplePay bool `json:"enable_apple_pay" yaml:"enable_apple_pay"`

	// 支付宝
	AlipaySandbox           bool   `json:"alipay_sandbox" yaml:"alipay_sandbox"`
	EnableAlipay            bool   `json:"enable_alipay" yaml:"enable_alipay"`
	AliPayAppID             string `json:"alipay_appid" yaml:"alipay_appid"`
	AliPayAppPrivateKeyPath string `json:"alipay_app_private_key_path" yaml:"alipay_app_private_key_path"`
	AliPayAppPublicKeyPath  string `json:"alipay_app_public_key_path" yaml:"alipay_app_public_key_path"`
	AliPayRootCertPath      string `json:"alipay_root_cert_path" yaml:"alipay_root_cert_path"`
	AliPayPublicKeyPath     string `json:"alipay_public_key_path" yaml:"alipay_public_key_path"`
	AliPayNotifyURL         string `json:"alipay_notify_url" yaml:"alipay_notify_url"`
	AliPayReturnURL         string `json:"alipay_return_url" yaml:"alipay_return_url"`

	// 短信通道
	SMSChannels []string `json:"sms_channels" yaml:"sms_channels"`

	// 钉钉通知设置
	DingDingSlackMode bool   `json:"dingding_slack_mode" yaml:"dingding_slack_mode"`
	DingDingToken     string `json:"-" yaml:"dingding_token"`
	DingDingSecret    string `json:"-" yaml:"dingding_secret"`

	// 国产化模式
	CNLocalMode    bool   `json:"cn_local_mode" yaml:"cn_local_mode"`
	CNLocalOnlyIOS bool   `json:"cn_local_only_ios" yaml:"cn_local_only_ios"`
	CNLocalModel   string `json:"cn_local_model" yaml:"cn_local_model"`
	CNLocalVendor  string `json:"cn_local_vendor" yaml:"cn_local_vendor"`

	// 文生图、图生图控制
	DefaultImageToImageModel string `json:"default_image_to_image_model" yaml:"default_image_to_image_model"`
	DefaultTextToImageModel  string `json:"default_text_to_image_model" yaml:"default_text_to_image_model"`

	// 图生图图像识别处理模型，用于识别图像内容，生成图生图的提示语
	ImageToImageRecognitionProvider string `json:"img2img-recognition-provider" yaml:"img2img-recognition-provider"`

	// 字体文件路径
	FontPath string `json:"font_path" yaml:"font_path"`
	// 服务状态页面
	ServiceStatusPage string `json:"service_status_page" yaml:"service_status_page"`

	// 免费 Chat 请求 （仅限 IOS）
	FreeChatEnabled bool `json:"free_chat_enabled" yaml:"free_chat_enabled"`
	// 免费 Chat 每日限制(每 IP)
	FreeChatDailyLimit int `json:"free_chat_daily_limit" yaml:"free_chat_daily_limit"`
	// 免费 Chat 每日全局限制（不区分 IP）
	FreeChatDailyGlobalLimit int `json:"free_chat_daily_global_limit" yaml:"free_chat_daily_global_limit"`

	// 微信开放平台配置
	WeChatAppID  string `json:"wechat_appid" yaml:"wechat_appid"`
	WeChatSecret string `json:"wechat_secret" yaml:"wechat_secret"`
	// 微信支付配置
	WeChatPayEnabled            bool   `json:"wechat_pay_enabled" yaml:"wechat_pay_enabled"`
	WeChatPayMchID              string `json:"wechat_pay_mchid" yaml:"wechat_pay_mchid"`
	WeChatPayCertSerialNumber   string `json:"wechat_pay_cert_serial_number" yaml:"wechat_pay_cert_serial_number"`
	WeChatPayCertPrivateKeyPath string `json:"wechat_pay_cert_private_key_path" yaml:"wechat_pay_cert_private_key_path"`
	WeChatPayAPIv3Key           string `json:"wechat_pay_apiv3_key" yaml:"wechat_pay_apiv3_key"`
	WeChatPayNotifyURL          string `json:"wechat_pay_notify_url" yaml:"wechat_pay_notify_url"`

	// Stripe 支付
	Stripe StripeConfig `json:"stripe" yaml:"stripe"`

	// 首页默认常用模型
	DefaultHomeModels    []string `json:"default_home_models" yaml:"default_home_models"`
	DefaultHomeModelsIOS []string `json:"default_home_models_ios" yaml:"default_home_models_ios"`
	DefaultRoleModel     string   `json:"default_role_model" yaml:"default_role_model"`

	// 文本转语言
	// TextToVoiceEngine 文本转语音引擎：minimax/openai/azure
	TextToVoiceEngine string `json:"text_to_voice_engine" yaml:"text_to_voice_engine"`
	// TextToVoiceAzureRegion Azure 语音服务区域
	TextToVoiceAzureRegion string `json:"text_to_voice_azure_region" yaml:"text_to_voice_azure_region"`
	// TextToVoiceAzureKey Azure 语音服务密钥
	TextToVoiceAzureKey string `json:"text_to_voice_azure_key" yaml:"text_to_voice_azure_key"`

	// 是否允许语音转文本
	EnableVoiceToText bool `json:"enable_voice_to_text" yaml:"enable_voice_to_text"`
	// 是否允许文本转语音
	EnableTextToVoice bool `json:"enable_text_to_voice" yaml:"enable_text_to_voice"`

	// MiniMax 配置
	MiniMaxAPIKey  string `json:"minimax_api_key" yaml:"minimax_api_key"`
	MiniMaxGroupID string `json:"minimax_group_id" yaml:"minimax_group_id"`

	// 总结能力
	EnableSummarizer bool   `json:"enable_summarizer" yaml:"enable_summarizer"`
	SummarizerModel  string `json:"summarizer_model" yaml:"summarizer_model"`

	// Flux model
	FluxAPIServer string `json:"flux_api_server" yaml:"flux_api_server"`
	FluxAPIKey    string `json:"flux_api_key" yaml:"flux_api_key"`

	// Search 配置
	SearchEngine string `json:"search_engine" yaml:"search_engine"`
	// AvailableSearchEngines 可用的搜索引擎
	AvailableSearchEngines []string `json:"available_search_engines" yaml:"available_search_engines"`
	// BigModel Search 配置
	BigModelSearchAPIKey string `json:"bigmodel_search_api_key" yaml:"bigmodel_search_api_key"`
	// Bochaai Search 配置
	BochaaiSearchAPIKey string `json:"bochaai_search_api_key" yaml:"bochaai_search_api_key"`
	// Search Assistant 配置 (用于将用户的对话上下文转换为搜索查询
	SearchAssistantModel   string `json:"search_assistant_model" yaml:"search_assistant_model"`
	SearchAssistantAPIBase string `json:"search_assistant_api_base" yaml:"search_assistant_api_base"`
	SearchAssistantAPIKey  string `json:"search_assistant_api_key" yaml:"search_assistant_api_key"`
}

func (conf *Config) SupportProxy() bool {
	return conf.Socks5Proxy != "" || conf.ProxyURL != ""
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
	// 加载命令行选项
	initCmdFlags(ins)

	// 配置文件读取
	ins.Singleton(func(ctx infra.FlagContext) *Config {
		if ctx.String("conf") == "" {
			log.Warning("没有指定配置文件，使用默认配置（通过命令行选项 --conf config.yaml 指定配置文件）")
		}

		var appleSecret string
		appleSecretFile := ctx.String("apple-secret")
		if appleSecretFile != "" {
			data, err := os.ReadFile(appleSecretFile)
			if err != nil {
				panic(err)
			}

			appleSecret = string(data)
		}

		// 加载价格表
		priceTableFile := ctx.String("price-table-file")
		if priceTableFile != "" {
			if err := coins.LoadPriceInfo(priceTableFile); err != nil {
				panic(fmt.Errorf("价格表加载失败: %w", err))
			}

			coins.DebugPrintPriceInfo()
		}

		// 加载 Stripe 配置
		stripe := StripeConfig{
			Enabled:        ctx.Bool("enable-stripe"),
			PublishableKey: ctx.String("stripe-publishable-key"),
			SecretKey:      ctx.String("stripe-secret-key"),
			WebhookSecret:  ctx.String("stripe-webhook-secret"),
		}
		stripe.Init()

		// 七牛云配置
		storageCallbacks := ctx.StringSlice("storage-callbacks")
		if len(storageCallbacks) == 0 && ctx.String("storage-callback") != "" {
			storageCallbacks = append(storageCallbacks, ctx.String("storage-callback"))
		}
		storageCallbacks = array.Uniq(array.Filter(
			array.Map(storageCallbacks, func(callback string, _ int) string {
				return strings.TrimSuffix(strings.TrimSpace(callback), "/")
			}),
			func(callback string, _ int) bool { return callback != "" },
		))

		return &Config{
			Listen:              ctx.String("listen"),
			DBURI:               ctx.String("db-uri"),
			SessionSecret:       ctx.String("session-secret"),
			PrometheusToken:     ctx.String("prometheus-token"),
			EnableRecordChat:    ctx.Bool("enable-recordchat"),
			EnableCORS:          ctx.Bool("enable-cors"),
			EnableWebsocket:     ctx.Bool("enable-websocket"),
			DebugWithSQL:        ctx.Bool("debug-with-sql"),
			UniversalLinkConfig: strings.TrimSpace(ctx.String("universal-link-config")),
			ShouldBindPhone:     ctx.Bool("should-bind-phone"),

			EnableSummarizer: ctx.Bool("enable-summarizer"),
			SummarizerModel:  ctx.String("summarizer-model"),

			BaseURL:      strings.TrimSuffix(ctx.String("base-url"), "/"),
			IsProduction: ctx.Bool("production"),
			TempDir:      ctx.String("temp-dir"),

			EnableModelRateLimit:   ctx.Bool("enable-model-rate-limit"),
			EnableCustomHomeModels: ctx.Bool("enable-custom-home-models"),
			EnableAPIKeys:          ctx.Bool("enable-api-keys"),

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

			EnableOpenAIDalle:       ctx.Bool("enable-openai-dalle"),
			DalleUsingOpenAISetting: ctx.Bool("dalle-using-openai-setting"),
			OpenAIDalleAzure:        ctx.Bool("openai-dalle-azure"),
			OpenAIDalleAPIVersion:   ctx.String("openai-dalle-apiversion"),
			OpenAIDalleAutoProxy:    ctx.Bool("openai-dalle-autoproxy"),
			OpenAIDalleOrganization: ctx.String("openai-dalle-organization"),
			OpenAIDalleServers:      ctx.StringSlice("openai-dalle-servers"),
			OpenAIDalleKeys:         ctx.StringSlice("openai-dalle-keys"),

			EnableFallbackOpenAI:       ctx.Bool("enable-fallback-openai"),
			FallbackOpenAIAzure:        ctx.Bool("fallback-openai-azure"),
			FallbackOpenAIServers:      ctx.StringSlice("fallback-openai-servers"),
			FallbackOpenAIKeys:         ctx.StringSlice("fallback-openai-keys"),
			FallbackOpenAIOrganization: ctx.String("fallback-openai-organization"),
			FallbackOpenAIAPIVersion:   ctx.String("fallback-openai-apiversion"),
			FallbackOpenAIAutoProxy:    ctx.Bool("fallback-openai-autoproxy"),

			EnableAnthropic:    ctx.Bool("enable-anthropic"),
			AnthropicAutoProxy: ctx.Bool("anthropic-autoproxy"),
			AnthropicServer:    ctx.String("anthropic-server"),
			AnthropicAPIKey:    ctx.String("anthropic-apikey"),

			EnableGoogleAI:    ctx.Bool("enable-googleai"),
			GoogleAIAutoProxy: ctx.Bool("googleai-autoproxy"),
			GoogleAIServer:    ctx.String("googleai-server"),
			GoogleAIKey:       ctx.String("googleai-key"),

			EnableBaiduWXAI: ctx.Bool("enable-baiduwxai"),
			BaiduWXKey:      ctx.String("baiduwx-key"),
			BaiduWXSecret:   ctx.String("baiduwx-secret"),

			EnableDashScopeAI: ctx.Bool("enable-dashscopeai"),
			DashScopeKey:      ctx.String("dashscope-key"),
			DashScopeKeys:     ctx.StringSlice("dashscope-keys"),

			EnableXFYunAI:  ctx.Bool("enable-xfyunai"),
			XFYunAppID:     ctx.String("xfyun-appid"),
			XFYunAPIKey:    ctx.String("xfyun-apikey"),
			XFYunAPISecret: ctx.String("xfyun-apisecret"),

			EnableSenseNovaAI:  ctx.Bool("enable-sensenovaai"),
			SenseNovaKeyID:     ctx.String("sensenova-keyid"),
			SenseNovaKeySecret: ctx.String("sensenova-keysecret"),

			EnableBaichuan: ctx.Bool("enable-baichuan"),
			BaichuanAPIKey: ctx.String("baichuan-apikey"),
			BaichuanSecret: ctx.String("baichuan-secret"),

			EnableSky:    ctx.Bool("enable-sky"),
			SkyAppKey:    ctx.String("sky-appkey"),
			SkyAppSecret: ctx.String("sky-appsecret"),

			EnableGPT360: ctx.Bool("enable-gpt360"),
			GPT360APIKey: ctx.String("gpt360-apikey"),

			EnableZhipuAI: ctx.Bool("enable-zhipuai"),
			ZhipuAIKey:    ctx.String("zhipuai-key"),

			EnableMoonshot: ctx.Bool("enable-moonshot"),
			MoonshotAPIKey: ctx.String("moonshot-apikey"),

			EnableOneAPI: ctx.Bool("enable-oneapi"),
			OneAPIServer: ctx.String("oneapi-server"),
			OneAPIKey:    ctx.String("oneapi-key"),

			OpenRouterSupportModels: ctx.StringSlice("openrouter-support-models"),
			EnableOpenRouter:        ctx.Bool("enable-openrouter"),
			OpenRouterAutoProxy:     ctx.Bool("openrouter-autoproxy"),
			OpenRouterServer:        ctx.String("openrouter-server"),
			OpenRouterKey:           ctx.String("openrouter-key"),

			Socks5Proxy: ctx.String("socks5-proxy"),
			ProxyURL:    ctx.String("proxy-url"),

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

			EnableLeptonAI:    ctx.Bool("enable-leptonai"),
			LeptonAIAutoProxy: ctx.Bool("leptonai-autoproxy"),
			LeptonAIQRServers: ctx.StringSlice("leptonai-qr-servers"),
			LeptonAIKeys:      ctx.StringSlice("leptonai-keys"),

			EnableFromstonAI: ctx.Bool("enable-fromstonai"),
			FromstonServer:   ctx.String("fromston-server"),
			FromstonKey:      ctx.String("fromston-key"),

			EnableTranslate: ctx.Bool("enable-translate"),
			TranslateServer: ctx.String("translate-server"),
			TranslateAPPID:  ctx.String("translate-appid"),
			TranslateAPPKey: ctx.String("translate-appkey"),

			StorageAppKey:       ctx.String("storage-appkey"),
			StorageAppSecret:    ctx.String("storage-secret"),
			StorageBucket:       ctx.String("storage-bucket"),
			StorageCallback:     strings.Join(storageCallbacks, ";"),
			StorageCallbackHost: ctx.String("storage-callback-host"),
			StorageDomain:       ctx.String("storage-domain"),
			StorageRegion:       ctx.String("storage-region"),

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
			TencentSMSTemplateID:  ctx.String("tencent-smstemplateid"),
			TencentSMSSign:        ctx.String("tencent-smssign"),
			EnableTencentAI:       ctx.Bool("enable-tencentai"),

			AliyunAccessKeyID:   ctx.String("aliyun-key"),
			AliyunAccessSecret:  ctx.String("aliyun-secret"),
			EnableContentDetect: ctx.Bool("enable-contentdetect"),
			AliyunSMSTemplateID: ctx.String("aliyun-smstemplateid"),
			AliyunSMSSign:       ctx.String("aliyun-smssign"),

			EnableApplePay: ctx.Bool("enable-applepay"),

			EnableAlipay:            ctx.Bool("enable-alipay"),
			AliPayAppID:             ctx.String("alipay-appid"),
			AliPayAppPrivateKeyPath: ctx.String("alipay-app-private-key"),
			AliPayAppPublicKeyPath:  ctx.String("alipay-app-public-key"),
			AliPayRootCertPath:      ctx.String("alipay-root-cert"),
			AliPayPublicKeyPath:     ctx.String("alipay-public-key"),
			AliPayNotifyURL:         ctx.String("alipay-notify-url"),
			AliPayReturnURL:         ctx.String("alipay-return-url"),
			AlipaySandbox:           ctx.Bool("alipay-sandbox"),

			SMSChannels: ctx.StringSlice("sms-channels"),

			DingDingSlackMode: ctx.Bool("dingding-slack-mode"),
			DingDingToken:     ctx.String("dingding-token"),
			DingDingSecret:    ctx.String("dingding-secret"),

			CNLocalMode:    ctx.Bool("cnlocal-mode"),
			CNLocalOnlyIOS: ctx.Bool("cnlocal-onlyios"),
			CNLocalVendor:  ctx.String("cnlocal-vendor"),
			CNLocalModel:   ctx.String("cnlocal-model"),

			DefaultImageToImageModel: ctx.String("default-img2img-model"),
			DefaultTextToImageModel:  ctx.String("default-txt2img-model"),

			ImageToImageRecognitionProvider: ctx.String("img2img-recognition-provider"),

			FontPath:          ctx.String("font-path"),
			ServiceStatusPage: ctx.String("service-status-page"),

			FreeChatEnabled:          ctx.Bool("free-chat-enabled"),
			FreeChatDailyLimit:       ctx.Int("free-chat-daily-limit"),
			FreeChatDailyGlobalLimit: ctx.Int("free-chat-daily-global-limit"),

			WeChatAppID:                 ctx.String("wechat-appid"),
			WeChatSecret:                ctx.String("wechat-secret"),
			WeChatPayEnabled:            ctx.Bool("wechatpay-enabled"),
			WeChatPayMchID:              ctx.String("wechatpay-mchid"),
			WeChatPayCertSerialNumber:   ctx.String("wechatpay-cert-serial-number"),
			WeChatPayCertPrivateKeyPath: ctx.String("wechatpay-cert-private-key-path"),
			WeChatPayAPIv3Key:           ctx.String("wechatpay-api-v3-key"),
			WeChatPayNotifyURL:          ctx.String("wechatpay-notify-url"),

			Stripe: stripe,

			DefaultHomeModels:    ctx.StringSlice("default-home-models"),
			DefaultHomeModelsIOS: ctx.StringSlice("default-home-models-ios"),
			DefaultRoleModel:     ctx.String("default-role-model"),

			TextToVoiceEngine:      ctx.String("text-to-voice-engine"),
			TextToVoiceAzureRegion: ctx.String("text-to-voice-azure-region"),
			TextToVoiceAzureKey:    ctx.String("text-to-voice-azure-key"),

			EnableVoiceToText: ctx.Bool("enable-voice-to-text"),
			EnableTextToVoice: ctx.Bool("enable-text-to-voice"),

			MiniMaxAPIKey:  ctx.String("minimax-api-key"),
			MiniMaxGroupID: ctx.String("minimax-group-id"),

			FluxAPIServer: ctx.String("flux-api-server"),
			FluxAPIKey:    ctx.String("flux-api-key"),

			BigModelSearchAPIKey:   ctx.String("bigmodel-search-api-key"),
			BochaaiSearchAPIKey:    ctx.String("bochaai-search-api-key"),
			SearchEngine:           ctx.String("search-engine"),
			AvailableSearchEngines: array.Uniq(append(ctx.StringSlice("available-search-engines"), ctx.String("search-engine"))),
			SearchAssistantModel:   ctx.String("search-assistant-model"),
			SearchAssistantAPIBase: ctx.String("search-assistant-api-base"),
			SearchAssistantAPIKey:  ctx.String("search-assistant-api-key"),
		}
	})
}
