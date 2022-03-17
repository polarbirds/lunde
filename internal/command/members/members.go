package members

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

type memberHandler struct {
	session *session.Session
}

// CreateCommand creates a lunde command to fetch the roles of a user
func CreateCommand(srv *server.Server) (cmd command.LundeCommand, err error) {
	mh := memberHandler{srv.Session}
	cmd = command.LundeCommand{
		HandleInteraction: mh.handleInteraction,
		CommandData: api.CreateCommandData{
			Name:        "members",
			Description: "Check the members of a role",
			Options: []discord.CommandOption{
				&discord.RoleOption{
					OptionName:  "role",
					Description: "role to check members for",
					Required:    true,
				},
			},
		},
	}

	return
}

func (mh *memberHandler) handleInteraction(
	event *gateway.InteractionCreateEvent, options map[string]discord.CommandInteractionOption,
) (
	response *api.InteractionResponseData, err error,
) {
	roleFlake, err := options["role"].SnowflakeValue()
	if err != nil {
		err = fmt.Errorf("parsing role as flake: %v", err)
		return
	}

	guild, err := mh.session.Guild(event.GuildID)
	if err != nil {
		err = fmt.Errorf("mh.session.Guild(event.GuildID): %w", err)
		return nil, err
	}

	guildMembers, err := mh.session.Members(guild.ID, 0)
	if err != nil {
		err = fmt.Errorf("mh.session.Members(guild.ID, 0): %w", err)
		return nil, err
	}

	roles := guild.Roles
	role := fetchRole(roles, discord.RoleID(roleFlake))

	members := fetchMembers(role, guildMembers)

	response = &api.InteractionResponseData{}
	response.Content = option.NewNullableString(formatMessage(members, role))

	return
}

func fetchRole(roles []discord.Role, roleID discord.RoleID) (role discord.Role) {
	for _, r := range roles {
		if r.ID == roleID {
			role = r
			break
		}
	}

	return
}

func fetchMembers(role discord.Role, guildMembers []discord.Member) (
	members []string,
) {
	for _, m := range guildMembers {
		for _, roleID := range m.RoleIDs {
			if roleID == role.ID {
				userStr := "@" + m.User.Username
				if m.Nick != "" {
					userStr += " (" + m.Nick + ")"
				}
				members = append(members, userStr)
			}
		}
	}
	return
}

func formatMessage(members []string, role discord.Role) (msg string) {
	sort.Strings(members)

	msg = fmt.Sprintf("Members in role %s:\n```", role.Name)

	for _, m := range members {
		msg += fmt.Sprintf("\n%s", m)
	}

	msg += "```"
	return
}
