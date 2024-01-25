package main

import (
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/bwmarrin/discordgo"
	_ "github.com/joho/godotenv/autoload"
)

type DomainProps struct {
	Domain        string
	RequiredPaths []*regexp.Regexp
}

var REACTION_EMOJI = "concreteBONK:959613362612887582"

var HANDLED_DOMAINS = map[string]DomainProps{
	// Twitter added embeds, but who knows how long that'll last; leaving this here for now
	// "twitter.com": {
	// 	Domain:        "vxtwitter.com",
	// 	RequiredPaths: []*regexp.Regexp{regexp.MustCompile(`\/.*\/status\/`)},
	// },
	// "x.com": {
	// 	Domain:        "fixvx.com",
	// 	RequiredPaths: []*regexp.Regexp{regexp.MustCompile(`\/.*\/status\/`)},
	// },
	"instagram.com": {
		Domain:        "ddinstagram.com",
		RequiredPaths: []*regexp.Regexp{regexp.MustCompile(`\/p\/`)},
	},
	"tiktok.com": {
		Domain:        "vxtiktok.com",
		RequiredPaths: []*regexp.Regexp{regexp.MustCompile(`\/t\/`)},
	},
}

func main() {
	if os.Getenv("DISCORD_TOKEN") == "" {
		panic("DISCORD_TOKEN is not set")
	}
	if os.Getenv("REACTION_EMOJI") != "" {
		REACTION_EMOJI = os.Getenv("REACTION_EMOJI")
	}

	dgo, err := discordgo.New("Bot " + os.Getenv("DISCORD_TOKEN"))
	if err != nil {
		panic(err)
	}

	dgo.AddHandler(messageCreate)

	err = dgo.Open()
	if err != nil {
		panic(err)
	}

	println("Bot is running...")
	defer dgo.Close()

	// Create a channel to receive the SIGINT signal
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Block until a signal is received
	<-sigint
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}

	urlRegex := regexp.MustCompile(`https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-z]{2,4}\b([-a-zA-Z0-9@:%_\+.~#?&//=]*)`)
	tldRegex := regexp.MustCompile(`\.?([^.]*.com)`)

	matches := urlRegex.FindAllStringSubmatch(m.Content, -1)
	var content = ""
	for _, match := range matches {
		url, err := url.Parse(match[0])
		if err != nil {
			fmt.Printf("%s\n", err)
			continue
		}
		if len(tldRegex.FindStringSubmatch(url.Hostname())) < 2 {
			fmt.Printf("No TLD found for %s\n", url.Hostname())
			fmt.Printf("Regex: %s\n", tldRegex.FindStringSubmatch(url.Hostname()))
			continue
		}
		tld := tldRegex.FindStringSubmatch(url.Hostname())[1]
		if val, ok := HANDLED_DOMAINS[tld]; ok {
			for _, path := range val.RequiredPaths {
				if ok := path.MatchString(url.Path); ok {
					url.Host = val.Domain
					content += fmt.Sprintf("%s\n", url.String())
					break
				}
			}

		}
	}
	if content == "" {
		return
	}
	fmt.Printf("Sending %s in guild %s in response to user %s\n", content, m.GuildID, m.Author.Username)
	_, err := s.ChannelMessageSend(m.ChannelID, content)
	if err != nil {
		fmt.Printf("%s\n", err)
		return
	}

	err = s.MessageReactionAdd(m.ChannelID, m.ID, REACTION_EMOJI)
	if err != nil {
		fmt.Printf("%s\n", err)
		return
	}
}
