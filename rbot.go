package main

import (
  "os"
  "io/ioutil"
  "fmt"
  "encoding/json"
  "github.com/bwmarrin/discordgo"
  "syscall"
  "os/signal"
)

type Config struct {
  Token string `json:"token"`
  Game string `json:"game"`
  Server []ConfigServer `json:"server"`
}

type ConfigServer struct {
  ServerID string `json:"serverid"`
  ChannelID string `json:"channelid"`
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
  bot_main()
  sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}

func bot_main() {
  discord, err := discordgo.New("Bot " + config.Token)
  if err != nil {
    fmt.Println("Failed to connect to discord api")
    return
  }

  defer discord.Close()
  discord.AddHandler(messageCreate)
  discord.AddHandler(reactionAdd)
  discord.AddHandler(reactionRemove)

  err = discord.Open()
  if err != nil {
    fmt.Println("Failed to connect to discord api")
    return
  }

  fmt.Println("Bot is booted up and ready!")
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
    }

    newconfig.Server = append(newconfig.Server, server)
    updateConfig(newconfig)
    fmt.Printf("Added server %v to rbot config\n", m.GuildID)
  }

}

func reactionAdd(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
  for _, server := range config.Server {
    if server.ServerID == m.GuildID {
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
    if server.ServerID == m.GuildID {
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
