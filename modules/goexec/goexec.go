package goexec

import (
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/PaulSonOfLars/gotgbot"
	"github.com/PaulSonOfLars/gotgbot/ext"
	"github.com/PaulSonOfLars/gotgbot/handlers"
	"github.com/mattn/anko/env"
	"github.com/mattn/anko/vm"
	"go.uber.org/zap"
)

func ExecHandler(b ext.Bot, u *gotgbot.Update) error {
	if !utils.IsUserOwner(u.EffectiveUser.Id) {
		return nil
	}
	message := u.EffectiveMessage
	code := utils.ParseMessageArgs(message.Text)
	if code == "" {
		engine.SendMessage(b, "Provide Code to execute.", message)
		return nil
	}
	en := env.NewEnv()
	en.Define("print", func(out interface{}) {
		var err error
		if reflect.TypeOf(out).Kind() == reflect.Ptr {
			out, err = json.MarshalIndent(out, "", " ")
			if err != nil {
				b.Logger.Info(err)
			}
		}
		str := SanitizeString(fmt.Sprintf("%s", out))
		if len(str) > utils.GetMaxMessageTextLength() {
			SendAsDocument(b, str, message.Chat.Id)
		} else if str != "" {
			engine.SendMessage(b, str, message)
		} else {
			engine.SendMessage(b, "No output", message)
		}
	})
	en.Define("message", message)
	en.Define("update", u)
	en.Define("bot", b)
	en.Define("Send", engine.SendMessage)
	en.Define("Delete", engine.DeleteMessage)
	out, err := vm.Execute(en, nil, code)
	if err != nil {
		engine.SendMessage(b, err.Error(), message)
	}
	if out != nil {
		fmt.Println(out)
	}
	return nil
}

func SanitizeString(str string) string {
	return strings.ReplaceAll(strings.ReplaceAll(str, "<", ""), ">", "")
}

func SendAsDocument(b ext.Bot, str string, chatId int) {
	f, _ := os.Create("exec.txt")
	f.WriteString(str)
	f.Sync()
	reader, _ := os.Open("exec.txt")
	file := b.NewFileReader("exec.txt", reader)
	b.SendDocument(chatId, file)
}

func LoadExecHandler(updater *gotgbot.Updater, l *zap.SugaredLogger) {
	defer l.Info("Exec Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("go", ExecHandler))
}
