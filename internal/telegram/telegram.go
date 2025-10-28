package telegram

import (
	"time"

	"github.com/spf13/cast"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
	"gopkg.in/telebot.v3/middleware"
)

type Telegram struct {
	logger   *zap.Logger
	settings Settings
	client   *tele.Bot
}

type Option func(telegram *Telegram)

func NewTelegram(logger *zap.Logger, settings Settings, options ...Option) (*Telegram, error) {

	poller := &tele.LongPoller{Timeout: 10 * time.Second}

	userMiddleware := tele.NewMiddlewarePoller(poller, func(u *tele.Update) bool {

		return true
	})

	client, err := tele.NewBot(tele.Settings{
		ParseMode: tele.ModeMarkdown,
		Token:     settings.Token,
		Poller:    userMiddleware,
		Client:    settings.Client,
	})
	if err != nil {
		return nil, err
	}

	//client.Use(middleware.Logger())
	client.Use(middleware.AutoRespond())

	var (
		menu = &tele.ReplyMarkup{ResizeKeyboard: true}
	)

	err = client.SetCommands([]tele.Command{
		{Text: "/start", Description: "启动机器人，显示主菜单"},
		{Text: "/help", Description: "获取帮助信息"},
		{Text: "/status", Description: "查看系统状态"},
	})
	if err != nil {
		return nil, err
	}

	menu.Reply()

	bot := &Telegram{
		logger:   logger,
		settings: settings,
		client:   client,
	}

	for _, option := range options {
		option(bot)
	}

	return bot, nil
}

func (r *Telegram) Start() {
	go r.client.Start()
}

func (r *Telegram) Notify(chatId, msg string) error {
	_chatId := cast.ToInt(chatId)
	_, err := r.client.Send(tele.ChatID(_chatId), msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown})
	return err
}
