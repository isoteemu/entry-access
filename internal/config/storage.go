package config

type Storage struct {
	SQLite *SQLLiteStorage `mapstructure:"local,omitempty"`
	// PostgreSQL *StoragePostgreSQL `mapstructure:"postgresql,omitempty"`
}

type SQLLiteStorage struct {
	Path string `mapstructure:"path,omitempty"`
}
