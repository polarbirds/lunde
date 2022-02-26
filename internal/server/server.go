package server

import (
	"fmt"
	"math/rand"
	"reflect"
	"regexp"
	"sync"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/session"
	"github.com/haraldfw/cfger"
	"github.com/polarbirds/lunde/internal/command"
	"github.com/sirupsen/logrus"
	"gopkg.in/go-playground/validator.v9"
)

// CreateCommand is a function that returns a list of LundeCommands
type CreateCommand func(*Server) (command.LundeCommand, error)

var nicePattern = regexp.MustCompile(`(^|\D)69(\D|$)`)

// Server is the config and main server
type Server struct {
	Token            string            `yaml:"token" validate:"required"`
	AppID            discord.AppID     `yaml:"appID" validate:"required"`
	GuildID          discord.GuildID   `yaml:"guildID" validate:"required"`
	BacklogChannelID discord.ChannelID `yaml:"backlogChannelID" validate:"required"`

	MessagesToGetForDataBuild uint `yaml:"messagesToGetForDataBuild"`

	commands map[string]command.LundeCommand

	Session               *session.Session
	LastMessages          map[discord.ChannelID]*gateway.MessageCreateEvent
	lastMessageWriteMutex sync.Mutex

	CountData  map[discord.UserID]map[string]int
	CountMutex sync.RWMutex

	BuildingDataDone bool
}

// New creates a new server instance with initialized variables
func New() (srv Server, err error) {
	srv = Server{
		LastMessages: make(map[discord.ChannelID]*gateway.MessageCreateEvent),
	}

	_, err = cfger.ReadStructuredCfgRecursive("env::CONFIG", &srv)
	if err != nil {
		return
	}

	validate := validator.New()
	err = validate.Struct(&srv)
	if err != nil {
		err = fmt.Errorf("validating server config: %w", err)
		return
	}

	srv.commands = map[string]command.LundeCommand{}

	return
}

// Initialize the server with the given session
func (srv *Server) Initialize(s *session.Session, commandCreators []CreateCommand) error {
	srv.Session = s

	existingCmds, err := srv.Session.GuildCommands(srv.AppID, srv.GuildID)
	if err != nil {
		return fmt.Errorf("error fetching existing commands: %v", err)
	}

	logrus.Infof("creating/updating %d guild commands...", len(commandCreators))

	existingCommandsEdited := 0
	i := 0
	cmdMap := make(map[string]command.LundeCommand)

	for _, createCMD := range commandCreators {
		i++
		cmd, err := createCMD(srv)
		if err != nil {
			return fmt.Errorf("error creating command: %v", err)
		}

		cmdMap[cmd.CommandData.Name] = cmd

		foundIndex := -1
		for existingIndex, existingCommand := range existingCmds {
			if existingCommand.Name == cmd.CommandData.Name {
				foundIndex = existingIndex
				break
			}
		}

		if foundIndex > -1 {
			existingCommand := existingCmds[foundIndex]
			if cmd.CommandData.Description == existingCommand.Description &&
				reflect.DeepEqual(cmd.CommandData.Options, existingCommand.Options) {
				logrus.Infof("(%d/%d) existing guild command %q unchanged",
					i, len(commandCreators), cmd.CommandData.Name)
			} else {
				_, err := srv.Session.EditGuildCommand(
					srv.AppID, srv.GuildID, existingCommand.ID, cmd.CommandData)
				if err != nil {
					return fmt.Errorf("error occurred editing existing guild command %q: %v",
						cmd.CommandData.Name, err)
				}
				logrus.Infof("(%d/%d) edited existing guild command %q",
					i, len(commandCreators), cmd.CommandData.Name)
			}

			existingCommandsEdited++
			existingCmds = removeCommandFromSlice(existingCmds, foundIndex)
			continue
		}

		logrus.Infof("(%d/%d) creating guild command %q",
			i+1, len(commandCreators), cmd.CommandData.Name)
		_, err = srv.Session.CreateGuildCommand(srv.AppID, srv.GuildID, cmd.CommandData)
		if err != nil {
			return fmt.Errorf("createGuildCommand: %w", err)
		}
	}

	if len(existingCmds) > 0 {
		logrus.Infof("deleting %d guild commands...", len(existingCmds))
		for i, cmd := range existingCmds {
			logrus.Infof("(%d/%d) deleting guild command %q", i+1, len(existingCmds), cmd.Name)
			err := srv.Session.DeleteGuildCommand(cmd.AppID, srv.GuildID, cmd.ID)
			if err != nil {
				return fmt.Errorf("error occurred deleting guild command %q: %v", cmd.Name, err)
			}
		}

		logrus.Infof("deleted %d guild commands", len(existingCmds))
	}

	go srv.buildData()

	srv.commands = cmdMap
	return nil
}

// MessageCreateHandler handles every incoming normal message
func (srv *Server) MessageCreateHandler(c *gateway.MessageCreateEvent) {
	srv.lastMessageWriteMutex.Lock()
	srv.LastMessages[c.ChannelID] = c
	srv.lastMessageWriteMutex.Unlock()

	go srv.buildCountMessages([]discord.Message{c.Message})

	if nicePattern.Match([]byte(c.Content)) {
		var emojiAPIString discord.APIEmoji = "nice:536833842078810112"
		if rand.Intn(2) == 1 {
			emojiAPIString = "♋"
		}
		err := srv.Session.React(c.ChannelID, c.ID, emojiAPIString)
		if err != nil {
			logrus.Errorf("error occurred adding reaction: %v", err)
			return
		}
	}

	if c.ChannelID == srv.BacklogChannelID {
		err := srv.Session.React(c.ChannelID, c.ID, "🗑")
		if err != nil {
			logrus.Errorf("error occurred adding reaction: %v", err)
			return
		}
	}
}

// HandleInteraction is a handler-function handling interaction-events
func (srv *Server) HandleInteraction(ev *gateway.InteractionCreateEvent) {
	switch ev.Data.(type) {
	case *discord.CommandInteraction:
		data := ev.Data.(*discord.CommandInteraction)
		srv.handleCommandInteraction(ev, data)
		return
	}
}

//revive:disable-next-line:cyclomatic
// handleCommandInteraction is a handler-function handling interaction-events
func (srv *Server) handleCommandInteraction(
	event *gateway.InteractionCreateEvent,
	data *discord.CommandInteraction,
) {
	log := logrus.WithField("command", data.Name)

	options, err := opsToMap(data.Options)
	if err != nil {
		log.Errorf("error occurred converting ops to a map: %v", err)
		return
	}

	for name, val := range options {
		log = log.WithField(name, val)
	}

	cmd, exists := srv.commands[data.Name]
	if !exists {
		log.Errorf("command %s does not exist", data.Name)
		return
	}

	responseData, err := cmd.HandleInteraction(event, options)
	if err != nil {
		log.Warnf("error occurred handling interaction: %v", err)
		dm, dmErr := srv.Session.CreatePrivateChannel(event.Member.User.ID)
		if dmErr != nil {
			log.Errorf("error occurred creating private channel to report error: %v", dmErr)
			return
		}

		_, dmErr = srv.Session.SendMessage(dm.ID, err.Error())
		if dmErr != nil {
			log.Errorf("error occurred sending DM to report error: %v", dmErr)
			return
		}

		return
	}

	interactionResp := api.InteractionResponse{
		Type: api.MessageInteractionWithSource,
		Data: responseData,
	}
	if err := srv.Session.RespondInteraction(event.ID, event.Token, interactionResp); err != nil {
		log.Errorf("failed to send interaction callback: %v", err)
		return
	}

	log.Infof("responded to interaction")
}

func opsToMap(ops discord.CommandInteractionOptions) (opMap map[string]string, err error) {
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
	cmds, err := srv.Session.GuildCommands(srv.AppID, srv.GuildID)
	if err != nil {
		return fmt.Errorf("fetching existing commands: %v", err)
	}

	for i, cmd := range cmds {
		err = srv.Session.DeleteGuildCommand(
			cmd.AppID,
			srv.GuildID,
			cmd.ID)
		if err != nil {
			return fmt.Errorf("deleting command %s: %v", cmd.Name, err)
		}
		logrus.Infof("deleted guild command (%d/%d) %q", i+1, len(cmds), cmd.Name)
	}

	return nil
}

func removeCommandFromSlice(s []discord.Command, i int) []discord.Command {
	if len(s) <= 1 {
		return []discord.Command{}
	}

	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}
