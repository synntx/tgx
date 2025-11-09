package tgx

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/harshyadavone/tgx/models"
	"github.com/harshyadavone/tgx/pkg/logger"
)

const (
	ParseModeMarkdown = "MarkdownV2"
	ParseModeHTML     = "HTML"
)

type (
	ErrorHandler    func(ctx *Context, err error)
	Handler         func(ctx *Context) error
	callbackHandler func(ctx *CallbackContext) error
)

type Bot struct {
	token      string
	webhookURL string

	messageHandlers  map[string]Handler
	commandHandler   map[string]Handler
	callbackHandlers map[string]callbackHandler
	errorHandler     ErrorHandler

	logger logger.Logger
}

type WebhookInfo struct {
	URL                          string   `json:"url"`
	HasCustomCertificate         bool     `json:"has_custom_certificate"`
	PendingUpdateCount           int      `json:"pending_update_count"`
	IPAddress                    string   `json:"ip_address,omitempty"`
	LastErrorDate                int64    `json:"last_error_date,omitempty"`
	LastErrorMessage             string   `json:"last_error_message,omitempty"`
	LastSynchronizationErrorDate int64    `json:"last_synchronization_error_date,omitempty"`
	MaxConnections               int      `json:"max_connections,omitempty"`
	AllowedUpdates               []string `json:"allowed_updates,omitempty"`
}

func NewBot(token, webhookURL string, logger logger.Logger) *Bot {
	return &Bot{
		token:            token,
		webhookURL:       webhookURL,
		messageHandlers:  make(map[string]Handler),
		commandHandler:   make(map[string]Handler),
		callbackHandlers: make(map[string]callbackHandler),
		logger:           logger,
		errorHandler:     defaultErrorHandler,
	}
}

func defaultErrorHandler(ctx *Context, err error) {
	ctx.bot.logger.Error("Bot error:", err)
	payload := &SendMessageRequest{
		ChatId: ctx.ChatID,
		Text:   "Sorry, something went wrong. Please try again later.",
	}
	ctx.ReplyWithOpts(payload)
}

func (b *Bot) SetWebhook() error {
	return makeAPIRequest(b.token, "setWebhook", map[string]interface{}{
		"url": b.webhookURL,
	})
}

func (b *Bot) DeleteWebhook() error {
	return makeAPIRequest(b.token, "deleteWebhook", map[string]interface{}{})
}

func (b *Bot) GetWebhookInfo() (*WebhookInfo, error) {
	result, err := makeAPIRequestWithResult(b.token, "getWebhookInfo", nil)
	if err != nil {
		return nil, err
	}

	var webhookInfo WebhookInfo
	if err := json.Unmarshal(result, &webhookInfo); err != nil {
		return nil, &BotError{
			Code:    http.StatusBadRequest,
			Message: "failed to decode webhook info",
			Err:     err,
		}
	}
	return &webhookInfo, nil
}

func (b *Bot) GetMe() (*models.User, error) {
	result, err := makeAPIRequestWithResult(b.token, "getMe", nil)
	if err != nil {
		return nil, err
	}

	var user models.User
	if err := json.Unmarshal(result, &user); err != nil {
		return nil, &BotError{
			Code:    http.StatusBadRequest,
			Message: "failed to decode webhook info",
			Err:     err,
		}
	}
	return &user, nil
}

func (b *Bot) logOut() error {
	return makeAPIRequest(b.token, "getMe", nil)
}

func (b *Bot) close() error {
	return makeAPIRequest(b.token, "close", nil)
}

func (b *Bot) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		b.logger.Error("Invalid HTTP method: %s", r.Method)
		http.Error(w, "Only POST requests are allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		b.logger.Error("Code : %d,\nMessage: %s,\nErr: %v", http.StatusBadRequest, "Failed to read request body", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var update models.Update
	if err := json.Unmarshal(body, &update); err != nil {
		b.logger.Error("Code : %d,\nMessage: %s,\nErr: %v", http.StatusBadRequest, "Failed to decode body", err)
		http.Error(w, "Failed to decode update", http.StatusBadRequest)
		return
	}

	defer func() {
		if r := recover(); r != nil {
			b.logger.Error("Panic recovered in update handler: %v", r)
		}
	}()

	if update.Message != nil {
		if err := b.handleMessageUpdate(update.Message); err != nil {
			b.logger.Error("Error handling message update: %v", err)
		}
	} else if update.CallbackQuery != nil {
		if err := b.handleCallbackQuery(update.CallbackQuery); err != nil {
			b.logger.Error("Error handling callback query: %v", err)
		}
	} else {
		b.logger.Warn("Received update with no message or callback query")
	}

	w.WriteHeader(http.StatusOK)
}

func (b *Bot) handleMessageUpdate(message *models.Message) error {
	if message == nil {
		return &BotError{
			Code:    http.StatusBadRequest,
			Message: "Empty message received",
		}
	}

	defer func() {
		if r := recover(); r != nil {
			b.logger.Error("Panic recovered in handleMessageUpdate: %v", r)
		}
	}()

	ctx := &Context{
		Text:      message.Text,
		UserID:    message.From.Id,
		Username:  message.From.Username,
		MessageId: message.MessageId,
		ChatID:    message.Chat.Id,
		bot:       b,
	}

	if strings.HasPrefix(message.Text, "/") {

		parts := strings.Split(message.Text, " ")
		if len(parts) < 1 {
			b.logger.Error("Not a valid command")
			return &BotError{
				Code:    http.StatusInternalServerError,
				Message: "Failed to parse command",
				Err:     fmt.Errorf("Failed to parse command"),
			}
		}

		command := strings.Split(parts[0], "/")
		if len(command) <= 1 {
			b.logger.Error("Not a valid command")
			return &BotError{
				Code:    http.StatusInternalServerError,
				Message: "Not a valid command",
				Err:     fmt.Errorf("Failed to split parts"),
			}
		}

		if len(parts) > 1 {
			args := parts[1:]
			ctx.Args = args
		}

		b.logger.Debug("Received message: %s", message.Text)
		b.logger.Debug("Parsed command: %s", command[1])
		if len(ctx.Args) > 0 {
			b.logger.Debug("Arguments: [%s]", strings.Join(ctx.Args, ", "))
		}

		if handler, ok := b.commandHandler[command[1]]; ok {
			b.logger.Info("Executing command: %s", command[1])

			return b.safeExecute(ctx, handler)
		} else {
			return &BotError{
				Code:    http.StatusNotFound,
				Message: "Unknown command",
				Err:     fmt.Errorf("command '%s' not found", command[1]),
			}
		}

	}
	switch {
	case message.Text != "":
		if handler, ok := b.messageHandlers["Text"]; ok {
			return b.safeExecute(ctx, handler)
		}
	case message.Photo != nil:
		if handler, ok := b.messageHandlers["Photo"]; ok {
			ctx.Photo = message.Photo
			return b.safeExecute(ctx, handler)
		}
	case message.Video != nil:
		if handler, ok := b.messageHandlers["Video"]; ok {
			ctx.Video = message.Video
			return b.safeExecute(ctx, handler)
		}
	case message.Voice != nil:
		if handler, ok := b.messageHandlers["Voice"]; ok {
			ctx.Voice = message.Voice
			return b.safeExecute(ctx, handler)
		}
	case message.Document != nil:
		if handler, ok := b.messageHandlers["Document"]; ok {
			ctx.Document = message.Document
			return b.safeExecute(ctx, handler)
		}
	case message.Animation != nil:
		if handler, ok := b.messageHandlers["Animation"]; ok {
			ctx.Animation = message.Animation
			return b.safeExecute(ctx, handler)
		}
	case message.Sticker != nil:
		if handler, ok := b.messageHandlers["Sticker"]; ok {
			ctx.Sticker = message.Sticker
			return b.safeExecute(ctx, handler)
		}
	case message.Audio != nil:
		if handler, ok := b.messageHandlers["Audio"]; ok {
			ctx.Audio = message.Audio
			return b.safeExecute(ctx, handler)
		}
	case message.VideoNote != nil:
		if handler, ok := b.messageHandlers["VideoNote"]; ok {
			ctx.VideoNote = message.VideoNote
			return b.safeExecute(ctx, handler)
		}
	default:
		return &BotError{
			Code:    http.StatusBadRequest,
			Message: "Unsupported message type received. No handler is available for this message type.",
			Err:     fmt.Errorf("no handler found for message type: %v", message),
		}
	}

	return nil
}

func (b *Bot) safeExecute(ctx *Context, handler Handler) error {
	defer func() {
		if r := recover(); r != nil {
			b.logger.Error("Panic in handler execution:", r)
			err := fmt.Errorf("handler panic: %v", r)
			if b.errorHandler != nil {
				b.errorHandler(ctx, err)
			}
		}
	}()
	err := handler(ctx)
	if err != nil {
		switch {
		case IsAPIError(err, 403):
			b.logger.Warn("Bot blocked by user: %d", ctx.UserID)
			return err
		case IsAPIError(err, 429):
			b.logger.Info("Rate limited")
			return err
		default:
			if b.errorHandler != nil {
				b.errorHandler(ctx, err)
			}
			return err
		}
	}
	return nil
}

// SendMessage

func (b *Bot) SendMessage(chatID int64, text string) error {
	return makeAPIRequest(b.token, "sendMessage", map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	})
}

func (b *Bot) SendMessageWithOpts(req *SendMessageRequest) error {
	payload := map[string]interface{}{
		"chat_id": req.ChatId,
		"text":    req.Text,
	}

	if req.ParseMode != "" {
		if req.ParseMode != ParseModeHTML && req.ParseMode != ParseModeMarkdown {
			return &BotError{
				Code:    http.StatusBadRequest,
				Message: "Parse mode can be only 'MarkdownV2' or 'HTML'",
				Err:     fmt.Errorf("Parse mode can be only 'MarkdownV2' or 'HTML'"),
			}
		}
		payload["parse_mode"] = req.ParseMode
	}

	if req.ReplyMarkup != nil {
		payload["reply_markup"] = req.ReplyMarkup
	}

	if req.ReplyParams != nil && req.ReplyParams.MessageId != 0 {
		replyParam := map[string]interface{}{
			"message_id": req.ReplyParams.MessageId,
		}
		if req.ReplyParams.ChatId != 0 {
			replyParam["chat_id"] = req.ReplyParams.ChatId
		}
		payload["reply_parameters"] = replyParam
	}

	return makeAPIRequest(b.token, "sendMessage", payload)
}

// Forward Message

func (b *Bot) ForwardMessage(chatId, fromChatId, messageId int64) error {
	return makeAPIRequest(b.token, "forwardMessage", map[string]interface{}{
		"chat_id":      chatId,
		"from_chat_id": fromChatId,
		"message_id":   messageId,
	})
}

func (b *Bot) ForwardMessageWithOpts(req *ForwardMessageRequest) error {
	payload := map[string]interface{}{
		"chat_id":      req.ChatId,
		"from_chat_id": req.FromChatId,
		"message_id":   req.MessageId,
	}

	if req.DisableNotification {
		payload["disable_notification"] = req.DisableNotification
	}

	if req.ProtectContent {
		payload["protect_content"] = req.ProtectContent
	}

	return makeAPIRequest(b.token, "forwardMessage", payload)
}

// ForwardMessages

func (b *Bot) ForwardMessages(chatId, fromChatId int64, messageId []int64) error {
	return makeAPIRequest(b.token, "forwardMessages", map[string]interface{}{
		"chat_id":      chatId,
		"from_chat_id": fromChatId,
		"message_id":   messageId,
	})
}

func (b *Bot) ForwardMessagesWithOpts(req *ForwardMessagesRequest) error {
	payload := map[string]interface{}{
		"chat_id":      req.ChatId,
		"from_chat_id": req.FromChatId,
		"message_id":   req.MessageIds,
	}

	if req.DisableNotification {
		payload["disable_notification"] = req.DisableNotification
	}

	if req.ProtectContent {
		payload["protect_content"] = req.ProtectContent
	}

	return makeAPIRequest(b.token, "forwardMessages", payload)
}

// CopyMessage

func (b *Bot) CopyMessage(chatId, fromChatId, messageId int64) error {
	return makeAPIRequest(b.token, "copyMessage", map[string]interface{}{
		"chat_id":      chatId,
		"from_chat_id": fromChatId,
		"message_id":   messageId,
	})
}

func (b *Bot) CopyMessageWithOpts(req *CopyMessageRequest) error {
	payload := map[string]interface{}{
		"chat_id":      req.ChatId,
		"from_chat_id": req.FromChatId,
		"message_id":   req.MessageId,
	}

	if req.Caption != "" {
		payload["caption"] = req.Caption
	}

	if req.ParseMode != "" {
		if req.ParseMode != ParseModeHTML && req.ParseMode != ParseModeMarkdown {
			return &BotError{
				Code:    http.StatusBadRequest,
				Message: "Parse mode can be only 'MarkdownV2' or 'HTML'",
				Err:     fmt.Errorf("Parse mode can be only 'MarkdownV2' or 'HTML'"),
			}
		}
		payload["parse_mode"] = req.ParseMode
	}

	if req.ShowCaptionAboveMedia {
		payload["show_caption_above_media"] = req.ShowCaptionAboveMedia
	}

	if req.AllowPaidBroadCast {
		payload["allow_paid_broadcast"] = req.AllowPaidBroadCast
	}

	if req.DisableNotification {
		payload["disable_notification"] = req.DisableNotification
	}

	if req.ProtectContent {
		payload["protect_content"] = req.ProtectContent
	}

	if req.ReplyMarkup != nil {
		payload["reply_markup"] = req.ReplyMarkup
	}

	if req.ReplyParams != nil && req.ReplyParams.MessageId != 0 {
		replyParam := map[string]interface{}{
			"message_id": req.ReplyParams.MessageId,
		}
		if req.ReplyParams.ChatId != 0 {
			replyParam["chat_id"] = req.ReplyParams.ChatId
		}
		payload["reply_parameters"] = replyParam
	}

	return makeAPIRequest(b.token, "copyMessage", payload)
}

// CopyMessages

func (b *Bot) CopyMessages(chatId, fromChatId int64, messageId []int64) error {
	return makeAPIRequest(b.token, "copyMessages", map[string]interface{}{
		"chat_id":      chatId,
		"from_chat_id": fromChatId,
		"message_id":   messageId,
	})
}

func (b *Bot) CopyMessagesWithOpts(req *CopyMessagesRequest) error {
	payload := map[string]interface{}{
		"chat_id":      req.ChatId,
		"from_chat_id": req.FromChatId,
		"message_id":   req.MessageIds,
	}

	if req.DisableNotification {
		payload["disable_notification"] = req.DisableNotification
	}

	if req.ProtectContent {
		payload["protect_content"] = req.ProtectContent
	}

	if req.RemoveCaption {
		payload["remove_caption"] = req.RemoveCaption
	}

	return makeAPIRequest(b.token, "copyMessages", payload)
}

// file_id or url
func (b *Bot) SendPhoto(req *SendPhotoRequest) error {
	builder := NewParamBuilder().
		Add("chat_id", req.ChatId).
		Add("caption", req.Caption).
		Add("photo", req.Photo).
		Add("disable_notification", req.DisableNotification).
		Add("show_caption_above_media", req.ShowCaptionAboveMedia).
		Add("has_spoiler", req.HasSpoiler)

	if req.ReplyParams != nil {
		builder.Add("reply_to_message_id", req.ReplyParams.MessageId)
	}
	if req.ReplyMarkup != nil {
		replyBytes, _ := json.Marshal(req.ReplyMarkup)
		builder.Add("reply_markup", string(replyBytes))
	}

	params := builder.Build()
	return makeAPIRequest(b.token, "sendPhoto", params)
}

// file_path
func (b *Bot) SendPhotoFile(req *SendPhotoRequest) error {
	b.logger.Debug("Preparing to send photo")

	if req.Photo == "" {
		b.logger.Error("Photo is nil in SendPhotoFile request")
		return &BotError{
			Message: "photo can't be nil",
		}
	}

	builder := NewParamBuilder().
		Add("chat_id", req.ChatId).
		Add("caption", req.Caption)

	if req.ReplyParams != nil {
		builder.Add("reply_to_message_id", req.ReplyParams.MessageId)
	}

	if req.ReplyMarkup != nil {
		replyBytes, _ := json.Marshal(req.ReplyMarkup)
		builder.Add("reply_markup", string(replyBytes))
	}

	params := builder.Build()
	b.logger.Debug("Sending photo request")
	return makeMultipartReq(b.token, "sendPhoto", params, "photo", req.Photo)
}

// Send Audio with file_id or URL
func (b *Bot) SendAudio(req *SendAudioRequest) error {
	builder := NewParamBuilder().
		Add("chat_id", req.ChatId).
		Add("audio", req.Audio).
		Add("caption", req.Caption).
		Add("duration", req.Duration).
		Add("performer", req.Performer).
		Add("title", req.Title).
		Add("disable_notification", req.DisableNotification).
		Add("protect_content", req.ProtectContent)

	if req.ReplyParams != nil {
		builder.Add("reply_to_message_id", req.ReplyParams.MessageId)
	}
	if req.ReplyMarkup != nil {
		replyBytes, _ := json.Marshal(req.ReplyMarkup)
		builder.Add("reply_markup", string(replyBytes))
	}
	params := builder.Build()
	return makeAPIRequest(b.token, "sendAudio", params)
}

// Send Audio with file path
func (b *Bot) SendAudioFile(req *SendAudioRequest) error {
	b.logger.Debug("Preparing to send audio")
	if req.Audio == "" {
		b.logger.Error("Audio is nil in SendAudioFile request")
		return &BotError{
			Message: "audio can't be nil",
		}
	}

	builder := NewParamBuilder().
		Add("chat_id", req.ChatId).
		Add("caption", req.Caption).
		Add("duration", req.Duration).
		Add("performer", req.Performer).
		Add("title", req.Title).
		Add("disable_notification", req.DisableNotification).
		Add("protect_content", req.ProtectContent)

	if req.ReplyParams != nil {
		builder.Add("reply_to_message_id", req.ReplyParams.MessageId)
	}
	if req.ReplyMarkup != nil {
		replyBytes, _ := json.Marshal(req.ReplyMarkup)
		builder.Add("reply_markup", string(replyBytes))
	}
	params := builder.Build()
	b.logger.Debug("Sending audio request")
	return makeMultipartReq(b.token, "sendAudio", params, "audio", req.Audio)
}

// Send Video with file_id or URL
func (b *Bot) SendVideo(req *SendVideoRequest) error {
	builder := NewParamBuilder().
		Add("chat_id", req.ChatId).
		Add("video", req.Video).
		Add("caption", req.Caption).
		Add("duration", req.Duration).
		Add("width", req.Width).
		Add("height", req.Height).
		Add("supports_streaming", req.SupportsStreaming).
		Add("has_spoiler", req.HasSpoiler).
		Add("show_caption_above_media", req.ShowCaptionAboveMedia).
		Add("disable_notification", req.DisableNotification).
		Add("protect_content", req.ProtectContent)

	if req.ReplyParams != nil {
		builder.Add("reply_to_message_id", req.ReplyParams.MessageId)
	}
	if req.ReplyMarkup != nil {
		replyBytes, _ := json.Marshal(req.ReplyMarkup)
		builder.Add("reply_markup", string(replyBytes))
	}
	params := builder.Build()
	return makeAPIRequest(b.token, "sendVideo", params)
}

// Send Video with file path
func (b *Bot) SendVideoFile(req *SendVideoRequest) error {
	b.logger.Debug("Preparing to send video")
	if req.Video == "" {
		b.logger.Error("Video is nil in SendVideoFile request")
		return &BotError{
			Message: "video can't be nil",
		}
	}

	builder := NewParamBuilder().
		Add("chat_id", req.ChatId).
		Add("caption", req.Caption).
		Add("duration", req.Duration).
		Add("width", req.Width).
		Add("height", req.Height).
		Add("supports_streaming", req.SupportsStreaming).
		Add("has_spoiler", req.HasSpoiler).
		Add("show_caption_above_media", req.ShowCaptionAboveMedia).
		Add("disable_notification", req.DisableNotification).
		Add("protect_content", req.ProtectContent)

	if req.ReplyParams != nil {
		builder.Add("reply_to_message_id", req.ReplyParams.MessageId)
	}
	if req.ReplyMarkup != nil {
		replyBytes, _ := json.Marshal(req.ReplyMarkup)
		builder.Add("reply_markup", string(replyBytes))
	}
	params := builder.Build()
	b.logger.Debug("Sending video request")
	return makeMultipartReq(b.token, "sendVideo", params, "video", req.Video)
}

// Send Document with file_id or URL
func (b *Bot) SendDocument(req *SendDocumentRequest) error {
	builder := NewParamBuilder().
		Add("chat_id", req.ChatId).
		Add("document", req.Document).
		Add("caption", req.Caption).
		Add("disable_content_type_detection", req.DisableContentTypeDetection).
		Add("disable_notification", req.DisableNotification).
		Add("protect_content", req.ProtectContent)

	if req.ReplyParams != nil {
		builder.Add("reply_to_message_id", req.ReplyParams.MessageId)
	}
	if req.ReplyMarkup != nil {
		replyBytes, _ := json.Marshal(req.ReplyMarkup)
		builder.Add("reply_markup", string(replyBytes))
	}
	params := builder.Build()
	return makeAPIRequest(b.token, "sendDocument", params)
}

// Send Document with file path
func (b *Bot) SendDocumentFile(req *SendDocumentRequest) error {
	b.logger.Debug("Preparing to send document")
	if req.Document == "" {
		b.logger.Error("Document is nil in SendDocumentFile request")
		return &BotError{
			Message: "document can't be nil",
		}
	}

	builder := NewParamBuilder().
		Add("chat_id", req.ChatId).
		Add("caption", req.Caption).
		Add("disable_content_type_detection", req.DisableContentTypeDetection).
		Add("disable_notification", req.DisableNotification).
		Add("protect_content", req.ProtectContent)

	if req.ReplyParams != nil {
		builder.Add("reply_to_message_id", req.ReplyParams.MessageId)
	}
	if req.ReplyMarkup != nil {
		replyBytes, _ := json.Marshal(req.ReplyMarkup)
		builder.Add("reply_markup", string(replyBytes))
	}
	params := builder.Build()
	b.logger.Debug("Sending document request")
	return makeMultipartReq(b.token, "sendDocument", params, "document", req.Document)
}

// Send Animation with file_id or URL
func (b *Bot) SendAnimation(req *SendAnimationRequest) error {
	builder := NewParamBuilder().
		Add("chat_id", req.ChatId).
		Add("animation", req.Animation).
		Add("caption", req.Caption).
		Add("duration", req.Duration).
		Add("width", req.Width).
		Add("height", req.Height).
		Add("has_spoiler", req.HasSpoiler).
		Add("show_caption_above_media", req.ShowCaptionAboveMedia).
		Add("disable_notification", req.DisableNotification).
		Add("protect_content", req.ProtectContent)

	if req.ReplyParams != nil {
		builder.Add("reply_to_message_id", req.ReplyParams.MessageId)
	}
	if req.ReplyMarkup != nil {
		replyBytes, _ := json.Marshal(req.ReplyMarkup)
		builder.Add("reply_markup", string(replyBytes))
	}
	params := builder.Build()
	return makeAPIRequest(b.token, "sendAnimation", params)
}

// Send Animation with file path
func (b *Bot) SendAnimationFile(req *SendAnimationRequest) error {
	b.logger.Debug("Preparing to send animation")
	if req.Animation == "" {
		b.logger.Error("Animation is nil in SendAnimationFile request")
		return &BotError{
			Message: "animation can't be nil",
		}
	}

	builder := NewParamBuilder().
		Add("chat_id", req.ChatId).
		Add("caption", req.Caption).
		Add("duration", req.Duration).
		Add("width", req.Width).
		Add("height", req.Height).
		Add("has_spoiler", req.HasSpoiler).
		Add("show_caption_above_media", req.ShowCaptionAboveMedia).
		Add("disable_notification", req.DisableNotification).
		Add("protect_content", req.ProtectContent)

	if req.ReplyParams != nil {
		builder.Add("reply_to_message_id", req.ReplyParams.MessageId)
	}
	if req.ReplyMarkup != nil {
		replyBytes, _ := json.Marshal(req.ReplyMarkup)
		builder.Add("reply_markup", string(replyBytes))
	}
	params := builder.Build()
	b.logger.Debug("Sending animation request")
	return makeMultipartReq(b.token, "sendAnimation", params, "animation", req.Animation)
}

// Send Voice with file_id or URL
func (b *Bot) SendVoice(req *SendVoiceRequest) error {
	builder := NewParamBuilder().
		Add("chat_id", req.ChatId).
		Add("voice", req.Voice).
		Add("caption", req.Caption).
		Add("duration", req.Duration).
		Add("disable_notification", req.DisableNotification).
		Add("protect_content", req.ProtectContent)

	if req.ReplyParams != nil {
		builder.Add("reply_to_message_id", req.ReplyParams.MessageId)
	}
	if req.ReplyMarkup != nil {
		replyBytes, _ := json.Marshal(req.ReplyMarkup)
		builder.Add("reply_markup", string(replyBytes))
	}
	params := builder.Build()
	return makeAPIRequest(b.token, "sendVoice", params)
}

// Send Voice with file path
func (b *Bot) SendVoiceFile(req *SendVoiceRequest) error {
	b.logger.Debug("Preparing to send voice")
	if req.Voice == "" {
		b.logger.Error("Voice is nil in SendVoiceFile request")
		return &BotError{
			Message: "voice can't be nil",
		}
	}

	builder := NewParamBuilder().
		Add("chat_id", req.ChatId).
		Add("caption", req.Caption).
		Add("duration", req.Duration).
		Add("disable_notification", req.DisableNotification).
		Add("protect_content", req.ProtectContent)

	if req.ReplyParams != nil {
		builder.Add("reply_to_message_id", req.ReplyParams.MessageId)
	}
	if req.ReplyMarkup != nil {
		replyBytes, _ := json.Marshal(req.ReplyMarkup)
		builder.Add("reply_markup", string(replyBytes))
	}
	params := builder.Build()
	b.logger.Debug("Sending voice request")
	return makeMultipartReq(b.token, "sendVoice", params, "voice", req.Voice)
}

// Send VideoNote with file_id or URL
func (b *Bot) SendVideoNote(req *SendVideoNoteRequest) error {
	builder := NewParamBuilder().
		Add("chat_id", req.ChatId).
		Add("video_note", req.VideoNote).
		Add("duration", req.Duration).
		Add("length", req.Length).
		Add("disable_notification", req.DisableNotification).
		Add("protect_content", req.ProtectContent)

	if req.ReplyParams != nil {
		builder.Add("reply_to_message_id", req.ReplyParams.MessageId)
	}
	if req.ReplyMarkup != nil {
		replyBytes, _ := json.Marshal(req.ReplyMarkup)
		builder.Add("reply_markup", string(replyBytes))
	}
	params := builder.Build()
	return makeAPIRequest(b.token, "sendVideoNote", params)
}

// Send VideoNote with file path
func (b *Bot) SendVideoNoteFile(req *SendVideoNoteRequest) error {
	b.logger.Debug("Preparing to send video note")
	if req.VideoNote == "" {
		b.logger.Error("VideoNote is nil in SendVideoNoteFile request")
		return &BotError{
			Message: "video note can't be nil",
		}
	}

	builder := NewParamBuilder().
		Add("chat_id", req.ChatId).
		Add("duration", req.Duration).
		Add("length", req.Length).
		Add("disable_notification", req.DisableNotification).
		Add("protect_content", req.ProtectContent)

	if req.ReplyParams != nil {
		builder.Add("reply_to_message_id", req.ReplyParams.MessageId)
	}
	if req.ReplyMarkup != nil {
		replyBytes, _ := json.Marshal(req.ReplyMarkup)
		builder.Add("reply_markup", string(replyBytes))
	}
	params := builder.Build()
	b.logger.Debug("Sending video note request")
	return makeMultipartReq(b.token, "sendVideoNote", params, "video_note", req.VideoNote)
}

// SendSticker sends a sticker using file ID or URL
func (b *Bot) SendSticker(req *SendStickerRequest) error {
	builder := NewParamBuilder().
		Add("chat_id", req.ChatId).
		Add("sticker", req.Sticker).
		Add("disable_notification", req.DisableNotification).
		Add("protect_content", req.ProtectContent)

	if req.ReplyParams != nil {
		builder.Add("reply_to_message_id", req.ReplyParams.MessageId)
	}
	if req.ReplyMarkup != nil {
		replyBytes, _ := json.Marshal(req.ReplyMarkup)
		builder.Add("reply_markup", string(replyBytes))
	}
	params := builder.Build()
	return makeAPIRequest(b.token, "sendSticker", params)
}

// SendStickerFile sends a sticker using a local file path
func (b *Bot) SendStickerFile(req *SendStickerRequest) error {
	b.logger.Debug("Preparing to send sticker")
	if req.Sticker == "" {
		b.logger.Error("Sticker is nil in SendStickerFile request")
		return &BotError{
			Message: "sticker can't be nil",
		}
	}

	builder := NewParamBuilder().
		Add("chat_id", req.ChatId).
		Add("disable_notification", req.DisableNotification).
		Add("protect_content", req.ProtectContent)

	if req.ReplyParams != nil {
		builder.Add("reply_to_message_id", req.ReplyParams.MessageId)
	}
	if req.ReplyMarkup != nil {
		replyBytes, _ := json.Marshal(req.ReplyMarkup)
		builder.Add("reply_markup", string(replyBytes))
	}
	params := builder.Build()
	b.logger.Debug("Sending sticker request")
	return makeMultipartReq(b.token, "sendSticker", params, "sticker", req.Sticker)
}

// SendMediaGroup
func (b *Bot) SendMediaGroup(chatID int64, media []InputMedia, files []MediaFile) error {
	req := &SendMediaGroupRequest{
		ChatID: chatID,
		Media:  media,
	}

	return makeMultipartMediaGroupReq(b.token, "sendMediaGroup", req, files)
}

// sendChatAction
func (b *Bot) SendChatAction(chatId int64, action string) error {
	return makeAPIRequest(b.token, "sendChatAction", map[string]interface{}{
		"chat_id": chatId,
		"action":  action,
	})
}

func boolPtr(b bool) *bool {
	return &b
}

// banChatMember
func (b *Bot) BanChatMember(chatId string, userId, untilDate int32, revokeMessages *bool) error {
	params := map[string]interface{}{
		"chat_id":    chatId,
		"user_id":    userId,
		"until_date": untilDate,
	}

	if revokeMessages != nil {
		params["revoke_messages"] = *revokeMessages
	}

	return makeAPIRequest(b.token, "banChatMember", params)
}

// unbanChatMember
func (b *Bot) UnbanChatMember(chatId string, userId int32, onlyIfBanned *bool) error {
	params := map[string]interface{}{
		"chat_id": chatId,
		"user_id": userId,
	}

	if onlyIfBanned != nil {
		params["only_if_banned"] = *onlyIfBanned
	}

	return makeAPIRequest(b.token, "unbanChatMember", params)
}

// restrictChatMember
func (b *Bot) RestrictChatMember(req *RestrictChatMember) error {
	if req.ChatId == "" {
		return fmt.Errorf("chat_id is required")
	}
	if req.UserId == 0 {
		return fmt.Errorf("user_id is required")
	}

	if (req.Permissions == ChatPermissions{}) {
		return fmt.Errorf("permissions are required to restrict a chat member")
	}

	params := map[string]interface{}{
		"chat_id":                          req.ChatId,
		"user_id":                          req.UserId,
		"use_independent_chat_permissions": req.UseIndependentChatPermissions,
		"until_date":                       req.UntilDate,
	}

	return makeAPIRequest(b.token, "restrictChatMember", params)
}

// PromoteChatMember promotes a user to an administrator in a chat
func (b *Bot) PromoteChatMember(req *PromoteChatMember) error {
	if req.ChatId == "" {
		return fmt.Errorf("chat_id is required")
	}
	if req.UserId == 0 {
		return fmt.Errorf("user_id is required")
	}

	builder := NewParamBuilder().
		Add("chat_id", req.ChatId).
		Add("user_id", req.UserId).
		Add("is_anonymous", req.IsAnonymous).
		Add("can_manage_chat", req.CanManageChat).
		Add("can_delete_messages", req.CanDeleteMessages).
		Add("can_manage_video_chats", req.CanManageVideoChats).
		Add("can_restrict_members", req.CanRestrictMembers).
		Add("can_promote_members", req.CanPromoteMembers).
		Add("can_change_info", req.CanChangeInfo).
		Add("can_invite_users", req.CanInviteUsers).
		Add("can_post_stories", req.CanPostStories).
		Add("can_edit_stories", req.CanEditStories).
		Add("can_delete_stories", req.CanDeleteStories).
		Add("can_post_messages", req.CanPostMessages).
		Add("can_edit_messages", req.CanEditMessages).
		Add("can_pin_messages", req.CanPinMessages).
		Add("can_manage_topics", req.CanManageTopics)

	err := makeAPIRequest(b.token, "promoteChatMember", builder.Build())
	if err != nil {
		return fmt.Errorf("failed to promote chat member: %w", err)
	}

	return nil
}

// setChatAdministratorCustomTitle
func (b *Bot) SetChatAdministratorCustomTitle(chatId string, userId int32, customTitle string) error {
	return makeAPIRequest(b.token, "setChatAdministratorCustomTitle", map[string]interface{}{
		"chat_id":      chatId,
		"user_id":      userId,
		"custom_title": customTitle,
	})
}

// banChatSenderChat
func (b *Bot) BanChatSenderChat(chatId string, senderChatId int32) error {
	return makeAPIRequest(b.token, "banChatSenderChat", map[string]interface{}{
		"chat_id":        chatId,
		"sender_chat_id": senderChatId,
	})
}

// unbanChatSenderChat
func (b *Bot) UnbanChatSenderChat(chatId string, senderChatId int32) error {
	return makeAPIRequest(b.token, "unbanChatSenderChat", map[string]interface{}{
		"chat_id":        chatId,
		"sender_chat_id": senderChatId,
	})
}

// setChatPermissions
func (b *Bot) SetChatPermissions(chatId string, chatPermissions ChatPermissions, useIndependentChatPermissions *bool) error {
	permissions, _ := json.Marshal(chatPermissions)
	params := map[string]interface{}{
		"chat_id":                          chatId,
		"permissions":                      string(permissions),
		"use_independent_chat_permissions": useIndependentChatPermissions,
	}
	if useIndependentChatPermissions != nil {
		params["use_independent_chat_permissions"] = *useIndependentChatPermissions
	}

	return makeAPIRequest(b.token, "setChatPermissions", params)
}

// exportChatInviteLink
func (b *Bot) ExportChatInviteLink(chatId string) (string, error) {
	// Make the API request, assuming it returns json.RawMessage
	response, err := makeAPIRequestWithResult(b.token, "exportChatInviteLink", map[string]interface{}{
		"chat_id": chatId,
	})
	if err != nil {
		return "", fmt.Errorf("failed to export chat invite link: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(response, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	inviteLink, ok := result["result"].(string)
	if !ok {
		return "", fmt.Errorf("unexpected response format: missing 'result' field or invalid type")
	}

	return inviteLink, nil
}

// createChatInviteLink
func (b *Bot) CreateChatInviteLink(chatId string) (map[string]interface{}, error) {
	response, err := makeAPIRequestWithResult(b.token, "createChatInviteLink", map[string]interface{}{
		"chat_id": chatId,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create chat invite link: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	inviteLink, ok := result["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format: missing 'result' field or invalid type")
	}

	return inviteLink, nil
}

// editChatInviteLink
func (b *Bot) EditChatInviteLink(chatId string, inviteLink string, name *string, expireDate *int32, memberLimit *int32, createsJoinRequest *bool) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"chat_id":     chatId,
		"invite_link": inviteLink,
	}

	if name != nil {
		params["name"] = *name
	}
	if expireDate != nil {
		params["expire_date"] = *expireDate
	}
	if memberLimit != nil {
		params["member_limit"] = *memberLimit
	}
	if createsJoinRequest != nil {
		params["creates_join_request"] = *createsJoinRequest
	}

	response, err := makeAPIRequestWithResult(b.token, "editChatInviteLink", params)
	if err != nil {
		return nil, fmt.Errorf("failed to edit chat invite link: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	editedInviteLink, ok := result["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format: missing 'result' field or invalid type")
	}

	return editedInviteLink, nil
}

func (b *Bot) CreateChatSubscriptionInviteLink(chatId string, name *string, subscriptionPeriod int32, subscriptionPrice int32) (map[string]interface{}, error) {
	if subscriptionPeriod != 2592000 {
		return nil, fmt.Errorf("subscription period must always be 2592000 (30 days)")
	}
	if subscriptionPrice < 1 || subscriptionPrice > 2500 {
		return nil, fmt.Errorf("subscription price must be between 1 and 2500")
	}

	params := map[string]interface{}{
		"chat_id":             chatId,
		"subscription_period": subscriptionPeriod,
		"subscription_price":  subscriptionPrice,
	}

	if name != nil {
		params["name"] = *name
	}

	response, err := makeAPIRequestWithResult(b.token, "createChatSubscriptionInviteLink", params)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat subscription invite link: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	chatInviteLink, ok := result["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format: missing 'result' field or invalid type")
	}

	return chatInviteLink, nil
}

func (b *Bot) EditChatSubscriptionInviteLink(chatId string, inviteLink string, name *string) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"chat_id":     chatId,
		"invite_link": inviteLink,
	}

	if name != nil {
		params["name"] = *name
	}

	response, err := makeAPIRequestWithResult(b.token, "editChatSubscriptionInviteLink", params)
	if err != nil {
		return nil, fmt.Errorf("failed to edit chat subscription invite link: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	chatInviteLink, ok := result["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format: missing 'result' field or invalid type")
	}

	return chatInviteLink, nil
}

func (b *Bot) RevokeChatInviteLink(chatId string, inviteLink string) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"chat_id":     chatId,
		"invite_link": inviteLink,
	}

	response, err := makeAPIRequestWithResult(b.token, "revokeChatInviteLink", params)
	if err != nil {
		return nil, fmt.Errorf("failed to revoke chat invite link: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	chatInviteLink, ok := result["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format: missing 'result' field or invalid type")
	}

	return chatInviteLink, nil
}

func (b *Bot) ApproveChatJoinRequest(chatId string, userId int) (bool, error) {
	params := map[string]interface{}{
		"chat_id": chatId,
		"user_id": userId,
	}

	response, err := makeAPIRequestWithResult(b.token, "approveChatJoinRequest", params)
	if err != nil {
		return false, fmt.Errorf("failed to approve chat join request: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(response, &result); err != nil {
		return false, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	approved, ok := result["result"].(bool)
	if !ok {
		return false, fmt.Errorf("unexpected response format: missing 'result' field or invalid type")
	}

	return approved, nil
}

func (b *Bot) DeclineChatJoinRequest(chatId string, userId int) (bool, error) {
	params := map[string]interface{}{
		"chat_id": chatId,
		"user_id": userId,
	}

	response, err := makeAPIRequestWithResult(b.token, "declineChatJoinRequest", params)
	if err != nil {
		return false, fmt.Errorf("failed to decline chat join request: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(response, &result); err != nil {
		return false, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	declined, ok := result["result"].(bool)
	if !ok {
		return false, fmt.Errorf("unexpected response format: missing 'result' field or invalid type")
	}

	return declined, nil
}

func (b *Bot) SetChatPhoto(chatId string, photoPath string) (bool, error) {
	params := map[string]interface{}{
		"chat_id": chatId,
	}

	err := makeMultipartReq(b.token, "setChatPhoto", params, "photo", photoPath)
	if err != nil {
		return false, fmt.Errorf("failed to set chat photo: %w", err)
	}

	return true, nil
}

func (b *Bot) DeleteChatPhoto(chatId string) error {
	return makeAPIRequest(b.token, "deleteChatPhoto", map[string]interface{}{
		"chat_id": chatId,
	})
}

func (b *Bot) SetChatTitle(chatId, title string) error {
	return makeAPIRequest(b.token, "setChatTitle", map[string]interface{}{
		"chat_id": chatId,
		"title":   title,
	})
}

func (b *Bot) SetChatDescription(chatId, description string) error {
	return makeAPIRequest(b.token, "setChatDescription", map[string]interface{}{
		"chat_id":     chatId,
		"description": description,
	})
}

func (b *Bot) PinChatMessage(chatId string, messageId int64, DisableNotification bool) error {
	return makeAPIRequest(b.token, "pinChatMessage", map[string]interface{}{
		"chat_id":              chatId,
		"message_id":           messageId,
		"disable_notification": DisableNotification,
	})
}

func (b *Bot) UnpinChatMessage(chatId string, messageId int64) error {
	return makeAPIRequest(b.token, "unpinChatMessage", map[string]interface{}{
		"chat_id":    chatId,
		"message_id": messageId,
	})
}

func (b *Bot) UnpinAllChatMessages(chatId string) error {
	return makeAPIRequest(b.token, "unpinAllChatMessages", map[string]interface{}{
		"chat_id": chatId,
	})
}

func (b *Bot) LeaveChat(chatId string) error {
	return makeAPIRequest(b.token, "leaveChat", map[string]interface{}{
		"chat_id": chatId,
	})
}

func (b *Bot) GetChat(chatId string) (json.RawMessage, error) {
	return makeAPIRequestWithResult(b.token, "getChat", map[string]interface{}{
		"chat_id": chatId,
	})
}

func (b *Bot) GetChatAdministrators(chatId string) (json.RawMessage, error) {
	return makeAPIRequestWithResult(b.token, "getChatAdministrators", map[string]interface{}{
		"chat_id": chatId,
	})
}

func (b *Bot) GetChatMemberCount(chatId string) (json.RawMessage, error) {
	return makeAPIRequestWithResult(b.token, "getChatMemberCount", map[string]interface{}{
		"chat_id": chatId,
	})
}

func (b *Bot) GetChatMember(chatId string, userId int64) (json.RawMessage, error) {
	return makeAPIRequestWithResult(b.token, "getChatMember", map[string]interface{}{
		"chat_id": chatId,
		"user_id": userId,
	})
}

func (b *Bot) SetStickerSet(chatId, stickerSetName string) (json.RawMessage, error) {
	return makeAPIRequestWithResult(b.token, "setStickerSet", map[string]interface{}{
		"chat_id":          chatId,
		"sticker_set_name": stickerSetName,
	})
}

func (b *Bot) DeleteStickerSet(chatId string) (json.RawMessage, error) {
	return makeAPIRequestWithResult(b.token, "deleteChatStickerSet", map[string]interface{}{
		"chat_id": chatId,
	})
}

func (b *Bot) GetForumTopicIconStickers() (json.RawMessage, error) {
	return makeAPIRequestWithResult(b.token, "getForumTopicIconStickers", map[string]interface{}{})
}

func (b *Bot) answerCallbackQuery(req *AnswerCallbackQueryRequest) error {
	params := map[string]interface{}{
		"callback_query_id": req.CallbackQueryId,
	}

	if req.Text != "" {
		params["text"] = req.Text
	}

	if req.ShowAlert {
		params["show_alert"] = req.ShowAlert
	}

	if req.URL != "" {
		params["url"] = req.URL
	}

	if req.CacheTime != 0 {
		params["cache_time"] = req.CacheTime
	}

	return makeAPIRequest(b.token, "answerCallbackQuery", params)
}

func (b *Bot) GetUserChatBoosts(chatId string, userId int64) (json.RawMessage, error) {
	return makeAPIRequestWithResult(b.token, "getUserChatBoosts", map[string]interface{}{
		"chat_id": chatId,
		"user_id": userId,
	})
}

func (b *Bot) SetMyCommands(commands []BotCommand) error {
	botCommands, _ := json.Marshal(commands)
	return makeAPIRequest(b.token, "setMyCommands", map[string]interface{}{
		"commands": string(botCommands),
	})
}

func (b *Bot) DeleteMyCommands() error {
	return makeAPIRequest(b.token, "deleteMyCommands", map[string]interface{}{})
}

func (b *Bot) GetMyCommands() (json.RawMessage, error) {
	return makeAPIRequestWithResult(b.token, "getMyCommands", map[string]interface{}{})
}

func (b *Bot) SetMyName(name, langagueCode string) (json.RawMessage, error) {
	return makeAPIRequestWithResult(b.token, "setMyName", map[string]interface{}{
		"name":          name,
		"language_code": langagueCode,
	})
}

func (b *Bot) GetMyName(name, langagueCode string) (json.RawMessage, error) {
	return makeAPIRequestWithResult(b.token, "getMyName", map[string]interface{}{
		"language_code": langagueCode,
	})
}

func (b *Bot) SetMyDescription(description, langagueCode string) error {
	return makeAPIRequest(b.token, "setMyDescription", map[string]interface{}{
		"description":   description,
		"language_code": langagueCode,
	})
}

func (b *Bot) GetMyDescription(description, langagueCode string) (json.RawMessage, error) {
	return makeAPIRequestWithResult(b.token, "getMyDescription", map[string]interface{}{
		"language_code": langagueCode,
	})
}

func (b *Bot) SetMyShortDescription(shortDescription, langagueCode string) error {
	return makeAPIRequest(b.token, "setMyShortDescription", map[string]interface{}{
		"short_description": shortDescription,
		"language_code":     langagueCode,
	})
}

func (b *Bot) GetMyShortDescription(langagueCode string) (json.RawMessage, error) {
	return makeAPIRequestWithResult(b.token, "getMyShortDescription", map[string]interface{}{
		"language_code": langagueCode,
	})
}

func (b *Bot) EditMessageText(req *EditMessageTextRequest) error {
	builer := NewParamBuilder().Add("chat_id", req.ChatId).Add("message_id", req.MessageId)
	return makeAPIRequest(b.token, "getMyShortDescription", builer.Build())
}
