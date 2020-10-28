package main

import (
	"log"
	"sync"
	"testing"
	"time"
)

func TestCheckIn(t *testing.T) {
	cMap := make(map[string]chan interface{})
	var cLock sync.Mutex
	id := "test_id"

	log.Println("Testing check-in without a request for time, should take 60 seconds...")

	res := checkIn(id, cMap, &cLock)

	ch, ok := cMap[id]
	if !ok {
		t.Error("Could not find channel in commandMap for test client ID after check in")
	}

	if res.AskForTime {
		t.Error("AskForTime was set to true, but no ask was issued")
	}

	done := make(chan interface{})

	go func() {
		ch <- nil
	}()

	go func() {
		res = checkIn(id, cMap, &cLock)
		close(done)
	}()

	<-done

	if !res.AskForTime {
		t.Error("AskForTime was set to false, but an ask was issued")
	}
}

func TestSetTime(t *testing.T) {
	tMap := make(map[string]chan time.Time)
	var tLock sync.Mutex
	id := "test_id"
	timeFmt := "Mon, 02 Jan 2006 15:04:05 MST"
	ch := make(chan time.Time)
	tMap[id] = ch

	res := setTime(id, timeFmt, timeFmt, tMap, &tLock)

	if res.Error {
		t.Errorf("Was unable to parse time into string in setTime, err: %s", res.Msg)
	}

	testTime := <-ch
	if testTime.Format(timeFmt) != timeFmt {
		t.Errorf("Did not recieve expected time from time channel")
	}
}

func TestGetTime(t *testing.T) {

	id := "test_id"
	timeFmt := "Mon, 02 Jan 2006 15:04:05 MST"

	ts, _ := time.Parse(timeFmt, timeFmt)

	tMap := make(map[string]chan time.Time)
	var tLock sync.Mutex

	tCh := make(chan time.Time)
	tMap[id] = tCh

	cMap := make(map[string]chan interface{})
	var cLock sync.Mutex

	cCh := make(chan interface{})
	cMap[id] = cCh

	done := make(chan interface{})
	var res timeRes

	go func() {
		res = getTime(id, cMap, tMap, &cLock, &tLock)
		if res.Error {
			t.Errorf("Failed in TestGetTime, err: %s", res.Msg)
		}
		close(done)
	}()

	<-cCh
	tCh <- ts
	<-done

	log.Println("Testing 5 second time out...")

	res = getTime(id, cMap, tMap, &cLock, &tLock)
	if !res.Error {
		t.Error("Should have failed with a timeout, but did not")
	}
}
