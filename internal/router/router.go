package router

import (
	"github.com/bonus2k/xray-tlg/internal/handlers"
	"github.com/go-telegram/bot"
)

func GetRouter(h *handlers.Handler) []bot.Option {
	return []bot.Option{
		bot.WithDefaultHandler(h.DefaultHandler),
		bot.WithCallbackQueryDataHandler("speedtest", bot.MatchTypePrefix, h.SpeedtestHandler),
		bot.WithCallbackQueryDataHandler("ls_config", bot.MatchTypePrefix, h.ListConfigXrayHandler),
		bot.WithCallbackQueryDataHandler("main", bot.MatchTypePrefix, h.MainHandler),
		bot.WithCallbackQueryDataHandler("cp_", bot.MatchTypePrefix, h.CopyConfigXrayHandler),
		bot.WithCallbackQueryDataHandler("restart", bot.MatchTypePrefix, h.RestartXrayHandler),
	}
}
