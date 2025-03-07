package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type TranslationResponse struct {
	OriginalText   []string `json:"original_text"`
	TranslatedText []string `json:"translated_text"`
}

type TranslationService struct {
	apiURL string
}

func NewTranslationService(apiURL string) *TranslationService {
	return &TranslationService{
		apiURL: apiURL,
	}
}

func (s *TranslationService) Translate(texts []string) ([]string, error) {
	// 构建请求 URL
	baseURL, err := url.Parse(s.apiURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing API URL: %v", err)
	}

	// 添加查询参数
	query := baseURL.Query()
	for _, text := range texts {
		query.Add("list", text)
	}
	baseURL.RawQuery = query.Encode()

	// 发送 GET 请求
	resp, err := http.Get(baseURL.String())
	if err != nil {
		return nil, fmt.Errorf("error making translation request: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned non-200 status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result TranslationResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("error decoding response: %v, body: %s", err, string(body))
	}

	return result.TranslatedText, nil
}
