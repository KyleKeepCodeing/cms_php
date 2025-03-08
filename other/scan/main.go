package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"scan/models"
	"scan/services"
	"time"
)

func loadConfig() (*models.Config, error) {
	file, err := os.ReadFile("config.json")
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	var config models.Config
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	return &config, nil
}

const batchSize = 10 // 每批处理的记录数

func main() {
	// 设置日志格式
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// 加载配置
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化数据库服务
	dbService, err := services.NewDatabaseService(config.Database)
	if err != nil {
		log.Fatalf("Failed to initialize database service: %v", err)
	}

	// 初始化表信息
	if err := dbService.InitializeTableInfo(config.TranslationTables); err != nil {
		log.Fatalf("Failed to initialize table info: %v", err)
	}

	// 初始化翻译服务
	translationService := services.NewTranslationService(config.TranslationAPI.URL)

	// 处理每个配置的表
	for _, table := range config.TranslationTables {
		log.Printf("Processing table: %s", table.TableName)

		// 获取需要翻译的文本
		records, err := dbService.GetTextsForTranslation(table.TableName, table.PrimaryKey, table.Fields)
		if err != nil {
			log.Printf("Error getting texts from table %s: %v", table.TableName, err)
			continue
		}

		if len(records) == 0 {
			log.Printf("No texts to translate in table %s", table.TableName)
			continue
		}

		log.Printf("Found %d records to translate in table %s", len(records), table.TableName)

		// 批量处理翻译
		for i := 0; i < len(records); i += batchSize {
			end := i + batchSize
			if end > len(records) {
				end = len(records)
			}

			log.Printf("Processing batch %d to %d of %d", i+1, end, len(records))

			// 处理当前批次的记录
			for _, record := range records[i:end] {
				// 准备要翻译的文本
				textsToTranslate := make([]string, 0, len(table.Fields))
				for _, field := range table.Fields {
					if text, ok := record.Fields[field.Name]; ok && text != "" {
						textsToTranslate = append(textsToTranslate, text)
					}
				}

				if len(textsToTranslate) == 0 {
					log.Printf("No texts to translate for record %d", record.ID)
					continue
				}

				log.Printf("Translating texts for record %d: %v", record.ID, textsToTranslate)

				// 翻译文本
				translatedTexts, err := translationService.Translate(textsToTranslate)
				if err != nil {
					log.Printf("Error translating texts for record %d: %v", record.ID, err)
					continue
				}

				log.Printf("Received translations for record %d: %v", record.ID, translatedTexts)

				// 准备更新的翻译结果
				translations := make(map[string]string)
				for j, field := range table.Fields {
					if j < len(textsToTranslate) { // 确保有对应的翻译结果
						translations[field.TranslatedName] = translatedTexts[j]
					}
				}

				// 更新数据库
				if err := dbService.UpdateTranslatedTexts(table.TableName, table.PrimaryKey, record.ID, translations); err != nil {
					log.Printf("Error updating translations for record %d: %v", record.ID, err)
					continue
				}

				log.Printf("Successfully translated and updated record %d in table %s", record.ID, table.TableName)
			}

			// 添加短暂延迟以避免请求过快
			time.Sleep(100 * time.Millisecond)
		}

		log.Printf("Completed processing table: %s", table.TableName)
	}

	log.Println("Translation process completed")
}
