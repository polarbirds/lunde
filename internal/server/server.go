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

type cmdExec struct {
	messageAuthorID  discord.UserID
	messageChannelID discord.ChannelID
	messageID        discord.Snowflake
	replyID          string
}

// Server is the config and main server
type Server struct {
	MaxCMDHistory int    `yaml:"maxCMDHistory" validate:"min=1"`
	Token         string `yaml:"token" validate:"required"`
	AppID         string `yaml:"appID" validate:"required"`
	GuildID       string `yaml:"guildID" validate:"required"`

	RemindsFilePath string `yaml:"remindsFilePath" validate:"required"`

	appIDSnowFlake   discord.AppID
	guildIDSnowFlake discord.GuildID

	sess           *session.Session
	cmdExecHistory map[discord.UserID]map[discord.ChannelID][]cmdExec
	reminder       remind.Reminder
	LastMessages   map[discord.ChannelID]*gateway.MessageCreateEvent
}

// New creates a new server instance with initialized variables
func New() (srv Server, err error) {
	srv = Server{
		// lastExecs:    make(map[string]map[string]execution),
		LastMessages:   make(map[discord.ChannelID]*gateway.MessageCreateEvent),
		cmdExecHistory: make(map[discord.UserID]map[discord.ChannelID][]cmdExec),
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
		{
			Name:        "undo",
			Description: "undo your last <num> command(s)",
			Options: []discord.CommandOption{
				{
					Name: "num",
					Type: discord.StringOption,
					Description: "how many commands to undo, defaults to 1 (CANNOT BE INT FOR " +
						"SOME GOD-FORSAKEN REASON)",
					Required: false,
				},
			},
		},
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

// InteractionHandler is a handler-function handling interaction-events
func (srv *Server) InteractionHandler(ev *gateway.InteractionCreateEvent) {
	var response *api.InteractionResponseData
	var err error
	options := opsToMap(ev.Data.Options)

	logrus.Infof("%+v", ev)

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
	case "undo":
		logrus.Infof("undo with amount string %v", options["num"])
		var amount = 1
		if amountStr := options["num"]; amountStr != "" {
			amount, err = strconv.Atoi(amountStr)
			if err != nil {
				err = fmt.Errorf("error handling /undo: err reading amount as int: %v", err)
				break
			}
		}

		logrus.Infof("undo with amount int %v", amount)
		err = srv.handleUndo(ev.Member.User.ID, ev.ChannelID, amount)
		if err != nil {
			err = fmt.Errorf("error handling /undo: %v", err)
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
		} else {
			srv.storeExec(cmdExec{
				messageAuthorID:  ev.Member.User.ID,
				messageChannelID: ev.ChannelID,
				messageID:        ev.Data.ID,
			})
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

func (srv *Server) storeExec(cmd cmdExec) {
	user, userExists := srv.cmdExecHistory[cmd.messageAuthorID]
	if !userExists {
		user = make(map[discord.ChannelID][]cmdExec)
	}

	userChan, userChanExists := user[cmd.messageChannelID]
	if !userChanExists {
		userChan = []cmdExec{cmd}
	} else {
		userChan = append([]cmdExec{cmd}, userChan...)
		maxLenOrMax := len(userChan)
		if maxLenOrMax > srv.MaxCMDHistory {
			maxLenOrMax = srv.MaxCMDHistory
		}

		userChan = userChan[:maxLenOrMax]
	}

	user[cmd.messageChannelID] = userChan
	srv.cmdExecHistory[cmd.messageAuthorID] = user
}

func (srv *Server) handleUndo(
	authorID discord.UserID,
	channel discord.ChannelID,
	amount int,
) error {
	if amount < 1 {
		return errors.New("amount must be 1 or larger")
	}

	user, userExists := srv.cmdExecHistory[authorID]
	if !userExists {
		return errors.New("no commands found for given user")
	}

	userChan, userChanExists := user[channel]
	if !userChanExists || len(userChan) == 0 {
		return errors.New("no commands found for user in given channel")
	}

	defer func() {
		user[channel] = userChan
		srv.cmdExecHistory[authorID] = user
	}()

	for deleted := 0; deleted < amount; deleted++ {
		cmd := userChan[0]
		logrus.Infof("deleting message %+v", cmd)
		err := srv.sess.DeleteMessage(channel, discord.MessageID(cmd.messageID))
		if err != nil {
			return fmt.Errorf("deleting message: %v", err)
		}

		userChan = userChan[1:]
		if len(userChan) == 0 {
			// no more cmds to delete
			return nil
		}
	}

	return nil
}

func opsToMap(ops []gateway.InteractionOption) (opMap map[string]string) {
	opMap = make(map[string]string)
	for _, op := range ops {
		opMap[op.Name] = op.Value
	}

	return
}
