package openai

import (
	"github.com/mylxsw/aidea-server/pkg/proxy"
	"github.com/mylxsw/go-utils/ternary"
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/infra"
	"github.com/sashabaranov/go-openai"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config, resolver infra.Resolver) *DalleImageClient {
		var proxyDialer *proxy.Proxy
		if conf.SupportProxy() && ((conf.DalleUsingOpenAISetting && conf.OpenAIAutoProxy) || (!conf.DalleUsingOpenAISetting && conf.OpenAIDalleAutoProxy)) {
			resolver.MustResolve(func(pp *proxy.Proxy) {
				proxyDialer = pp
			})
		}

		return NewDalleImageClient(parseDalleConfig(conf), proxyDialer)
	})

	binder.MustSingleton(func(conf *config.Config, resolver infra.Resolver) Client {
		var proxyDialer *proxy.Proxy
		if conf.SupportProxy() {
			resolver.MustResolve(func(pp *proxy.Proxy) {
				proxyDialer = pp
			})
		}

		var mainClient, backupClient Client
		if conf.EnableOpenAI {
			mainClient = NewOpenAIClient(parseMainConfig(conf), proxyDialer)
		}

		if conf.EnableFallbackOpenAI {
			backupClient = NewOpenAIClient(parseBackupConfig(conf), proxyDialer)
		}

		return NewOpenAIProxy(mainClient, backupClient)
	})
}

func NewOpenAIClient(conf *Config, pp *proxy.Proxy) Client {
	clients := make([]*openai.Client, 0)

	// 如果是 Azure API，则每一个 Server 对应一个 Key
	// 否则 Servers 和 Keys 取笛卡尔积
	if conf.OpenAIAzure {
		for i, server := range conf.OpenAIServers {
			clients = append(clients, createOpenAIClient(
				true,
				conf.OpenAIAPIVersion,
				server,
				"",
				conf.OpenAIKeys[i],
				ternary.If(conf.AutoProxy, pp, nil),
			))
		}
	} else {
		for _, server := range conf.OpenAIServers {
			for _, key := range conf.OpenAIKeys {
				clients = append(clients, createOpenAIClient(
					false,
					"",
					server,
					conf.OpenAIOrganization,
					key,
					ternary.If(conf.AutoProxy, pp, nil),
				))
			}
		}
	}

	return New(conf, clients)
}

func createOpenAIClient(isAzure bool, apiVersion string, server, organization, key string, pp *proxy.Proxy) *openai.Client {
	openaiConf := openai.DefaultConfig(key)
	openaiConf.BaseURL = server
	openaiConf.OrgID = organization
	openaiConf.HTTPClient.Timeout = 180 * time.Second
	if pp != nil {
		openaiConf.HTTPClient.Transport = pp.BuildTransport()
	} else {
		openaiConf.HTTPClient.Transport = &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 120 * time.Second,
			}).DialContext,
		}
	}

	if isAzure {
		openaiConf.APIType = openai.APITypeAzure
		openaiConf.APIVersion = apiVersion
		openaiConf.AzureModelMapperFunc = func(model string) string {
			// TODO 应该使用配置文件配置，注意，这里返回的应该是 Azure 部署名称
			switch model {
			case "gpt-3.5-turbo", "gpt-3.5-turbo-0613":
				return "gpt35-turbo"
			case "gpt-3.5-turbo-16k", "gpt-3.5-turbo-16k-0613":
				return "gpt35-turbo-16k"
			case "gpt-4", "gpt-4-0613":
				return "gpt4"
			case "gpt-4-32k", "gpt-4-32k-0613":
				return "gpt4-32k"
			}

			return regexp.MustCompile(`[.:]`).ReplaceAllString(model, "")
		}
	}

	return openai.NewClientWithConfig(openaiConf)
}
