package config

type AuthType string

const (
	AuthHTTPBasic AuthType = "httpbasic"
	AuthToken     AuthType = "token"
)

type AuthConfig struct {
	Type     AuthType
	Username string // for httpbasic
	Password string // for httpbasic
	Token    string // for token
}

type ClientConfig struct {
	Host      string
	Port      int
	Auth      AuthConfig
	UseTLS    bool
	VerifyTLS bool // Ignored if UseTLS is false
}
