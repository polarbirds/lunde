package server

import (
	"errors"
	"flag"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/haraldfw/cfger"
	"github.com/polarbirds/lunde/pkg/remind"
	log "github.com/sirupsen/logrus"
)

type execution struct {
	messageAuthorID  string
	replyID          string
	messageChannelID string
	messageID        string
}

// LundeServer is the config and main server
type LundeServer struct {
	Token           string `yaml:"token"`
	RemindsFilePath string `yaml:"remindsFile"`

	session      *discordgo.Session
	lastExecs    map[string]map[string]execution
	reminder     remind.Reminder
	lastMessages map[string]*discordgo.Message
}

// New creates a new server instance with initialized variables
func New() (srv LundeServer, err error) {
	srv = LundeServer{
		lastExecs:    make(map[string]map[string]execution),
		lastMessages: make(map[string]*discordgo.Message),
	}

	_, err = cfger.ReadStructuredCfgRecursive("env::CONFIG", &srv)
	if err != nil {
		return
	}

	if srv.Token == "" {
		err = errors.New("blank token value in config")
		return
	}

	srv.Token = strings.TrimSpace(srv.Token)

	if srv.RemindsFilePath == "" {
		defaultRemindsFilePath := "./reminds.json"
		log.Warnf("blank remindsFile value in config, using %s", defaultRemindsFilePath)
		srv.RemindsFilePath = defaultRemindsFilePath
		return
	}

	return
}

// AddHandlers registers the handlers to the discord-session
// If a handler is not registered in this function, the handler will not be called
func (srv *LundeServer) AddHandlers(s *discordgo.Session) {
	s.AddHandler(srv.reddit)
}

// Initialize Initializes the server with the given session. May panic if given session is nil
func (srv *LundeServer) Initialize(s *discordgo.Session) error {
	srv.session = s

	rmd := remind.Reminder{
		DiscordSession: s,
	}

	err := rmd.Start()
	if err != nil {
		return err
	}

	return nil
}

type MessageWriter struct {
	channelID  string
	userID     string
	usePrivate bool
	session    *discordgo.Session
}

func (mw MessageWriter) Write(p []byte) (n int, err error) {
	msg := string(p)
	cid := mw.channelID
	if mw.usePrivate {
		var pc *discordgo.Channel
		pc, err = mw.session.UserChannelCreate(mw.userID)
		if err != nil {
			return
		}
		cid = pc.ID
	}
	_, err = mw.session.ChannelMessageSend(cid, msg)
	return
}

func commandNameIs(message string, commandName string) bool {
	if len(message) <= 1 {
		return false
	}

	splits := strings.Split(message[1:], " ")
	return splits[0] == commandName
}

func (srv *LundeServer) reddit(_ *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == srv.session.State.User.ID {
		// ignore because this message was sent by me
		return
	}
	if len(m.Content) == 0 || m.Content[0] != '!' {
		// this is not a command, ignore it
		return
	}

	mw := MessageWriter{
		channelID: m.ChannelID,
		userID:    m.Author.ID,
		session:   srv.session,
	}

	fs := flag.NewFlagSet("reddit", flag.ContinueOnError)
	fs.SetOutput(&mw)
	err := fs.Parse([]string{m.Content})

	// positional args
	sortAlgo := fs.Arg(0)
	sub := fs.Arg(1)
	postIndexStr := fs.Arg(2) // optional

	if sortAlgo == "" || sub == "" {
		fs.Usage()
		return
	}

	postIndex := 0
	if postIndexStr != "" {
		postIndex, err = strconv.Atoi(postIndexStr)
		if err != nil {
			mw.usePrivate = true
			fs.Usage()
			return
		}
	}
	log.Info(postIndex)

	if err != nil {
		log.Warn(err)
	}
}

// ------------------------------

// revive:disable-next-line:cyclomatic
// func (srv *lundeServer) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
// 	if m.Author.ID == s.State.User.ID {
// 		return
// 	}

// 	if !strings.HasPrefix(m.Content, "!") {
// 		lastMessage[m.ChannelID] = m.Message
// 		return
// 	}

// 	source, scheme, argument, err := command.GetCommand(m.Content)
// 	if err != nil {
// 		log.Error(err)
// 		return
// 	}

// 	var discErr error
// 	var reply *discordgo.Message
// 	switch strings.ToLower(source) {
// 	case "reddit":
// 		var msg meme.Post
// 		msg, err = reddit.GetMeme(scheme, argument)
// 		if err != nil {
// 			break
// 		}

// 		c, err := s.Channel(m.ChannelID)
// 		if err != nil {
// 			break
// 		}
// 		if msg.NSFW && !c.NSFW {
// 			reply, discErr = s.ChannelMessageSend(m.ChannelID,
// 				fmt.Sprintf("no %s, this is a christian channel", m.Author.Username))
// 			break
// 		}

// 		if msg.Embed.Title != "" {
// 			reply, discErr = s.ChannelMessageSendEmbed(m.ChannelID, &msg.Embed)
// 		} else {
// 			reply, discErr = s.ChannelMessageSend(
// 				m.ChannelID, fmt.Sprintf("%s\n%s", msg.Title, msg.Message))
// 		}
// 	case "xkcd":
// 		var msg meme.Post
// 		msg, err = xkcd.GetMeme(scheme, argument)
// 		if msg.Embed.Title != "" {
// 			reply, discErr = s.ChannelMessageSendEmbed(m.ChannelID, &msg.Embed)
// 		} else {
// 			reply, discErr = s.ChannelMessageSend(
// 				m.ChannelID, fmt.Sprintf("%s\n%s", msg.Title, msg.Message))
// 		}
// 	case "shrug":
// 		discErr = s.ChannelMessageDelete(m.ChannelID, m.ID)
// 		reply, discErr = s.ChannelMessageSend(m.ChannelID, "¯\\_(ツ)_/¯")
// 	case "say":
// 		index := strings.Index(strings.ToLower(m.Content), "!say")
// 		text := strings.Trim(m.Content[index+len("!say"):], " ")
// 		if len(text) > 0 {
// 			discErr = s.ChannelMessageDelete(m.ChannelID, m.ID)
// 			reply, discErr = s.ChannelMessageSendTTS(m.ChannelID, text)
// 		}
// 	case "pumpit":
// 		reply, discErr = s.ChannelMessageSend(m.ChannelID,
// 			"https://cdn.discordapp.com/"+
// 				"attachments/145942475805032449/471311185782898698/pumpItInTheClub.gif")
// 	case "status":
// 		index := strings.Index(strings.Trim(strings.ToLower(m.Content), " "), "!status")
// 		discErr = s.UpdateStatus(0, m.Content[index+len("!status"):])
// 	case "remind":
// 		err = reminder.CreateRemindStrict(scheme, argument, m.ChannelID)
// 		if err == nil {
// 			if err2 := s.MessageReactionAdd(m.ChannelID, m.ID, "✅"); err2 != nil {
// 				log.Error(err2)
// 				err = errors.New("remind added, but failed to add reaction. Contact @servermonkey")
// 			}
// 		}
// 	case "selfdestruct", "kys", "die", "kill", "stop", "quit", "killmyself", "killyourself":
// 		reply, discErr = s.ChannelMessageSendTTS(
// 			m.ChannelID,
// 			fmt.Sprintf("I'm sorry, %s. I'm afraid I can't do that.", m.Author.Username))
// 	case "undo":
// 		lastExec, exists := srv.getLastExec(m.ChannelID, m.Author.ID)
// 		if !exists {
// 			break
// 		}
// 		log.Info("undo lastExec: ", lastExec)
// 		if m.Author.ID == lastExec.messageAuthorID &&
// 			m.ChannelID == lastExec.messageChannelID {
// 			if lastExec.messageID != "" {
// 				s.ChannelMessageDelete(lastExec.messageChannelID, lastExec.messageID)
// 			}
// 			if lastExec.replyID != "" {
// 				s.ChannelMessageDelete(lastExec.messageChannelID, lastExec.replyID)
// 			}
// 			s.ChannelMessageDelete(m.ChannelID, m.Message.ID)
// 		}
// 	case "text":
// 		lm, ok := lastMessage[m.ChannelID]
// 		lmc := ""
// 		if ok {
// 			lmc = lm.Content
// 		}
// 		reply, err = text.Generate(s, m, scheme, argument, lmc)
// 		if err == nil {
// 			discErr = s.ChannelMessageDelete(m.ChannelID, m.ID)
// 		}
// 	case "slap":
// 		reply, err, discErr = slap.Generate(scheme, argument, s, m)
// 		if err == nil {
// 			discErr = s.ChannelMessageDelete(m.ChannelID, m.ID)
// 		}
// 	case "define":
// 		reply, err, discErr = define.Define(scheme, s, m)
// 	default:
// 		return
// 	}

// 	lastExec := execution{
// 		messageID:        m.Message.ID,
// 		messageChannelID: m.Message.ChannelID,
// 		messageAuthorID:  m.Message.Author.ID,
// 	}

// 	if reply != nil {
// 		lastExec.replyID = reply.ID
// 	}

// 	srv.putLastExec(lastExec)

// 	srv.reportErrorIfExists(err, m, s)
// 	srv.reportErrorIfExists(discErr, m, s)
// }

// func (srv *lundeServer) getLastExec(chanID string, userID string) (command execution, exists bool) {
// 	lastExecsInChan, exists := srv.lastExecs[chanID]
// 	if !exists {
// 		return
// 	}

// 	command = lastExecsInChan[userID]
// 	return
// }

// func (srv *lundeServer) putLastExec(lastExec execution) {
// 	// check if map for channel exists, if not create it
// 	_, exists := srv.lastExecs[lastExec.messageChannelID]
// 	if !exists {
// 		srv.lastExecs[lastExec.messageChannelID] = make(map[string]execution)
// 		return
// 	}

// 	srv.lastExecs[lastExec.messageChannelID][lastExec.messageAuthorID] = lastExec
// }

// func (srv *lundeServer) reportErrorIfExists(
// 	repErr error,
// 	m *discordgo.MessageCreate,
// 	s *discordgo.Session,
// ) {
// 	if repErr == nil {
// 		return
// 	}

// 	if err2 := s.MessageReactionAdd(m.ChannelID, m.ID, "❌"); err2 != nil {
// 		log.Error(err2)
// 	}

// 	log.Info(repErr)
// 	dmChannel, err := s.UserChannelCreate(m.Author.ID)
// 	if err != nil {
// 		log.Error(err)
// 		return
// 	}

// 	log.Info("sending text ", repErr.Error())
// 	_, err = s.ChannelMessageSend(
// 		dmChannel.ID,
// 		fmt.Sprintf("Command was: %s\nError occurred: %s", m.Content, repErr.Error()),
// 	)
// 	if err != nil {
// 		log.Error(err)
// 	}
// }
