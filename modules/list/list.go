package list

import (
	"MirrorBotGo/db"
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"
	"fmt"
	"net/http"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"go.uber.org/zap"
)

func ListHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !db.IsAuthorized(ctx.EffectiveMessage) {
		return nil
	}
	var out *gotgbot.Message
	var err error
	message := ctx.EffectiveMessage
	name := utils.ParseMessageArgs(message.Text)
	if name == "" {
		engine.SendMessage(b, "Provide search query.", message)
		return nil
	}
	outMsg := ""
	client := engine.NewTransferServiceClient(utils.GetTransferServiceURL(), &http.Client{})
	res, err := client.ListFiles(&engine.ListFilesRequest{
		ParentID: utils.GetGDriveParentId(),
		Name:     name,
		Count:    20,
	})
	if err != nil {
		engine.SendMessage(b, err.Error(), message)
		return nil
	}
	files := res.Files
	if len(files) == 0 {
		outMsg += "No Result Found."
	}
	for _, file := range files {
		if engine.IsGDriveFolder(file.MimeType) {
			outMsg += fmt.Sprintf("⁍ <a href='%s'>%s</a> (folder)", engine.FormatGDriveLink(file.Id), strings.ReplaceAll(file.Name, "'", ""))
		} else {
			outMsg += fmt.Sprintf("⁍ <a href='%s'>%s</a> (%s)", engine.FormatGDriveLink(file.Id), strings.ReplaceAll(file.Name, "'", ""), utils.GetHumanBytes(file.Size))
		}
		inUrl := utils.GetIndexUrl()
		if inUrl != "" {
			inUrl = inUrl + "/" + strings.ReplaceAll(file.Name, "'", "")
			if engine.IsGDriveFolder(file.MimeType) {
				inUrl += "/"
			}
			outMsg += fmt.Sprintf(" | <a href='%s'>Index url</a>", inUrl)
		}
		outMsg += "\n"
	}
	out = engine.SendMessage(b, outMsg, message)
	engine.AutoDeleteMessages(b, utils.GetAutoDeleteTimeOut(), out, message)
	return nil
}

func LoadListHandler(updater *ext.Updater, l *zap.SugaredLogger) {
	defer l.Info("List Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("list", ListHandler))
}
