package members

import (
	"fmt"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/session"
	"github.com/polarbirds/lunde/internal/command"
	"github.com/polarbirds/lunde/internal/server"
)

type memberHandler struct {
	session *session.Session
}

// CreateCommand creates a lunde command to fetch the roles of a user
func CreateCommand(srv *server.Server) (cmd command.LundeCommand, err error) {
	mh := memberHandler{srv.Session}
	cmd = command.LundeCommand{
		HandleInteraction: mh.handleInteraction,
		CommandData: api.CreateCommandData{
			Name: 		 "members",
			Description: "Check the members of a role",
			Options: []discord.CommandOption{
				{
					Name: 		 "role",
					Type: 		 discord.UserOption,
					Description: "role to check members for",
					Required: 	 true,
				},
			},
		},
	}
	
	return
}

func (mh *memberHandler) handleInteraction(
	event *gateway.InteractionCreateEvent, options map[string]string) (
	response *api.InteractionResponseData, err error,
) {
	role, err := discord.ParseSnowflake(options["role"])
	if err != nil {
		err = fmt.Errorf("discord.ParseSnowflake(options[\"role\"]): %w", err)
		return nil, err
	}

	guild, err := mh.session.Guild(event.GuildID)
	if err != nil {
		err = fmt.Errorf("mh.session.Guild(event.GuildID): %w", err)
		return nil, err
	}

	roles, err := mh.session.Roles(guild.ID)
	if err != nil {
		err = fmt.Errorf("mh.session.Roles(guild.ID): %w", err)
		return nil, err
	}


}