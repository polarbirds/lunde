package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/polarbirds/lunde/internal/command"
	"github.com/polarbirds/lunde/internal/meme"
	"github.com/polarbirds/lunde/pkg/reddit"
	"github.com/polarbirds/lunde/pkg/remind"
	"github.com/polarbirds/lunde/pkg/text"
	"github.com/polarbirds/lunde/pkg/xkcd"
	log "github.com/sirupsen/logrus"
)

type execution struct {
	messageAuthorID  string
	replyID          string
	messageChannelID string
	messageID        string
}

var (
	reminder    remind.Reminder
	lastExec    execution
	lastMessage map[string]*discordgo.Message
)

func main() {
	lastMessage = make(map[string]*discordgo.Message)
	var token string
	flag.StringVar(&token, "t", "", "Bot Token")
	flag.Parse()

	dg, err := discordgo.New("Bot " + token)
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

	reminder = remind.Reminder{
		DiscordSession: dg,
	}

	reminder.Start()

	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

// revive:disable-next-line:cyclomatic
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if !strings.HasPrefix(m.Content, "!") {
		lastMessage[m.ChannelID] = m.Message
		return
	}

	source, scheme, argument, err := command.GetCommand(m.Content)
	if err != nil {
		log.Error(err)
		return
	}

	var discErr error
	var reply *discordgo.Message
	switch strings.ToLower(source) {
	case "reddit":
		var msg meme.Post
		msg, err = reddit.GetMeme(scheme, argument)
		if msg.Embed.Title != "" {
			reply, discErr = s.ChannelMessageSendEmbed(m.ChannelID, &msg.Embed)
		} else {
			reply, discErr = s.ChannelMessageSend(
				m.ChannelID, fmt.Sprintf("%s\n%s", msg.Title, msg.Message))
		}
	case "xkcd":
		var msg meme.Post
		msg, err = xkcd.GetMeme(scheme, argument)
		if msg.Embed.Title != "" {
			reply, discErr = s.ChannelMessageSendEmbed(m.ChannelID, &msg.Embed)
		} else {
			reply, discErr = s.ChannelMessageSend(
				m.ChannelID, fmt.Sprintf("%s\n%s", msg.Title, msg.Message))
		}
	case "shrug":
		discErr = s.ChannelMessageDelete(m.ChannelID, m.ID)
		reply, discErr = s.ChannelMessageSend(m.ChannelID, "¯\\_(ツ)_/¯")
	case "say":
		index := strings.Index(strings.ToLower(m.Content), "!say")
		text := strings.Trim(m.Content[index+len("!say"):], " ")
		if len(text) > 0 {
			discErr = s.ChannelMessageDelete(m.ChannelID, m.ID)
			reply, discErr = s.ChannelMessageSendTTS(m.ChannelID, text)
		}
	case "pumpit":
		reply, discErr = s.ChannelMessageSend(m.ChannelID,
			"https://cdn.discordapp.com/"+
				"attachments/145942475805032449/471311185782898698/pumpItInTheClub.gif")
	case "status":
		index := strings.Index(strings.Trim(strings.ToLower(m.Content), " "), "!status")
		discErr = s.UpdateStatus(0, m.Content[index+len("!status"):])
	case "remind":
		err = reminder.CreateRemindStrict(scheme, argument, m.ChannelID)
	case "selfdestruct", "kys", "die", "kill", "stop", "quit", "killmyself", "killyourself":
		reply, discErr = s.ChannelMessageSendTTS(
			m.ChannelID,
			fmt.Sprintf(" I'm sorry, %s. I'm afraid I can't do that.", m.Author.Username))
	case "undo":
		log.Info("undo lastExec: ", lastExec)
		if m.Author.ID == lastExec.messageAuthorID &&
			m.ChannelID == lastExec.messageChannelID {
			if lastExec.messageID != "" {
				s.ChannelMessageDelete(lastExec.messageChannelID, lastExec.messageID)
			}
			if lastExec.replyID != "" {
				s.ChannelMessageDelete(lastExec.messageChannelID, lastExec.replyID)
			}
			s.ChannelMessageDelete(m.ChannelID, m.Message.ID)
		}
	case "text":
		lm, ok := lastMessage[m.ChannelID]
		lmc := ""
		if ok {
			lmc = lm.Content
		}
		reply, err = text.Generate(s, m, scheme, argument, lmc)
		if err == nil {
			discErr = s.ChannelMessageDelete(m.ChannelID, m.ID)
		}
	}

	lastExec = execution{}
	lastExec.messageID = m.Message.ID
	lastExec.messageChannelID = m.Message.ChannelID
	lastExec.messageAuthorID = m.Message.Author.ID
	if reply != nil {
		lastExec.replyID = reply.ID
	}

	if err != nil {
		log.Error(err)
		s.UpdateStatus(0, err.Error())
	} else {
		s.UpdateStatus(0, "")
	}

	if discErr != nil {
		log.Error(discErr)
	}
}
