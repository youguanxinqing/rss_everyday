package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mmcdole/gofeed"
)

// 基础环境配置
var (
	BotToken    *string
	ChannelID   *int64
	StartBy     *int64
	RSSFilePath *string
	DebugMode   *bool
)

func TokenValid() {
	if *BotToken == "" || *ChannelID == 0 {
		panic("BotToken && ChannelID cannot be empty")
	}
}

func init() {
	BotToken = flag.String("tg-bot", "", "Telegram bot token")
	ChannelID = flag.Int64("tg-channel", 0, "Telegram channel id")
	StartBy = flag.Int64("startby", 4, "Start by specified time(hour)")
	RSSFilePath = flag.String("rss-filepath", "rss.json", "Rss json file path")
	DebugMode = flag.Bool("debug", false, "Debug mode")
	flag.Parse()

	TokenValid()
	GetRssInfo()
}

// RSS 构成阶段
type RSSInfos struct {
	RssInfo []RssInfo `json:"rss_info"`
}

type RssInfo struct {
	Title       string `json:"title"`
	Url         string `json:"url"`
	FullContent bool   `json:"full_content"`
}

var RssInfos = RSSInfos{nil}

// 从 配置文件中获取 rss 链接
// 根据 rss 链接获取更新
func GetRssInfo() {
	rssFile, err := os.Open(*RSSFilePath)
	if err != nil {
		panic(err)
	}

	err = json.NewDecoder(rssFile).Decode(&RssInfos)
	if err != nil {
		panic(err)
	}

}

// 根据时间筛选昨天一整天的文章
func GetPosts() {
	msgList := make([]*gofeed.Item, 0)
	for _, info := range RssInfos.RssInfo {
		msgList = append(msgList, GetPostInfo(info)...)
	}
	PushPost(msgList)
}

func debugInfof(fmt string, v ...interface{}) {
	if !(*DebugMode) {
		return
	}

	if !strings.HasSuffix(fmt, "\n") {
		fmt = fmt + "\n"
	}
	log.Printf("debug: "+fmt, v...)
}

// getDatetime 从左到右, 按优先级返回有效 datetime
// 实在没有, 返回最后一个时间
func getDatetime(times ...*time.Time) *time.Time {
	for _, d := range times {
		if d != nil && !d.IsZero() {
			return d
		}
	}
	return times[len(times)-1]
}

func GetPostInfo(rss RssInfo) []*gofeed.Item {
	var msg = make([]*gofeed.Item, 0)

	now := time.Now().UTC()
	startTime := now.Add(-(time.Duration(*StartBy) * time.Hour))
	start := time.Date(startTime.Year(), startTime.Month(), startTime.Day(), startTime.Hour(), 0, 0, 0, now.Location()).Unix()
	end := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location()).Unix()

	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(rss.Url)
	if err != nil {
		log.Printf("parse url err: url=%s, %v", rss.Url, err)
	} else {
		for _, item := range feed.Items {
			debugInfof("Title=%s, Url=%s, Published=%v, Updated=%v", item.Title, item.Link, item.Published, item.Updated)

			parseDatetime := getDatetime(item.PublishedParsed, item.UpdatedParsed)
			if parseDatetime != nil && parseDatetime.Unix() >= start && parseDatetime.Unix() < end {
				msg = append(msg, item)
			}
		}
	}

	return msg
}

func logEveryArticle(msgList []*gofeed.Item) {
	for _, msg := range msgList {
		info := fmt.Sprintln(msg.Title, msg.Link)
		log.Printf("%s", info)
	}
}

func safeExtractName(author *gofeed.Person) string {
	if author == nil {
		return ""
	}
	return fmt.Sprintf("%s\n", author.Name)
}

func makeDisplayMsg(item *gofeed.Item) string {
	return fmt.Sprintf(
		"%s%s\n%s",
		safeExtractName(item.Author),
		item.Title,
		item.Link,
	)
}

// 从配置文件获取推送方式
// 使用对应的推送渠道推送文章
func PushPost(msgList []*gofeed.Item) {
	logEveryArticle(msgList)

	// directly return if debug mode
	if *DebugMode {
		return
	}

	bot, err := tgbotapi.NewBotAPI(*BotToken)
	if err != nil {
		panic(err)
	}
	for _, s := range msgList {
		displayMsg := makeDisplayMsg(s)
		_, _ = bot.Send(tgbotapi.NewMessage(*ChannelID, displayMsg))
	}
}

func main() {
	GetPosts()
}
