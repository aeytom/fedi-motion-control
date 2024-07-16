package main

import (
	"github.com/aeytom/fedi-motion-control/app"
	"github.com/aeytom/fedi-motion-control/toot"
)

func main() {
	cfg := app.LoadConfig()

	cfg.Motion.Init(cfg.Logger())

	m := toot.Init(&cfg.Mastodon, cfg.Logger())
	m.RegisterMotion(&cfg.Motion)
	m.ProcessNotifications()

	go cfg.Motion.ListenMotionWebhook(m)
	m.WatchNotifications()
}
