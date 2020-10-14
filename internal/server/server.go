package server

import (
	"errors"

	"github.com/bwmarrin/discordgo"
	"github.com/haraldfw/cfger"
	"github.com/polarbirds/lunde/pkg/remind"
)

type execution struct {
	messageAuthorID  string
	replyID          string
	messageChannelID string
	messageID        string
}

// LundeServer is the root config
type LundeServer struct {
	Token           string `yaml:"token"`
	RemindsFilePath string `yaml:"remindsFile"`

	sess         *discordgo.Session
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

	if srv.RemindsFilePath == "" {
		err = errors.New("blank remindsFile value in config")
		return
	}
	return
}

// Initialize Initializes the server with the given session. May panic if given session is nil
func (srv *LundeServer) Initialize(s *discordgo.Session) error {
	srv.sess = s

	rmd := remind.Reminder{
		DiscordSession: s,
	}

	err := rmd.Start()
	if err != nil {
		return err
	}

	return nil
}

// Handle handles a single discord message
func (srv *LundeServer) Handle(_ *discordgo.Session, m *discordgo.MessageCreate) {

}
