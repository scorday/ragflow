package models

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterAIDecisionsAndDiscovery(t *testing.T) {
	withSSRFBypass(t)
	config, err := decodeProviderConfig(readProviderConfig(t, "routerai.json"))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing provider credential")
		}
		switch r.URL.Path {
		case "/api/v1/models":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{
				{"id": "~typesafe/jev-latest"}, {"id": "typesafe/jev-1.14"},
				{"id": "deepseek/deepseek-v4-flash"}, {"id": "openai/text-embedding-3-small"},
			}})
		case "/api/v1/decisions":
			if r.Method != http.MethodPost {
				t.Errorf("method = %s", r.Method)
			}
			var request struct {
				Model     string                                         `json:"model"`
				State     string                                         `json:"state"`
				Questions map[string]struct{ Type, Instructions string } `json:"questions"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request.Model != "~typesafe/jev-latest" || request.State != "passage" || request.Questions["relevant"].Type != "noul" {
				t.Errorf("unexpected payload: %+v", request)
			}
			_, _ = w.Write([]byte(`{"answers":{"relevant":{"type":"noul","noul":0.95}},"usage":{"input_tokens":12,"output_tokens":3}}`))
		default:
			t.Errorf("unexpected endpoint: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	driver, err := NewModelFactory().CreateModelDriver(config.Name, config.URL, config.URLSuffix)
	if err != nil {
		t.Fatal(err)
	}
	driver = driver.NewInstance(map[string]string{"default": server.URL + "/api/v1"})
	key, name := "test-key", "~typesafe/jev-latest"
	apiConfig := &APIConfig{ApiKey: &key}
	models, err := driver.ListModels(t.Context(), apiConfig)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 4 {
		t.Fatalf("model discovery returned %d models", len(models))
	}
	for _, model := range models[:2] {
		if len(model.ModelTypes) != 1 || model.ModelTypes[0] != "decision" {
			t.Errorf("Jev discovered as %v", model.ModelTypes)
		}
	}
	response, err := driver.Decide(t.Context(), &name, &DecideRequest{
		State: "passage", Questions: map[string]Question{"relevant": Noul{Instructions: "Relevant?"}},
	}, apiConfig, nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.Answers["relevant"].Noul == nil || *response.Answers["relevant"].Noul != 0.95 || response.Usage.InputTokens != 12 {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestOpenAICompatibleDecisionRequiresEndpoint(t *testing.T) {
	driver := NewOpenAIAPICompatibleModel(nil, URLSuffix{})
	name, key := "~typesafe/jev-latest", "test-key"
	if _, err := driver.Decide(t.Context(), &name, &DecideRequest{
		Questions: map[string]Question{"probe": Noul{Instructions: "Relevant?"}},
	}, &APIConfig{ApiKey: &key}, nil); err == nil {
		t.Fatal("decision request accepted without a configured endpoint")
	}
}
