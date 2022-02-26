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
	roleStr, err := options["role"]
	if err != nil {
		err = fmt.Errorf("discord.ParseSnowflake(options[\"role\"]): %w", err)
		return nil, err
	}

	guild, err := mh.session.Guild(event.GuildID)
	if err != nil {
		err = fmt.Errorf("mh.session.Guild(event.GuildID): %w", err)
		return nil, err
	}

	roles := guild.Roles

	role, err := fetchRole(roles, roleStr)
	if err != nil {
		err = fmt.Errorf("fetchRole(roles, roleStr): %w", err)
		return nil, err
	}

	members, err := fetchMembers(role)
	if err != nil {
		err = fmt.Errorf("fetchMembers(role): %w", err)
		return nil, err
	}
}

func fetchRole(roles []discord.Role, roleStr string) (
	role discord.Role, err error,
) {
	for _, r := range roles {
		if r.Name == roleStr {
			role = r
		}
	}
	
	return
}

func fetchMembers(role discord.Role) (members []discord.Member, err error) {
	
}