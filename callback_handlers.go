package tgx

import (
	"strings"

	"github.com/harshyadavone/tgx/models"
)

func (b *Bot) OnCallback(data string, handler callbackHandler) {
	b.callbackHandlers[data] = handler
}

func (b *Bot) handleCallbackQuery(cb *models.CallbackQuery) error {
	ctx := &CallbackContext{
		QueryID:  cb.ID,
		Data:     cb.Data,
		Message:  cb.Message,
		UserID:   cb.From.Id,
		Username: cb.From.Username,
		bot:      b,
	}

	// check for exact match
	if handler, ok := b.callbackHandlers[cb.Data]; ok {
		ctx.bot.logger.Debug("callback handler called")
		if err := handler(ctx); err != nil {
			ctx.bot.logger.Error("error in calling handler %w: ", err)
			return err
		}
		return nil
	}

	// fallback: for prefix
	for data, handler := range b.callbackHandlers {
		if strings.HasPrefix(cb.Data, data) {
			ctx.bot.logger.Debug("callback handler called for prfix: %s", data)
			if err := handler(ctx); err != nil {
				ctx.bot.logger.Error("error in calling handler %w: ", err)
				return err
			}
			return nil
		}
	}

	ctx.bot.logger.Warn("No callback handler found for data: %s", cb.Data)
	return nil
}

func (ctx *CallbackContext) AnswerCallback(opts *CallbackAnswerOptions) error {
	ctx.bot.logger.Info("Answering callback query")
	payload := map[string]interface{}{
		"callback_query_id": ctx.QueryID,
	}

	if opts != nil {
		if opts.Text != "" {
			payload["text"] = opts.Text
		}
		if opts.ShowAlert {
			payload["show_alert"] = opts.ShowAlert
		}
		if opts.URL != "" {
			payload["url"] = opts.URL
		}
		if opts.CacheTime > 0 {
			payload["cache_time"] = opts.CacheTime
		}
	}

	return ctx.makeRequest("answerCallbackQuery", payload)
}

func (ctx *CallbackContext) EditMessage(newText string, opts *EditMessageOptions) error {
	payload := map[string]interface{}{
		"chat_id":    ctx.Message.Chat.Id,
		"message_id": ctx.Message.MessageId,
		"text":       newText,
	}

	if opts != nil {
		if opts.ParseMode == HTML || opts.ParseMode == MarkdownV2 {
			payload["parse_mode"] = opts.ParseMode
		}
		if opts.DisableWebPagePreview {
			payload["disable_web_page_preview"] = opts.DisableWebPagePreview
		}
		if opts.ReplyMarkup != nil {
			payload["reply_markup"] = opts.ReplyMarkup
		}
	}

	return ctx.makeRequest("editMessageText", payload)
}

func (ctx *CallbackContext) EditMarkup(markup *models.InlineKeyboardMarkup) error {
	return ctx.makeRequest("editMessageReplyMarkup", map[string]interface{}{
		"chat_id":      ctx.Message.Chat.Id,
		"message_id":   ctx.Message.MessageId,
		"reply_markup": markup,
	})
}

func (ctx *CallbackContext) Reply(text string, opts *SendMessageRequest) error {
	payload := map[string]interface{}{
		"chat_id": ctx.Message.Chat.Id,
		"text":    text,
	}

	if opts != nil {
		if opts.ParseMode == HTML || opts.ParseMode == MarkdownV2 {
			payload["parse_mode"] = opts.ParseMode
		}
		if opts.ReplyMarkup != nil {
			payload["reply_markup"] = opts.ReplyMarkup
		}
	}

	return ctx.makeRequest("sendMessage", payload)
}

// Helper Methods
func (ctx *CallbackContext) Alert(text string) error {
	return ctx.AnswerCallback(&CallbackAnswerOptions{
		Text:      text,
		ShowAlert: true,
	})
}

func (ctx *CallbackContext) DeleteMessage() error {
	return ctx.makeRequest("deleteMessage", map[string]interface{}{
		"chat_id":    ctx.Message.Chat.Id,
		"message_id": ctx.Message.MessageId,
	})
}

// Getters
func (ctx *CallbackContext) GetMessageID() int64 {
	return ctx.Message.MessageId
}

func (ctx *CallbackContext) GetChatID() int64 {
	return ctx.Message.Chat.Id
}

func (ctx *CallbackContext) GetUserID() int64 {
	return ctx.UserID
}

func (ctx *CallbackContext) GetUsername() string {
	return ctx.Username
}

func (ctx *CallbackContext) GetData() string {
	return ctx.Data
}
