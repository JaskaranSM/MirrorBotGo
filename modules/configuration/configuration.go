package configuration

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"go.uber.org/zap"
)

func SetGotdDownloadThreadsCountHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	threadCountString := utils.ParseMessageArgs(message.Text)
	if threadCountString == "" {
		engine.SendMessage(b, "Provide arg bruh", message)
		return nil
	}
	threadCountInt, err := strconv.Atoi(threadCountString)
	if err != nil {
		engine.L().Errorf("Error parsing gotd thread count: %s", err.Error())
		engine.SendMessage(b, fmt.Sprintf("Error parsing thread count: %s", err.Error()), message)
	}
	if threadCountInt <= 0 {
		engine.L().Errorf("Error setting gotd thread count: thread count must be above 0")
		engine.SendMessage(b, "Error setting gotd thread count: thread count must be above 0", message)
		return nil
	}
	engine.L().Infof("Setting gotd download threads count %d", threadCountInt)
	engine.SetGotdDownloadThreadsCount(threadCountInt)
	engine.SendMessage(b, fmt.Sprintf("Gotd download threads count has been set to %d", threadCountInt), message) //engine.GetGotdDownloadThreadsCount()), message)
	return nil
}

func GetGotdDownloadThreadsCountHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	engine.SendMessage(b, fmt.Sprintf("Gotd download thread count: <code>%d</code>", engine.GetGotdDownloadThreadsCount()), message)
	return nil
}

func MegaLoginHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	out := ""
	err := engine.PerformMegaLogin()
	if err != nil {
		out = fmt.Sprintf("Mega login failed: %s", err.Error())
	} else {
		out = "Mega login success."
	}
	engine.SendMessage(b, out, message)
	return nil
}

func SetTorrentClientConnectionsHandlers(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	connectionCountString := utils.ParseMessageArgs(message.Text)
	if connectionCountString == "" {
		engine.SendMessage(b, "Provide arg bruh", message)
		return nil
	}
	connectionCount, err := strconv.Atoi(connectionCountString)
	if err != nil {
		engine.L().Errorf("Error parsing torrent connection count: %s", err.Error())
		engine.SendMessage(b, fmt.Sprintf("Error parsing connection count: %s", err.Error()), message)
	}
	if connectionCount <= 0 {
		engine.L().Errorf("Error setting connection count: thread count must be above 0")
		engine.SendMessage(b, "Error setting connection count: thread count must be above 0", message)
		return nil
	}
	previous := engine.SetTorrentClientConnections(connectionCount)
	engine.L().Infof("torrent client connection limit changed %d -> %d", previous, connectionCount)
	engine.SendMessage(b, fmt.Sprintf("Torrent client connection limit changed %d -> %d", previous, connectionCount), message)
	return nil
}

func GetMirrorMessageHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	gid := utils.ParseMessageArgs(message.Text)
	dl := engine.GetMirrorByGid(gid)
	if dl == nil {
		engine.SendMessage(b, "mirror does not exist with this GID", message)
		return nil
	}
	var mirrorMessage *gotgbot.Message
	listener := dl.GetListener()
	cloneListener := dl.GetCloneListener()
	if listener != nil {
		mirrorMessage = listener.Update.Message
	} else {
		mirrorMessage = cloneListener.Update.Message
	}
	out := ""
	out += fmt.Sprintf("MessageText: <code>%s</code>\n", mirrorMessage.Text)
	out += fmt.Sprintf("FromName: <code>%s %s</code>\n", mirrorMessage.From.FirstName, mirrorMessage.From.LastName)
	out += fmt.Sprintf("FromID: <code>%d</code>\n", mirrorMessage.From.Id)
	if mirrorMessage.From.Username != "" {
		out += fmt.Sprintf("FromUsername: @%s\n", mirrorMessage.From.Username)
	}
	out += fmt.Sprintf("DateAndTime: <code>%s</code>\n", time.Unix(mirrorMessage.Date, 0))
	out += fmt.Sprintf("ChatID: <code>%d</code>\n", mirrorMessage.Chat.Id)
	messageLink := mirrorMessage.GetLink()
	if messageLink != "" {
		out += fmt.Sprintf("MessageLink: <a href='%s'>here</a>", messageLink)
	}
	if mirrorMessage.ReplyToMessage != nil && mirrorMessage.ReplyToMessage.Document != nil {
		_, err := b.SendDocument(message.Chat.Id, mirrorMessage.ReplyToMessage.Document.FileId, &gotgbot.SendDocumentOpts{
			ReplyToMessageId: message.MessageId,
			Caption:          out,
			ParseMode:        "HTML",
		})
		if err != nil {
			engine.L().Errorf("error occured while sending document to %s:%d - %v", message.Chat.Title, message.Chat.Id, err)
			engine.SendMessage(b, "internal error occurred when sending document, check logs", message)
		}
	} else {
		engine.SendMessage(b, out, message)
	}
	return nil
}

func AddDDLScriptHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	args := ctx.Args()
	if len(args) < 2 {
		engine.SendMessage(b, "/cmd {regex}\n{}code", message)
		return nil
	}
	regex := args[1]
	data := strings.SplitN(message.Text, "\n", 2)
	if len(data) < 2 {
		engine.SendMessage(b, "/cmd {regex}\n{}code", message)
		return nil
	}
	script := data[1]
	err := db.UpdateExtractor(regex, script)
	if err != nil {
		engine.SendMessage(b, err.Error(), message)
		return nil
	}
	engine.SendMessage(b, fmt.Sprintf("script with regex %s has been added", regex), message)
	return nil
}

func GetAllDDLsHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	out := ""
	extractors, err := db.GetExtractors()
	if err != nil {
		engine.SendMessage(b, err.Error(), message)
		return nil
	}
	for k, _ := range extractors {
		out += fmt.Sprintf("<code>%s</code>\n", k)
	}
	if out == "" {
		engine.SendMessage(b, "no extractor found", message)
		return nil
	}
	engine.SendMessage(b, out, message)
	return nil
}

func GetDLLCodeByRegexHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	regex := utils.ParseMessageArgs(message.Text)
	if regex == "" {
		engine.SendMessage(b, "/cmd {regex}", message)
		return nil
	}
	extractors, err := db.GetExtractors()
	if err != nil {
		engine.SendMessage(b, err.Error(), message)
		return nil
	}
	script, ok := extractors[regex]
	if !ok {
		engine.SendMessage(b, "extractor not found", message)
		return nil
	}
	engine.SendMessage(b, fmt.Sprintf("<code>%s</code>", script), message)
	return nil
}

func RemoveDDLHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	args := strings.SplitN(message.Text, " ", 2)
	if len(args) < 2 {
		engine.SendMessage(b, "/cmd {regex}", message)
		return nil
	}
	regex := args[1]
	err := db.RemoveExtractor(regex)
	if err != nil {
		engine.SendMessage(b, err.Error(), message)
		return nil
	}
	engine.SendMessage(b, fmt.Sprintf("script with regex %s has been removed", regex), message)
	return nil
}

func AddSecretHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	args := strings.SplitN(message.Text, " ", 2)
	if len(args) < 2 {
		engine.SendMessage(b, "/cmd {secret}={value}", message)
		return nil
	}
	if !strings.Contains(args[1], "=") {
		engine.SendMessage(b, "/cmd {secret}={value}", message)
		return nil
	}
	secret := strings.SplitN(args[1], "=", 2)
	secretKey := secret[0]
	secretValue := secret[1]
	err := db.UpdateSecret(secretKey, secretValue)
	if err != nil {
		engine.SendMessage(b, err.Error(), message)
		return nil
	}
	engine.SendMessage(b, fmt.Sprintf("secret with key %s has been added", secretKey), message)
	return nil
}

func RemoveSecretHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	args := strings.SplitN(message.Text, " ", 2)
	if len(args) < 2 {
		engine.SendMessage(b, "/cmd {secret}", message)
		return nil
	}
	secretKey := args[1]
	err := db.RemoveSecret(secretKey)
	if err != nil {
		engine.SendMessage(b, err.Error(), message)
		return nil
	}
	engine.SendMessage(b, fmt.Sprintf("secret with key %s has been removed", secretKey), message)
	return nil
}

func GetAllSecretsHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	out := ""
	secrets, err := db.GetSecrets()
	if err != nil {
		engine.SendMessage(b, err.Error(), message)
		return nil
	}
	for k, v := range secrets {
		out += fmt.Sprintf("<code>%s=%s</code>\n", k, v)
	}
	if out == "" {
		engine.SendMessage(b, "no secret found", message)
		return nil
	}
	engine.SendMessage(b, out, message)
	return nil
}

func GetLinkHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	message := ctx.EffectiveMessage
	args := ctx.Args()
	if len(args) < 2 {
		engine.SendMessage(b, "/cmd {link}", message)
		return nil
	}
	extractors, err := db.GetExtractors()
	if err != nil {
		engine.SendMessage(b, err.Error(), message)
		return nil
	}
	secrets, err := db.GetSecrets()
	if err != nil {
		engine.SendMessage(b, err.Error(), message)
		return nil
	}
	ddl, err := engine.ExtractDDL(args[1], extractors, secrets, b, ctx)
	if err != nil {
		engine.SendMessage(b, fmt.Sprintf("extraction failed: %v", err), message)
		return nil
	}
	engine.SendMessage(b, ddl, message)
	return nil
}

func LoadConfigurationHandlers(updater *ext.Updater, l *zap.SugaredLogger) {
	defer l.Info("Configuration Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("setgotdthreads", SetGotdDownloadThreadsCountHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("getgotdthreads", GetGotdDownloadThreadsCountHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("megalogin", MegaLoginHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("mirrormsg", GetMirrorMessageHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("settorrentconnections", SetTorrentClientConnectionsHandlers))
	updater.Dispatcher.AddHandler(handlers.NewCommand("addscript", AddDDLScriptHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("removescript", RemoveDDLHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("getallscripts", GetAllDDLsHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("getscript", GetDLLCodeByRegexHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("addsecret", AddSecretHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("removesecret", RemoveSecretHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("getsecrets", GetAllSecretsHandler))
	updater.Dispatcher.AddHandler(handlers.NewCommand("getlink", GetLinkHandler))
}
