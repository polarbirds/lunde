package roles

import (
	"fmt"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/session"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/polarbirds/lunde/internal/command"
	"github.com/polarbirds/lunde/internal/server"
)

type roleHandler struct {
	session *session.Session
}

// CreateCommand creates a lunde command to fetch the roles of a user
func CreateCommand(srv *server.Server) (cmd command.LundeCommand, err error) {
	rh := roleHandler{srv.Session}
	cmd = command.LundeCommand{
		HandleInteraction: rh.handleInteraction,
		CommandData: api.CreateCommandData{
			Name: 		 "roles",
			Description: "Check the roles of a user",
			Options: []discord.CommandOption{
				{
					Name: 		 "target",
					Type: 		 discord.UserOption,
					Description: "user to check the roles of",
					Required: 	 true,
				},
			},
		},
	}
	
	return
}

func (rh *roleHandler) handleInteraction(
	event *gateway.InteractionCreateEvent, options map[string]string) (
	response *api.InteractionResponseData, err error,
) {
	target, err := discord.ParseSnowflake(options["target"])
	if err != nil {
		err = fmt.Errorf("parseSnowflake(target): %w", err)
		return
	}

	guild, err := rh.session.Guild(event.GuildID)
	if err != nil {
		err = fmt.Errorf("rh.session.Guild(target): %w", err)
		return
	}

	roles, err := rh.fetchRoles(target, guild)
	if err != nil {
		err = fmt.Errorf("fetchRoles(target, guild): %w", err)
		return
	}

	response = &api.InteractionResponseData{}
	response.Content = option.NewNullableString(strings.Join(roles, ","))

	return
}

func (rh *roleHandler) fetchRoles(target discord.Snowflake, guild *discord.Guild) (
	roles []string, err error,
) {
	member, err := rh.session.Member(guild.ID, discord.UserID(target))
	if err != nil {
		err = fmt.Errorf("rh.session.Member(guild.ID, discord.UserID): %w", err)
		return nil, err
	}

	guildRoles := guild.Roles 

	for _, r := range member.RoleIDs {
		for _, gr := range guildRoles {
			if r == gr.ID {
				roles = append(roles, gr.Name)
			}
		}
	}

	return
}