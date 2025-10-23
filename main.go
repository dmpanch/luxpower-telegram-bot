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
	checkInterval = 1 * time.Minute // Check every minute. BTW, the inverter pushes data to the LP cloud every 2 minutes
	recheckDelay  = 1 * time.Minute // Delay before rechecking after state change
)

var (
	telegramBotToken = getenv("TELEGRAM_BOT_TOKEN", "")
	luxpowerAccount  = getenv("LUXPOWER_ACCOUNT", "")
	luxpowerPassword = getenv("LUXPOWER_PASSWORD", "")
	luxpowerStation  = getenv("LUXPOWER_STATION", "")
	luxpowerBaseURL  = getenv("LUXPOWER_BASEURL", "")
	luxpowerProxyURL = getenv("LUXPOWER_PROXY_URL", "")
	subscriberDBPath = getenv("SUBSCRIBERS_DB_PATH", "subscribers.db")
)

type LuxpowerResponse struct {
	GridToLoad int `json:"GridToLoad"`
}

type subscriberStore interface {
	List() ([]int64, error)
	Save(int64) error
}

type Bot struct {
	bot               *tgbotapi.BotAPI
	store             subscriberStore
	currentGridState  int
	previousGridState int
	mu                sync.Mutex
	chatIDs           map[int64]bool // Map for Chat IDs
	recheckScheduled  bool           // Flag to avoid multiple rechecks
	sendMessage       func(int64, string) error
	fetchGridState    func() (int, error)
}

func NewBot(token string, store subscriberStore) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	newBot := &Bot{
		bot:               bot,
		store:             store,
		currentGridState:  -1, // Initialize with a value that cannot be the power supply state
		previousGridState: -1,
		chatIDs:           make(map[int64]bool),
		recheckScheduled:  false,
	}

	newBot.sendMessage = func(chatID int64, message string) error {
		msg := tgbotapi.NewMessage(chatID, message)
		_, err := bot.Send(msg)
		return err
	}
	newBot.fetchGridState = newBot.getCurrentGridState

	if err := newBot.loadSubscribers(); err != nil {
		return nil, err
	}

	return newBot, nil
}

func (b *Bot) Start() {
	b.bot.Debug = true // Bot debug

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.bot.GetUpdatesChan(u)

	// Separate goroutine for processing updates
	go b.handleUpdates(updates)

	// Broadcast current state to existing subscribers at startup
	b.broadcastInitialStatus()

	// Cycle to periodically check the status of the power supply system
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		<-ticker.C

		gridState, err := b.fetchGridState()
		if err != nil {
			log.Println("Error getting current grid state:", err)
			continue
		}

		b.mu.Lock()
		if gridState == 0 && b.previousGridState != 0 {
			log.Printf("Grid state changed: %d -> %d\n", b.previousGridState, gridState)

			// Set current state
			b.currentGridState = gridState

			// Schedule recheck after recheckDelay if not already scheduled
			if !b.recheckScheduled {
				b.recheckScheduled = true
				time.AfterFunc(recheckDelay, func() {
					currentState, err := b.fetchGridState()
					if err != nil {
						log.Println("Error re-checking current grid state:", err)
						b.mu.Lock()
						b.recheckScheduled = false
						b.mu.Unlock()
						return
					}

					b.mu.Lock()
					if currentState == 0 {
						log.Println("Grid state is still 0 after recheck, sending notification.")
						b.sendToAllGroups("Стан змінився: світла немає.")
						b.previousGridState = currentState
					} else {
						log.Println("Grid state changed during recheck: 0 ->", currentState)
						b.currentGridState = currentState
						b.previousGridState = currentState
					}

					b.recheckScheduled = false // Reset recheck flag
					b.mu.Unlock()
				})
			}
		} else if gridState != 0 && b.previousGridState == 0 {
			log.Printf("Grid state changed: %d -> %d\n", b.previousGridState, gridState)
			b.currentGridState = gridState
			b.sendToAllGroups("Стан змінився: світло є.")
			b.previousGridState = gridState
		}
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
			b.registerChatID(chatID)
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
	gridStateStr := b.statusMessage()
	if err := b.sendMessageToGroup(chatID, gridStateStr); err != nil {
		log.Println("Error sending message:", err)
	}
}

func (b *Bot) getCurrentGridState() (int, error) {
	cmd := buildLuxpowerCommand()
	output, err := cmd.Output()
	if err != nil {
		return -1, err // Return -1 to indicate an error
	}

	var response LuxpowerResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return -1, err // Return -1 to indicate an error
	}

	return response.GridToLoad, nil
}

func buildLuxpowerCommand() *exec.Cmd {
	cmd := exec.Command("./go-luxpower", "live", "--json",
		"--accountname", luxpowerAccount,
		"--password", luxpowerPassword,
		"--station", luxpowerStation,
		"--baseurl", luxpowerBaseURL)

	if luxpowerProxyURL != "" {
		cmd.Env = append(os.Environ(),
			"HTTP_PROXY="+luxpowerProxyURL,
			"HTTPS_PROXY="+luxpowerProxyURL,
		)
	}

	return cmd
}

func (b *Bot) sendToAllGroups(message string) {
	chatIDs := b.snapshotChatIDs()
	for _, chatID := range chatIDs {
		if err := b.sendMessageToGroup(chatID, message); err != nil {
			log.Println("Error sending message:", err)
		}
	}
}

func (b *Bot) sendMessageToGroup(chatID int64, message string) error {
	if b.sendMessage == nil {
		return nil
	}
	return b.sendMessage(chatID, message)
}

func (b *Bot) snapshotChatIDs() []int64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.chatIDs) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(b.chatIDs))
	for chatID := range b.chatIDs {
		ids = append(ids, chatID)
	}
	return ids
}

func (b *Bot) loadSubscribers() error {
	if b.store == nil {
		return nil
	}

	ids, err := b.store.List()
	if err != nil {
		return err
	}

	b.mu.Lock()
	for _, id := range ids {
		b.chatIDs[id] = true
	}
	b.mu.Unlock()
	return nil
}

func (b *Bot) registerChatID(chatID int64) {
	b.mu.Lock()
	if b.chatIDs[chatID] {
		b.mu.Unlock()
		return
	}
	if b.store != nil {
		if err := b.store.Save(chatID); err != nil {
			b.mu.Unlock()
			log.Printf("Error saving chat ID %d: %v", chatID, err)
			return
		}
	}
	b.chatIDs[chatID] = true
	b.mu.Unlock()
	log.Printf("Bot added to new chat: %d\n", chatID)
}

func (b *Bot) broadcastInitialStatus() {
	gridState, err := b.fetchGridState()
	if err != nil {
		log.Println("Error getting current grid state:", err)
		return
	}

	b.mu.Lock()
	b.currentGridState = gridState
	b.previousGridState = gridState
	hasSubscribers := len(b.chatIDs) > 0
	b.mu.Unlock()

	if hasSubscribers {
		b.sendToAllGroups(b.statusMessageForState(gridState))
	}
}

func (b *Bot) statusMessage() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.statusMessageForState(b.currentGridState)
}

func (b *Bot) statusMessageForState(state int) string {
	if state == 0 {
		return "Світла немає."
	}
	return "Світло є."
}

func getenv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func main() {
	store, err := NewSubscriberStore(subscriberDBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	bot, err := NewBot(telegramBotToken, store)
	if err != nil {
		log.Fatal(err)
	}

	// Run the bot
	bot.Start()
}
