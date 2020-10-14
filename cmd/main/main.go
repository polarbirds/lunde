package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/polarbirds/lunde/internal/command"
	"github.com/polarbirds/lunde/internal/define"
	"github.com/polarbirds/lunde/internal/meme"
	"github.com/polarbirds/lunde/internal/server"
	"github.com/polarbirds/lunde/internal/slap"
	"github.com/polarbirds/lunde/pkg/reddit"
	"github.com/polarbirds/lunde/pkg/text"
	"github.com/polarbirds/lunde/pkg/xkcd"
	log "github.com/sirupsen/logrus"
)

// revive:disable-next-line:cyclomatic
func (srv *lundeServer) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
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
		if err != nil {
			break
		}

		c, err := s.Channel(m.ChannelID)
		if err != nil {
			break
		}
		if msg.NSFW && !c.NSFW {
			reply, discErr = s.ChannelMessageSend(m.ChannelID,
				fmt.Sprintf("no %s, this is a christian channel", m.Author.Username))
			break
		}

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
		if err == nil {
			if err2 := s.MessageReactionAdd(m.ChannelID, m.ID, "✅"); err2 != nil {
				log.Error(err2)
				err = errors.New("remind added, but failed to add reaction. Contact @servermonkey")
			}
		}
	case "selfdestruct", "kys", "die", "kill", "stop", "quit", "killmyself", "killyourself":
		reply, discErr = s.ChannelMessageSendTTS(
			m.ChannelID,
			fmt.Sprintf("I'm sorry, %s. I'm afraid I can't do that.", m.Author.Username))
	case "undo":
		lastExec, exists := srv.getLastExec(m.ChannelID, m.Author.ID)
		if !exists {
			break
		}
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
	case "slap":
		reply, err, discErr = slap.Generate(scheme, argument, s, m)
		if err == nil {
			discErr = s.ChannelMessageDelete(m.ChannelID, m.ID)
		}
	case "define":
		reply, err, discErr = define.Define(scheme, s, m)
	default:
		return
	}

	lastExec := execution{
		messageID:        m.Message.ID,
		messageChannelID: m.Message.ChannelID,
		messageAuthorID:  m.Message.Author.ID,
	}

	if reply != nil {
		lastExec.replyID = reply.ID
	}

	srv.putLastExec(lastExec)

	srv.reportErrorIfExists(err, m, s)
	srv.reportErrorIfExists(discErr, m, s)
}

func (srv *lundeServer) getLastExec(chanID string, userID string) (command execution, exists bool) {
	lastExecsInChan, exists := srv.lastExecs[chanID]
	if !exists {
		return
	}

	command = lastExecsInChan[userID]
	return
}

func (srv *lundeServer) putLastExec(lastExec execution) {
	// check if map for channel exists, if not create it
	_, exists := srv.lastExecs[lastExec.messageChannelID]
	if !exists {
		srv.lastExecs[lastExec.messageChannelID] = make(map[string]execution)
		return
	}

	srv.lastExecs[lastExec.messageChannelID][lastExec.messageAuthorID] = lastExec
}

func (srv *lundeServer) reportErrorIfExists(
	repErr error,
	m *discordgo.MessageCreate,
	s *discordgo.Session,
) {
	if repErr == nil {
		return
	}

	if err2 := s.MessageReactionAdd(m.ChannelID, m.ID, "❌"); err2 != nil {
		log.Error(err2)
	}

	log.Info(repErr)
	dmChannel, err := s.UserChannelCreate(m.Author.ID)
	if err != nil {
		log.Error(err)
		return
	}

	log.Info("sending text ", repErr.Error())
	_, err = s.ChannelMessageSend(
		dmChannel.ID,
		fmt.Sprintf("Command was: %s\nError occurred: %s", m.Content, repErr.Error()),
	)
	if err != nil {
		log.Error(err)
	}
}

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

	dg.AddHandler(srv.Handle)

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
