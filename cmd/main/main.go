package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/polarbirds/lunde/internal/server"
	log "github.com/sirupsen/logrus"
)

func main() {
	srv, err := server.New()
	if err != nil {
		log.Fatal(err)
	}

	dg, err := discordgo.New("Bot " + srv.Token)
	if err != nil {
		log.Fatal("error creating Discord session, ", err)
		return
	}

	srv.AddHandlers(dg)

	err = dg.Open()
	if err != nil {
		log.Fatal("error opening connection, ", err)
		return
	}

	err = srv.Initialize(dg)

	log.Info("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}
