package openai

import (
	"github.com/mylxsw/aidea-server/config"
	"net/http"
)

type Config struct {
	Enable             bool
	OpenAIAzure        bool
	OpenAIAPIVersion   string
	OpenAIOrganization string
	OpenAIServers      []string
	OpenAIKeys         []string
	AutoProxy          bool
	Header             http.Header
}

func parseMainConfig(conf *config.Config) *Config {
	return &Config{
		Enable:             conf.EnableOpenAI,
		OpenAIAzure:        conf.OpenAIAzure,
		OpenAIAPIVersion:   conf.OpenAIAPIVersion,
		OpenAIOrganization: conf.OpenAIOrganization,
		OpenAIServers:      conf.OpenAIServers,
		OpenAIKeys:         conf.OpenAIKeys,
		AutoProxy:          conf.OpenAIAutoProxy,
	}
}

func parseBackupConfig(conf *config.Config) *Config {
	return &Config{
		Enable:             conf.EnableFallbackOpenAI,
		OpenAIAzure:        conf.FallbackOpenAIAzure,
		OpenAIAPIVersion:   conf.FallbackOpenAIAPIVersion,
		OpenAIOrganization: conf.FallbackOpenAIOrganization,
		OpenAIServers:      conf.FallbackOpenAIServers,
		OpenAIKeys:         conf.FallbackOpenAIKeys,
		AutoProxy:          conf.FallbackOpenAIAutoProxy,
	}
}

func parseDalleConfig(conf *config.Config) *Config {
	if conf.DalleUsingOpenAISetting {
		return &Config{
			Enable:             conf.EnableOpenAI && conf.EnableOpenAIDalle,
			OpenAIAzure:        conf.OpenAIAzure,
			OpenAIAPIVersion:   conf.OpenAIAPIVersion,
			OpenAIOrganization: conf.OpenAIOrganization,
			OpenAIServers:      conf.OpenAIServers,
			OpenAIKeys:         conf.OpenAIKeys,
			AutoProxy:          conf.OpenAIAutoProxy,
		}
	}

	return &Config{
		Enable:             conf.EnableOpenAIDalle,
		OpenAIAzure:        conf.OpenAIDalleAzure,
		OpenAIAPIVersion:   conf.OpenAIDalleAPIVersion,
		OpenAIOrganization: conf.OpenAIDalleOrganization,
		OpenAIServers:      conf.OpenAIDalleServers,
		OpenAIKeys:         conf.OpenAIDalleKeys,
		AutoProxy:          conf.OpenAIDalleAutoProxy,
	}
}

type CustomRequestTransport struct {
	Origin http.RoundTripper
	Header http.Header
}

func (t *CustomRequestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range t.Header {
		req.Header.Set(k, v[0])
	}

	return t.Origin.RoundTrip(req)
}

func NewCustomRequestTransport(origin http.RoundTripper, header http.Header) *CustomRequestTransport {
	return &CustomRequestTransport{Origin: origin, Header: header}
}
