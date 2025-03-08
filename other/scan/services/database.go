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
	// 缓存表的字段信息
	tableFields map[string]map[string]*ColumnInfo
}

// 翻译字段结构
type TranslationField struct {
	ID     uint              `gorm:"column:id"`
	Fields map[string]string // 字段名到内容的映射
}

// 字段信息结构
type ColumnInfo struct {
	ColumnName   string `gorm:"column:COLUMN_NAME"`
	ColumnType   string `gorm:"column:COLUMN_TYPE"`
	CharacterMax int64  `gorm:"column:CHARACTER_MAXIMUM_LENGTH"`
	HasIndex     bool
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

	service := &DatabaseService{
		db:          db,
		tableFields: make(map[string]map[string]*ColumnInfo),
	}

	return service, nil
}

// InitializeTableInfo 初始化表信息
func (s *DatabaseService) InitializeTableInfo(tables []models.TranslationTable) error {
	for _, table := range tables {
		// 检查表是否存在
		var tableExists bool
		err := s.db.Raw("SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?",
			table.TableName).Scan(&tableExists).Error
		if err != nil {
			return fmt.Errorf("error checking table %s: %v", table.TableName, err)
		}
		if !tableExists {
			return fmt.Errorf("table %s does not exist", table.TableName)
		}

		// 初始化字段映射
		s.tableFields[table.TableName] = make(map[string]*ColumnInfo)

		// 获取所有需要的字段信息
		fields := []string{table.PrimaryKey}
		for _, field := range table.Fields {
			fields = append(fields, field.Name, field.TranslatedName)
		}

		// 批量获取字段信息
		var columns []struct {
			ColumnName   string `gorm:"column:COLUMN_NAME"`
			ColumnType   string `gorm:"column:COLUMN_TYPE"`
			CharacterMax int64  `gorm:"column:CHARACTER_MAXIMUM_LENGTH"`
		}

		err = s.db.Raw(`
			SELECT COLUMN_NAME, COLUMN_TYPE, CHARACTER_MAXIMUM_LENGTH 
			FROM information_schema.columns 
			WHERE table_schema = DATABASE() 
			AND table_name = ? 
			AND column_name IN (?)`,
			table.TableName,
			fields,
		).Scan(&columns).Error

		if err != nil {
			return fmt.Errorf("error getting columns for table %s: %v", table.TableName, err)
		}

		// 获取索引信息
		var indexes []struct {
			ColumnName string `gorm:"column:COLUMN_NAME"`
		}

		err = s.db.Raw(`
			SELECT DISTINCT COLUMN_NAME
			FROM information_schema.statistics
			WHERE table_schema = DATABASE()
			AND table_name = ?
			AND column_name IN (?)`,
			table.TableName,
			fields,
		).Scan(&indexes).Error

		if err != nil {
			return fmt.Errorf("error getting indexes for table %s: %v", table.TableName, err)
		}

		// 创建索引字段集合
		indexedFields := make(map[string]bool)
		for _, idx := range indexes {
			indexedFields[idx.ColumnName] = true
		}

		// 保存字段信息到缓存
		for _, col := range columns {
			s.tableFields[table.TableName][col.ColumnName] = &ColumnInfo{
				ColumnName:   col.ColumnName,
				ColumnType:   col.ColumnType,
				CharacterMax: col.CharacterMax,
				HasIndex:     indexedFields[col.ColumnName],
			}
		}

		// 验证所有必需的字段都存在
		for _, fieldName := range fields {
			if _, exists := s.tableFields[table.TableName][fieldName]; !exists {
				return fmt.Errorf("field %s does not exist in table %s", fieldName, table.TableName)
			}
		}
	}

	return nil
}

// GetTextsForTranslation 获取需要翻译的文本
func (s *DatabaseService) GetTextsForTranslation(tableName string, primaryKey string, fields []models.Field) ([]TranslationField, error) {
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

// checkFieldIndex 检查字段是否有索引
func (s *DatabaseService) checkFieldIndex(tableName, fieldName string) (bool, error) {
	var indexes []struct {
		IndexName  string `gorm:"column:INDEX_NAME"`
		ColumnName string `gorm:"column:COLUMN_NAME"`
		SeqInIndex int    `gorm:"column:SEQ_IN_INDEX"`
	}

	err := s.db.Raw(`
		SELECT INDEX_NAME, COLUMN_NAME, SEQ_IN_INDEX
		FROM information_schema.statistics
		WHERE table_schema = DATABASE()
		AND table_name = ?
		AND column_name = ?`,
		tableName,
		fieldName,
	).Scan(&indexes).Error

	if err != nil {
		return false, fmt.Errorf("error checking indexes: %v", err)
	}

	return len(indexes) > 0, nil
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

			// 检查字段是否有索引
			hasIndex, err := s.checkFieldIndex(tableName, fieldName)
			if err != nil {
				return fmt.Errorf("error checking index for field %s: %v", fieldName, err)
			}

			if hasIndex {
				log.Printf("Field %s has index, modifying with index length limit", fieldName)
				// 对于有索引的字段，我们设置一个较小的长度以避免索引长度限制问题
				alterQuery := fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s VARCHAR(191) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",
					tableName, fieldName)
				if err := s.db.Exec(alterQuery).Error; err != nil {
					return fmt.Errorf("error modifying indexed field %s: %v", fieldName, err)
				}
			} else {
				// 对于没有索引的字段，我们可以使用完整的长度
				alterQuery := fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",
					tableName, fieldName)
				if err := s.db.Exec(alterQuery).Error; err != nil {
					return fmt.Errorf("error modifying field %s: %v", fieldName, err)
				}
			}

			log.Printf("Successfully modified field %s in table %s", fieldName, tableName)
		}
	}

	return nil
}

// getColumnInfo 获取字段信息
func (s *DatabaseService) getColumnInfo(tableName, fieldName string) (*ColumnInfo, error) {
	var columnInfo ColumnInfo

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
		return nil, fmt.Errorf("error getting column info for %s: %v", fieldName, err)
	}

	// 检查是否有索引
	hasIndex, err := s.checkFieldIndex(tableName, fieldName)
	if err != nil {
		return nil, err
	}
	columnInfo.HasIndex = hasIndex

	return &columnInfo, nil
}

// ensureFieldCapacity 确保字段容量足够
func (s *DatabaseService) ensureFieldCapacity(tableName, fieldName string, requiredLength int64) error {
	columnInfo, exists := s.tableFields[tableName][fieldName]
	if !exists {
		return fmt.Errorf("field %s not found in table %s", fieldName, tableName)
	}

	// 如果需要的长度大于当前长度，则需要扩展字段
	if requiredLength > columnInfo.CharacterMax {
		log.Printf("Field %s in table %s needs to be extended (current: %d, required: %d)",
			fieldName, tableName, columnInfo.CharacterMax, requiredLength)

		var newLength int64
		if columnInfo.HasIndex {
			maxIndexLength := int64(191)
			if requiredLength > maxIndexLength {
				log.Printf("Warning: Field %s has index, cannot extend beyond %d characters", fieldName, maxIndexLength)
				newLength = maxIndexLength
			} else {
				newLength = requiredLength
			}
		} else {
			newLength = requiredLength + 100
			if newLength > 65535 {
				newLength = 65535
			}
		}

		alterQuery := fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s VARCHAR(%d) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",
			tableName, fieldName, newLength)

		if err := s.db.Exec(alterQuery).Error; err != nil {
			return fmt.Errorf("error extending field %s: %v", fieldName, err)
		}

		// 更新缓存中的字段长度
		columnInfo.CharacterMax = newLength
		log.Printf("Successfully extended field %s in table %s from %d to %d characters",
			fieldName, tableName, columnInfo.CharacterMax, newLength)
	}

	return nil
}

// UpdateTranslatedTexts 更新翻译后的文本
func (s *DatabaseService) UpdateTranslatedTexts(tableName string, primaryKey string, id uint, translations map[string]string) error {
	if len(translations) == 0 {
		return fmt.Errorf("no translations provided")
	}

	// 检查并确保所有字段的容量足够
	for field, text := range translations {
		requiredLength := int64(len(text))
		if err := s.ensureFieldCapacity(tableName, field, requiredLength); err != nil {
			return fmt.Errorf("error ensuring field capacity: %v", err)
		}
	}

	// 构建更新语句
	sets := make([]string, 0, len(translations))
	values := make([]interface{}, 0, len(translations)+1)
	for field, text := range translations {
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
