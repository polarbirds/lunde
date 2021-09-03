package server

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

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

// Server is the config and main server
type Server struct {
	MaxCMDHistory int    `yaml:"maxCMDHistory" validate:"min=1"`
	Token         string `yaml:"token" validate:"required"`
	AppID         string `yaml:"appID" validate:"required"`
	GuildID       string `yaml:"guildID" validate:"required"`

	RemindsFilePath string `yaml:"remindsFilePath" validate:"required"`

	appIDSnowFlake   discord.AppID
	guildIDSnowFlake discord.GuildID

	sess                  *session.Session
	reminder              remind.Reminder
	lastMessages          map[discord.ChannelID]*gateway.MessageCreateEvent
	lastMessageWriteMutex sync.Mutex
}

// New creates a new server instance with initialized variables
func New() (srv Server, err error) {
	srv = Server{
		// lastExecs:    make(map[string]map[string]execution),
		lastMessages: make(map[discord.ChannelID]*gateway.MessageCreateEvent),
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

	existingCmds, err := srv.sess.GuildCommands(srv.appIDSnowFlake, srv.guildIDSnowFlake)
	if err != nil {
		return fmt.Errorf("error fetching existing commands: %v", err)
	}

	logrus.Infof("deleting %d guild commands...", len(existingCmds))
	for i, cmd := range existingCmds {
		logrus.Infof("deleting guild command (%d/%d) %q", i+1, len(existingCmds), cmd.Name)
		err := srv.sess.DeleteGuildCommand(
			cmd.AppID,
			srv.guildIDSnowFlake,
			cmd.ID)
		if err != nil {
			return fmt.Errorf("error occurred deleting guild command %q: %v", cmd.Name, err)
		}
	}

	logrus.Infof("deleted %d guild commands", len(existingCmds))

	cmds := []api.CreateCommandData{
		reddit.CommandData(),
		define.CommandData(),
		slap.CommandData(),
		text.CommandData(),
		remind.CommandData(),
	}

	logrus.Infof("creating %d guild commands...", len(cmds))
	for i, cmdData := range cmds {
		logrus.Infof("creating guild command (%d/%d) %q", i+1, len(cmds), cmdData.Name)
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

// MessageCreateHandler handles every incoming normal message
func (srv *Server) MessageCreateHandler(c *gateway.MessageCreateEvent) {
	srv.lastMessageWriteMutex.Lock()

	srv.lastMessages[c.ChannelID] = c

	srv.lastMessageWriteMutex.Unlock()
}

//revive:disable-next-line:cyclomatic
// HandleInteraction is a handler-function handling interaction-events
func (srv *Server) HandleInteraction(ev *gateway.InteractionCreateEvent) {
	var response *api.InteractionResponseData
	var err error
	options := opsToMap(ev.Data.Options)

	logrus.Debugf("event received: %+v", ev)

	switch ev.Data.Name {
	case "reddit":
		var isNSFW bool
		response, isNSFW, err = reddit.HandleReddit(options["sort"], options["sub"])
		if err != nil {
			err = fmt.Errorf("error handling /reddit: %v", err)
			break
		}

		var recChan *discord.Channel
		recChan, err = srv.sess.Channel(ev.ChannelID)
		if err != nil {
			err = fmt.Errorf("error when getting channel when handling /reddit: %v", err)
			break
		}

		if isNSFW && !recChan.NSFW {
			response = &api.InteractionResponseData{
				Content: fmt.Sprintf("this is a christian channel, %s", ev.Member.Nick),
			}
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
			if lMsg, ok := srv.lastMessages[ev.ChannelID]; ok {
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

		// TODO: implement xkcd. Beware of 3s limit of initial response
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
