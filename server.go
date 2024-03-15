package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	firebase "firebase.google.com/go"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	openai "github.com/sashabaranov/go-openai"
	"google.golang.org/api/option"
)

const (
	DefaultPrompt       = "You are an assistant helping an user enroll in a Digital Physical therapy program."
	DefaultAskMorePrompt = "At least, if there is any inconclusive answer, ask the user for this inconclusive information."
)

type NeedAnswer struct {
	Key  string `json:"key"`
	Value string `json:"value"`
	PossibleValues []string `json:"possible_values"`
}

type Response struct {
	NeededAnswers 	[]NeedAnswer `json:"needed_answers"`
	Text              string   `json:"text"`
}

type SuggestedAnswer struct {
	Key   string `json:"key"`
	Value any `json:"value"`
}


func csvStringToObject(csvString string) []any {
	fmt.Println(csvString)
    reader := csv.NewReader(strings.NewReader(csvString))
    reader.FieldsPerRecord = -1 // see the Reader struct information below
    csvData, err := reader.ReadAll()
    if err != nil {
		fmt.Println(err)
        return nil
    }

    ans := make([]any, (len(csvData) / 2) - 1)

    for i, each := range csvData {
		if i == 0 {
			continue
		}

		ans = append(ans, each[1])
    }

    return ans
}

func joinValue(a []NeedAnswer, s string) string {
	if len(s) == 1 {
		return a[0].Value
	}


	var sb strings.Builder
	for i, v := range a {
		if i > 0 {
			sb.WriteString(s)
		}
		sb.WriteString(v.Value)
	}
	return sb.String()
}

func getPattern(s []NeedAnswer) []string {
	var pattern []string
	for _, v := range s {
		if len(v.PossibleValues) > 0 {
			pattern = append(pattern, strings.Join(v.PossibleValues, " OR "))
		}
	}
	return pattern
}

func processQuestionPrompt(expectedQuestions, text string, pattern []string) []openai.ChatCompletionMessage {
	var sb strings.Builder

	prompt := fmt.Sprintf("%s You will be provided with the user's response and you will have extract the answer for the following questions:\n\n'%s'\n\nFor the response, create a two column CSV with question name and answer.:", DefaultPrompt, expectedQuestions)

	if len(pattern) > 0 {
		for i, p := range pattern {
			sb.WriteString(fmt.Sprintf("\nFor the %d question, answer with one of these answer options: %s\n", i+1, p))
		}

		prompt = prompt + sb.String()
	}

	return []openai.ChatCompletionMessage{
		{
			Role:   openai.ChatMessageRoleSystem,
			Content: prompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: text,
		},
	}
}

func getOpenAI(client *openai.Client, messages []openai.ChatCompletionMessage) string {
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: messages,
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
		log.Fatal("Error loading .env file")
	}

	apiKey := os.Getenv("API_KEY")
	baseUrl := os.Getenv("BASE_URL")

	e := echo.New()

	ctx := context.Background()
	sa := option.WithCredentialsFile("./sword-buddy.json")
	app, err := firebase.NewApp(ctx, nil, sa)
	if err != nil {
		log.Fatal(err)
	}

	client, err := app.Firestore(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	e.GET("/health-check", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok!")
	})

	e.POST("/process/:enrollment_id", func(c echo.Context) error {
		response := new(Response)

		if err := c.Bind(response); err != nil {
			return err
		}

		enrollmentID := c.Param("enrollment_id")

		clientConfig := openai.DefaultAzureConfig(apiKey, baseUrl)

		openaiClient := openai.NewClientWithConfig(clientConfig)

		result := getOpenAI(openaiClient, processQuestionPrompt(joinValue(response.NeededAnswers, "\n"), response.Text, getPattern(response.NeededAnswers)))

		cvH := ConversationHistory{
			EnrollmentID: enrollmentID,
			History: []History{
				{
					CreatedAt: time.Now(),
					GptOutput: result,
					UserInput: response.Text,
				},
			},
		}

		saveHistory(ctx, client, cvH)

		gptParsedAnswers := csvStringToObject(result)

		fmt.Printf("GPT Parsed Answers: %v\n", gptParsedAnswers)

		suggestedAnswers := make([]SuggestedAnswer, len(response.NeededAnswers))

		for i, v := range response.NeededAnswers {
			suggestedAnswers[i] = SuggestedAnswer{
				Key:   v.Key,
				Value: gptParsedAnswers[i],
			}
		}

		rsp := struct {
			SuggestedAnswers []SuggestedAnswer `json:"suggested_answers"`
			Text 		   string            `json:"text"`
		}{
			SuggestedAnswers: suggestedAnswers,
			Text:             "Perfect, it's all for now. Can we go to the next step?",
		}

		return c.JSON(http.StatusOK, rsp)
	})

	e.Logger.Fatal(e.Start(":8080"))
}