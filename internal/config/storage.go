package config

type Storage struct {
	SQLite *SQLLiteStorage `mapstructure:"sqlite,omitempty"`
	// PostgreSQL *StoragePostgreSQL `mapstructure:"postgresql,omitempty"`
}

type SQLLiteStorage struct {
	Path string `mapstructure:"path,omitempty"`
}
