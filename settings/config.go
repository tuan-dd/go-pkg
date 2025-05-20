package settings

type ServerSetting struct {
	Port        int    `mapstructure:"PORT"`
	Mode        string `mapstructure:"MODE"`
	Level       string `mapstructure:"LEVEL"`
	ServiceName string `mapstructure:"SERVICE_NAME"`
	Version     string `mapstructure:"VERSION"`
	Environment string `mapstructure:"ENVIRONMENT"`
}

type CacheSetting struct {
	Host     string `mapstructure:"CACHE_HOST"`
	Port     int    `mapstructure:"CACHE_PORT"`
	Password string `mapstructure:"CACHE_PASSWORD"`
	PoolSize int    `mapstructure:"CACHE_POOL_SIZE"`
}

type AsynQSetting struct {
	Host     string `mapstructure:"CACHE_HOST"`
	Port     int    `mapstructure:"CACHE_PORT"`
	Password string `mapstructure:"CACHE_PASSWORD"`
}

type SQLSetting struct {
	Host            string `mapstructure:"DB_HOST"`
	Port            int    `mapstructure:"DB_PORT"`
	Username        string `mapstructure:"DB_USERNAME"`
	Password        string `mapstructure:"DB_PASSWORD"`
	DBname          string `mapstructure:"DB_DBNAME"`
	SSLMode         string `mapstructure:"SSL_MODE"`
	RDBMS           string `mapstructure:"RDBMS"`
	MaxConnIdleTime uint32 `mapstructure:"MAX_CONN_IDLE_TIME"`
	MaxConnLifetime uint64 `mapstructure:"MAX_CONN_LIFE_TIME"`
	MaxConns        uint8  `mapstructure:"MAX_CONNS"`
	MaxIdleConns    uint8  `mapstructure:"MAX_IDLE_CONNS"`
}

type LoggerSetting struct {
	Level      string `mapstructure:"LOG_LEVEL"`
	MaxSize    int    `mapstructure:"LOG_MAX_SIZE"`
	MaxAge     int    `mapstructure:"LOG_MAX_AGE"`
	MaxBackups int    `mapstructure:"LOG_MAX_BACKUPS"`
	Compress   bool   `mapstructure:"LOG_COMPRESS"`
}
