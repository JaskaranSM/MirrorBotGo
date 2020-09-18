package authorization

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
	"github.com/PaulSonOfLars/gotgbot/handlers"
	"go.uber.org/zap"
)

func ExtractUserId(message *ext.Message) int {
	var userId int
	arg := utils.ParseMessageArgs(message.Text)
	if message.ReplyToMessage != nil {
		userId = message.ReplyToMessage.From.Id
	} else if arg != "" {
		userId = int(utils.ParseStringToInt64(arg))
	}
	return userId
}

func ExtractChatId(message *ext.Message) int {
	var chatId int
	arg := utils.ParseMessageArgs(message.Text)
	if arg != "" {
		chatId = int(utils.ParseStringToInt64(arg))
	} else {
		chatId = message.Chat.Id
	}
	return chatId
}

func AuthorizeUserHandler(b ext.Bot, u *gotgbot.Update) error {
	if !utils.IsUserOwner(u.EffectiveUser.Id) {
		return nil
	}
	message := u.EffectiveMessage
	userId := ExtractUserId(message)
	if userId == 0 {
		engine.SendMessage(b, "Provide Proper userId", message)
		return nil
	}
	if db.IsUserAuthorized(userId) {
		engine.SendMessage(b, "User is already authorized.", message)
		return nil
	}
	done := db.AuthorizeUserDb(userId)
	if done {
		engine.SendMessage(b, "Authorized User.", message)
	} else {
		engine.SendMessage(b, "Error while authorizing user.", message)
	}
	return nil
}

func AuthorizeChatHandler(b ext.Bot, u *gotgbot.Update) error {
	if !utils.IsUserOwner(u.EffectiveUser.Id) {
		return nil
	}
	message := u.EffectiveMessage
	chatId := ExtractChatId(message)
	if chatId == 0 {
		engine.SendMessage(b, "Provide Proper chatId", message)
		return nil
	}
	if db.IsChatAuthorized(chatId) {
		engine.SendMessage(b, "Chat is already authorized.", message)
		return nil
	}
	done := db.AuthorizeChatDb(chatId)
	if done {
		engine.SendMessage(b, "Authorized Chat.", message)
	} else {
		engine.SendMessage(b, "Error while authorizing Chat.", message)
	}
	return nil
}

func UnAuthorizeUserHandler(b ext.Bot, u *gotgbot.Update) error {
	if !utils.IsUserOwner(u.EffectiveUser.Id) {
		return nil
	}
	message := u.EffectiveMessage
	userId := ExtractUserId(message)
	if userId == 0 {
		engine.SendMessage(b, "Provide Proper userId", message)
		return nil
	}
	if !db.IsUserAuthorized(userId) {
		engine.SendMessage(b, "User was not authorized in the first place.", message)
		return nil
	}
	done := db.UnAuthorizeUserDb(userId)
	if done {
		engine.SendMessage(b, "UnAuthorized User.", message)
	} else {
		engine.SendMessage(b, "Error while UnAuthorizing user.", message)
	}
	return nil
}

func UnAuthorizeChatHandler(b ext.Bot, u *gotgbot.Update) error {
	if !utils.IsUserOwner(u.EffectiveUser.Id) {
		return nil
	}
	message := u.EffectiveMessage
	chatId := ExtractChatId(message)
	if chatId == 0 {
		engine.SendMessage(b, "Provide Proper chatId", message)
		return nil
	}
	if !db.IsChatAuthorized(chatId) {
		engine.SendMessage(b, "Chat was not authorized in the first place.", message)
		return nil
	}
	done := db.UnAuthorizeChatDb(chatId)
	if done {
		engine.SendMessage(b, "UnAuthorized Chat.", message)
	} else {
		engine.SendMessage(b, "Error while UnAuthorizing Chat.", message)
	}
	return nil
}

func LoadAuthorizationHandlers(updater *gotgbot.Updater, l *zap.SugaredLogger) {
	defer l.Info("Authorization Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("adduser", AuthorizeUserHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("rmuser", UnAuthorizeUserHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("addchat", AuthorizeChatHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("rmchat", UnAuthorizeChatHandler))
}
