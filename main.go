package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"maps"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
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

func (vid *Video) id() string {
	id, err := getVideoID(vid.Url)
	if err != nil {
		id = "dQw4w9WgXcQ"
	}
	return id
}

type Subtitle struct {
	ChannelName string `json:"name"`
	URL         string `json:"url"`
}

type Channel struct {
	Name      string
	WatchTime time.Duration
}

func getVidDurCache() map[string]time.Duration {
	content, err := os.ReadFile("./cache/videoDurationCache.json")
	var videoDurationCache map[string]time.Duration
	err = json.Unmarshal(content, &videoDurationCache)
	if err != nil {
		fmt.Println("Err unmarshalling cache")
		panic(err)
	}
	return videoDurationCache
}

func getVidsDuration(videos []Video, apiKey string) ([]Video, error) {
	if len(videos) > 50 {
		return nil, fmt.Errorf("Cannot get video durations for more than 50 videos at a time; trying to get %d", len(videos))
	}

	videoDurationCache := getVidDurCache()

	vidsById := make(map[string]Video)
	cachedVidsById := make(map[string]Video)
	for _, video := range videos {
		duration, ok := videoDurationCache[video.Url]
		if !ok {
			vidsById[video.id()] = video
		} else {
			video.Duration = duration
			cachedVidsById[video.id()] = video
		}
	}

	if len(vidsById) == 0 {
		return slices.Collect(maps.Values(cachedVidsById)), nil
	}

	fmt.Printf("Requesting data for %s\n", strings.Join(slices.Collect(maps.Keys(vidsById)), ", "))

	videoIDsQuery := strings.Join(slices.Collect(maps.Keys(vidsById)), ",")
	searchURL := fmt.Sprintf("https://www.googleapis.com/youtube/v3/videos?id=%s&key=%s&part=contentDetails", videoIDsQuery, apiKey)

	resp, err := http.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch video data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}

	var data struct {
		Items []struct {
			Id             string `json:"id"`
			ContentDetails struct {
				Duration string `json:"duration"`
			} `json:"contentDetails"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	for _, vidResponse := range data.Items {
		video, ok := vidsById[vidResponse.Id]
		if !ok {
			// panic(fmt.Errorf("Cannot find video id for response %s.", vidResponse.Id))
			continue
		}

		duration, err := parseYouTubeDuration(vidResponse.ContentDetails.Duration)
		if err != nil {
			duration = 10 * time.Minute
		}
		video.Duration = duration
		// fmt.Printf("Setting duration of vid %s to %s\n", video.id(), video.Duration)

		vidsById[vidResponse.Id] = video
	}
	// readable, err := json.Marshal(vidsById)
	// fmt.Printf("response: %s", readable)

	finalVids := slices.Collect(maps.Values(vidsById))
	finalVids = append(finalVids, slices.Collect(maps.Values(cachedVidsById))...)
	return finalVids, nil
}

func parseYouTubeDuration(iso string) (time.Duration, error) {
	re := regexp.MustCompile(`^P(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+(?:\.\d+)?)S)?)?$`)
	matches := re.FindStringSubmatch(iso)
	if matches == nil {
		return 0, fmt.Errorf("invalid ISO 8601 duration: %q", iso)
	}

	parseInt := func(s string) (int64, error) {
		if s == "" {
			return 0, nil
		}
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, err
		}
		return v, nil
	}

	days, err := parseInt(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid days in duration: %w", err)
	}
	hours, err := parseInt(matches[2])
	if err != nil {
		return 0, fmt.Errorf("invalid hours in duration: %w", err)
	}
	minutes, err := parseInt(matches[3])
	if err != nil {
		return 0, fmt.Errorf("invalid minutes in duration: %w", err)
	}

	var seconds float64
	if matches[4] != "" {
		seconds, err = strconv.ParseFloat(matches[4], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid seconds in duration: %w", err)
		}
	}

	totalSeconds := float64(days)*24*3600 +
		float64(hours)*3600 +
		float64(minutes)*60 +
		seconds

	dur := time.Duration(totalSeconds * float64(time.Second))
	return dur, nil
}

func getVideoID(videoURL string) (string, error) {
	u, err := url.Parse(videoURL)
	if err != nil {
		return "", err
	}

	if u.Host == "youtu.be" {
		id := strings.TrimPrefix(u.Path, "/")
		if id != "" {
			return id, nil
		}
	}

	if strings.Contains(u.Host, "youtube.com") {
		query := u.Query()
		if v := query.Get("v"); v != "" {
			return v, nil
		}

		if after, ok := strings.CutPrefix(u.Path, "/embed/"); ok {
			return after, nil
		}
	}

	return "", errors.New("invalid YouTube URL or no video ID found")
}

var indiceType string

func init() {
	flag.StringVar(&indiceType, "type", "month", "What kind of indice to use (month, day, year, hour)")
}

func main() {
	flag.Parse()
	fmt.Println("Organizing by " + indiceType)

	token, err := os.ReadFile("./token.txt")
	if err != nil {
		fmt.Println("Error reading token file. Does it exist?")
		return
	}

	content, err := os.ReadFile("./watch-history.json")
	if err != nil {
		fmt.Println("Error reading file: ", err)
		return
	}

	var watchHistory []Video
	if err := json.Unmarshal(content, &watchHistory); err != nil {
		panic(err)
	}

	sort.Slice(watchHistory, func(i, j int) bool {
		return watchHistory[i].Time.Before(watchHistory[j].Time)
	})

	chunks := slices.Chunk(watchHistory, 50)
	watchHistoryChunks := []Video{}
	for chunk := range chunks {
		adjustedChunk, err := getVidsDuration(chunk, string(token))
		if err != nil {
			panic(err)
		}
		watchHistoryChunks = append(watchHistoryChunks, adjustedChunk...)
		// jsonData, err := json.MarshalIndent(watchHistory, "", " ")
		// if err != nil {
		// 	panic(err)
		// }
		// fmt.Println(string(jsonData))
		// return
	}

	watchHistory = watchHistoryChunks

	vidDurCache := getVidDurCache()
	for _, vid := range watchHistory {
		vidDurCache[vid.Url] = vid.Duration
	}

	cacheJsonData, err := json.MarshalIndent(vidDurCache, "", "  ")
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
	} else {
		err = os.WriteFile("cache/videoDurationCache.json", cacheJsonData, 0644)
		if err != nil {
			fmt.Println("Error writing cache:", err)
		}
	}

	slices.SortFunc(watchHistory, func(i, j Video) int {
		return int(i.Time.Sub(j.Time))
	})
	for i, video := range watchHistory {
		if i == len(watchHistory)-1 {
			break
		}

		watchTime := watchHistory[i+1].Time.Sub(video.Time)
		if watchTime < 0 {
			panic(fmt.Errorf("Negative watch time of %s for %s\n", watchTime, video.Title))
		}

		// bc of 2x watch speed
		adjustedDuration := time.Duration(float64(video.Duration) / 1.5)

		if watchTime > adjustedDuration {
			video.WatchTime = adjustedDuration
			// fmt.Printf("Truncating watch time of %s from %s to %s (real duration %s)\n", video.Title, watchTime, adjustedDuration, video.Duration)
		} else {
			video.WatchTime = watchTime
		}

		// finishTime := video.Time.Add(video.WatchTime).Local()
		// if 2 < finishTime.Hour() && finishTime.Hour() < 8 {
		// 	fmt.Printf("Video %s theoretically finished at %s after %s\n", video.Title, finishTime, video.WatchTime)
		// }

		// fmt.Printf("%s: %s, %s", watchHistory[i].Title, watchHistory[i].WatchTime, watchHistory[i].Duration)
		watchHistory[i] = video
	}

	var prunedWatchHistory []Video
	for _, video := range watchHistory {
		if video.WatchTime < 30*time.Second && video.Duration > 30*time.Second {
			continue
		}

		if len(video.Subtitles) == 0 {
			fmt.Printf("Video %s has no channel name\n", video.Title)
			continue
		}

		prunedWatchHistory = append(prunedWatchHistory, video)
	}

	intermediateJsonData, err := json.Marshal(prunedWatchHistory)
	if err != nil {
		log.Fatal(err)
	}

	intermediateFile, err := os.Create("out/video_data.json")
	if err != nil {
		log.Fatal(err)
	}
	defer intermediateFile.Close()

	_, err = intermediateFile.Write(intermediateJsonData)
	if err != nil {
		log.Fatal(err)
	}

	// for _, video := range prunedWatchHistory {
	// 	fmt.Printf("%s was accessed at %s for %s\n", video.Title, video.Time, video.WatchTime)
	// }
	var totalTime time.Duration
	for _, video := range prunedWatchHistory {
		totalTime += video.WatchTime
	}
	fmt.Printf("Total watch time: %s\n", totalTime)

	channelsByIndex := make(map[int]map[string]Channel)
	for _, video := range prunedWatchHistory {
		var index int

		switch indiceType {
		case "day":
			index = video.Time.YearDay()
		case "month":
			index = int(video.Time.Month())
		case "hour":
			index = int(video.Time.YearDay()*24 + video.Time.Hour())
		case "year":
			index = int(video.Time.Year())
		case "weekday":
			index = int(video.Time.Weekday())
			if index == 0 {
				index = 7
			}
		default:
			index = int(video.Time.Month())
		}

		indexData, ok := channelsByIndex[int(index)]
		if !ok {
			channelsByIndex[int(index)] = make(map[string]Channel)
			indexData = channelsByIndex[int(index)]
		}

		channelName := video.Subtitles[0].ChannelName
		channel, ok := indexData[channelName]
		if !ok {
			indexData[channelName] = Channel{Name: channelName}
			channel = indexData[channelName]
		}

		channel.WatchTime += video.WatchTime
		channelsByIndex[int(index)][channelName] = channel
	}

	var maxMonth int
	for k := range channelsByIndex {
		if k > maxMonth {
			maxMonth = k
		}
	}

	data := make([][]Channel, maxMonth)
	for index, indexData := range channelsByIndex {
		values := make([]Channel, 0, len(indexData))
		for _, v := range indexData {
			values = append(values, v)
		}

		sort.Slice(values, func(i, j int) bool {
			return values[i].WatchTime < values[j].WatchTime
		})

		data[index-1] = values
	}

	for i, index := range data {
		if index == nil {
			data[i] = []Channel{}
		}
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatal(err)
	}

	file, err := os.Create("out/channels_by_index.json")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	_, err = file.Write(jsonData)
	if err != nil {
		log.Fatal(err)
	}
}
