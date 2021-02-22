package meme

import (
	"github.com/diamondburned/arikawa/v2/discord"
)

// Post represents a post from an internet page
type Post struct {
	Message string
	Title   string
	Embed   *discord.Embed
	NSFW    bool
}
