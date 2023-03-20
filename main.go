package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/fatih/color"
	openai "github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// GPT-4 is in limited beta
const model = openai.GPT3Dot5Turbo

// coloring
var prompt *color.Color
var option *color.Color
var gptResponse *color.Color

// a conversation struct to record previous conversations as context for future message
var conversation []openai.ChatCompletionMessage

func configureCommand(cmd *cobra.Command, args []string) {
	// Get input from the user
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter your OpenAI API key: ")
	apiKey, _ := reader.ReadString('\n')
	apiKey = strings.TrimSpace(apiKey)

	// Write the API key to the config file
	viper.Set("api-key", apiKey)
	if err := viper.WriteConfig(); err != nil {
		fmt.Println("Error writing config file:", err)
		os.Exit(1)
	}
	fmt.Println("API key saved to config file:", viper.ConfigFileUsed())
}

func generateCommand(cmd *cobra.Command, args []string) {
	conversation = make([]openai.ChatCompletionMessage, 0)

	// Initialize the OpenAI API client with the API key from the config file
	client, err := getClient()
	if err != nil {
		fmt.Println("Error initializing OpenAI API client:", err)
		os.Exit(1)
	}

	// Get input from the user
	reader := bufio.NewReader(os.Stdin)
	prompt.Print("Describe your request: ")
	sentence, _ := reader.ReadString('\n')
	sentence = strings.TrimSpace(sentence)

	// Generate a command line script from the sentence using OpenAI's GPT-3 API
	_, script, err := generateScript(client, sentence)

	if err != nil {
		fmt.Println("Error generating script:", err)
		os.Exit(1)
	}
	fmt.Println(script)
	fmt.Println()

	// Present the user with four options: execute, explain, edit, and chat
	// TODO: make this look better
	option.Print("[R] Run - ")
	option.Print("[E] Explain - ")
	option.Print("[C] Copy - ")
	option.Println("[T] Chat with AI")
	prompt.Print("Enter your choice: ")

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	switch choice {
	case "R", "r":
		// Execute the script
		if err := executeScript(script); err != nil {
			fmt.Printf("Error executing script: %s\n", err.Error())
			os.Exit(1)
		}

	case "E", "e":
		// Explain the script
		err := explainScript(client, script)
		if err != nil {
			fmt.Println("Error explaining script:", err)
			os.Exit(1)
		}

	case "C", "c":
		// Edit the script
		copyScript(script)

	case "T", "t":
		// Chat with the AI
		chatWithAI(client)

	default:
		fmt.Println("Invalid choice")
		os.Exit(1)
	}
}

func getClient() (*openai.Client, error) {
	// Get the API key from the config file
	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}
	apiKey := viper.GetString("api-key")

	// Initialize the OpenAI API client with the API key
	client := openai.NewClient(apiKey)
	return client, nil
}

// read gpt response stream and print the stream tokens
func outputStream(stream *openai.ChatCompletionStream) {
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			fmt.Println()
			return
		} else if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if len(resp.Choices) == 0 {
			fmt.Println()
			return
		}
		gptResponse.Print(resp.Choices[0].Delta.Content)
	}
}

func generateScript(client *openai.Client, sentence string) (string, string, error) {
	// Generate a command line script from the sentence using OpenAI's GPT-3 API
	conversation = append(conversation, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "Generate the bash script for sentence with no explanation. sentence: " + sentence + " =>",
	})

	doneChan := make(chan bool)
	bar := getProgressBar()
	go showSpinner(bar, doneChan)

	completion, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:    model,
			Messages: conversation,
		},
	)
	doneChan <- true
	bar.Finish()

	if err != nil {
		return "", "", err
	}
	script := completion.Choices[0].Message.Content

	script = strings.TrimSpace(script)
	script = strings.TrimPrefix(script, "```")
	script = strings.TrimSuffix(script, "```")
	script = strings.TrimSpace(script)

	return completion.ID, script, nil
}

// Generate an explanation of the script
func explainScript(client *openai.Client, script string) error {
	conversation = append(conversation, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "Explain the command",
	})
	doneChan := make(chan bool)
	bar := getProgressBar()
	go showSpinner(bar, doneChan)

	stream, err := client.CreateChatCompletionStream(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:    model,
			Messages: conversation,
		},
	)
	doneChan <- true
	bar.Finish()

	if err != nil {
		return err
	}
	outputStream(stream)
	return nil
}

func copyScript(script string) error {
	err := clipboard.WriteAll(script)
	if err != nil {
		return err
	}
	fmt.Println("Script copied to clipboard!")
	return nil
}

func chatWithAI(client *openai.Client) {
	// Start a chat session with OpenAI's GPT-3 API
	fmt.Println("Enter your message below or type 'quit' to exit the chat:")
	reader := bufio.NewReader(os.Stdin)
	for {
		prompt.Print("> ")
		message, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading message:", err)
			continue
		}
		message = strings.TrimSpace(message)
		if message == "quit" {
			break
		}

		conversation = append(conversation, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: "User: " + message + "\nAI:",
		})

		doneChan := make(chan bool)
		bar := getProgressBar()
		go showSpinner(bar, doneChan)

		stream, err := client.CreateChatCompletionStream(
			context.Background(),
			openai.ChatCompletionRequest{
				Model:    model,
				Messages: conversation,
			},
		)
		doneChan <- true
		bar.Finish()

		if err != nil {
			fmt.Println("Error generating AI response:", err)
			continue
		}
		outputStream(stream)
		fmt.Println()
	}
}

var rootCmd = &cobra.Command{
	Use:   "gptc",
	Short: "GPT-CLI: Supercharge your CLI with ChatGPT",
}

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure the OpenAI API key",
	Run:   configureCommand,
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate script",
	Run:   generateCommand,
}

func main() {
	prompt = color.New(color.FgGreen, color.Bold)
	option = color.New(color.FgYellow, color.Bold)
	gptResponse = color.New(color.Italic)

	// Initialize the CLI tool
	if err := viper.BindPFlags(rootCmd.PersistentFlags()); err != nil {
		fmt.Println("Error binding flags:", err)
		os.Exit(1)
	}
	if err := viper.BindPFlags(generateCmd.Flags()); err != nil {
		fmt.Println("Error binding flags:", err)
		os.Exit(1)
	}
	rootCmd.AddCommand(configureCmd)
	rootCmd.AddCommand(generateCmd)

	// set up config file
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	viper.SetConfigFile(fmt.Sprintf("%s/.gpt_cli.yaml", home))

	// Execute the CLI tool
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("Error executing CLI tool:", err)
		os.Exit(1)
	}
}
