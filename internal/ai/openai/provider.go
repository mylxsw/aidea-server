package openai

import (
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/sashabaranov/go-openai"
	"golang.org/x/net/proxy"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config, resolver infra.Resolver) *OpenAI {
		var proxyDialer proxy.Dialer
		if conf.Socks5Proxy != "" && conf.OpenAIAutoProxy {
			resolver.MustResolve(func(dialer proxy.Dialer) {
				proxyDialer = dialer
			})
		}

		clients := make([]*openai.Client, 0)

		// 如果是 Azure API，则每一个 Server 对应一个 Key
		// 否则 Servers 和 Keys 取笛卡尔积
		if conf.OpenAIAzure {
			for i, server := range conf.OpenAIServers {
				clients = append(clients, createOpenAIClient(true, conf.OpenAIAPIVersion, server, "", conf.OpenAIKeys[i], proxyDialer))
			}
		} else {
			for _, server := range conf.OpenAIServers {
				for _, key := range conf.OpenAIKeys {
					clients = append(clients, createOpenAIClient(false, "", server, conf.OpenAIOrganization, key, proxyDialer))
				}
			}
		}

		log.Debugf("create %d openai clients", len(clients))

		return New(conf, clients)
	})
}

func createOpenAIClient(isAzure bool, apiVersion string, server, organization, key string, proxy proxy.Dialer) *openai.Client {
	openaiConf := openai.DefaultConfig(key)
	openaiConf.BaseURL = server
	openaiConf.OrgID = organization
	openaiConf.HTTPClient.Timeout = 180 * time.Second
	if proxy != nil {
		openaiConf.HTTPClient.Transport = &http.Transport{Dial: proxy.Dial}
	} else {
		openaiConf.HTTPClient.Transport = &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 15 * time.Second,
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
