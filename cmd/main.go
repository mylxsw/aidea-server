package main

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"path/filepath"
	"time"

	"github.com/mylxsw/aidea-server/internal/ai/baidu"
	"github.com/mylxsw/aidea-server/internal/ai/chat"
	"github.com/mylxsw/aidea-server/internal/ai/dashscope"
	"github.com/mylxsw/aidea-server/internal/ai/deepai"
	"github.com/mylxsw/aidea-server/internal/ai/fromston"
	"github.com/mylxsw/aidea-server/internal/ai/leap"
	"github.com/mylxsw/aidea-server/internal/ai/openai"
	"github.com/mylxsw/aidea-server/internal/ai/stabilityai"
	"github.com/mylxsw/aidea-server/internal/ai/xfyun"
	"github.com/mylxsw/aidea-server/internal/dingding"
	"github.com/mylxsw/aidea-server/internal/mail"
	"github.com/mylxsw/aidea-server/internal/payment/alipay"
	"github.com/mylxsw/aidea-server/internal/payment/applepay"
	"github.com/mylxsw/aidea-server/internal/proxy"
	"github.com/mylxsw/aidea-server/internal/tencent"
	"github.com/mylxsw/aidea-server/internal/uploader"
	"github.com/mylxsw/aidea-server/internal/youdao"

	"github.com/mylxsw/aidea-server/internal/ai/getimgai"
	"github.com/mylxsw/aidea-server/internal/aliyun"
	"github.com/mylxsw/aidea-server/internal/jobs"
	"github.com/mylxsw/aidea-server/internal/queue"
	"github.com/mylxsw/aidea-server/internal/queue/consumer"
	"github.com/mylxsw/aidea-server/internal/rate"
	"github.com/mylxsw/aidea-server/internal/service"
	"github.com/mylxsw/aidea-server/internal/sms"
	"github.com/mylxsw/aidea-server/internal/token"
	"github.com/mylxsw/aidea-server/internal/voice"
	"github.com/mylxsw/asteria/formatter"
	"github.com/mylxsw/asteria/level"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/asteria/writer"

	_ "github.com/go-sql-driver/mysql"
	"github.com/mylxsw/aidea-server/api"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/redis"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/starter/app"
)

var GitCommit string
var Version string

func main() {
	// 初始化随机数种子
	rand.Seed(time.Now().UnixNano())

	ins := app.Create(fmt.Sprintf("%s(%s)", Version, GitCommit), 3).WithYAMLFlag("conf")

	// 命令行选项（使用配置文件的话，只需要指定 `--conf 配置文件地址`，格式为 YAML）

	ins.AddStringFlag("listen", ":8080", "Web 服务监听地址")
	ins.AddStringFlag("socks5-proxy", "", "socks5 proxy")
	ins.AddStringFlag("db-uri", "root:12345@tcp(127.0.0.1:3306)/aiserver?charset=utf8mb4&parseTime=True&loc=Local", "database url")
	ins.AddStringFlag("session-secret", "aidea-secret", "用户会话加密密钥")
	ins.AddBoolFlag("enable-recordchat", "是否记录聊天历史记录（目前只做记录，没有实际作用，只是为后期增加多端聊天记录同步做准备）")

	ins.AddStringFlag("redis-host", "127.0.0.1", "redis host")
	ins.AddIntFlag("redis-port", 6379, "redis port")
	ins.AddStringFlag("redis-password", "", "redis password")

	ins.AddIntFlag("queue-workers", 0, "任务队列工作线程（Goroutine）数量，设置为 0 则不启用任务队列")
	ins.AddBoolFlag("enable-scheduler", "是否启用定时任务")

	ins.AddBoolFlag("enable-openai", "是否启用 OpenAI")
	ins.AddBoolFlag("openai-azure", "使用 Azure 的 OpenAI 服务")
	ins.AddStringFlag("openai-apiversion", "2023-05-15", "required when openai-azure is true")
	ins.AddBoolFlag("openai-autoproxy", "使用 Socks5 代理访问 OpenAI 服务")
	ins.AddStringFlag("openai-organization", "", "openai organization")
	ins.AddStringSliceFlag("openai-servers", []string{"https://api.openai.com/v1"}, "OpenAI 服务地址，配置多个时会自动在多个服务之间平衡负载，不要忘记在在 URL 后面添加 /v1")
	ins.AddStringSliceFlag("openai-keys", []string{}, "OpenAI Keys，如果指定多个，会在多个服务之间平衡负载")

	ins.AddBoolFlag("enable-baiduwxai", "是否启用百度文心千帆大模型")
	ins.AddStringFlag("baiduwx-key", "", "百度文心大模型 Key")
	ins.AddStringFlag("baiduwx-secret", "", "百度文心大模型 Secret")

	ins.AddBoolFlag("enable-dashscopeai", "是否启用阿里灵积平台(通义千问)")
	ins.AddStringFlag("dashscope-key", "", "阿里灵积平台密钥")

	ins.AddBoolFlag("enable-xfyunai", "是否启用讯飞 星火 AI")
	ins.AddStringFlag("xfyun-appid", "", "讯飞星火 APP ID")
	ins.AddStringFlag("xfyun-apikey", "", "讯飞星火 API Key")
	ins.AddStringFlag("xfyun-apisecret", "", "讯飞星火 API Secret")

	ins.AddBoolFlag("enable-stabilityai", "是否启用 StabilityAI 文生图、图生图服务")
	ins.AddBoolFlag("stabilityai-autoproxy", "使用 socks5 代理访问 StabilityAI 服务")
	ins.AddStringFlag("stabilityai-organization", "", "stabilityai organization")
	ins.AddStringSliceFlag("stabilityai-servers", []string{"https://api.stability.ai"}, "stabilityai servers")
	ins.AddFlags(app.StringEnvFlag("stabilityai-key", "", "stabilityai key", "STABILITYAI_KEY"))

	ins.AddBoolFlag("enable-leapai", "是否启用 LeapAI 文生图、图生图服务")
	ins.AddBoolFlag("leapai-autoproxy", "使用 socks5 代理访问 Leap 服务")
	ins.AddStringSliceFlag("leapai-servers", []string{"https://api.tryleap.ai"}, "leapai servers")
	ins.AddFlags(app.StringEnvFlag("leapai-key", "", "stabilityai key", "LEAPAI_API_KEY"))

	ins.AddBoolFlag("enable-deepai", "是否启用 DeepAI 超分辨率、上色服务")
	ins.AddBoolFlag("deepai-autoproxy", "deepai auto proxy")
	ins.AddStringSliceFlag("deepai-servers", []string{"https://api.deepai.org"}, "deepai servers")
	ins.AddFlags(app.StringEnvFlag("deepai-key", "", "deepai key", "DEEPAI_KEY"))

	ins.AddBoolFlag("enable-getimgai", "是否启用 getimg.ai 文生图、图生图服务")
	ins.AddBoolFlag("getimgai-autoproxy", "使用 socks5 代理访问 getimg.ai 服务")
	ins.AddStringFlag("getimgai-server", "https://api.getimg.ai", "getimgai server")
	ins.AddFlags(app.StringEnvFlag("getimgai-key", "", "getimgai key", "GETIMGAI_KEY"))

	ins.AddBoolFlag("enable-fromstonai", "是否启用 6pen 的文生图、图生图服务")
	ins.AddStringFlag("fromston-server", "https://ston.6pen.art", "fromston server")
	ins.AddStringFlag("fromston-key", "", "fromston key")

	ins.AddBoolFlag("enable-translate", "是否启用翻译服务")
	ins.AddStringFlag("translate-server", "https://openapi.youdao.com/api", " 有道翻译 API 地址")
	ins.AddStringFlag("translate-appid", "", "有道翻译 APPID")
	ins.AddStringFlag("translate-appkey", "", "有道翻译 APPKEY")

	ins.AddStringFlag("storage-appkey", "", "七牛云 APP KEY")
	ins.AddStringFlag("storage-secret", "", "七牛云 APP SECRET")
	ins.AddStringFlag("storage-bucket", "aicode", "七牛云存储 Bucket 名称")
	ins.AddStringFlag("storage-callback", "https://YOUR_SERVER_HOST/v1/callback/storage/qiniu", "七牛云存储上传回调接口")
	ins.AddStringFlag("storage-domain", "", "七牛云存储资源访问域名（也可以用 CDN 域名），例如 https://cdn.example.com")

	ins.AddStringFlag("apple-keyid", "", "apple sign in key id")
	ins.AddStringFlag("apple-teamid", "", "apple sign in team id")
	ins.AddStringFlag("apple-secret", "", "apple sign in secret")

	ins.AddBoolFlag("enable-mail", "是否启用邮件发送服务")
	ins.AddStringFlag("mail-from", "", "mail from")
	ins.AddStringFlag("mail-host", "", "mail host")
	ins.AddIntFlag("mail-port", 465, "mail port")
	ins.AddStringFlag("mail-username", "", "mail username")
	ins.AddStringFlag("mail-password", "", "mail password")
	ins.AddBoolFlag("mail-ssl", "mail ssl")

	ins.AddStringFlag("tencent-id", "", "tencent app id")
	ins.AddStringFlag("tencent-key", "", "tencent app key")
	ins.AddStringFlag("tencent-smssdkappid", "", "tencent sms sdk app id")
	ins.AddBoolFlag("tencent-voice", "是否使用腾讯的语音转文本服务，不启用则使用 OpenAI 的 Whisper 模型")

	ins.AddStringFlag("aliyun-key", "", "aliyun app key")
	ins.AddStringFlag("aliyun-secret", "", "aliyun app secret")
	ins.AddBoolFlag("enable-contentdetect", "是否启用内容安全检测（使用阿里云的内容安全服务）")

	ins.AddBoolFlag("enable-applepay", "启用 Apple 应用内支付")

	ins.AddBoolFlag("enable-alipay", "启用支付宝支付支持，需要指定 alipay-xxx 的所有配置项")
	ins.AddStringFlag("alipay-appid", "", "支付宝 APP ID")
	ins.AddStringFlag("alipay-app-private-key", "path/to/alipay-app-private-key.txt", "支付宝 APP 私钥存储路径")
	ins.AddStringFlag("alipay-app-public-key", "path/to/appCertPublicKey_2021004100000000.crt", "支付宝 APP 公钥证书存储路径")
	ins.AddStringFlag("alipay-root-cert", "path/to/alipayRootCert.crt", "支付宝根证书路径")
	ins.AddStringFlag("alipay-public-key", "path/to/alipayCertPublicKey_RSA2.crt", "支付宝公钥证书路径")

	ins.AddStringSliceFlag("sms-channels", []string{}, "启用的短信通道，支持腾讯云和阿里云: tencent, aliyun，多个值时随机每次发送随机选择")

	ins.AddStringFlag("log-path", "", "日志文件存储目录，留空则写入到标准输出")

	ins.AddStringFlag("dingding-token", "", "钉钉群通知 Token，留空则不通知")
	ins.AddStringFlag("dingding-secret", "", "钉钉群通知 Secret")

	// 配置文件
	config.Register(ins)

	// MySQL 数据库连接
	ins.Singleton(func(conf *config.Config) (*sql.DB, error) {
		return sql.Open("mysql", conf.DBURI)
	})

	// 日志配置
	ins.Init(func(f infra.FlagContext) error {
		log.All().LogFormatter(formatter.NewJSONFormatter())
		if f.String("log-path") != "" {
			log.All().LogWriter(writer.NewDefaultRotatingFileWriter(context.TODO(), func(le level.Level, module string) string {
				return filepath.Join(f.String("log-path"), fmt.Sprintf("%s.%s.log", le.GetLevelName(), time.Now().Format("20060102")))
			}))
		}

		return nil
	})

	// 配置要加载的服务模块
	ins.Provider(
		api.Provider{},
		repo.Provider{},
		redis.Provider{},
		queue.Provider{},
		consumer.Provider{},
		token.Provider{},
		rate.Provider{},
		service.Provider{},
		jobs.Provider{},
		chat.Provider{},
		proxy.Provider{},
	)

	// 普通云服务商
	ins.Provider(
		uploader.Provider{},
		tencent.Provider{},
		aliyun.Provider{},
		sms.Provider{},
		mail.Provider{},
		dingding.Provider{},
		voice.Provider{},
		youdao.Provider{},
		alipay.Provider{},
		applepay.Provider{},
	)

	// AI 服务
	ins.Provider(
		openai.Provider{},
		stabilityai.Provider{},
		deepai.Provider{},
		fromston.Provider{},
		getimgai.Provider{},
		dashscope.Provider{},
		xfyun.Provider{},
		leap.Provider{},
		baidu.Provider{},
	)

	app.MustRun(ins)
}
