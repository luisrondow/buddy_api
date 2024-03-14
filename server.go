package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	openai "github.com/sashabaranov/go-openai"
)

type Response struct {
	ExpectedQuestions []string   `json:"expected_questions"`
	Pattern           []string `json:"pattern"`
	Text              string   `json:"text"`
}

func buildPossibleAnswersPrompt(possibleAnswers []string) string {
	formattedPossibleAnswers := make([]string, len(possibleAnswers))

	for i, answer := range possibleAnswers {
		formattedPossibleAnswers[i] = fmt.Sprintf("'%s'", answer)
	}

	return fmt.Sprintf("A: {%s}", strings.Join(formattedPossibleAnswers, "or "))
}

func buildPrompt(expectedQuestions string, pattern string) string {
	return fmt.Sprintf("Based on the user response, answer those following questions: %s\n\n Strict follow this pattern in your response, only answers with what is inside the curly brackets: %s", expectedQuestions, pattern)
}

func getOpenAI(apiKey, baseUrl, expectedQuestions, text, pattern string) string {
	clientConfig := openai.DefaultAzureConfig(apiKey, baseUrl)

	client := openai.NewClientWithConfig(clientConfig)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:   openai.ChatMessageRoleSystem,
					Content: buildPrompt(expectedQuestions, pattern),
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: text,
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return "Error"
	}

	return resp.Choices[0].Message.Content
}

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
		return
	}

	apiKey := os.Getenv("API_KEY")
	baseURL := os.Getenv("BASE_URL")

	e := echo.New()

	e.POST("/process", func(c echo.Context) error {
		response := new(Response)

		if err := c.Bind(response); err != nil {
			return err
		}

		result := getOpenAI(apiKey, baseURL, response.ExpectedQuestions[0], response.Text, buildPossibleAnswersPrompt(response.Pattern))

		return c.String(http.StatusCreated, result)
	})

	e.Logger.Fatal(e.Start(":1323"))
}