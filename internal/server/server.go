package server

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/session"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/polarbirds/lunde/internal/command/define"
	"github.com/polarbirds/lunde/internal/command/reddit"
	"github.com/polarbirds/lunde/internal/command/slap"
	"github.com/polarbirds/lunde/internal/command/text"
	"github.com/sirupsen/logrus"
)

var nicePattern = regexp.MustCompile("(^|\\D)69(\\D|$)")

// Server is the config and main server
type Server struct {
	Token   string `validate:"required"`
	AppID   string `validate:"required"`
	GuildID string `validate:"required"`

	appIDSnowFlake   discord.AppID
	guildIDSnowFlake discord.GuildID

	sess                  *session.Session
	lastMessages          map[discord.ChannelID]*gateway.MessageCreateEvent
	lastMessageWriteMutex sync.Mutex
}

// New creates a new server instance with initialized variables
func New() (srv Server, err error) {
	srv = Server{
		lastMessages: make(map[discord.ChannelID]*gateway.MessageCreateEvent),
	}

	srv.Token, _ = os.LookupEnv("TOKEN")
	srv.AppID, _ = os.LookupEnv("APPID")
	srv.GuildID, _ = os.LookupEnv("GUILDID")

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
	return
}

func removeCommandFromSlice(s []discord.Command, i int) []discord.Command {
	if len(s) <= 1 {
		return []discord.Command{}
	}

	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

//revive:disable-next-line:cyclomatic
// Initialize the server with the given session
func (srv *Server) Initialize(s *session.Session) error {
	srv.sess = s

	existingCmds, err := srv.sess.GuildCommands(srv.appIDSnowFlake, srv.guildIDSnowFlake)
	if err != nil {
		return fmt.Errorf("error fetching existing commands: %v", err)
	}

	newCMDs := []api.CreateCommandData{
		reddit.CommandData(),
		define.CommandData(),
		slap.CommandData(),
		text.CommandData(),
		{
			Name:        "test",
			Description: "test stuff",
		},
	}

	existingCommandsEdited := 0

	logrus.Infof("creating %d guild commands...", len(newCMDs))
	for i, cmdData := range newCMDs {
		foundIndex := -1
		for existingIndex, existingCommand := range existingCmds {
			if existingCommand.Name == cmdData.Name {
				foundIndex = existingIndex
				break
			}
		}

		if foundIndex > -1 {
			existingCommand := existingCmds[foundIndex]
			if cmdData.Description == existingCommand.Description &&
				reflect.DeepEqual(cmdData.Options, existingCommand.Options) {
				logrus.Infof("(%d/%d) existing guild command unchanged: %q",
					i+1, len(newCMDs), cmdData.Name)
			} else {
				_, err := srv.sess.EditGuildCommand(
					srv.appIDSnowFlake, srv.guildIDSnowFlake, existingCommand.ID, cmdData)
				if err != nil {
					return fmt.Errorf("error occurred editing existing guild command %q: %v",
						cmdData.Name, err)
				}
				logrus.Infof("(%d/%d) edited existing guild command %q",
					i+1, len(newCMDs), cmdData.Name)
			}

			existingCommandsEdited++
			existingCmds = removeCommandFromSlice(existingCmds, foundIndex)
			continue
		}

		logrus.Infof("creating guild command (%d/%d) %q", i+1, len(newCMDs), cmdData.Name)
		_, err := srv.sess.CreateGuildCommand(
			srv.appIDSnowFlake,
			srv.guildIDSnowFlake,
			cmdData)
		if err != nil {
			return fmt.Errorf("error occurred creating guild command %q: %v", cmdData.Name, err)
		}
	}

	if len(existingCmds) > 0 {
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
	}
	return nil
}

// MessageCreateHandler handles every incoming normal message
func (srv *Server) MessageCreateHandler(c *gateway.MessageCreateEvent) {
	srv.lastMessageWriteMutex.Lock()
	srv.lastMessages[c.ChannelID] = c
	srv.lastMessageWriteMutex.Unlock()

	if nicePattern.Match([]byte(c.Content)) {
		var emojiAPIString discord.APIEmoji = "nice:536833842078810112"
		if rand.Intn(2) == 1 {
			emojiAPIString = "♋"
		}
		err := srv.sess.React(c.ChannelID, c.ID, emojiAPIString)
		if err != nil {
			logrus.Errorf("error occurred adding reaction: %v", err)
		}
	}
}

// HandleComponentInteraction handles component interactions, e.g. button-presses
func (srv *Server) HandleComponentInteraction(ev *gateway.InteractionCreateEvent) {
	if ev.Interaction.Type != discord.ComponentInteraction {
		return
	}

	inter := ev.Data.(*discord.ComponentInteractionData)

	dm, err := srv.sess.CreatePrivateChannel(ev.Member.User.ID)
	if err != nil {
		logrus.Errorf("error occurred sending DM: %v", err)
	} else {
		srv.sess.SendMessage(
			dm.ID, fmt.Sprintf("you pressed the button with the custom ID: %s", inter.CustomID))
	}

	if err := srv.sess.RespondInteraction(ev.ID, ev.Token, api.InteractionResponse{
		Type: api.DeferredMessageUpdate,
	}); err != nil {
		logrus.Errorf("failed to send interaction callback: %v", err)
	}

	logrus.Infof("component interaction detected by %s: %s", ev.Member.Nick, inter.CustomID)
}

//revive:disable-next-line:cyclomatic
// HandleCommandInteraction is a handler-function handling interaction-events
func (srv *Server) HandleCommandInteraction(ev *gateway.InteractionCreateEvent) {
	var response *api.InteractionResponseData
	var err error

	if ev.Interaction.Type != discord.CommandInteraction {
		return
	}

	defer srv.handleResponse(&response, ev, &err)

	inter := ev.Data.(*discord.CommandInteractionData)
	options, err := opsToMap(inter.Options)
	if err != nil {
		err = fmt.Errorf("error occurred converting ops to a map: %v", err)
		return
	}

	switch inter.Name {
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
			nick := ev.Member.Nick
			if nick == "" {
				nick = ev.Member.User.Username
			}
			response = &api.InteractionResponseData{
				Content: option.NewNullableString(
					fmt.Sprintf("this is a christian channel, %s", nick)),
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
	case "test":
		response = &api.InteractionResponseData{
			Content: option.NewNullableString("BOIS WE HAVE BUTTS!"),
			Components: &[]discord.Component{
				&discord.ActionRowComponent{
					Components: []discord.Component{
						&discord.ButtonComponent{
							Label:    "First BUTT!",
							CustomID: "first_button",
							Emoji: &discord.ButtonEmoji{
								Name: "👋",
							},
							Style: discord.PrimaryButton,
						},
						&discord.ButtonComponent{
							Label:    "Second BUTT",
							CustomID: "second_button",
							Style:    discord.SecondaryButton,
						},
						&discord.ButtonComponent{
							Label:    "Success BUTT",
							CustomID: "success_button",
							Style:    discord.SuccessButton,
						},
						&discord.ButtonComponent{
							Label:    "DANGER BUTT",
							CustomID: "danger_button",
							Style:    discord.DangerButton,
						},
						&discord.ButtonComponent{
							Label: "Butts-BUTT",
							URL:   "https://reddit.com/r/corgibutts",
							Style: discord.LinkButton,
						},
					},
				},
			},
		}
	}
}

func (srv *Server) handleResponse(
	presponse **api.InteractionResponseData,
	ev *gateway.InteractionCreateEvent,
	perr *error,
) {
	if presponse == nil {
		return
	}
	response := *presponse

	if response != nil {
		data := api.InteractionResponse{
			Type: api.MessageInteractionWithSource,
			Data: response,
		}
		if err := srv.sess.RespondInteraction(ev.ID, ev.Token, data); err != nil {
			logrus.Errorf("failed to send interaction callback: %v", err)
		}
	}

	if perr != nil && *perr != nil {
		err := *perr
		logrus.Warnf("error occurred: %v", err)
		dm, dmErr := srv.sess.CreatePrivateChannel(ev.Member.User.ID)
		if dmErr != nil {
			logrus.Errorf("error occurred sending DM with prev error: %v", dmErr)
		} else {
			srv.sess.SendMessage(dm.ID, err.Error())
		}
	}
}

func opsToMap(ops []discord.InteractionOption) (opMap map[string]string, err error) {
	opMap = make(map[string]string)
	for _, op := range ops {
		var val string
		err = op.Value.UnmarshalTo(&val)
		if err != nil {
			err = fmt.Errorf("unmarshalling op by name %q to string, value was: %v",
				op.Name, op.Value)
			return
		}
		opMap[op.Name] = string(val)
	}

	return
}

// DeleteGuildCommands deletes all guild commands for the configured guild and app ID
func (srv *Server) DeleteGuildCommands() error {
	cmds, err := srv.sess.GuildCommands(srv.appIDSnowFlake, srv.guildIDSnowFlake)
	if err != nil {
		return fmt.Errorf("fetching existing commands: %v", err)
	}

	for i, cmd := range cmds {
		err = srv.sess.DeleteGuildCommand(
			cmd.AppID,
			srv.guildIDSnowFlake,
			cmd.ID)
		if err != nil {
			return fmt.Errorf("deleting command %s: %v", cmd.Name, err)
		}
		logrus.Infof("deleted guild command (%d/%d) %q", i+1, len(cmds), cmd.Name)
	}

	return nil
}
