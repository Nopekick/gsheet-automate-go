package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"
)

type SpreadsheetPushRequest struct {
	SpreadsheetID string        `json:"spreadsheet_id"`
	Range         string        `json:"range"`
	Values        []interface{} `json:"values"`
}

type Recent struct {
	ID        string  `json:"id"`
	Appid     int32   `json:"appid"`
	Name      string  `json:"name"`
	MP        float64 `json:"mp"`
	Phase     string  `json:"phase"`
	Wholesale float64 `json:"wholesale"`
	MyPrice   float64 `json:"my_price"`
}

type Global struct {
	ID        string  `json:"id"`
	Appid     int32   `json:"appid"`
	Name      string  `json:"name"`
	MP        float64 `json:"mp"`
	Effect    string  `json:"effect"`
	Wholesale float64 `json:"wholesale"`
	MyPrice   float64 `json:"my_price"`
}

type ListPop struct {
	ID        string  `json:"id"`
	Appid     int32   `json:"appid"`
	Name      string  `json:"name"`
	MP        float64 `json:"mp"`
	Wholesale float64 `json:"wholesale"`
	Date      string  `json:"date"`
}

var ctx = context.Background()

func checkError(err error) {
	if err != nil {
		panic(err.Error())
	}
}

func listListener(chan1 chan string, chan2 chan string, srv *sheets.Service) {
	rdb := redis.NewClient(&redis.Options{
		Password: os.Getenv("REDIS_PASSWORD"),
		Addr:     os.Getenv("REDIS_HOST"),
	})
	for {
		result, err := rdb.BLPop(ctx, 0, "test").Result()
		if err != nil {
			panic(err)
		}
		data := ListPop{}
		json.Unmarshal([]byte(result[1]), &data)

		var date string = data.Date
		var id string = data.ID
		var appid int32 = data.Appid
		var name string = data.Name
		var effect string = "None"
		var phase string = "None"
		var total float64 = data.MP + data.Wholesale

		select {
		case msg1 := <-chan1:
			subData := Recent{}
			json.Unmarshal([]byte(msg1), &subData)
			phase = subData.Phase
		case msg2 := <-chan2:
			subData := Global{}
			json.Unmarshal([]byte(msg2), &subData)
			effect = subData.Effect
		}

		object := SpreadsheetPushRequest{
			SpreadsheetID: os.Getenv("SPREADSHEET_ID"),
			Range:         "Sheet1!A1",
			Values:        []interface{}{date, id, appid, name, effect, phase, total},
		}

		var vr sheets.ValueRange
		vr.Values = append(vr.Values, object.Values)

		_, err2 := srv.Spreadsheets.Values.Append(object.SpreadsheetID, object.Range, &vr).ValueInputOption("USER_ENTERED").Do()
		if err2 != nil {
			fmt.Println("ERROR WITH SPREADSHEET APPEND: ", err)
		}
	}

}

func listener1(a chan string) {
	rdb := redis.NewClient(&redis.Options{
		Password: os.Getenv("REDIS_PASSWORD"),
		Addr:     os.Getenv("REDIS_HOST"),
	})

	sub := rdb.Subscribe(ctx, "test1")
	ch := sub.Channel()
	for msg := range ch {
		a <- msg.Payload
	}
}

func listener2(a chan string) {
	rdb := redis.NewClient(&redis.Options{
		Password: os.Getenv("REDIS_PASSWORD"),
		Addr:     os.Getenv("REDIS_HOST"),
	})

	sub := rdb.Subscribe(ctx, "test2")
	ch := sub.Channel()
	for msg := range ch {
		a <- msg.Payload
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	chan1 := make(chan string)
	chan2 := make(chan string)

	data, err := ioutil.ReadFile("keys.json")
	checkError(err)
	conf, err := google.JWTConfigFromJSON(data, sheets.SpreadsheetsScope)
	checkError(err)

	client := conf.Client(context.TODO())
	srv, err := sheets.New(client)
	checkError(err)

	go listListener(chan2, chan1, srv)
	go listener1(chan1)
	go listener2(chan2)

	c := make(chan int)
	for msg := range c {
		fmt.Println(msg)
	}
}
