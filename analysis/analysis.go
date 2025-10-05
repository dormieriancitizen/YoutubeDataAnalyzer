package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"time"
)

type Video struct {
	Header           string     `json:"header"`
	Title            string     `json:"title"`
	Url              string     `json:"titleUrl"`
	Subtitles        []Subtitle `json:"subtitles,omitempty"`
	Time             time.Time  `json:"time"`
	Products         []string   `json:"products,omitempty"`
	ActivityControls []string   `json:"activityControls,omitempty"`
	WatchTime        time.Duration
	Duration         time.Duration
}

type Subtitle struct {
	ChannelName string `json:"name"`
	URL         string `json:"url"`
}

type Channel struct {
	Name       string
	WatchTime  time.Duration
	TotalTime  time.Duration
	WatchRatio float64
}

func main() {
	content, err := os.ReadFile("./out/video_data.json")
	if err != nil {
		panic(err)
	}
	var videos []Video
	err = json.Unmarshal(content, &videos)
	if err != nil {
		panic(err)
	}

	channelsByName := map[string]*Channel{}
	for _, vid := range videos {
		if float64(vid.WatchTime)/float64(vid.Duration) < 0.20 {
			continue
		}
		if len(vid.Subtitles) == 0 {
			continue
		}
		channel, ok := channelsByName[vid.Subtitles[0].ChannelName]
		if !ok {
			channel = &Channel{Name: vid.Subtitles[0].ChannelName}
			channelsByName[vid.Subtitles[0].ChannelName] = channel
		}
		channel.WatchTime += vid.WatchTime
		channel.TotalTime += (vid.Duration / 2)
	}
	for _, channel := range channelsByName {
		channel.WatchRatio = (float64(channel.WatchTime) / float64(channel.TotalTime))
	}

	channels := slices.Collect(maps.Values(channelsByName))
	slices.SortFunc(channels, func(a, b *Channel) int {
		if a.WatchTime < b.WatchTime {
			return 1
		} else if a.WatchTime > b.WatchTime {
			return -1
		}
		return 0
	})
	for _, channel := range channels {
		fmt.Printf("%s\t%.2f\t%f\n", channel.Name, channel.WatchRatio, channel.WatchTime.Minutes())
	}
}
