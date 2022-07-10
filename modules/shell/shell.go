package shell

import (
	"MirrorBotGo/engine"
	"MirrorBotGo/utils"
	"os/exec"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"go.uber.org/zap"
)

type ShellOutputWriter struct {
	content   string
	completed bool
}

func (s *ShellOutputWriter) Write(p []byte) (int, error) {
	data := string(p)
	s.content += data
	return len(p), nil
}

func (s *ShellOutputWriter) GetContent() string {
	return s.content
}

func RunCommand(command string, writer *ShellOutputWriter) error {
	defer func() {
		writer.completed = true
	}()
	var cmd *exec.Cmd
	cmd = exec.Command("bash", "-c", "stdbuf -o0 "+command)
	cmd.Stdout = writer
	cmd.Stderr = writer
	return cmd.Run()
}

func UpdateMessage(data string, b *gotgbot.Bot, outMsg *gotgbot.Message) {
	if len(data) > 3800 {
		data = string(data[len(data)-3800:])
	}
	if data == "" {
		data = "No output"
	}
	if data != outMsg.Text {
		engine.EditMessage(b, data, outMsg)
		outMsg.Text = data
	}
}

func ShellHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserOwner(ctx.EffectiveUser.Id) {
		return nil
	}
	m := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	args := strings.SplitN(m.Text, " ", 2)
	if len(args) < 2 {
		engine.SendMessage(b, "Provide proper arguments", m)
		return nil
	}
	cmd := args[1]
	outMsg, err := b.SendMessage(chat.Id, "Executing..", nil)
	if err != nil {
		engine.SendMessage(b, err.Error(), m)
		return nil
	}
	output := &ShellOutputWriter{}
	go func() {
		for range time.Tick(3 * time.Second) {
			if output.completed {
				return
			}
			UpdateMessage(output.GetContent(), b, outMsg)
		}
	}()
	RunCommand(cmd, output)
	UpdateMessage(output.GetContent(), b, outMsg)
	b.SendMessage(chat.Id, "Execution completed", nil)
	return nil
}

func LoadShellHandlers(updater *ext.Updater, l *zap.SugaredLogger) {
	defer l.Info("Ping Module Loaded.")
	updater.Dispatcher.AddHandler(handlers.NewCommand("sh", ShellHandler))
}
