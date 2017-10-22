package main

import (
	"fmt"
	"flag"
	"github.com/lazylynx/slack"
	"os"
	"strconv"
	"time"
)

type HistoryType int

const (
	_     HistoryType = iota
	CHANNEL
	GROUP
)

var (
	cli *slack.Client
	usermap map[string]string
	emojis map[string]ReactionStats
	limit = 1000

	//	args
	token   = flag.String("t", "", "legacy token in your slack group")
	channel = flag.String("c", "", "channel name retrieving reactions from")
	group   = flag.String("g", "", "group name retrieving reactions from")
	verbose = flag.Bool("v", false, "output logs")
	emoji   = flag.String("e", "", "pass emoji name without ':' if needs filtering")
	since   = flag.String("s", "", "date stats start, pass in yyyyMMdd")
)

func main() {
	// parse options
	flag.Parse()
	// check
	if len(*token) == 0 {
		fmt.Fprint(os.Stderr,"set your token\n")
		os.Exit(1)
	}
	if len(*channel) == 0 && len(*group) == 0 {
		fmt.Fprint(os.Stderr,"must pass at least one channel or group\n")
		os.Exit(1)
	}

	if len(*channel) > 0 && len(*group) > 0 {
		fmt.Fprint(os.Stderr,"only one channel or group allowed\n")
		os.Exit(1)
	}

	cli = slack.New(*token)
	cli.SetDebug(*verbose)

	// retrieve user list
	users, err := cli.GetUsers()
	if err != nil {
		fmt.Fprintf(os.Stderr,"%s\n", err)
		os.Exit(1)
	}
	usermap = map[string]string{}
	for _, user := range users {
		usermap[user.ID] = user.Name
	}

	// convert date start stat to unixtime
	statsSince := ""
	if len(*since) > 0 {
		layout := "20060102"
		parsed, err := time.Parse(layout, *since)
		if err != nil {
			fmt.Fprintf(os.Stderr,"%s\nyou pass illegal date format\n", err)
			os.Exit(1)
		}
		statsSince = strconv.FormatInt(parsed.Unix(), 10)
	}

	emojis = map[string]ReactionStats{}

	// fetch reactions
	if len(*channel) > 0 {
		channels := retrieveChannels()
		channelId := channels[*channel]
		if len(channelId) == 0 {
			fmt.Fprintf(os.Stderr,"no such channel: %s\n", *channel)
			os.Exit(1)
		}
		makeStats(CHANNEL, channelId, statsSince)
	}

	if len(*group) > 0 {
		groups := retrieveGroups()
		groupId := groups[*group]
		if len(groupId) == 0 {
			fmt.Fprintf(os.Stderr, "no such group: %s\n", *group)
			os.Exit(1)
		}
		makeStats(GROUP, groupId, statsSince)
	}

	// output results
	for name, entry := range emojis {
		if len(*emoji) > 0 && name != *emoji {
			continue
		}
		fmt.Printf("emoji: %s\n", name)
		fmt.Printf("	count: %d\n", entry.count)
		fmt.Printf("	BY\n")
		for k, v := range entry.by {
			fmt.Printf("		%s %d\n", k, v)
		}
		fmt.Printf("	TO\n")
		for k, v := range entry.to {
			fmt.Printf("		%s %d\n", k, v)
		}
	}
}

// fetch history and make reaction stats
func makeStats(historyType HistoryType, identifier string, since string) {
	latest := strconv.FormatInt(time.Now().Unix(),10)
	for {
		params := slack.NewHistoryParameters()
		params.Count = limit
		params.Latest = latest
		if len(since) > 0 {
			params.Oldest = since
		}
		var history *slack.History
		var err error
		switch historyType {
		case CHANNEL:
			history, err = cli.GetChannelHistory(identifier, params)
			break
		case GROUP:
			history, err = cli.GetGroupHistory(identifier, params)
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr,"%s\n", err)
			os.Exit(1)
		}
		latest = analyzeReactions(history)
		if !history.HasMore || latest == "0" {
			break
		}
	}
}

// analyze reactions
// returns oldest message timestamp
// stores analyzed data to "emojis"
func analyzeReactions(history *slack.History) (string) {
	var oldestTimeStamp string = "0"
	messages := history.Messages
	for index, message := range messages {
		if index == len(messages) - 1 {
			oldestTimeStamp = message.Timestamp
		}
		messageUser := usermap[message.User]
		reactions := message.Reactions
		if len(reactions) > 0 {
			for _, reaction := range reactions {
				stats, contains := emojis[reaction.Name]
				if !contains {
					stats = ReactionStats{
						count: 0,
						to:    map[string]int{},
						by:    map[string]int{},
					}
				}
				// count
				stats.count += reaction.Count

				// user reactED
				countTo, contains := stats.to[messageUser]
				if !contains {
					stats.to[messageUser] = reaction.Count
				} else {
					stats.to[messageUser] = countTo + reaction.Count
				}

				// user reactS
				for _, user := range reaction.Users {
					userName, contains := usermap[user]
					if !contains {
						continue
					}
					countBy, contains := stats.by[userName]
					if !contains {
						stats.by[userName] = 1
					} else {
						stats.by[userName] = countBy + 1
					}
				}
				emojis[reaction.Name] = stats
			}
		}
	}
	return oldestTimeStamp
}

// stores reaction stats
type ReactionStats struct {
	count int
	by    map[string]int
	to    map[string]int
}

// retrieve channels name to id map
func retrieveChannels() (map[string]string) {
	ret := map[string]string{}
	channels, err := cli.GetChannels(false)
	if err != nil {
		fmt.Printf("%s\n", err)
		return ret
	}
	for _, channel := range channels {
		ret[channel.Name] = channel.ID
	}
	return ret
}

// retrieve groups name to id map
func retrieveGroups() (map[string]string) {
	ret := map[string]string{}
	groups, err := cli.GetGroups(false)
	if err != nil {
		fmt.Printf("%s\n", err)
		return ret
	}
	for _, group := range groups {
		ret[group.Name] = group.ID
	}
	return ret
}
