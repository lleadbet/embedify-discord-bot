package main

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/dgraph-io/ristretto"
	_ "github.com/joho/godotenv/autoload"
)

type DomainProps struct {
	Domain        string
	RequiredPaths []*regexp.Regexp
}

var REACTION_EMOJI = "concreteBONK:959613362612887582"
var DEV_MODE = false
var SUPPRESS_EMBEDS = true

var urlRegex = regexp.MustCompile(`https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-z]{2,4}\b([-a-zA-Z0-9@:%_\+.~#?&//=]*)`)
var tldRegex = regexp.MustCompile(`\.?([^.]*(.com|.it))`)

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
		RequiredPaths: []*regexp.Regexp{regexp.MustCompile(`\/t\/`), regexp.MustCompile(`\/video\/`)},
	},
	"reddit.com": {
		Domain:        "rxddit.com",
		RequiredPaths: []*regexp.Regexp{regexp.MustCompile(`\/r\/`)},
	},
	"redd.it": {
		Domain:        "rxddit.com",
		RequiredPaths: []*regexp.Regexp{regexp.MustCompile(`.*`)},
	},
}

type DiscordBotHandler struct {
	c *ristretto.Cache
	l *slog.Logger
}

func main() {
	level := slog.LevelInfo
	if os.Getenv("DISCORD_TOKEN") == "" {
		panic("DISCORD_TOKEN is not set")
	}
	if os.Getenv("REACTION_EMOJI") != "" {
		REACTION_EMOJI = os.Getenv("REACTION_EMOJI")
	}
	if strings.ToUpper(os.Getenv("ENV")) == "DEV" {
		DEV_MODE = true
		level = slog.LevelDebug
	}

	handler, err := NewDiscordHandler(level)
	if err != nil {
		panic(err)
	}

	suppress, _ := strconv.ParseBool(os.Getenv("ENABLE_TIKTOK_EMBED_SUPPRESSION"))
	if suppress {
		handler.l.Info("Suppressing TikTok & Reddit embeds")
		SUPPRESS_EMBEDS = suppress
	}

	dgo, err := discordgo.New("Bot " + os.Getenv("DISCORD_TOKEN"))
	if err != nil {
		panic(err)
	}

	dgo.AddHandler(handler.messageCreate)

	err = dgo.Open()
	if err != nil {
		panic(err)
	}

	handler.l.Info("Bot is now running. Press CTRL-C to exit.")
	defer dgo.Close()

	// Create a channel to receive the SIGINT signal
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Block until a signal is received
	<-sigint
}

func (d *DiscordBotHandler) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}

	if DEV_MODE && m.ChannelID != "1197329348051611690" {
		d.l.Debug("Ignoring message in channel", "channel", m.ChannelID)
		return
	} else if !DEV_MODE && m.ChannelID == "1197329348051611690" {
		d.l.Debug("Got dev channel message, ignoring")
		return
	}

	matches := urlRegex.FindAllStringSubmatch(m.Content, -1)
	var content = ""
	var shouldStripEmbed = false

	for _, match := range matches {
		d.l.Debug("Match", "match", match[0])
		url, err := url.Parse(match[0])
		if err != nil {
			d.l.Error("Error parsing URL", "error", err)
			continue
		}

		if len(tldRegex.FindStringSubmatch(url.Hostname())) < 2 {
			d.l.Debug("No TLD found", "hostname", url.Hostname())
			d.l.Debug("Regex", "regex", tldRegex.FindStringSubmatch(url.Hostname()))
			continue
		}

		tld := tldRegex.FindStringSubmatch(url.Hostname())[1]
		d.l.Debug("TLD", "tld", tld)
		if val, ok := HANDLED_DOMAINS[tld]; ok {
			for _, path := range val.RequiredPaths {
				if ok := path.MatchString(url.Path); ok {
					if tld == "tiktok.com" {
						shouldStripEmbed = true
					} else if tld == "reddit.com" {
						val, ok := d.c.Get(match[0])
						if ok {
							d.l.Debug("Cache hit", "match", match[0])
							if !val.(bool) {
								continue
							}
						} else {
							isVideo, err := d.isRedditVideo(match[0])
							if err != nil {
								d.l.Error("Error detecting Reddit video status", "error", err)
								continue
							}
							if !isVideo {
								continue
							}
						}
						shouldStripEmbed = true
					} else if url.Host == "v.redd.it" {
						paths := strings.Split(url.Path, "/")
						d.l.Debug("Paths", "paths", paths, "url", url.Host, "path", url.Path, "len", len(paths))
						if len(paths) != 2 || paths[1] == "" {
							continue
						}

						var redirect = ""
						// check cache to avoid unnecessary requests
						val, ok := d.c.Get(paths[1])
						d.l.Debug("Cache get", "id", paths[1], "val", val, "ok", ok)
						if ok {
							d.l.Debug("Cache hit", "id", paths[1])
							if val == "" {
								continue
							}
							redirect = val.(string)
						} else {
							redirect, err = d.getVRedditRedirect(paths[1])
							if err != nil || redirect == "" {
								d.l.Error("Error fetching Reddit video redirect", "error", err)
								continue
							}
						}
						url, err = url.Parse(redirect)
						if err != nil {
							d.l.Error("Error parsing Reddit video redirect", "error", err)
							continue
						}
						shouldStripEmbed = true
					}

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
	d.l.Info("Sending reply", "content", content, "guild", m.GuildID, "author", m.Author.Username)
	_, err := s.ChannelMessageSend(m.ChannelID, content)
	if err != nil {
		d.l.Error("Error sending Discord message", "error", err)
		return
	}

	err = s.MessageReactionAdd(m.ChannelID, m.ID, REACTION_EMOJI)
	if err != nil {
		d.l.Error("Error adding Discord reaction ", "error", err)
		return
	}

	// this technically isn't accurate as the suppress flag also is used for Reddit but it's fine for now
	if !shouldStripEmbed && SUPPRESS_EMBEDS {
		return
	}

	edit := discordgo.NewMessageEdit(m.ChannelID, m.ID)
	edit.Flags = discordgo.MessageFlagsSuppressEmbeds
	_, err = s.ChannelMessageEditComplex(edit)
	if err != nil {
		if strings.Contains(err.Error(), "Missing Permissions") {
			g, err := s.Guild(m.GuildID)
			if err != nil {
				d.l.Error("Error fetching guild information", "error", err)
				return
			}
			d.l.Warn("Bot does not have permission to suppress embeds", "guild", g.Name, "guild_id", g.ID)
		}
		return
	}
}

func NewDiscordHandler(level slog.Level) (*DiscordBotHandler, error) {
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	logger := slog.New(h)
	logger.Debug("Creating new DiscordBotHandler")
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e5,
		MaxCost:     1e6,
		BufferItems: 64,
	})
	if err != nil {
		return nil, err
	}

	return &DiscordBotHandler{
		c: cache,
		l: logger,
	}, nil
}
