package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
)

// GenerateAdvertRequest represents the input data from the frontend.
type GenerateAdvertRequest struct {
	HorseData string `json:"horseData"`
}

// GenerateAdvertResponse represents the output to the frontend.
type GenerateAdvertResponse struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// Config wires the AI service.
type Config struct {
	ApiURL      string
	ApiKey      string
	Model       string
	Temperature float64
	HTTPClient  *http.Client
}

type Service struct {
	apiURL      string
	apiKey      string
	model       string
	temperature float64
	httpClient  *http.Client
}

func NewService(cfg Config) (*Service, error) {
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	return &Service{
		apiURL:      cfg.ApiURL,
		apiKey:      cfg.ApiKey,
		model:       cfg.Model,
		temperature: cfg.Temperature,
		httpClient:  client,
	}, nil
}

// geminiRequest is the payload for the native Gemini API.
type geminiRequest struct {
	Contents         []geminiContent         `json:"contents"`
	GenerationConfig *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	Temperature      float64 `json:"temperature"`
	ResponseMimeType string  `json:"responseMimeType,omitempty"`
}

// geminiResponse is the response from the native Gemini API.
type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

const systemPrompt = `Sen profesyonel bir metin yazarısın. Sana verilen yarış atı (TJK) verilerini analiz ederek satılık yarış atı ilanı için çarpıcı ve ÇOK KISA (maksimum 4-5 kelime) bir ilan başlığı ve HTML formatında yapılandırılmış bir metin açıklaması oluşturacaksın. Açıklama metninin yapısı şu şekilde olmalı: 1) Atı tanıtan profesyonel ve akıcı kısa bir giriş paragrafı (<p>). 2) Atın verilerini (pedigri, anne/baba, koşular, doğum tarihi vb.) "Neden [Atın Adı]?" ve "Genel Bilgiler" gibi başlıklar altında <ul>, <li> ve <strong> etiketleriyle madde madde listele. DİKKAT: İlanın sonuna "kaçırılmayacak fırsat", "vizyoner yetiştiriciler", "yatırım fırsatı" gibi yorum, pazarlama veya kapanış cümleleri KESİNLİKLE EKLEME. Madde işaretli liste bittikten sonra metni anında bitir, listenin altına hiçbir paragraf yazma. JSON formatında yanıt ver. JSON şu anahtarları içermeli: "title" (İlan başlığı), "description" (HTML formatında açıklama).`

func (s *Service) GenerateAdvert(ctx context.Context, req GenerateAdvertRequest) (GenerateAdvertResponse, error) {
	if s.apiKey == "" {
		// Graceful degradation
		return GenerateAdvertResponse{
			Title:       "",
			Description: "",
		}, nil
	}

	payload := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: systemPrompt + "\n\nİşte atın verileri:\n" + req.HorseData},
				},
			},
		},
		GenerationConfig: &geminiGenerationConfig{
			Temperature:      s.temperature,
			ResponseMimeType: "application/json",
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return GenerateAdvertResponse{}, apperr.Internal(fmt.Errorf("Failed to marshal AI request: %w", err))
	}

	// Construct native Gemini URL
	apiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", s.model, s.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return GenerateAdvertResponse{}, apperr.Internal(fmt.Errorf("Failed to create AI request: %w", err))
	}

	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := s.httpClient.Do(httpReq)
	if err != nil {
		fmt.Printf("Gemini API Network Error: %v\n", err)
		// Return empty on failure for graceful degradation
		return GenerateAdvertResponse{}, nil
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		// Log the error body to understand why it failed
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		fmt.Printf("Gemini API Error (Status %d): %s\n", httpResp.StatusCode, string(bodyBytes))
		// Return empty on failure for graceful degradation
		return GenerateAdvertResponse{}, nil
	}

	var aiResp geminiResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&aiResp); err != nil {
		fmt.Printf("Gemini API Decode Error: %v\n", err)
		return GenerateAdvertResponse{}, nil
	}

	if len(aiResp.Candidates) == 0 || len(aiResp.Candidates[0].Content.Parts) == 0 {
		return GenerateAdvertResponse{}, nil
	}

	content := aiResp.Candidates[0].Content.Parts[0].Text
	var result GenerateAdvertResponse
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		fmt.Printf("Gemini API JSON Unmarshal Error: %v\nContent: %s\n", err, content)
		return GenerateAdvertResponse{}, nil
	}

	fmt.Printf("Parsed AI Response: Title='%s', DescriptionLength=%d\n", result.Title, len(result.Description))
	if len(result.Description) == 0 {
		fmt.Printf("Raw Content was: %s\n", content)
	}

	return result, nil
}
