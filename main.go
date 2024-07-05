package main

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	checkInterval    = 1 * time.Minute // Check every minute. BTW, the invertor push data to the LP cloud every 2 minutes
)

var (
	telegramBotToken = getenv("TELEGRAM_BOT_TOKEN", "")
	luxpowerAccount  = getenv("LUXPOWER_ACCOUNT", "")
	luxpowerPassword = getenv("LUXPOWER_PASSWORD", "")
	luxpowerStation  = getenv("LUXPOWER_STATION", "")
	luxpowerBaseURL  = getenv("LUXPOWER_BASEURL", "")
)

type LuxpowerResponse struct {
	GridToLoad int `json:"GridToLoad"`
}

type Bot struct {
	bot              *tgbotapi.BotAPI
	currentGridState int
	previousGridState int
	mu               sync.Mutex
	chatIDs          map[int64]bool // Map for Chat IDs
}

func NewBot(token string) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &Bot{
		bot:     bot,
		currentGridState: -1, //Initialize with a value that cannot be the power supply state
		previousGridState: -1,
		chatIDs: make(map[int64]bool),
	}, nil
}

func (b *Bot) Start() {
	b.bot.Debug = true // Bot debug

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.bot.GetUpdatesChan(u)

	// Separate goroutine for processing updates
	go b.handleUpdates(updates)

	// Cycle to periodically check the status of the power supply system
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		<-ticker.C

		gridState, err := b.getCurrentGridState()
		if err != nil {
			log.Println("Error getting current grid state:", err)
			continue
		}

		b.mu.Lock()
		if (b.previousGridState == 0 && gridState != 0) || (b.previousGridState != 0 && gridState == 0) {
			log.Printf("Grid state changed: %d -> %d\n", b.previousGridState, gridState)
			b.currentGridState = gridState

			// Send a status change message to all groups
			message := "Electricity state changed: "
			if gridState == 0 {
				message += "Electricity is absent."
			} else {
				message += "Electricity is present."
			}
			b.sendToAllGroups(message)
		}
		b.previousGridState = gridState
		b.mu.Unlock()
	}
}

func (b *Bot) handleUpdates(updates tgbotapi.UpdatesChannel) {
	for update := range updates {
		if update.Message == nil { // Ignore updates that are not messages
			continue
		}

		if update.Message.Chat != nil {
			chatID := update.Message.Chat.ID
			if !b.chatIDs[chatID] {
				log.Printf("Bot added to new chat: %d\n", chatID)
				b.chatIDs[chatID] = true
			}
		}

		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "status":
				b.handleStatusCommand(update.Message.Chat.ID)
			}
		}
	}
}

func (b *Bot) handleStatusCommand(chatID int64) {
	gridStateStr := "Electricity is present."
	if b.currentGridState == 0 {
		gridStateStr = "Electricity is absent."
	}

	msg := tgbotapi.NewMessage(chatID, "Current status: "+gridStateStr)
	if _, err := b.bot.Send(msg); err != nil {
		log.Println("Error sending message:", err)
	}
}

func (b *Bot) getCurrentGridState() (int, error) {
	cmd := exec.Command("./go-luxpower", "live", "--json",
		"--accountname", luxpowerAccount,
		"--password", luxpowerPassword,
		"--station", luxpowerStation,
		"--baseurl", luxpowerBaseURL)

	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	var response LuxpowerResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return 0, err
	}

	return response.GridToLoad, nil
}

func (b *Bot) sendToAllGroups(message string) {
	for chatID := range b.chatIDs {
		b.sendMessageToGroup(chatID, message)
	}
}

func (b *Bot) sendMessageToGroup(chatID int64, message string) {
	msg := tgbotapi.NewMessage(chatID, message)
	if _, err := b.bot.Send(msg); err != nil {
		log.Println("Error sending message:", err)
	}
}

func getenv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func main() {
	bot, err := NewBot(telegramBotToken)
	if err != nil {
		log.Fatal(err)
	}

	// Run the bot
	bot.Start()
}
