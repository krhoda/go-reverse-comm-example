package main

import (
	"flag"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

var (
	flagPort string

	timeFmtStr = "Mon, 02 Jan 2006 15:04:05 MST"

	// mutexes to protect the maps.
	commandLock sync.Mutex
	timeLock    sync.Mutex

	// A map of client IDs to channels that the server can use to
	// issue a check in command to the client.
	clientCommandMap = make(map[string]chan interface{})

	// A map of client IDs to channels that recieve the timestamp
	// request data.
	clientTimeMap = make(map[string]chan time.Time)
)

func main() {
	// check for port arg.
	parseFlags()

	// My prefered golang webserver.
	// There's nothing wrong with Gorilla or standard lib, though.
	// Can't speak to others.
	e := echo.New()

	// This example doesn't care about cross-origin security.
	e.Use(middleware.CORSWithConfig(
		middleware.CORSConfig{
			AllowOrigins: []string{"*"},
		},
	))

	// Clients use this route to keep alive a long-poll conx.
	e.GET("/client-long-poll/:clientID", handleCheckIn)
	// Once informed by the server it is needed, the client will use this to send the time.
	e.GET("/client-time/:clientID/:timestamp", handleSetTime)
	// Used to get a given Client's timestamp.
	e.GET("/clients/:clientID/system-time", handleGetTime)

	// Start the server, log fatal on crash.
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", flagPort)))
}

func parseFlags() {
	flag.StringVar(&flagPort, "port", "7777", "server port")
	flag.Parse()
}

type checkInResp struct {
	AskForTime bool `json:"ask_for_time"`
}

func handleCheckIn(c echo.Context) (err error) {
	id := c.Param("clientID")

	// this a simplistic approach, but sufficient for the example.
	commandLock.Lock()
	ch, ok := clientCommandMap[id]
	if !ok {
		ch = make(chan interface{})
		clientCommandMap[id] = ch
	}
	commandLock.Unlock()

	res := checkInResp{
		AskForTime: false,
	}

	select {
	case <-ch:
		// a command was heard, set to true.
		res.AskForTime = true
	case <-time.After(time.Minute):
		// continue execution
	}

	return c.JSON(http.StatusOK, res)
}

type setRes struct {
	Error bool   `json:"error"`
	Msg   string `json:"msg"`
}

func handleSetTime(c echo.Context) (err error) {
	res := setRes{
		Error: false,
	}

	id := c.Param("clientID")
	tStr := c.Param("timestamp")

	ts, err := time.Parse(timeFmtStr, tStr)
	if err != nil {
		res.Error = true
		res.Msg = err.Error()
		return c.JSON(http.StatusBadRequest, res)
	}

	timeLock.Lock()
	// not checking for ok because mechanically, the entry must
	// exist at this point, and it is a demo.
	// In production code, I'd be defensive enough to check
	// the ok anyway.
	ch := clientTimeMap[id]
	timeLock.Unlock()

	// Asynchronously write to the channel to free the client back up
	// to continue longpolling.
	go func() {
		ch <- ts
	}()

	return c.JSON(http.StatusOK, res)
}

type timeRes struct {
	setRes
	TS string `json:"ts"`
}

func handleGetTime(c echo.Context) (err error) {
	var res timeRes
	res.Error = false

	id := c.Param("clientID")

	// First, check the time map, because this function has the
	// responsibility to initialize the time channel.
	timeLock.Lock()
	tCh, ok := clientTimeMap[id]
	if !ok {
		tCh = make(chan time.Time)
		clientTimeMap[id] = tCh
	}
	timeLock.Unlock()

	commandLock.Lock()
	ch, ok := clientCommandMap[id]
	commandLock.Unlock()

	if !ok {
		res.Error = true
		res.Msg = fmt.Sprintf("Unknown Client ID: %s", id)
		return c.JSON(http.StatusNotFound, res)
	}

	// avoid deadlock, but leak a routine.
	// there are better ways, like a buffered channel.
	go func() {
		// issue the command
		ch <- nil
	}()

	// listen for the response
	select {
	case ts := <-tCh:
		res.TS = ts.Format(timeFmtStr)
		return c.JSON(http.StatusOK, res)

	case <-time.After(time.Second * 5):
		res.Error = true
		res.Msg = fmt.Sprintf("Client at ID %s did not reply within 5 seconds", id)
		return c.JSON(http.StatusNotFound, res)
	}
}
