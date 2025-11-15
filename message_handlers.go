package tgx

import (
	"github.com/harshyadavone/tgx/models"
)

func (b *Bot) OnMessage(messageType string, handler Handler) {
	if b.messageHandlers == nil {
		b.messageHandlers = make(map[string]Handler)
	}
	b.messageHandlers[messageType] = handler
}

func (ctx *Context) Reply(text string) error {
	payload := map[string]interface{}{
		"chat_id":               ctx.ChatID,
		"text":                  text,
		"reply_with_parameters": ctx.MessageId,
	}

	return ctx.makeRequest("sendMessage", payload)
}

func (ctx *Context) ReplyWithOpts(req *SendMessageRequest) error {
	payload := map[string]interface{}{
		"chat_id": ctx.ChatID,
		"text":    req.Text,
	}

	if req.ParseMode == HTML || req.ParseMode == MarkdownV2 {
		payload["parse_mode"] = req.ParseMode
	}

	if req.ReplyMarkup != nil {
		payload["reply_markup"] = req.ReplyMarkup
	}

	if req.ReplyParams != nil && req.ReplyParams.MessageId != 0 {
		replyParam := map[string]interface{}{
			"message_id": ctx.MessageId,
		}
		if req.ReplyParams.ChatId != 0 {
			replyParam["chat_id"] = ctx.ChatID
		}
		payload["reply_parameters"] = replyParam
	}

	return ctx.makeRequest("sendMessage", payload)
}

func (ctx *Context) ReplyWithInlineKeyboard(text string, buttons [][]models.InlineKeyboardButton) error {
	return ctx.makeRequest("sendMessage", map[string]any{
		"chat_id": ctx.ChatID,
		"text":    text,
		"reply_markup": map[string]any{
			"inline_keyboard": buttons,
		},
	})
}
