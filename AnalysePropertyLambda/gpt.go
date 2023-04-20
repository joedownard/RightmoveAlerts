package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type GPTRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Temperature      int `json:"temperature"`
	MaxTokens        int `json:"max_tokens"`
	TopP             int `json:"top_p"`
	FrequencyPenalty int `json:"frequency_penalty"`
	PresencePenalty  int `json:"presence_penalty"`
}

type GPTResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

var (
	GptApiKey   = os.Getenv("GPT_API_KEY")
	GptEndpoint = "https://api.openai.com/v1/chat/completions"
)

func RunGPTPromptWithDescription(desc string, prompt string) string {
	gptReq := GPTRequest{
		Model: "gpt-3.5-turbo",
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{{Role: "user", Content: prompt + desc}},
		Temperature:      0,
		MaxTokens:        185,
		TopP:             1,
		FrequencyPenalty: 0,
		PresencePenalty:  0,
	}

	reqJson, err := json.Marshal(gptReq)
	if err != nil {
		fmt.Println(err)
	}

	request, err := http.NewRequest(http.MethodPost, GptEndpoint, bytes.NewBuffer(reqJson))
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	request.Header.Set("Authorization", "Bearer "+GptApiKey)

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	body, _ := ioutil.ReadAll(response.Body)

	var resp GPTResponse
	err = json.Unmarshal(body, &resp)
	if err != nil {
		fmt.Println(err)
	}
	return strings.TrimLeft(resp.Choices[0].Message.Content, "\n")
}
