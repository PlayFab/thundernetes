package controllers

// Config is a struct containing configuration from environment variables
// source: https://github.com/caarlos0/env
type Config struct {
	ApiServiceSecurity                     string `env:"API_SERVICE_SECURITY"`
	TlsSecretName                          string `env:"TLS_SECRET_NAME" envDefault:"tls-secret"`
	TlsSecretNamespace                     string `env:"TLS_SECRET_NAMESPACE" envDefault:"thundernetes-system"`
	TlsCertificateName                     string `env:"TLS_CERTIFICATE_FILENAME" envDefault:"tls.crt"`
	TlsPrivateKeyFilename                  string `env:"TLS_PRIVATE_KEY_FILENAME" envDefault:"tls.key"`
	PortRegistryExclusivelyGameServerNodes bool   `env:"PORT_REGISTRY_EXCLUSIVELY_GAME_SERVER_NODES" envDefault:"false"`
	LogLevel                               string `env:"LOG_LEVEL" envDefault:"info"`
	MinPort                                int32  `env:"MIN_PORT" envDefault:"10000"`
	MaxPort                                int32  `env:"MAX_PORT" envDefault:"12000"`
	AllocationApiSvcPort                   int32  `env:"ALLOC_API_SVC_PORT" envDefault:"5000"`
	InitContainerImageLinux                string `env:"THUNDERNETES_INIT_CONTAINER_IMAGE,notEmpty"`
	InitContainerImageWin                  string `env:"THUNDERNETES_INIT_CONTAINER_IMAGE_WIN,notEmpty"`
	MaxNumberOfGameServersToAdd            int  `env:"MAX_NUM_GS_TO_ADD" envDefault:"20"`
	MaxNumberOfGameServersToDelete         int  `env:"MAX_NUM_GS_TO_DEL" envDefault:"20"`
}