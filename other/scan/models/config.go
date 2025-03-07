package models

type Config struct {
	Database          DatabaseConfig     `json:"database"`
	TranslationTables []TranslationTable `json:"translation_tables"`
	TranslationAPI    TranslationAPI     `json:"translation_api"`
}

type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
}

type TranslationTable struct {
	TableName  string  `json:"table_name"`
	PrimaryKey string  `json:"primary_key"` // 主键字段名
	Fields     []Field `json:"fields"`
}

type Field struct {
	Name           string `json:"name"`
	TranslatedName string `json:"translated_name"` // 翻译后存储的字段名
}

type TranslationAPI struct {
	URL string `json:"url"`
}
