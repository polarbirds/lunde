package command

import (
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/gateway"
)

// LundeCommand is data about a command and the function to handle interactions in the way described
// by the data
type LundeCommand struct {
	CommandData       api.CreateCommandData
	HandleInteraction func(
		event *gateway.InteractionCreateEvent,
		options map[string]string,
	) (*api.InteractionResponseData, error)
}
