package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/diamondburned/arikawa/v2/gateway"
	"github.com/diamondburned/arikawa/v2/session"
	"github.com/polarbirds/lunde/internal/server"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.Info("starting lunde")
	srv, err := server.New()
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Info("creating sessions")
	sess, err := session.New("Bot " + srv.Token)
	if err != nil {
		logrus.Fatalf("error creating Discord session: %v", err)
		return
	}

	logrus.Info("adding handlers and intents")
	sess.AddHandler(srv.MessageCreateHandler)

	sess.AddHandler(srv.HandleInteraction)

	sess.Gateway.AddIntents(gateway.IntentGuilds)
	sess.Gateway.AddIntents(gateway.IntentGuildMessages)
	sess.Gateway.AddIntents(gateway.IntentGuildMessageReactions)

	logrus.Info("opening session with discord")
	err = sess.Open()
	if err != nil {
		logrus.Fatalf("error opening connection: %v", err)
		return
	}

	defer sess.Close()

	logrus.Info("initializing server")
	err = srv.Initialize(sess)
	if err != nil {
		logrus.Fatalf("error initializing server: %v", err)
		return
	}

	logrus.Info("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	sigRec := <-sc

	logrus.Infof("signal %v received, exiting", sigRec)
}
