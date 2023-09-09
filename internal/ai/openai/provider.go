package openai

import (
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
	openaiConf.HTTPClient.Timeout = 120 * time.Second
	if proxy != nil {
		openaiConf.HTTPClient.Transport = &http.Transport{Dial: proxy.Dial}
	}

	if isAzure {
		openaiConf.APIType = openai.APITypeAzure
		openaiConf.APIVersion = apiVersion
		openaiConf.AzureModelMapperFunc = func(model string) string {
			return regexp.MustCompile(`[.:]`).ReplaceAllString(model, "")
		}
	}

	return openai.NewClientWithConfig(openaiConf)
}
