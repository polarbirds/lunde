package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/polarbirds/lunde/internal/command"
	"github.com/polarbirds/lunde/internal/meme"
	"github.com/polarbirds/lunde/pkg/reddit"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var (
	Token string
)

func init() {

	flag.StringVar(&Token, "t", "", "Bot Token")
	flag.Parse()
}

func main() {
	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	dg.AddHandler(messageCreate)

	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if strings.Contains(m.Content, "!") {
		source, scheme, argument, err := command.GetCommand(m.Content)
		if err != nil {
			log.Error(err)
			s.UpdateStatus(0, err.Error())
			return
		}

		var discErr error

		if source == "reddit" {
			var msg meme.Post
			msg, err = reddit.GetMeme(scheme, argument)
			if msg.Embed.Title != "" {
				_, discErr = s.ChannelMessageSendEmbed(m.ChannelID, &msg.Embed)
			} else {
				_, discErr = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s\n%s", msg.Title, msg.Message))
			}
		} else if source == "shrug" {
			discErr = s.ChannelMessageDelete(m.ChannelID, m.ID)
			_, discErr = s.ChannelMessageSend(m.ChannelID, "¯\\_(ツ)_/¯")
		} else if source == "say" {
			index := strings.Index(m.Content, "!say")
			text := strings.Trim(m.Content[index+len("!say"):], " ")
			if len(text) > 0 {
				discErr = s.ChannelMessageDelete(m.ChannelID, m.ID)
				_, discErr = s.ChannelMessageSendTTS(m.ChannelID, text)
			}
		} else if source == "pumpit" {
			_, discErr = s.ChannelMessageSend(m.ChannelID, "https://cdn.discordapp.com/attachments/145942475805032449/471311185782898698/pumpItInTheClub.gif")
		} else if source == "status" {
			index := strings.Index(m.Content, "!status")
			discErr = s.UpdateStatus(0, m.Content[index+len("!status"):])
		} else {
			err = errors.New(fmt.Sprintf("unsupported source %q", source))
		}

		if err != nil {
			log.Error(err)
			s.UpdateStatus(0, err.Error())
		}

		if discErr != nil {
			log.Error(discErr)
		}
	}
}
