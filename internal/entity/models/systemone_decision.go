//
//  Copyright 2026 The InfiniFlow Authors. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

package models

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"ragflow/internal/common"
)

// Decide asks probability questions about one state in a single request.
func (m *OpenAIAPICompatibleModel) Decide(ctx context.Context, modelName *string, request *DecideRequest, apiConfig *APIConfig, modelUsage *common.ModelUsage) (*DecideResponse, error) {
	if m.baseModel.URLSuffix.Decision == "" {
		return nil, fmt.Errorf("%s: decision endpoint is not configured", m.Name())
	}
	if modelName == nil || strings.TrimSpace(*modelName) == "" {
		return nil, fmt.Errorf("systemone: model name is required")
	}
	if request == nil || len(request.Questions) == 0 || len(request.Questions) > MaxDecisionQuestions {
		return nil, fmt.Errorf("systemone: between 1 and %d questions are required", MaxDecisionQuestions)
	}
	for id, question := range request.Questions {
		if _, ok := question.(Noul); !ok {
			return nil, fmt.Errorf("systemone: question %q must be a noul question", id)
		}
	}
	if apiConfig == nil || apiConfig.ApiKey == nil || strings.TrimSpace(*apiConfig.ApiKey) == "" {
		return nil, fmt.Errorf("systemone: api key is required")
	}
	url, err := m.baseModel.GetBaseURL(apiConfig)
	if err != nil {
		return nil, err
	}
	suffix := strings.Trim(m.baseModel.URLSuffix.Decision, "/")
	if suffix != "" && !strings.HasSuffix(strings.ToLower(url), "/"+strings.ToLower(suffix)) {
		url = joinURL(url, suffix)
	}
	body, err := m.baseModel.doRequest(ctx, url, apiConfig, map[string]any{
		"model": *modelName, "state": request.State, "questions": request.Questions,
	}, nonStreamCallTimeout)
	if err != nil {
		return nil, fmt.Errorf("systemone: %w", err)
	}
	var response DecideResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("systemone: decode response: %w", err)
	}
	if len(response.Answers) == 0 {
		return nil, fmt.Errorf("systemone: no answers in response")
	}
	for id, answer := range response.Answers {
		if answer.Noul != nil && (math.IsNaN(*answer.Noul) || math.IsInf(*answer.Noul, 0) || *answer.Noul < 0 || *answer.Noul > 1) {
			return nil, fmt.Errorf("systemone: invalid probability for question %q", id)
		}
	}
	recordResponseUsage(modelUsage, response.ID, &TokenUsage{
		PromptTokens: response.Usage.InputTokens, CompletionTokens: response.Usage.OutputTokens,
		TotalTokens: response.Usage.InputTokens + response.Usage.OutputTokens,
	}, "decision")
	return &response, nil
}
