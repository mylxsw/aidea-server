package main

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/api"
	"github.com/mylxsw/aidea-server/migrate"
	"github.com/mylxsw/aidea-server/pkg/ai/anthropic"
	"github.com/mylxsw/aidea-server/pkg/ai/baichuan"
	"github.com/mylxsw/aidea-server/pkg/ai/baidu"
	"github.com/mylxsw/aidea-server/pkg/ai/chat"
	"github.com/mylxsw/aidea-server/pkg/ai/dashscope"
	"github.com/mylxsw/aidea-server/pkg/ai/deepai"
	"github.com/mylxsw/aidea-server/pkg/ai/fromston"
	"github.com/mylxsw/aidea-server/pkg/ai/getimgai"
	"github.com/mylxsw/aidea-server/pkg/ai/gpt360"
	"github.com/mylxsw/aidea-server/pkg/ai/leap"
	"github.com/mylxsw/aidea-server/pkg/ai/lepton"
	"github.com/mylxsw/aidea-server/pkg/ai/oneapi"
	"github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/ai/sensenova"
	"github.com/mylxsw/aidea-server/pkg/ai/stabilityai"
	"github.com/mylxsw/aidea-server/pkg/ai/tencentai"
	"github.com/mylxsw/aidea-server/pkg/ai/xfyun"
	"github.com/mylxsw/aidea-server/pkg/aliyun"
	"github.com/mylxsw/aidea-server/pkg/dingding"
	"github.com/mylxsw/aidea-server/pkg/mail"
	"github.com/mylxsw/aidea-server/pkg/proxy"
	"github.com/mylxsw/aidea-server/pkg/rate"
	"github.com/mylxsw/aidea-server/pkg/redis"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/service"
	"github.com/mylxsw/aidea-server/pkg/sms"
	"github.com/mylxsw/aidea-server/pkg/tencent"
	"github.com/mylxsw/aidea-server/pkg/token"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"github.com/mylxsw/aidea-server/pkg/voice"
	"github.com/mylxsw/aidea-server/pkg/youdao"
	"math/rand"
	"path/filepath"
	"time"

	"github.com/mylxsw/aidea-server/internal/jobs"
	"github.com/mylxsw/aidea-server/internal/payment/alipay"
	"github.com/mylxsw/aidea-server/internal/payment/applepay"
	"github.com/mylxsw/aidea-server/internal/queue"
	"github.com/mylxsw/aidea-server/internal/queue/consumer"
	"github.com/mylxsw/asteria/formatter"
	"github.com/mylxsw/asteria/level"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/asteria/writer"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/server"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/starter/app"
)

var GitCommit string
var Version string

func main() {
	// 初始化随机数种子
	rand.Seed(time.Now().UnixNano())
	// 关闭框架的 WARN 日志
	infra.WARN = false

	ins := app.Create(fmt.Sprintf("%s(%s)", Version, GitCommit), 3).WithYAMLFlag("conf")

	// 配置文件
	// 命令行选项（使用配置文件的话，只需要指定 `--conf 配置文件地址`，格式为 YAML）
	config.Register(ins)

	// 日志配置
	ins.Init(func(f infra.FlagContext) error {
		if !f.Bool("log-colorful") {
			log.All().LogFormatter(formatter.NewJSONFormatter())
		}

		if f.String("log-path") != "" {
			log.All().LogWriter(writer.NewDefaultRotatingFileWriter(context.TODO(), func(le level.Level, module string) string {
				return filepath.Join(f.String("log-path"), fmt.Sprintf("%s.%s.log", le.GetLevelName(), time.Now().Format("20060102")))
			}))
		}

		return nil
	})

	//ins.Async(func(conf *config.Config) {
	//	log.With(conf).Debugf("configuration loaded")
	//})

	ins.OnServerReady(func(conf *config.Config) {
		log.Infof("服务启动成功，监听地址为 %s", conf.Listen)
	})

	// 配置要加载的服务模块
	ins.Provider(
		api.Provider{},
		server.Provider{},
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
		migrate.Provider{},
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
		sensenova.Provider{},
		tencentai.Provider{},
		anthropic.Provider{},
		baichuan.Provider{},
		gpt360.Provider{},
		oneapi.Provider{},
		lepton.Provider{},
	)

	app.MustRun(ins)
}
