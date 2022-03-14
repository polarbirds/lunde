package roles

import (
	"fmt"
	"sort"

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
			Name:        "roles",
			Description: "Check the roles of a user",
			Options: []discord.CommandOption{
				&discord.UserOption{
					OptionName:  "target",
					Description: "user to check the roles of",
					Required:    true,
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
	targetSnowflake, err := discord.ParseSnowflake(options["targetSnowflake"])
	if err != nil {
		err = fmt.Errorf("parseSnowflake(options[\"targetSnowflake\"]): %w", err)
		return
	}

	guild, err := rh.session.Guild(event.GuildID)
	if err != nil {
		err = fmt.Errorf("rh.session.Guild(event.GuildID): %w", err)
		return
	}

	member, err := rh.session.Member(guild.ID, discord.UserID(targetSnowflake))
	if err != nil {
		err = fmt.Errorf("rh.session.Member(guild.ID, discord.UserID): %w", err)
		return nil, err
	}

	roles, err := rh.fetchRoles(member, guild)
	if err != nil {
		err = fmt.Errorf("rh.fetchRoles(member, guild): %w", err)
		return
	}

	response = &api.InteractionResponseData{}
	response.Content = option.NewNullableString(formatMessage(roles, member))

	return
}

func (rh *roleHandler) fetchRoles(member *discord.Member, guild *discord.Guild) (
	roles []string, err error,
) {
	guildRoles := guild.Roles

	for _, r := range member.RoleIDs {
		for _, gr := range guildRoles {
			if r == gr.ID {
				roles = append(roles, "@"+gr.Name)
			}
		}
	}

	return
}

func formatMessage(roles []string, member *discord.Member) (msg string) {
	sort.Strings(roles)

	msg = fmt.Sprintf("Roles for user %s:\n```", member.User.Username)

	for _, r := range roles {
		msg += fmt.Sprintf("\n%s", r)
	}

	msg += "```"
	return
}
