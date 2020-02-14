package main

import (
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

/*
	Type config holds all information about the bots current
	servers and a path to its corresponding json file.
	Config should be updated using Config.updateConfig() so changes
	will be saved.
*/
type Config struct {
	Token      string         `json:"token"`
	Game       string         `json:"game"`
	Server     []ConfigServer `json:"server"`
	ConfigFile string
}

/*
	Type ConfigServer represents one server and holds its
	Server specific settings.
*/
type ConfigServer struct {
	ServerID        string        `json:"serverid"`
	ChannelID       string        `json:"channelid"`
	Admin           string        `json:"adminid"`
	SecondaryAdmins []string      `json:"secondary_admins"`
	Roles           []ConfigRoles `json:"roles"`
}

/* Type ConfigRoles holds a emoji/roleid pair */
type ConfigRoles struct {
	Emoji string `json:"emoji"`
	Role  string `json:"role"`
}

var config Config

const configFile = "rbot.json"

func loadConfig() {
	file, err := os.Open(configFile)

	if err != nil {
		panic(err)
	}

	defer file.Close()
	buf, err2 := ioutil.ReadAll(file)

	if err2 != nil {
		panic(err2)
	}

	config.ConfigFile = configFile
	json.Unmarshal(buf, &config)
}

func (c ConfigServer) isAdmin(userID string) bool {
	if c.Admin == userID {
		return true
	}

	for _, admin := range c.SecondaryAdmins {
		if userID == admin {
			return true
		}
	}

	return false
}

func main() {
	loadConfig()
	discord, _ := bot_main()
	defer discord.Close()

	// This basically waits for some Interrupt signal and then stops the bot
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}

func botMain() (d *discordgo.Session, e error) {
	discord, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		fmt.Println("Failed to connect to discord api")
		e = err
		return
	}

	discord.AddHandler(messageCreate)
	discord.AddHandler(reactionAdd)
	discord.AddHandler(reactionRemove)

	err = discord.Open()
	if err != nil {
		fmt.Println("Failed to connect to discord api")
		e = err
		return
	}

	fmt.Println("Bot is booted up and ready!")
	d = discord
	e = nil
	return
}

// Function will be called when some message is send
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Content == "!addserver" {
		config.addServer(m.Message)
	}

	if strings.Index(m.Content, "!addrole") == 0 {
		config.addRoleToServer(m.Message)
	}

	if strings.Index(m.Content, "!remrole") == 0 {
		config.removeRoleFromServer(m.Message)
	}

	if strings.Index(m.Content, "!addadmin") == 0 {
		id, err := extractUserId(m.Message)
		server := config.getServer(m.GuildID)

		if server == nil || err != nil {
			return
		}

		server.addSecondaryAdmin(id)
		config.updateConfig()
	}

	if strings.Index(m.Content, "!remadmin") == 0 {
		id, err := extractUserId(m.Message)
		server := config.getServer(m.GuildID)

		if server == nil || err != nil {
			return
		}

		server.removeSecondaryAdmin(id)
		config.updateConfig()
	}
}

// Will be called when !addserver command was send
func (c *Config) addServer(m *discordgo.Message) {
	if config.getServer(m.GuildID) != nil {
		return // Dont add server that already exist
	}

	server := ConfigServer{
		ServerID:  m.GuildID,
		ChannelID: m.ChannelID,
		Roles:     make([]ConfigRoles, 1),
		Admin:     m.Author.ID,
	}

	c.Server = append(c.Server, server)
	c.updateConfig()
	fmt.Printf("Server %v has been added to this bot.\n", server.ServerID)
}

// will be called when discord Reaction add event fires
func reactionAdd(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	server := config.getServer(m.GuildID)

	if server == nil {
		return
	}

	if server.ChannelID != m.ChannelID {
		return
	}

	for _, role := range server.Roles {
		if role.Emoji == m.Emoji.Name {
			fmt.Printf("Adding role %v to user %v\n", role.Role, m.UserID)
			err := s.GuildMemberRoleAdd(m.GuildID, m.UserID, role.Role)

			if err != nil {
				fmt.Println(err)
			}
		}
	}
}

func reactionRemove(s *discordgo.Session, m *discordgo.MessageReactionRemove) {
	server := config.getServer(m.GuildID)

	if server == nil {
		return
	}

	if server.ChannelID != m.ChannelID {
		return
	}

	for _, role := range server.Roles {
		if role.Emoji == m.Emoji.Name {
			fmt.Printf("Removing role %v from user %v\n", role.Role, m.UserID)
			err := s.GuildMemberRoleRemove(m.GuildID, m.UserID, role.Role)

			if err != nil {
				fmt.Println(err)
			}
		}
	}
}

// Update config function writes the Config struct c to a json file
func (c Config) updateConfig() {
	data, err := json.Marshal(c)

	if err != nil {
		fmt.Printf("Failed to update config: %v\n", err)
	}

	file, err2 := os.Create(c.ConfigFile)
	defer file.Close()

	if err2 != nil {
		fmt.Printf("Can't replace config file: %v\n", err2)
	}

	_, err2 = file.Write(data)

	if err2 != nil {
		fmt.Printf("Can't write to config file: %v\n", err2)
	}
}

func (c *Config) getServer(id string) *ConfigServer {
	for i, server := range c.Server {
		if server.ServerID == id {
			return &c.Server[i]
		}
	}
	return nil
}

// GenericError is a placeholder for actual errors
type GenericError struct{}

func (g GenericError) Error() string {
	return "error"
}

/*
	The next 3 functions extract ids for users, roles and emojis
	from a message. Discords id's are represented in a string as follows:
	RoleID: <@&id>
	UserID: <@!id>
	EmojiID: <:name:id>		Note that non custom emojis will be send as Unicode runes
*/
func extractRoleId(message *discordgo.Message) (s string, e error) {
	i := strings.Index(message.Content, "<@&")

	if i == -1 {
		e = GenericError{}
		return
	}

	slice := message.Content[i+3:]
	j := strings.Index(slice, ">")

	if j == -1 {
		e = GenericError{}
		return
	}

	s = slice[:j]
	return
}

func extractUserId(message *discordgo.Message) (s string, e error) {
	i := strings.Index(message.Content, "<@!")

	if i == -1 {
		e = GenericError{}
		return
	}

	slice := message.Content[i+3:]
	j := strings.Index(slice, ">")

	if j == -1 {
		e = GenericError{}
		return
	}

	s = slice[:j]
	return
}

func extractEmojiName(message *discordgo.Message) (s string, e error) {
	i := strings.Index(message.Content, "<:")

	if i == -1 {
		e = GenericError{}
		return
	}

	slice := message.Content[i+2:]
	j := strings.Index(slice, ":")

	if j == -1 {
		e = GenericError{}
		return
	}

	s = slice[:j]
	return
}

func (c *Config) addRoleToServer(message *discordgo.Message) error {
	id, err := extractRoleId(message)
	server := c.getServer(message.GuildID)
	emoji, err2 := extractEmojiName(message)

	if err != nil || err2 != nil || server == nil {
		return GenericError{}
	}

	roles := ConfigRoles{
		Emoji: emoji,
		Role:  id,
	}

	if server.isAdmin(message.Author.ID) {
		server.Roles = append(server.Roles, roles)
		c.updateConfig()
	}

	return nil
}

func (c *Config) removeRoleFromServer(message *discordgo.Message) error {
	id, err := extractRoleId(message)
	server := c.getServer(message.GuildID)

	if err != nil || server == nil {
		return GenericError{}
	}

	if !server.isAdmin(message.Author.ID) {
		fmt.Println("Permission denied")
		return GenericError{}
	}

	for j, role := range server.Roles {
		if role.Role == id {
			fmt.Printf("Deleting role %v from server\n", id)
			server.Roles[len(server.Roles)-1], server.Roles[j] =
				server.Roles[j], server.Roles[len(server.Roles)-1]

			server.Roles = server.Roles[:len(server.Roles)-1]
			c.updateConfig()
			return nil
		}
	}

	return GenericError{}
}

func (c *ConfigServer) addSecondaryAdmin(userID string) error {
	c.SecondaryAdmins = append(c.SecondaryAdmins, userID)
	return nil
}

func (c *ConfigServer) removeSecondaryAdmin(userID string) error {
	for i, id := range c.SecondaryAdmins {
		if id == userID {
			c.SecondaryAdmins[i], c.SecondaryAdmins[len(c.SecondaryAdmins)-1] =
				c.SecondaryAdmins[len(c.SecondaryAdmins)-1], c.SecondaryAdmins[i]

			c.SecondaryAdmins = c.SecondaryAdmins[:len(c.SecondaryAdmins)-1]
		}
	}

	return nil
}
