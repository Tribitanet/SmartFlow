package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	apiKey := os.Getenv("OPEN_ROUTER_API_KEY")

	if apiKey == "" {
		log.Println("API key is required")
	}

	ctx := context.Background()

	url := "https://openrouter.ai/api/v1"

	client := openai.NewClient(
		option.WithBaseURL(url),
		option.WithAPIKey(apiKey),
	)

	jsonData, err := os.ReadFile("topics.json")
	if err != nil {
		log.Fatalf("Ошибка чтения файла тем: %v", err)
	}

	systemPrompt := fmt.Sprintf("Ты — классификатор. Твоя задача — вернуть JSON объект, где ключи — это темы из списка: %s. Значения — true/false. ОТВЕЧАЙ ТОЛЬКО ВАЛИДНЫМ JSON. Никакого markdown, никаких пояснений.", string(jsonData))

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
	}

	model := "meta-llama/llama-3.3-70b-instruct:free"

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
		}

		output := res.Choices[0].Message.Content
		fmt.Print("\n\n")
		fmt.Println(output)

		params.Messages = append(params.Messages, openai.AssistantMessage(output))
	}
}
