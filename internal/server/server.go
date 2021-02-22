package server

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/diamondburned/arikawa/v2/api"
	"github.com/diamondburned/arikawa/v2/discord"
	"github.com/diamondburned/arikawa/v2/gateway"
	"github.com/diamondburned/arikawa/v2/session"
	"github.com/haraldfw/cfger"
	"github.com/polarbirds/lunde/internal/define"
	"github.com/polarbirds/lunde/internal/reddit"
	"github.com/polarbirds/lunde/internal/remind"
	"github.com/polarbirds/lunde/internal/slap"
	"github.com/polarbirds/lunde/internal/text"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

type execution struct {
	messageAuthorID  string
	replyID          string
	messageChannelID string
	messageID        string
}

// Server is the config and main server
type Server struct {
	Token   string `yaml:"token" validate:"required"`
	AppID   string `yaml:"appID" validate:"required"`
	GuildID string `yaml:"guildID" validate:"required"`

	RemindsFilePath string `yaml:"remindsFilePath" validate:"required"`

	appIDSnowFlake   discord.AppID
	guildIDSnowFlake discord.GuildID

	sess *session.Session
	// lastExecs    map[string]map[string]execution
	reminder     remind.Reminder
	LastMessages map[discord.ChannelID]*gateway.MessageCreateEvent
}

// New creates a new server instance with initialized variables
func New() (srv Server, err error) {
	srv = Server{
		// lastExecs:    make(map[string]map[string]execution),
		LastMessages: make(map[discord.ChannelID]*gateway.MessageCreateEvent),
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
	srv.AppID = strings.TrimSpace(srv.AppID)
	srv.GuildID = strings.TrimSpace(srv.GuildID)

	appIDUInt64, err := strconv.ParseUint(srv.AppID, 10, 64)
	if err != nil {
		return
	}
	guildIDUInt64, err := strconv.ParseUint(srv.GuildID, 10, 64)
	if err != nil {
		return
	}

	srv.appIDSnowFlake = discord.AppID(appIDUInt64)
	srv.guildIDSnowFlake = discord.GuildID(guildIDUInt64)

	if srv.RemindsFilePath == "" {
		defaultRemindsFilePath := "./reminds.json"
		log.Warnf("blank remindsFile value in config, using %s", defaultRemindsFilePath)
		srv.RemindsFilePath = defaultRemindsFilePath
		return
	}

	return
}

// Initialize Initializes the server with the given session. May panic if given session is nil
func (srv *Server) Initialize(s *session.Session) error {
	srv.sess = s

	rmd := remind.Reminder{
		DiscordSession: s,
	}

	err := rmd.Start()
	if err != nil {
		return err
	}
	srv.reminder = rmd

	cmds := []api.CreateCommandData{
		reddit.CommandData(),
		define.CommandData(),
		slap.CommandData(),
		text.CommandData(),
		remind.CommandData(),
	}
	for _, cmdData := range cmds {
		_, err := srv.sess.CreateGuildCommand(
			srv.appIDSnowFlake,
			srv.guildIDSnowFlake,
			cmdData)
		if err != nil {
			return fmt.Errorf("error occurred creating guild command %q: %v", cmdData.Name, err)
		}
	}

	return nil
}

// InteractionHandler is a handler-function handling interaction-events
func (srv *Server) InteractionHandler(ev *gateway.InteractionCreateEvent) {
	var response *api.InteractionResponseData
	var err error
	options := opsToMap(ev.Data.Options)

	switch ev.Data.Name {
	case "reddit":
		response, err = reddit.HandleReddit(options["sort"], options["sub"])
		if err != nil {
			err = fmt.Errorf("error handling /reddit: %v", err)
		}
	case "define":
		response, err = define.HandleDefine(options["term"])
		if err != nil {
			err = fmt.Errorf("error handling /define: %v", err)
		}
	case "slap":
		response, err = slap.HandleSlap(
			ev.Member.User,
			options["target"],
			options["reason"])
		if err != nil {
			err = fmt.Errorf("error handling /slap: %v", err)
		}
	case "text":
		msg := options["message"]
		if msg == "" {
			if lMsg, ok := srv.LastMessages[ev.ChannelID]; ok {
				msg = lMsg.Content
			}
		}
		response, err = text.HandleText(options["algo"], msg)
		if err != nil {
			err = fmt.Errorf("error handling /text: %v", err)
		}
	case "remind":
		chanID := ev.ChannelID
		if givenChan, ok := options["channel"]; ok {
			sf, err := discord.ParseSnowflake(givenChan)
			if err != nil {
				err = fmt.Errorf("error parsing channel arg as snowflake when handling /remind: %v",
					err)
			} else {
				chanID = discord.ChannelID(sf)
			}
		}
		err = srv.reminder.CreateRemindStrict(options["when"], options["message"], chanID)
		if err != nil {
			err = fmt.Errorf("error handling /remind: %v", err)
		}
	}

	if response != nil {
		data := api.InteractionResponse{
			Type: api.MessageInteractionWithSource,
			Data: response,
		}
		if err := srv.sess.RespondInteraction(ev.ID, ev.Token, data); err != nil {
			logrus.Errorf("failed to send interaction callback: %v", err)
		}
	}

	if err != nil {
		log.Warn(err)
		dm, dmErr := srv.sess.CreatePrivateChannel(ev.Member.User.ID)
		if dmErr != nil {
			log.Errorf("error occurred sending DM with prev error: %v", dmErr)
		} else {
			srv.sess.SendText(dm.ID, err.Error())
		}
	}
}

func opsToMap(ops []gateway.InteractionOption) (opMap map[string]string) {
	opMap = make(map[string]string)
	for _, op := range ops {
		opMap[op.Name] = op.Value
	}

	return
}

// AddHandlers registers the handlers to the discord-session
// If a handler is not registered in this function, the handler will not be called
// func (srv *Server) AddHandlers() {
// 	srv.session.AddHandler(srv.reddit)
// }

// type MessageWriter struct {
// 	channelID  string
// 	userID     string
// 	usePrivate bool
// 	session    *discordgo.Session
// }

// func (mw MessageWriter) Write(p []byte) (n int, err error) {
// 	msg := string(p)
// 	cid := mw.channelID
// 	if mw.usePrivate {
// 		var pc *discordgo.Channel
// 		pc, err = mw.session.UserChannelCreate(mw.userID)
// 		if err != nil {
// 			return
// 		}
// 		cid = pc.ID
// 	}
// 	_, err = mw.session.ChannelMessageSend(cid, msg)
// 	return
// }

// func commandNameIs(message string, commandName string) bool {
// 	if len(message) <= 1 {
// 		return false
// 	}

// 	splits := strings.Split(message[1:], " ")
// 	return splits[0] == commandName
// }

// func (srv *Server) reddit(_ *discordgo.Session, m *discordgo.MessageCreate) {
// 	if m.Author.ID == srv.sess.UserAgent.State.User.ID {
// 		// ignore because this message was sent by me
// 		return
// 	}
// 	if len(m.Content) == 0 || m.Content[0] != '!' {
// 		// this is not a command, ignore it
// 		return
// 	}

// 	mw := MessageWriter{
// 		channelID: m.ChannelID,
// 		userID:    m.Author.ID,
// 		session:   srv.session,
// 	}

// 	fs := flag.NewFlagSet("reddit", flag.ContinueOnError)
// 	fs.SetOutput(&mw)
// 	err := fs.Parse([]string{m.Content})

// 	// positional args
// 	sortAlgo := fs.Arg(0)
// 	sub := fs.Arg(1)
// 	postIndexStr := fs.Arg(2) // optional

// 	if sortAlgo == "" || sub == "" {
// 		fs.Usage()
// 		return
// 	}

// 	postIndex := 0
// 	if postIndexStr != "" {
// 		postIndex, err = strconv.Atoi(postIndexStr)
// 		if err != nil {
// 			mw.usePrivate = true
// 			fs.Usage()
// 			return
// 		}
// 	}
// 	log.Info(postIndex)

// 	if err != nil {
// 		log.Warn(err)
// 	}
// }

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

// func (srv *lundeServer) getLastExec(chanID string, userID string) (
//		command execution, exists bool) {
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
