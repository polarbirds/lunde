package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/session"
	"github.com/polarbirds/lunde/internal/command/define"
	"github.com/polarbirds/lunde/internal/command/promote"
	"github.com/polarbirds/lunde/internal/command/reddit"
	"github.com/polarbirds/lunde/internal/command/slap"
	"github.com/polarbirds/lunde/internal/command/text"
	"github.com/polarbirds/lunde/internal/healthcheck"
	"github.com/polarbirds/lunde/internal/server"
	"github.com/sirupsen/logrus"
)

// CommandCreators is the list of handlers of the commands that are active
var CommandCreators = []server.CreateCommand{
	reddit.CreateCommand,
	slap.CreateCommand,
	define.CreateCommand,
	text.CreateCommand,
	promote.CreateCommand,
}

func main() {
	logrus.Info("starting lunde")
	srv, err := server.New()
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Info("creating discord session")
	sess, err := session.New("Bot " + srv.Token)
	if err != nil {
		logrus.Fatalf("error creating Discord session: %v", err)
		return
	}

	logrus.Info("adding handlers and intents")
	sess.AddHandler(srv.MessageCreateHandler)
	sess.AddHandler(srv.HandleCommandInteraction)
	sess.AddHandler(srv.HandleComponentInteraction)

	sess.Gateway.AddIntents(gateway.IntentGuilds)
	sess.Gateway.AddIntents(gateway.IntentGuildMessages)
	sess.Gateway.AddIntents(gateway.IntentGuildMessageReactions)
	sess.Gateway.AddIntents(gateway.IntentDirectMessages)

	logrus.Info("opening discord session")
	err = sess.Open(context.Background())
	if err != nil {
		logrus.Fatalf("error opening connection: %v", err)
		return
	}

	defer sess.Close()

	logrus.Info("initializing server")
	err = srv.Initialize(sess, CommandCreators)
	if err != nil {
		logrus.Fatalf("error initializing server: %v", err)
		return
	}

	deletecommands := flag.Bool(
		"deletecommands",
		false,
		"if true, the program will delete all guild commands on startup, and then exit")

	flag.Parse()
	if deletecommands != nil && *deletecommands {
		logrus.Info("deletecommands flag detected, deleting guild commands...")
		err = srv.DeleteGuildCommands()
		if err != nil {
			logrus.Fatalf("error deleting commands: %v", err)
		}

		logrus.Info("deleted commands, exiting...")
		os.Exit(0)
	}

	go healthcheck.StartHandlerIfEnabled()

	logrus.Info("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	sigRec := <-sc

	logrus.Infof("signal %v received, exiting...", sigRec)
}