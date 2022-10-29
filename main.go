package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	scribble "github.com/nanobox-io/golang-scribble"
	"github.com/robfig/cron/v3"
	"golang.org/x/net/http2"
	tele "gopkg.in/telebot.v3"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	DbFile         = "/data/config/data.db"
	CollectionList = "ddsuChannel"
	ChatListKey    = "chatList"
	TimeLocal, _   = time.LoadLocation("Asia/Shanghai")
	HttpPrefix     = "http://192.168.200.6/sensor/"

	httpClient         *http.Client
	MaxIdleConnections int = 20
	RequestTimeout     int = 30
)

type ChannelList struct {
	ChatId map[int64]string `json:"chat_id"`
}

type SensorData struct {
	Id    string  `json:"id,omitempty"`
	Value float64 `json:"value,omitempty"`
	State string  `json:"state,omitempty"`
}

func init() {
	httpClient = createHttClient()
}

func createHttClient() *http.Client {
	client := &http.Client{
		Timeout: time.Duration(RequestTimeout) * time.Second,
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			}}}
	return client
}

func readChannelList(collection, key string) (ChannelList, error) {
	db, err := scribble.New(DbFile, nil)
	if err != nil {
		fmt.Println("Error", err)
		return ChannelList{}, err
	}
	// Read a fish from the database (passing fish by reference)
	channelList := ChannelList{}
	if err := db.Read(collection, key, &channelList); err != nil {
		fmt.Println("Error", err)
		if strings.Contains(err.Error(), "no such file or directory") {
			return ChannelList{}, nil
		} else {
			return ChannelList{}, err
		}
	}
	return channelList, nil
}

func writeChannelList(collection, key string, list ChannelList) error {
	db, err := scribble.New(DbFile, nil)
	if err != nil {
		fmt.Println("Error", err)
		return err
	}
	// Write a fish to the database
	if err := db.Write(collection, key, list); err != nil {
		fmt.Println("Error", err)
		return err
	}
	return nil
}

func main() {
	// telegram bot token
	token := os.Getenv("TOKEN")
	// cronSpec like "0 */1 * * * ?"
	cronSpec := os.Getenv("CRON_SPEC")
	// http prefix
	HttpPrefix = os.Getenv("HTTP_PREFIX")

	if token == "" || cronSpec == "" {
		log.Printf("TOKEN or CRON_SPEC not fund")
	}

	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
	}

	// start
	b.Handle("/start", func(c tele.Context) error {
		return c.Send("Hello!")
	})

	// 注册消息推送 并发送一次
	b.Handle("/ddsu", func(c tele.Context) error {
		err := register(c)
		sendDDsu666(b)
		return err
	})

	// 定时发送
	crontab := cron.New(cron.WithSeconds(), cron.WithLocation(TimeLocal))
	_, err = crontab.AddFunc(cronSpec, func() {
		// 定时采集
		sendDDsu666(b)
	})
	if err != nil {
		log.Printf("add Cron Func err %v\n", err)
	}
	crontab.Start()

	fmt.Println("bot start...")
	b.Start()
}

// 注册
func register(c tele.Context) error {
	chatId := c.Chat().ID
	chat, err := json.Marshal(c.Chat())
	if err != nil {
		return err
	}
	channelList, err := readChannelList(CollectionList, ChatListKey)
	if err != nil {
		log.Println(fmt.Sprintf("read db err %v", err))
		c.Send(fmt.Sprintf("register failed %v", err))
	} else {

		if channelList.ChatId[chatId] == "" {
			channelList.ChatId = map[int64]string{}
		}
		channelList.ChatId[chatId] = string(chat)
		err := writeChannelList(CollectionList, ChatListKey, channelList)
		if err != nil {
			log.Println(fmt.Sprintf("read db err %v", err))
			c.Send(fmt.Sprintf("register failed %v", err))
		} else {
			c.Send(fmt.Sprintf("register success"))
		}
	}
	return nil
}

// 发送
func sendDDsu666(bot *tele.Bot) {
	channelList, err := readChannelList(CollectionList, ChatListKey)
	if err != nil {
		log.Println(fmt.Sprintf("not found %v", err))
		return
	}
	for chatId, _ := range channelList.ChatId {
		// regular send options
		chat, err := bot.ChatByID(chatId)
		if err != nil {
			log.Println(fmt.Sprintf("chat by id err %v", err))
			continue
		}
		// 获取采样数据
		data, err := GetPowerData()
		if err != nil {
			data = fmt.Sprintf("采样失败: %v", err)
		}
		_, err = bot.Send(chat, data, &tele.SendOptions{
			// ...
		})
		if err != nil {
			log.Println(fmt.Sprintf("send message err %v", err))
		}
	}
}

func GetPowerData() (string, error) {

	sensorMap := map[string]*SensorData{}
	sensorIdList := []string{"p1_ep", "p1_freq", "p1_i", "p1_p", "p1_pf", "p1_q", "p1_s", "p1_u", "p2_ep", "p2_freq", "p2_i", "p2_p", "p2_pf", "p2_q", "p2_s", "p2_u"}
	nameMap := map[string]string{
		"p1_ep":   "主电表度数: %s",
		"p1_freq": "主电表频率: %s",
		"p1_i":    "主电表电流: %s",
		"p1_p":    "主电表功率: %s",
		"p1_pf":   "主电表功率系数: %s",
		"p1_q":    "主电表无功功率: %s",
		"p1_s":    "主电表伏安: %s",
		"p1_u":    "主电表电压: %s\n",
		"p2_ep":   "备电表度数: %s",
		"p2_freq": "备电表频率: %s",
		"p2_i":    "备电表电流: %s",
		"p2_p":    "备电表功率: %s",
		"p2_pf":   "备电表功率系数: %s",
		"p2_q":    "备电表无功功率: %s",
		"p2_s":    "备电表伏安: %s",
		"p2_u":    "备电表电压: %s",
	}
	startTime := time.Now()
	for _, sensorId := range sensorIdList {
		requestURL := fmt.Sprintf("%s%s", HttpPrefix, sensorId)
		res, err := httpClient.Get(requestURL)
		if err != nil {
			log.Printf("error making http request: %s\n", err)
		} else {
			sen := &SensorData{}
			err := json.NewDecoder(res.Body).Decode(sen)
			if err != nil {
				log.Printf("error decode http response: %s\n", err)
			}
			sensorMap[sensorId] = sen
		}
	}
	// 采样时间
	var timeStr = fmt.Sprintf("采样时间: %s\n耗时: %v\n", time.Now().In(TimeLocal).Format("2006-01-02 15:04:05"), time.Since(startTime))
	resultMessage := []string{timeStr}

	// 合并数据标题
	for _, sensorId := range sensorIdList {
		if sensorData, ok := sensorMap[sensorId]; ok {
			msgLine := fmt.Sprintf(nameMap[sensorId], sensorData.State)
			resultMessage = append(resultMessage, msgLine)
		}
	}
	return strings.Join(resultMessage, "\n"), nil
}
