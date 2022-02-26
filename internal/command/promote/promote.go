package promote

import (
	"fmt"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/polarbirds/lunde/internal/command"
	"github.com/polarbirds/lunde/internal/server"
)

// CreateCommand creates the define command
func CreateCommand(_ *server.Server) (cmd command.LundeCommand, err error) {
	cmd = command.LundeCommand{
		HandleInteraction: handleInteraction,
		CommandData: api.CreateCommandData{
			Name:        "promote",
			Description: "promote someone to birb",
			Options: []discord.CommandOption{
				&discord.UserOption{
					OptionName:  "target",
					Description: "who to promote",
					Required:    true,
				},
			},
		},
	}

	return
}

func handleInteraction(_ *gateway.InteractionCreateEvent, options map[string]string) (
	response *api.InteractionResponseData, err error,
) {
	targetSnowflake, err := discord.ParseSnowflake(options["target"])
	if err != nil {
		err = fmt.Errorf("parsing target ID: %v", err)
		return
	}

	response = &api.InteractionResponseData{
		Content: option.NewNullableString(fmt.Sprintf(
			"error occurred promoting user %s, this issue is likely temporary so you should try "+
				"again later",
			discord.UserID(targetSnowflake).Mention())),
	}
	return
}
