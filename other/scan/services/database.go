package services

import (
	"fmt"
	"log"
	"scan/models"
	"strings"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type DatabaseService struct {
	db *gorm.DB
}

// 翻译字段结构
type TranslationField struct {
	ID     uint              `gorm:"column:id"`
	Fields map[string]string // 字段名到内容的映射
}

func NewDatabaseService(config models.DatabaseConfig) (*DatabaseService, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.Username,
		config.Password,
		config.Host,
		config.Port,
		config.DBName,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	return &DatabaseService{db: db}, nil
}

// GetTextsForTranslation 获取需要翻译的文本
func (s *DatabaseService) GetTextsForTranslation(tableName string, primaryKey string, fields []models.Field) ([]TranslationField, error) {
	// 首先检查并修改字段长度
	if err := s.ensureFieldLength(tableName, fields); err != nil {
		return nil, fmt.Errorf("error ensuring field length: %v", err)
	}

	// 验证表名和字段名
	if err := s.validateTableAndField(tableName, primaryKey); err != nil {
		return nil, fmt.Errorf("invalid primary key: %v", err)
	}

	for _, field := range fields {
		if err := s.validateTableAndField(tableName, field.Name); err != nil {
			return nil, err
		}
		// 验证翻译后的字段是否存在
		if err := s.validateTableAndField(tableName, field.TranslatedName); err != nil {
			return nil, err
		}
	}

	// 构建字段列表
	fieldList := primaryKey
	for _, field := range fields {
		fieldList += fmt.Sprintf(", %s", field.Name)
	}

	// 构建查询条件
	conditions := make([]string, 0, len(fields))
	for _, field := range fields {
		conditions = append(conditions, fmt.Sprintf("(%s IS NOT NULL AND %s != '' AND %s NOT LIKE '%%\\0%%')",
			field.Name, field.Name, field.Name))
	}

	// 构建完整查询
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s",
		fieldList,
		tableName,
		strings.Join(conditions, " OR "))

	rows, err := s.db.Raw(query).Rows()
	if err != nil {
		return nil, fmt.Errorf("error querying table %s: %v", tableName, err)
	}
	defer rows.Close()

	var results []TranslationField
	for rows.Next() {
		// 准备扫描的变量
		scanVars := make([]interface{}, len(fields)+1)
		var id uint
		scanVars[0] = &id
		fieldValues := make([]string, len(fields))
		for i := range fields {
			scanVars[i+1] = &fieldValues[i]
		}

		// 扫描行
		if err := rows.Scan(scanVars...); err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// 构建结果
		fieldMap := make(map[string]string)
		for i, field := range fields {
			fieldMap[field.Name] = fieldValues[i]
		}

		results = append(results, TranslationField{
			ID:     id,
			Fields: fieldMap,
		})
	}

	return results, nil
}

// ensureFieldLength 确保字段长度至少为255
func (s *DatabaseService) ensureFieldLength(tableName string, fields []models.Field) error {
	// 获取所有需要检查的字段
	allFields := make([]string, 0, len(fields)*2)
	for _, field := range fields {
		allFields = append(allFields, field.Name, field.TranslatedName)
	}

	// 获取字段信息
	for _, fieldName := range allFields {
		var columnInfo struct {
			ColumnName   string `gorm:"column:COLUMN_NAME"`
			ColumnType   string `gorm:"column:COLUMN_TYPE"`
			CharacterMax int64  `gorm:"column:CHARACTER_MAXIMUM_LENGTH"`
		}

		err := s.db.Raw(`
			SELECT COLUMN_NAME, COLUMN_TYPE, CHARACTER_MAXIMUM_LENGTH 
			FROM information_schema.columns 
			WHERE table_schema = DATABASE() 
			AND table_name = ? 
			AND column_name = ?`,
			tableName,
			fieldName,
		).Scan(&columnInfo).Error

		if err != nil {
			return fmt.Errorf("error getting column info for %s: %v", fieldName, err)
		}

		// 检查字段长度是否需要修改
		if columnInfo.CharacterMax < 255 {
			log.Printf("Field %s in table %s needs to be modified (current length: %d)", fieldName, tableName, columnInfo.CharacterMax)

			// 修改字段长度
			alterQuery := fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",
				tableName, fieldName)

			if err := s.db.Exec(alterQuery).Error; err != nil {
				return fmt.Errorf("error modifying field %s: %v", fieldName, err)
			}

			log.Printf("Successfully modified field %s in table %s to VARCHAR(255)", fieldName, tableName)
		}
	}

	return nil
}

// UpdateTranslatedTexts 更新翻译后的文本
func (s *DatabaseService) UpdateTranslatedTexts(tableName string, primaryKey string, id uint, translations map[string]string) error {
	if len(translations) == 0 {
		return fmt.Errorf("no translations provided")
	}

	// 验证主键
	if err := s.validateTableAndField(tableName, primaryKey); err != nil {
		return fmt.Errorf("invalid primary key: %v", err)
	}

	// 构建更新语句
	sets := make([]string, 0, len(translations))
	values := make([]interface{}, 0, len(translations)+1)
	for field, text := range translations {
		// 验证字段
		if err := s.validateTableAndField(tableName, field); err != nil {
			return err
		}
		// 清理翻译文本
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		sets = append(sets, fmt.Sprintf("%s = ?", field))
		values = append(values, text)
	}

	if len(sets) == 0 {
		return fmt.Errorf("no valid translations to update")
	}

	// 添加ID到values
	values = append(values, id)

	// 执行更新
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?",
		tableName,
		strings.Join(sets, ", "),
		primaryKey)

	result := s.db.Exec(query, values...)
	if result.Error != nil {
		return fmt.Errorf("error updating table %s: %v", tableName, result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("no record found with %s = %d in table %s", primaryKey, id, tableName)
	}

	return nil
}

// validateTableAndField 验证表名和字段名
func (s *DatabaseService) validateTableAndField(tableName, fieldName string) error {
	// 检查表是否存在
	var tableExists bool
	err := s.db.Raw("SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", tableName).
		Scan(&tableExists).Error
	if err != nil {
		return fmt.Errorf("error checking table existence: %v", err)
	}
	if !tableExists {
		return fmt.Errorf("table %s does not exist", tableName)
	}

	// 检查字段是否存在
	var fieldExists bool
	err = s.db.Raw("SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?",
		tableName, fieldName).Scan(&fieldExists).Error
	if err != nil {
		return fmt.Errorf("error checking field existence: %v", err)
	}
	if !fieldExists {
		return fmt.Errorf("field %s does not exist in table %s", fieldName, tableName)
	}

	return nil
}
