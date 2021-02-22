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
		logrus.Warnf("blank remindsFile value in config, using %s", defaultRemindsFilePath)
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
		// case "undo":
		// 	lastExec, exists := srv.getLastExec(m.ChannelID, m.Author.ID)
		// 	if !exists {
		// 		break
		// 	}
		// 	logrus.Info("undo lastExec: ", lastExec)
		// 	if m.Author.ID == lastExec.messageAuthorID &&
		// 		m.ChannelID == lastExec.messageChannelID {
		// 		if lastExec.messageID != "" {
		// 			s.ChannelMessageDelete(lastExec.messageChannelID, lastExec.messageID)
		// 		}
		// 		if lastExec.replyID != "" {
		// 			s.ChannelMessageDelete(lastExec.messageChannelID, lastExec.replyID)
		// 		}
		// 		s.ChannelMessageDelete(m.ChannelID, m.Message.ID)
		// 	}
		// case "xkcd":
		// 	var msg meme.Post
		// 	msg, err = xkcd.GetMeme(scheme, argument)
		// 	if msg.Embed.Title != "" {
		// 		reply, discErr = s.ChannelMessageSendEmbed(m.ChannelID, &msg.Embed)
		// 	} else {
		// 		reply, discErr = s.ChannelMessageSend(
		// 			m.ChannelID, fmt.Sprintf("%s\n%s", msg.Title, msg.Message))
		// 	}
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
		logrus.Warn(err)
		dm, dmErr := srv.sess.CreatePrivateChannel(ev.Member.User.ID)
		if dmErr != nil {
			logrus.Errorf("error occurred sending DM with prev error: %v", dmErr)
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
