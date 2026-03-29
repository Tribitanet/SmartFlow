package main

import (
	//базовые
	"bufio"
	"fmt"
	"log"
	"os"

	//переменные окружения
	"github.com/joho/godotenv"

	//openAI SDK
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"

	//контекст
	"context"
)

func main() {
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("Read .env error")
	}

	apiKey := os.Getenv("OPEN_ROUTER_API_KEY_STOP_THEMES")

	if apiKey == "" {
		log.Println("API key is required")
	}

	url := "https://openrouter.ai/api/v1"

	ctx := context.Background()

	client := openai.NewClient(
		option.WithBaseURL(url),
		option.WithAPIKey(apiKey),
	)

	jsonData, err := os.ReadFile("stopTopics.json")
	if err != nil {
		log.Fatalf("Ошибка чтения файла стоп-тем: %v", err)
	}

	systemPropmt := fmt.Sprintf("Ты — фильтратор. Твоя задача — вернуть JSON объект, где ключи — это стоп-темы из списка: %s. Значения — true/false. true - проходит эту стоп-тему, false - не проходит. ОТВЕЧАЙ ТОЛЬКО ВАЛИДНЫМ JSON. Никакого markdown, никаких пояснений.", string(jsonData))

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPropmt),
	}

	model := "openai/gpt-oss-120b:free"

	params := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: messages,
	}

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Println("Вставтье однострочный текст новости:")
		fmt.Print("> ")

		if !scanner.Scan() {
			break
		}

		input := scanner.Text()

		if input == "exit" {
			fmt.Print("goodbye")
			break
		}

		if input == "" {
			continue
		}

		if input == "clear" {
			fmt.Print("\033[H\033[2J")
		}

		params.Messages = append(params.Messages, openai.UserMessage(input))

		res, err := client.Chat.Completions.New(ctx, params)
		if err != nil {
			log.Println(err)
			continue
		}

		output := res.Choices[0].Message.Content
		fmt.Println()
		fmt.Println(output)

		params.Messages = append(params.Messages, openai.AssistantMessage(output))

	}

}
