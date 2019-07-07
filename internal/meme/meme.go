package meme

import "github.com/polarbirds/discordgo"

// Post represents a post from an internet page
type Post struct {
	Message string
	Title   string
	Embed   discordgo.MessageEmbed
}
