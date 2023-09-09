package auth

type ClientInfo struct {
	Version         string `json:"version"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	Language        string `json:"language"`
	IP              string `json:"ip"`
}
