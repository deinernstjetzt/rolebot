package main

import (
  "os"
  "io/ioutil"
  "fmt"
  "encoding/json"
  "github.com/bwmarrin/discordgo"
  "syscall"
  "os/signal"
  "strings"
)

type Config struct {
  Token string `json:"token"`
  Game string `json:"game"`
  Server []ConfigServer `json:"server"`
}

type ConfigServer struct {
  ServerID string `json:"serverid"`
  ChannelID string `json:"channelid"`
  Admin string `json:"adminid"`
  Roles []ConfigRoles `json:"roles"`
}

type ConfigRoles struct {
  Emoji string `json:"emoji"`
  Role string `json:"role"`
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

  json.Unmarshal(buf, &config)
}


func main() {
  loadConfig()
  discord, _ := bot_main()
  defer discord.Close()
  sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}

func bot_main() (d *discordgo.Session, e error) {
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

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
  if m.Content == "ping" {
    s.ChannelMessageSend(m.ChannelID, "pong!")
  }

  if m.Content == "!addserver" {
    for _, server := range config.Server {
      if server.ServerID == m.GuildID {
        fmt.Println("Already added!")
        _, err := s.ChannelMessageSend(m.ChannelID, "This server has already been added!")
        if err != nil {
          fmt.Println(err)
        }
        return
      }
    }

    newconfig := config
    server := ConfigServer {
      ServerID: m.GuildID,
      ChannelID: m.ChannelID,
      Roles: make([]ConfigRoles, 1),
      Admin: m.Author.ID,
    }

    newconfig.Server = append(newconfig.Server, server)
    updateConfig(newconfig)
    fmt.Printf("Added server %v to rbot config\n", m.GuildID)
    s.ChannelMessageSend(m.ChannelID, "This server has been successfully added")
  }

  if strings.Index(m.Content, "!addrole") == 0 {
    //fmt.Println(extractEmojiName(m.Message))
    addRoleToServer(m.Message)
  }

  if strings.Index(m.Content, "!remrole") == 0 {
    fmt.Println("yoink")
    removeRoleFromServer(m.Message)
  }
}

func reactionAdd(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
  for _, server := range config.Server {
    if server.ServerID == m.GuildID && server.ChannelID == m.ChannelID {
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
  }
}

func reactionRemove(s *discordgo.Session, m *discordgo.MessageReactionRemove) {
  for _, server := range config.Server {
    if server.ServerID == m.GuildID && server.ChannelID == m.ChannelID {
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
  }
}

func updateConfig(newconfig Config) {
  data, err := json.Marshal(newconfig)

  if err != nil {
    fmt.Printf("Failed to update config: %v\n", err)
  }

  file, err2 := os.Create(configFile)
  defer file.Close()

  if err2 != nil {
    fmt.Printf("Can't replace config file: %v\n", err2)
  }

  _, err2 = file.Write(data)

  if err2 != nil {
    fmt.Printf("Can't write to config file: %v\n", err2)
  }

  config = newconfig
}

type GenericError struct {}
func (g GenericError) Error() string {
  return "error"
}

func getRoleId(message *discordgo.Message) (s string, e error) {
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

func extractEmojiName(message *discordgo.Message) (s string, e error) {
  i := strings.Index(message.Content, "<:")

  if i == -1 {
    e = GenericError {}
    return
  }

  slice := message.Content[i+2:]
  j := strings.Index(slice, ":")

  if j == -1 {
    e = GenericError {}
    return
  }

  s = slice[:j]
  return
}

func addRoleToServer(message *discordgo.Message) error {
  id, err := getRoleId(message)

  if err != nil {
    return err
  }

  emoji, err2 := extractEmojiName(message)

  if err2 != nil {
    return err
  }

  roles := ConfigRoles {
    Emoji: emoji,
    Role: id,
  }

  fmt.Println(roles)

  newconfig := config
  for i, server := range newconfig.Server {
    if server.ServerID == message.GuildID {
      if message.Author.ID == server.Admin {
        newconfig.Server[i].Roles = append(newconfig.Server[i].Roles, roles)
        updateConfig(newconfig)
      } else {
        fmt.Println("permission denied")
      }
    }
  }

  return nil
}

func arrayRemove(arr []ConfigRoles, index int) []ConfigRoles {
  arr[index], arr[len(arr) - 1] = arr[len(arr) - 1], arr[index]
  return arr[:len(arr) - 1]
}

func removeRoleFromServer(message *discordgo.Message) error {
  id, err := getRoleId(message)

  if err != nil {
    return err
  }

  newconfig := config
  for i, server := range newconfig.Server {
    if server.ServerID == message.GuildID {
      if message.Author.ID != server.Admin {
        fmt.Println("Permission denied")
        return GenericError {}
      }
      for j, role := range server.Roles {
        if role.Role == id {
          fmt.Printf("Deleting %v from server\n", id)
          newconfig.Server[i].Roles = arrayRemove(newconfig.Server[i].Roles, j)
          updateConfig(newconfig)
        }
      }
    }
  }

  return nil
}
