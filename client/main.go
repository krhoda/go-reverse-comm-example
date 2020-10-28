package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

var (
	flagHost string
	flagPort string

	timeFmtStr = "Mon, 02 Jan 2006 15:04:05 MST"

	client = http.Client{
		Timeout: time.Minute * 2,
	}
)

type checkInResp struct {
	AskForTime bool `json:"ask_for_time"`
}

func main() {
	parseFlags()

	idStruct := uuid.New()
	id := idStruct.String()

	log.Printf("Client id %s is running, looking for a server at %s:%s\n", id, flagHost, flagPort)
	log.Printf("A GET request to http://%s:%s/clients/%s/system-time will result in reporting this client's system time\n", flagHost, flagPort, id)

	checkURL := fmt.Sprintf("http://%s:%s/client-long-poll/%s", flagHost, flagPort, id)
	respURLPrefix := fmt.Sprintf("http://%s:%s/client-time/%s/", flagHost, flagPort, id)

	// If this client were doing useful work, I would run this
	// loop in a goroutine. But it isn't, so I'm not.
	for {
		// Confine the actual action to a function to allow
		// the deferal of closures of bodies. Otherwise, would
		// have to manually close at the end.
		err := runReq(checkURL, respURLPrefix)
		if err != nil {
			// Not much else can be done.
			panic(err)
		}
	}
}

func runReq(checkURL, respURLPrefix string) error {
	resp, err := client.Get(checkURL)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var checkResp checkInResp
	err = json.Unmarshal(body, &checkResp)
	if err != nil {
		return err
	}

	if checkResp.AskForTime {
		// I would normally run this in a goroutine,
		// But err reporting infrastructure for that
		// Would be beyond the scope of the exercise,
		// So we will briefly divert before re-checking in.
		err = sendTime(respURLPrefix)
	}

	return err
}

type setRes struct {
	Error bool   `json:"error"`
	Msg   string `json:"msg"`
}

func sendTime(prefix string) (err error) {
	t := time.Now()
	ts := t.Format(timeFmtStr)

	nextURL := fmt.Sprintf("%s%s", prefix, ts)

	resp, err := client.Get(nextURL)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var result setRes
	err = json.Unmarshal(body, &result)
	if err != nil {
		return
	}

	if result.Error {
		err = fmt.Errorf(result.Msg)
	}

	return
}

func parseFlags() {
	flag.StringVar(&flagPort, "port", "7777", "server port")
	flag.StringVar(&flagHost, "host", "localhost", "server host")
	flag.Parse()
}
