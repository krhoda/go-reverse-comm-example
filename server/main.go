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

	res := checkIn(id, clientCommandMap, &commandLock)

	return c.JSON(http.StatusOK, res)
}

func checkIn(id string, cMap map[string]chan interface{}, lock *sync.Mutex) checkInResp {
	// this a simplistic approach, but sufficient for the example.
	lock.Lock()
	ch, ok := cMap[id]
	if !ok {
		ch = make(chan interface{})
		cMap[id] = ch
	}
	lock.Unlock()

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

	return res
}

type setRes struct {
	Error bool   `json:"error"`
	Msg   string `json:"msg"`
}

func handleSetTime(c echo.Context) (err error) {
	id := c.Param("clientID")
	tStr := c.Param("timestamp")

	res := setTime(id, tStr, timeFmtStr, clientTimeMap, &timeLock)

	return c.JSON(http.StatusOK, res)
}

func setTime(id, tStr, timeFmt string, timeMap map[string]chan time.Time, timeLock *sync.Mutex) setRes {
	res := setRes{
		Error: false,
	}

	ts, err := time.Parse(timeFmtStr, tStr)
	if err != nil {
		res.Error = true
		res.Msg = err.Error()
		return res
	}

	timeLock.Lock()
	// not checking for ok because mechanically, the entry must
	// exist at this point, and it is a demo.
	// In production code, I'd be defensive enough to check
	// the ok anyway.
	ch := timeMap[id]
	timeLock.Unlock()

	// Asynchronously write to the channel to free the client back up
	// to continue longpolling.
	go func() {
		ch <- ts
	}()

	return res
}

type timeRes struct {
	setRes
	TS string `json:"ts"`
}

func handleGetTime(c echo.Context) (err error) {
	id := c.Param("clientID")
	res := getTime(id, clientCommandMap, clientTimeMap, &commandLock, &timeLock)
	if res.Error {
		return c.JSON(http.StatusNotFound, res)
	}

	return c.JSON(http.StatusOK, res)
}

func getTime(id string, cMap map[string]chan interface{}, tMap map[string]chan time.Time, cLock, tLock *sync.Mutex) timeRes {
	var res timeRes
	res.Error = false
	// First, check the time map, because this function has the
	// responsibility to initialize the time channel.
	tLock.Lock()
	tCh, ok := tMap[id]
	if !ok {
		tCh = make(chan time.Time)
		tMap[id] = tCh
	}
	tLock.Unlock()

	cLock.Lock()
	ch, ok := cMap[id]
	cLock.Unlock()

	if !ok {
		res.Error = true
		res.Msg = fmt.Sprintf("Unknown Client ID: %s", id)
		return res
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

	case <-time.After(time.Second * 5):
		res.Error = true
		res.Msg = fmt.Sprintf("Client at ID %s did not reply within 5 seconds", id)
	}

	return res
}
