package tgx

import "github.com/harshyadavone/tgx/models"

type ParseMode string

const (
	HTML       ParseMode = "HTML"
	MarkdownV2 ParseMode = "MarkdownV2"
)

type SendMessageRequest struct {
	ChatId      int64       `json:"chat_id"`              // Required
	Text        string      `json:"text"`                 // Required
	ParseMode   ParseMode   `json:"parse_mode,omitempty"` // MarkdownV2 || HTML
	ReplyMarkup ReplyMarkup `json:"reply_markup,omitempty"`
	ReplyParams *ReplyParam `json:"reply_paramaters,omitempty"`
}

type ReplyParam struct {
	MessageId int64 `json:"message_id"` // Required
	ChatId    int64 `json:"chat_id,omitempty"`
}

// can be InlineKeyboardMarkup or ReplyKeyboardMarkup or ReplyKeyboardRemove or ForceReply
type ReplyMarkup interface{}

type ForwardMessageRequest struct {
	ChatId              int64 `json:"chat_id"`
	FromChatId          int64 `json:"from_chat_id"`
	MessageId           int64 `json:"message_id"`
	DisableNotification bool  `json:"disable_notification"`
	ProtectContent      bool  `json:"protect_content"`
}

type ForwardMessagesRequest struct {
	ChatId              int64   `json:"chat_id"`
	FromChatId          int64   `json:"from_chat_id"`
	MessageIds          []int64 `json:"message_ids"`
	DisableNotification bool    `json:"disable_notification"`
	ProtectContent      bool    `json:"protect_content"`
}

type CopyMessageRequest struct {
	ChatId                int64        `json:"chat_id"`      // Required
	FromChatId            int64        `json:"from_chat_id"` // Required
	MessageId             int64        `json:"message_id"`   // Required
	Caption               string       `json:"caption"`
	ParseMode             ParseMode    `json:"parse_mode"`
	ShowCaptionAboveMedia bool         `json:"show_caption_above_media"`
	AllowPaidBroadCast    bool         `json:"allow_paid_broadcast"`
	DisableNotification   bool         `json:"disable_notification"`
	ProtectContent        bool         `json:"protect_content"`
	ReplyParams           *ReplyParam  `json:"reply_parameters"`
	ReplyMarkup           *ReplyMarkup `json:"reply_markup"`
}

type CopyMessagesRequest struct {
	ChatId              int64   `json:"chat_id"`      // Required
	FromChatId          int64   `json:"from_chat_id"` // Required
	MessageIds          []int64 `json:"message_ids"`  // Required
	ProtectContent      bool    `json:"protect_content"`
	DisableNotification bool    `json:"disable_notification"`
	RemoveCaption       bool    `json:"remove_caption"`
}

type BaseMediaRequest struct {
	ChatId              int64       `json:"chat_id"`                // Required
	Caption             string      `json:"caption,omitempty"`      // Optional
	ParseMode           ParseMode   `json:"parse_mode,omitempty"`   // Optional
	ReplyMarkup         ReplyMarkup `json:"reply_markup,omitempty"` // Optional
	ReplyParams         *ReplyParam `json:"reply_parameters,omitempty"`
	DisableNotification bool        `json:"disable_notification,omitempty"`
	ProtectContent      bool        `json:"protect_content,omitempty"`
	AllowPaidBroadCast  bool        `json:"allow_paid_broadcast,omitempty"`
}

type SendPhotoRequest struct {
	BaseMediaRequest
	Photo                 string `json:"photo"` // Required: file_path or file_id
	ShowCaptionAboveMedia bool   `json:"show_caption_above_media,omitempty"`
	HasSpoiler            bool   `json:"has_spoiler,omitempty"`
}

type SendAudioRequest struct {
	BaseMediaRequest
	Audio     string `json:"audio"`               // Required: file_path or file_id
	Duration  int64  `json:"duration,omitempty"`  // Optional: duration in seconds
	Performer string `json:"performer,omitempty"` // Optional
	Title     string `json:"title,omitempty"`     // Optional
}

type SendVideoRequest struct {
	BaseMediaRequest
	Video                 string `json:"video"`              // Required: file_path or file_id
	Duration              int64  `json:"duration,omitempty"` // Optional: duration in seconds
	Width                 int64  `json:"width,omitempty"`    // Optional
	Height                int64  `json:"height,omitempty"`   // Optional
	HasSpoiler            bool   `json:"has_spoiler,omitempty"`
	ShowCaptionAboveMedia bool   `json:"show_caption_above_media,omitempty"`
	SupportsStreaming     bool   `json:"supports_streaming,omitempty"` // Optional
}

type SendDocumentRequest struct {
	BaseMediaRequest
	Document                    string `json:"document"`                       // Required: file_path or file_id
	DisableContentTypeDetection bool   `json:"disable_content_type_detection"` // Optional
}

type SendAnimationRequest struct {
	BaseMediaRequest
	Animation             string `json:"animation"`          // Required: file_path or file_id
	Duration              int64  `json:"duration,omitempty"` // Optional: duration in seconds
	Width                 int64  `json:"width,omitempty"`    // Optional
	Height                int64  `json:"height,omitempty"`   // Optional
	ShowCaptionAboveMedia bool   `json:"show_caption_above_media,omitempty"`
	HasSpoiler            bool   `json:"has_spoiler,omitempty"`
}

type SendVoiceRequest struct {
	BaseMediaRequest
	Voice    string `json:"voice"`              // Required: file_path or file_id
	Duration int64  `json:"duration,omitempty"` // Optional: duration in seconds
}

type SendVideoNoteRequest struct {
	BaseMediaRequest
	VideoNote string `json:"video_note"`         // Required: file_path or file_id
	Duration  int64  `json:"duration,omitempty"` // Optional: duration in seconds
	Length    int64  `json:"length,omitempty"`   // Optional
}

type SendStickerRequest struct {
	BaseMediaRequest
	Sticker string `json:"sticker"`
	Emoji   string `json:"emoji,omitempty"` // only for uploaded stickers
}

// MediaFile represents a file to be sent in a media group
type MediaFile struct {
	ParamName string // e.g., "photo", "video"
	FilePath  string
	FileType  string // mime type
	Media     string // this will be used in the media array json
}

// SendMediaGroupRequest represents the structure for sending multiple media files
type SendMediaGroupRequest struct {
	ChatID              int64        `json:"chat_id"`
	Media               []InputMedia `json:"media"`
	DisableNotification bool         `json:"disable_notification,omitempty"`
	ProtectContent      bool         `json:"protect_content,omitempty"`
	ReplyParams         *ReplyParam  `json:"reply_parameters,omitempty"`
}

// InputMedia represents a single media in the group
type InputMedia struct {
	Type              string    `json:"type"`  // "photo", "video", etc.
	Media             string    `json:"media"` // file_id or "attach://<file_name>"
	Caption           string    `json:"caption,omitempty"`
	ParseMode         ParseMode `json:"parse_mode,omitempty"`
	HasSpoiler        bool      `json:"has_spoiler,omitempty"`
	Duration          int       `json:"duration,omitempty"`           // For videos
	Width             int       `json:"width,omitempty"`              // For videos
	Height            int       `json:"height,omitempty"`             // For videos
	SupportsStreaming bool      `json:"supports_streaming,omitempty"` // For videos
}

type SendChatActionRequest struct {
	ChatId int64  `json:"chat_id"`
	Action string `json:"action"` // typing, upload_photo
}

type EditMessageTextRequest struct {
	ChatId      int64                       `json:"chat_id"`
	MessageId   int64                       `json:"message_id"`
	Text        string                      `json:"text"`
	ReplyMarkup models.InlineKeyboardMarkup `json:"reply_markup"`
}

type DeleteMessageRequest struct {
	ChatId    int64 `json:"chat_id"`
	MessageId int64 `json:"message_id"`
}

type ChatPermissions struct {
	CanSendMessages       *bool `json:"can_send_messages,omitempty"`         // Optional. True if the user is allowed to send text messages, contacts, giveaways, etc.
	CanSendAudios         *bool `json:"can_send_audios,omitempty"`           // Optional. True if the user is allowed to send audios.
	CanSendDocuments      *bool `json:"can_send_documents,omitempty"`        // Optional. True if the user is allowed to send documents.
	CanSendPhotos         *bool `json:"can_send_photos,omitempty"`           // Optional. True if the user is allowed to send photos.
	CanSendVideos         *bool `json:"can_send_videos,omitempty"`           // Optional. True if the user is allowed to send videos.
	CanSendVideoNotes     *bool `json:"can_send_video_notes,omitempty"`      // Optional. True if the user is allowed to send video notes.
	CanSendVoiceNotes     *bool `json:"can_send_voice_notes,omitempty"`      // Optional. True if the user is allowed to send voice notes.
	CanSendPolls          *bool `json:"can_send_polls,omitempty"`            // Optional. True if the user is allowed to send polls.
	CanSendOtherMessages  *bool `json:"can_send_other_messages,omitempty"`   // Optional. True if the user is allowed to send animations, games, etc.
	CanAddWebPagePreviews *bool `json:"can_add_web_page_previews,omitempty"` // Optional. True if the user is allowed to add web page previews to their messages.
	CanChangeInfo         *bool `json:"can_change_info,omitempty"`           // Optional. True if the user is allowed to change chat settings. Ignored in public supergroups.
	CanInviteUsers        *bool `json:"can_invite_users,omitempty"`          // Optional. True if the user is allowed to invite new users to the chat.
	CanPinMessages        *bool `json:"can_pin_messages,omitempty"`          // Optional. True if the user is allowed to pin messages. Ignored in public supergroups.
	CanManageTopics       *bool `json:"can_manage_topics,omitempty"`         // Optional. True if the user is allowed to create forum topics. Defaults to `CanPinMessages` if omitted.
}

type RestrictChatMember struct {
	ChatId                        string          `json:"chat_id"`
	UserId                        int32           `json:"user_id"`
	Permissions                   ChatPermissions `json:"permissions"`
	UseIndependentChatPermissions bool            `json:"use_independent_chat_permissions"`
	UntilDate                     int32           `json:"until_date"`
}

type PromoteChatMember struct {
	ChatId              string `json:"chat_id"`                          // Unique identifier for the target chat or username of the target channel (@channelusername)
	UserId              int    `json:"user_id"`                          // Unique identifier of the target user
	IsAnonymous         *bool  `json:"is_anonymous,omitempty"`           // Optional. True if the administrator's presence in the chat is hidden
	CanManageChat       *bool  `json:"can_manage_chat,omitempty"`        // Optional. True if the admin can access chat event log and other admin privileges
	CanDeleteMessages   *bool  `json:"can_delete_messages,omitempty"`    // Optional. True if the admin can delete messages of other users
	CanManageVideoChats *bool  `json:"can_manage_video_chats,omitempty"` // Optional. True if the admin can manage video chats
	CanRestrictMembers  *bool  `json:"can_restrict_members,omitempty"`   // Optional. True if the admin can restrict, ban, or unban members
	CanPromoteMembers   *bool  `json:"can_promote_members,omitempty"`    // Optional. True if the admin can add new administrators or demote others
	CanChangeInfo       *bool  `json:"can_change_info,omitempty"`        // Optional. True if the admin can change chat title, photo, and settings
	CanInviteUsers      *bool  `json:"can_invite_users,omitempty"`       // Optional. True if the admin can invite new users
	CanPostStories      *bool  `json:"can_post_stories,omitempty"`       // Optional. True if the admin can post stories to the chat
	CanEditStories      *bool  `json:"can_edit_stories,omitempty"`       // Optional. True if the admin can edit others' stories and manage story settings
	CanDeleteStories    *bool  `json:"can_delete_stories,omitempty"`     // Optional. True if the admin can delete stories posted by others
	CanPostMessages     *bool  `json:"can_post_messages,omitempty"`      // Optional. True if the admin can post messages in the channel (channels only)
	CanEditMessages     *bool  `json:"can_edit_messages,omitempty"`      // Optional. True if the admin can edit messages of other users (channels only)
	CanPinMessages      *bool  `json:"can_pin_messages,omitempty"`       // Optional. True if the admin can pin messages (supergroups only)
	CanManageTopics     *bool  `json:"can_manage_topics,omitempty"`      // Optional. True if the user can manage forum topics (supergroups only)
}

type AnswerCallbackQueryRequest struct {
	CallbackQueryId string `json:"callback_query_id"`
	Text            string `json:"text"`
	ShowAlert       bool   `json:"show_alert"`
	URL             string `json:"url"`
	CacheTime       int    `json:"cache_time"`
}

type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

type CallbackAnswerOptions struct {
	Text      string `json:"text,omitempty"`
	ShowAlert bool   `json:"show_alert,omitempty"`
	URL       string `json:"url,omitempty"`
	CacheTime int    `json:"cache_time,omitempty"`
}

type EditMessageOptions struct {
	ParseMode             ParseMode                    `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool                         `json:"disable_web_page_preview,omitempty"`
	ReplyMarkup           *models.InlineKeyboardMarkup `json:"reply_markup,omitempty"`
}
