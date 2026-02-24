package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/showwin/speedtest-go/speedtest"
	"go.uber.org/zap"
)

const (
	defaultLockTimeout = 90 * time.Second
)

type Handler struct {
	xrayConfigsDir string
	xrayConfigPath string
	serviceName    string
	logger         *zap.Logger

	mutex       sync.Mutex
	busyUntil   time.Time
	busyAction  string
	lockTimeout time.Duration
}

type commandBusyError struct {
	action    string
	remaining time.Duration
}

func (e commandBusyError) Error() string {
	return fmt.Sprintf("action %q is busy for %s", e.action, e.remaining.Round(time.Second))
}

func NewHandler(xrayConfigsDir, xrayConfigPath, serviceName string, lockTimeout time.Duration, logger *zap.Logger) (*Handler, error) {
	if logger == nil {
		return nil, errors.New("logger is required")
	}

	dirInfo, err := os.Stat(xrayConfigsDir)
	if err != nil {
		return nil, fmt.Errorf("xray configs dir check failed: %w", err)
	}
	if !dirInfo.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", xrayConfigsDir)
	}

	if strings.TrimSpace(xrayConfigPath) == "" {
		return nil, errors.New("xray config path is required")
	}
	if strings.TrimSpace(serviceName) == "" {
		return nil, errors.New("service name is required")
	}
	if lockTimeout <= 0 {
		lockTimeout = defaultLockTimeout
	}

	return &Handler{
		xrayConfigsDir: xrayConfigsDir,
		xrayConfigPath: xrayConfigPath,
		serviceName:    serviceName,
		logger:         logger.Named("handler"),
		lockTimeout:    lockTimeout,
	}, nil
}

var mainMenuKeyboard = &models.InlineKeyboardMarkup{
	InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: "📂 Select Config", CallbackData: "ls_config"}},
		{{Text: "📶 Run Speedtest", CallbackData: "speedtest"}},
		{{Text: "🔄 Restart Xray", CallbackData: "restart"}},
	},
}

func (h *Handler) ListConfigXrayHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	h.handleCallbackCommand(ctx, b, update, "list_configs", func(ctx context.Context, b *bot.Bot, chatID int64, messageID int, update *models.Update) error {
		h.logger.Info("listing config files", zap.String("path", h.xrayConfigsDir))

		dirEntries, err := os.ReadDir(h.xrayConfigsDir)
		if err != nil {
			return fmt.Errorf("read xray configs dir: %w", err)
		}

		if _, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        "📂 Choose a config to activate:",
			ReplyMarkup: buildConfigListKeyboard(dirEntries),
		}); err != nil {
			return fmt.Errorf("edit config list message: %w", err)
		}

		return nil
	})
}

func (h *Handler) RestartXrayHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	h.handleCallbackCommand(ctx, b, update, "restart_service", func(ctx context.Context, b *bot.Bot, chatID int64, messageID int, update *models.Update) error {
		h.logger.Info("restart requested", zap.String("service", h.serviceName))

		if _, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "🔄 Restarting the service, please wait...",
		}); err != nil {
			return fmt.Errorf("set restart progress message: %w", err)
		}

		if err := restartService(h.serviceName); err != nil {
			return err
		}

		if _, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        fmt.Sprintf("✅ Service <code>%s</code> restarted successfully.", h.serviceName),
			ParseMode:   models.ParseModeHTML,
			ReplyMarkup: mainMenuKeyboard,
		}); err != nil {
			return fmt.Errorf("set restart success message: %w", err)
		}

		return nil
	})
}

func (h *Handler) CopyConfigXrayHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	h.handleCallbackCommand(ctx, b, update, "copy_config", func(ctx context.Context, b *bot.Bot, chatID int64, messageID int, update *models.Update) error {
		fileName := strings.TrimPrefix(update.CallbackQuery.Data, "cp_")
		h.logger.Info("copy config requested", zap.String("file", fileName))

		if _, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "🛠 Applying the selected config...",
		}); err != nil {
			return fmt.Errorf("set copy progress message: %w", err)
		}

		if err := copyConfigFile(filepath.Join(h.xrayConfigsDir, fileName), h.xrayConfigPath); err != nil {
			return err
		}

		if _, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        fmt.Sprintf("✅ Config <code>%s</code> was applied to <code>%s</code>.", fileName, h.xrayConfigPath),
			ParseMode:   models.ParseModeHTML,
			ReplyMarkup: mainMenuKeyboard,
		}); err != nil {
			return fmt.Errorf("set copy success message: %w", err)
		}

		return nil
	})
}

func (h *Handler) SpeedtestHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	h.handleCallbackCommand(ctx, b, update, "speedtest", func(ctx context.Context, b *bot.Bot, chatID int64, messageID int, update *models.Update) error {
		h.logger.Info("speedtest requested")

		if _, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "📶 Running speedtest. This can take up to 90 seconds...",
		}); err != nil {
			return fmt.Errorf("set speedtest progress message: %w", err)
		}

		result, err := runSpeedTest(ctx, h.lockTimeout)
		if err != nil {
			return err
		}

		if _, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        formatSpeedTestMessage(result),
			ParseMode:   models.ParseModeHTML,
			ReplyMarkup: mainMenuKeyboard,
		}); err != nil {
			return fmt.Errorf("set speedtest result message: %w", err)
		}

		return nil
	})
}

func (h *Handler) DefaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	release, err := h.acquireCommandLock("main_menu")
	if err != nil {
		h.sendBusyMessage(ctx, b, update, err)
		return
	}
	defer release()

	chatID, ok := getMessageChatID(update)
	if !ok {
		h.logger.Warn("default handler update missing message")
		return
	}

	h.logger.Info("open main menu", zap.Int64("chat_id", chatID))
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        "👋 Choose an action:\n• apply config\n• check speed\n• restart Xray",
		ReplyMarkup: mainMenuKeyboard,
	}); err != nil {
		h.logger.Error("send main menu failed", zap.Error(err), zap.Int64("chat_id", chatID))
	}
}

func (h *Handler) MainHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	h.handleCallbackCommand(ctx, b, update, "main_menu", func(ctx context.Context, b *bot.Bot, chatID int64, messageID int, update *models.Update) error {
		if _, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        "🏠 Main menu. Choose an action:",
			ReplyMarkup: mainMenuKeyboard,
		}); err != nil {
			return fmt.Errorf("set main menu message: %w", err)
		}

		return nil
	})
}

func (h *Handler) handleCallbackCommand(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
	action string,
	run func(context.Context, *bot.Bot, int64, int, *models.Update) error,
) {
	callback := update.CallbackQuery
	if callback == nil ||
		callback.Message.Type != models.MaybeInaccessibleMessageTypeMessage ||
		callback.Message.Message == nil {
		h.logger.Warn("callback update is incomplete", zap.String("action", action))
		return
	}

	release, err := h.acquireCommandLock(action)
	if err != nil {
		h.sendBusyMessage(ctx, b, update, err)
		return
	}
	defer release()

	chatID := callback.Message.Message.Chat.ID
	messageID := callback.Message.Message.ID

	if _, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
		ShowAlert:       false,
	}); err != nil {
		h.logger.Warn("answer callback failed", zap.Error(err), zap.String("action", action))
	}

	h.logger.Info("handling callback", zap.String("action", action), zap.Int64("chat_id", chatID), zap.String("data", callback.Data))
	if err := run(ctx, b, chatID, messageID, update); err != nil {
		h.logger.Error("callback handler failed", zap.String("action", action), zap.Error(err), zap.Int64("chat_id", chatID))
		h.sendHandlerError(ctx, b, chatID, messageID)
		return
	}

	h.logger.Info("callback handled", zap.String("action", action), zap.Int64("chat_id", chatID))
}

func (h *Handler) acquireCommandLock(action string) (func(), error) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	now := time.Now()
	if now.Before(h.busyUntil) {
		return nil, commandBusyError{
			action:    h.busyAction,
			remaining: time.Until(h.busyUntil),
		}
	}

	h.busyAction = action
	h.busyUntil = now.Add(h.lockTimeout)

	release := func() {
		h.mutex.Lock()
		defer h.mutex.Unlock()
		h.busyUntil = time.Time{}
		h.busyAction = ""
	}

	return release, nil
}

func (h *Handler) sendBusyMessage(ctx context.Context, b *bot.Bot, update *models.Update, err error) {
	busyErr, ok := err.(commandBusyError)
	if !ok {
		h.logger.Error("unexpected lock error", zap.Error(err))
		return
	}

	messageText := fmt.Sprintf("⏳ Command already in progress: %s. Try again in %s.", busyErr.action, roundDurationToSeconds(busyErr.remaining))

	if update.CallbackQuery != nil {
		if _, callbackErr := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			ShowAlert:       true,
			Text:            messageText,
		}); callbackErr != nil {
			h.logger.Warn("send busy callback alert failed", zap.Error(callbackErr))
		}
		return
	}

	chatID, ok := getMessageChatID(update)
	if !ok {
		return
	}

	if _, sendErr := b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: messageText}); sendErr != nil {
		h.logger.Warn("send busy message failed", zap.Error(sendErr), zap.Int64("chat_id", chatID))
	}
}

func (h *Handler) sendHandlerError(ctx context.Context, b *bot.Bot, chatID int64, messageID int) {
	if _, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        "⚠️ Something went wrong. Please try again.",
		ReplyMarkup: mainMenuKeyboard,
	}); err != nil {
		h.logger.Error("failed to send user-facing error message", zap.Error(err), zap.Int64("chat_id", chatID))
	}
}

func buildConfigListKeyboard(entries []os.DirEntry) *models.InlineKeyboardMarkup {
	buttons := make([][]models.InlineKeyboardButton, 0, len(entries)+1)
	buttons = append(buttons, []models.InlineKeyboardButton{{Text: "⬅️ Back to Main Menu", CallbackData: "main"}})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if entry.Name() == "config.json" {
			continue
		}

		buttons = append(buttons, []models.InlineKeyboardButton{{
			Text:         shortenFileName(entry.Name()),
			CallbackData: makeCopyFileCallbackData(entry.Name()),
		}})
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

func shortenFileName(fileName string) string {
	fileName = strings.TrimSpace(fileName)
	if len(fileName) <= 24 {
		return fileName
	}
	return fileName[:12] + "..." + fileName[len(fileName)-9:]
}

func makeCopyFileCallbackData(fileName string) string {
	return "cp_" + fileName
}

func copyConfigFile(sourcePath, destinationPath string) error {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("source file check failed: %w", err)
	}
	if sourceInfo.IsDir() {
		return fmt.Errorf("source path is a directory: %s", sourcePath)
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer func() {
		_ = sourceFile.Close()
	}()

	tempPath := destinationPath + ".tmp"
	destinationFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("create destination temp file: %w", err)
	}

	if _, err := io.Copy(destinationFile, sourceFile); err != nil {
		_ = destinationFile.Close()
		return fmt.Errorf("copy config file: %w", err)
	}

	if err := destinationFile.Close(); err != nil {
		return fmt.Errorf("close destination temp file: %w", err)
	}

	if err := os.Rename(tempPath, destinationPath); err != nil {
		return fmt.Errorf("replace destination file: %w", err)
	}

	return nil
}

func restartService(serviceName string) error {
	cmd := exec.Command("systemctl", "restart", serviceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restart service %s: %w", serviceName, err)
	}
	return nil
}

type speedTestResult struct {
	Host       string
	ServerName string
	Sponsor    string
	Country    string
	Latency    time.Duration
	Jitter     time.Duration
	PacketLoss string
	Download   string
	Upload     string
}

func runSpeedTest(ctx context.Context, timeout time.Duration) (speedTestResult, error) {
	var result speedTestResult

	testCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := speedtest.New()
	serverList, err := client.FetchServers()
	if err != nil {
		return result, fmt.Errorf("fetch speedtest servers: %w", err)
	}

	targets, err := serverList.FindServer([]int{})
	if err != nil {
		return result, fmt.Errorf("find speedtest target: %w", err)
	}
	if len(targets) == 0 {
		return result, errors.New("no speedtest server available")
	}

	target := targets[0]
	if err := target.PingTestContext(testCtx, nil); err != nil {
		return result, fmt.Errorf("ping test failed: %w", err)
	}
	if err := target.DownloadTestContext(testCtx); err != nil {
		return result, fmt.Errorf("download test failed: %w", err)
	}
	if err := target.UploadTestContext(testCtx); err != nil {
		return result, fmt.Errorf("upload test failed: %w", err)
	}

	result = speedTestResult{
		Host:       target.Host,
		ServerName: target.Name,
		Sponsor:    target.Sponsor,
		Country:    target.Country,
		Latency:    target.Latency,
		Jitter:     target.Jitter,
		PacketLoss: target.PacketLoss.String(),
		Download:   target.DLSpeed.String(),
		Upload:     target.ULSpeed.String(),
	}

	return result, nil
}

func formatSpeedTestMessage(r speedTestResult) string {
	return fmt.Sprintf(
		"<b>📶 Speedtest Result</b>\n\n<b>🛰 Server:</b> %s (%s, %s)\n<b>🌐 Host:</b> <code>%s</code>\n<b>⏱ Latency:</b> %s\n<b>📉 Jitter:</b> %s\n<b>📦 Packet loss:</b> %s\n<b>⬇️ Download:</b> %s\n<b>⬆️ Upload:</b> %s",
		r.ServerName,
		r.Sponsor,
		r.Country,
		r.Host,
		r.Latency.Round(time.Millisecond),
		r.Jitter.Round(time.Millisecond),
		r.PacketLoss,
		r.Download,
		r.Upload,
	)
}

func roundDurationToSeconds(duration time.Duration) time.Duration {
	if duration < 0 {
		return 0
	}
	return duration.Round(time.Second)
}

func getMessageChatID(update *models.Update) (int64, bool) {
	if update == nil || update.Message == nil {
		return 0, false
	}
	return update.Message.Chat.ID, true
}
