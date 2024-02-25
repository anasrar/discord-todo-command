package main

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Bot parameters
var (
	GuildID        = flag.String("guild", "", "Test guild ID. If not passed - bot registers commands globally")
	BotToken       = flag.String("token", "", "Bot access token")
	RemoveCommands = flag.Bool("rmcmd", true, "Remove all commands after shutdowning or not")
)

var s *discordgo.Session

func init() { flag.Parse() }

func init() {
	var err error
	s, err = discordgo.New("Bot " + *BotToken)
	if err != nil {
		log.Fatalf("Invalid bot parameters: %v", err)
	}
}

func allCommands(channelId string, msgId string) string {
	return "```" + "\n" +
		"/help-task" + "\n" +
		"```" +
		"```" + "\n" +
		"/add-task channel-id:" + channelId + " message-id:" + msgId + " title:" + "\n" +
		"```" +
		"```" + "\n" +
		"/remove-task channel-id:" + channelId + " message-id:" + msgId + " hash:" + "\n" +
		"```" +
		"```" + "\n" +
		"/message-to-task channel-id:" + channelId + " message-id:" + msgId + " target-message-id:" + "\n" +
		"```" +
		"```" + "\n" +
		"/change-status-task channel-id:" + channelId + " message-id:" + msgId + " hash: status:" + "\n" +
		"```"
}

func generateHash(title string) string {
	hash := md5.Sum([]byte(title))
	return hex.EncodeToString(hash[:])[:8]
}

var (
	integerOptionMinValue          = 1.0
	dmPermission                   = false
	defaultMemberPermissions int64 = discordgo.PermissionManageServer

	commands = []*discordgo.ApplicationCommand{
		{
			Name:        "help-task",
			Description: "Show help for task slash command",
		},
		{
			Name:        "create-task",
			Description: "Create new task list",
		},
		{
			Name:        "add-task",
			Description: "Add new task to the list",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "channel-id",
					Description: "Target thread using channel id",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "message-id",
					Description: "Target thread using message id",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "title",
					Description: "Task title",
					Required:    true,
				},
			},
		},
		{
			Name:        "remove-task",
			Description: "Remove task from the list",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "channel-id",
					Description: "Target thread using channel id",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "message-id",
					Description: "Target thread using message id",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "hash",
					Description: "Task hash",
					Required:    true,
				},
			},
		},
		{
			Name:        "message-to-task",
			Description: "Add task from the list from message",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "channel-id",
					Description: "Target thread using channel id",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "message-id",
					Description: "Target thread using message id",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "target-message-id",
					Description: "Target message to be added to task list",
					Required:    true,
				},
			},
		},
		{
			Name:        "change-status-task",
			Description: "Change status task from the list",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "channel-id",
					Description: "Target thread using channel id",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "message-id",
					Description: "Target thread using message id",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "hash",
					Description: "Task hash",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "status",
					Description: "Task status",
					Required:    true,
				},
			},
		},
	}

	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"help-task": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: allCommands("", ""),
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		},
		"create-task": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			msg, errCreateNewMessage := s.ChannelMessageSend(i.ChannelID, "Reply with command `/add-task` to add new task to the list"+"\n"+
				"This message will got replace with task list")
			if errCreateNewMessage != nil {
				log.Fatalf("Cannot create new message: %v", errCreateNewMessage)
			}
			s.ChannelMessageEdit(msg.ChannelID, msg.ID, "Reply with command `/add-task` to add new task to the list"+"\n"+
				"This message will got replace with task list"+"\n"+
				allCommands(msg.ChannelID, msg.ID))
			_, errCreateNewThread := s.MessageThreadStart(i.ChannelID, msg.ID, time.Now().Format(time.RFC3339), 60)
			if errCreateNewThread != nil {
				log.Fatalf("Cannot create new thread: %v", errCreateNewThread)
			}
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "New task created",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		},
		"add-task": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options

			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			channelId := optionMap["channel-id"].StringValue()
			messageId := optionMap["message-id"].StringValue()
			title := optionMap["title"].StringValue()

			targetMsg, errGetMsg := s.ChannelMessage(channelId, messageId)

			if errGetMsg != nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "channel-id or message-id incorrect",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			isNew := !strings.HasPrefix(targetMsg.Content, "- ")

			if isNew {
				newHash := generateHash(title)
				s.ChannelMessageEdit(channelId, messageId, "- `"+newHash+"` :black_large_square: "+title+"\n"+
					"\n"+
					allCommands(channelId, messageId))
			} else {
				scanner := bufio.NewScanner(strings.NewReader(targetMsg.Content))
				var targetResult string = ""
				for scanner.Scan() {
					currentLine := scanner.Text()
					if currentLine != "" {
						targetResult += currentLine + "\n"
					} else {
						newHash := generateHash(title)
						s.ChannelMessageEdit(channelId, messageId, targetResult+"- `"+newHash+"` :black_large_square: "+title+"\n"+
							"\n"+
							allCommands(channelId, messageId))
						break
					}
				}
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Added new task",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		},
		"remove-task": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options

			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			channelId := optionMap["channel-id"].StringValue()
			messageId := optionMap["message-id"].StringValue()
			hash := optionMap["hash"].StringValue()

			targetMsg, errGetMsg := s.ChannelMessage(channelId, messageId)

			if errGetMsg != nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "channel-id or message-id incorrect",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			isNew := !strings.HasPrefix(targetMsg.Content, "- ")

			if isNew {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Task empty",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			} else {
				pattern, _ := regexp.Compile("^- `" + hash + "`")
				scanner := bufio.NewScanner(strings.NewReader(targetMsg.Content))
				var targetResult string = ""
				for scanner.Scan() {
					currentLine := scanner.Text()
					if currentLine != "" {
						if !pattern.Match([]byte(currentLine)) {
							targetResult += currentLine + "\n"
						}
					} else {
						s.ChannelMessageEdit(channelId, messageId, targetResult+
							"\n"+
							allCommands(channelId, messageId))
						break
					}
				}
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Task updated",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		},
		"message-to-task": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options

			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			channelId := optionMap["channel-id"].StringValue()
			messageId := optionMap["message-id"].StringValue()
			targetMessageId := optionMap["target-message-id"].StringValue()

			targetMsg, errGetMsg := s.ChannelMessage(channelId, messageId)

			if errGetMsg != nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "channel-id or message-id incorrect",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			targetTaskMsg, errGetTargetMsg := s.ChannelMessage(channelId, targetMessageId)

			if errGetTargetMsg != nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "channel-id or target-message-id incorrect",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			isNew := !strings.HasPrefix(targetMsg.Content, "- ")

			if isNew {
				scanner := bufio.NewScanner(strings.NewReader(targetTaskMsg.Content))
				var targetResult string = ""
				for scanner.Scan() {
					currentLine := scanner.Text()
					if currentLine != "" {
						newHash := generateHash(currentLine)
						targetResult += "- `" + newHash + "` :black_large_square: " + currentLine + "\n"
					}
				}
				s.ChannelMessageEdit(channelId, messageId, targetResult+
					"\n"+
					allCommands(channelId, messageId))
			} else {
				scanner := bufio.NewScanner(strings.NewReader(targetMsg.Content))
				var targetResult string = ""
				for scanner.Scan() {
					currentLine := scanner.Text()
					if currentLine != "" {
						targetResult += currentLine + "\n"
					} else {
						break
					}
				}
				scanner = bufio.NewScanner(strings.NewReader(targetTaskMsg.Content))
				for scanner.Scan() {
					currentLine := scanner.Text()
					if currentLine != "" {
						newHash := generateHash(currentLine)
						targetResult += "- `" + newHash + "` :black_large_square: " + currentLine + "\n"
					}
				}
				s.ChannelMessageEdit(channelId, messageId, targetResult+
					"\n"+
					allCommands(channelId, messageId))
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Task updated",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		},
		"change-status-task": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options

			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			channelId := optionMap["channel-id"].StringValue()
			messageId := optionMap["message-id"].StringValue()
			hash := optionMap["hash"].StringValue()
			status := optionMap["status"].StringValue()

			targetMsg, errGetMsg := s.ChannelMessage(channelId, messageId)

			if errGetMsg != nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "channel-id or message-id incorrect",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			isNew := !strings.HasPrefix(targetMsg.Content, "- ")

			if isNew {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Task empty",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			} else {
				pattern, _ := regexp.Compile("^- `" + hash + "`")
				scanner := bufio.NewScanner(strings.NewReader(targetMsg.Content))
				var targetResult string = ""
				for scanner.Scan() {
					currentLine := scanner.Text()
					if currentLine != "" {
						if !pattern.Match([]byte(currentLine)) {
							targetResult += currentLine + "\n"
						} else {
							splitedCurrentLine := strings.Split(currentLine, " ")
							splitedCurrentLine[2] = status
							targetResult += strings.Join(splitedCurrentLine[:], " ") + "\n"
						}
					} else {
						s.ChannelMessageEdit(channelId, messageId, targetResult+
							"\n"+
							allCommands(channelId, messageId))
						break
					}
				}
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Task updated",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		},
	}
)

func init() {
	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})
}

func main() {
	s.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsMessageContent)

	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	})
	err := s.Open()
	if err != nil {
		log.Fatalf("Cannot open the session: %v", err)
	}

	log.Println("Adding commands...")
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, v := range commands {
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, *GuildID, v)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[i] = cmd
	}

	defer s.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	log.Println("Press Ctrl+C to exit")
	<-stop

	if *RemoveCommands {
		log.Println("Removing commands...")
		// // We need to fetch the commands, since deleting requires the command ID.
		// // We are doing this from the returned commands on line 375, because using
		// // this will delete all the commands, which might not be desirable, so we
		// // are deleting only the commands that we added.
		// registeredCommands, err := s.ApplicationCommands(s.State.User.ID, *GuildID)
		// if err != nil {
		// 	log.Fatalf("Could not fetch registered commands: %v", err)
		// }

		for _, v := range registeredCommands {
			err := s.ApplicationCommandDelete(s.State.User.ID, *GuildID, v.ID)
			if err != nil {
				log.Panicf("Cannot delete '%v' command: %v", v.Name, err)
			}
		}
	}

	log.Println("Gracefully shutting down.")
}
