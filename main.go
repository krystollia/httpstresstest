package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	serverUrl = ""
	finish    bool
	isDebug   bool
)

type Stats struct {
	max   time.Duration
	min   time.Duration
	total time.Duration
	count int
}

func main() {
	fs := flag.NewFlagSet("stress-test", flag.ExitOnError)
	workerNum := fs.Int("client-num", 3, "specify the clients num to run concurrently")
	testUrl := fs.String("url", "https://fds.so/d/54378fd28a6c81e3/8C36X45AE8", "specify the url to test")
	duration := fs.Int("duration", 10, "how long the testing lasts(in seconds)")
	debug := fs.Bool("debug", true, "wether to print debug log")
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
	log.Println("url:", *testUrl)
	serverUrl = *testUrl
	log.Println("debug:", *debug)
	isDebug = *debug
	log.Println("client-num:", *workerNum)
	log.Println("duration:", *duration)

	failchan := make(chan int, *workerNum)
	lapsechan := make(chan Stats, *workerNum)
	start := time.Now()
	runningDuration := time.Duration(0)

	queue := make(chan int, *workerNum)
	for i := 0; i < *workerNum; i++ {
		queue <- 1
	}
	for i := 0; i < *workerNum; i++ {
		go workerFunc(i, queue, failchan, lapsechan)
	}

	for {
		time.Sleep(time.Duration(1) * time.Second)
		if time.Since(start).Seconds() > float64(*duration) {
			finish = true
			runningDuration = time.Since(start)
			log.Println("should finish~!!!")
			break
		}
	}

	failedRequests := 0
	lapseMax := time.Duration(0)
	lapseMin := time.Duration(100) * time.Minute
	lapseTotal := time.Duration(0)
	runs := 0
	for i := 0; i < *workerNum; i++ {
		n := <-failchan
		lapses := <-lapsechan
		if n > 0 {
			failedRequests = failedRequests + n
		}
		if lapseMax < lapses.max {
			lapseMax = lapses.max
		}
		if lapseMin > lapses.min {
			lapseMin = lapses.min
		}
		lapseTotal += lapses.total
		runs += lapses.count
	}
	lapseMean := lapseTotal.Nanoseconds() / int64(runs) / 1000000
	log.Printf("Lapse: \n	max	%v	\n	min	%v	\n	mean	%dms	\n", lapseMax, lapseMin, lapseMean)
	log.Printf("Tested with (%d clients, totally %d runs), in %v, %d failed \n\n\n", *workerNum, runs, runningDuration, failedRequests)
}

func workerFunc(i int, queue chan int, failchan chan int, lapsechan chan Stats) {
	failedRequests := 0
	lapseMax := time.Duration(0)
	lapseMin := time.Duration(100) * time.Minute
	lapseTotal := time.Duration(0)
	count := 0
	for j := 0; ; j++ {
		if finish {
			break
		}
		count++
		<-queue

		succeed, lapse := sendRequestNi(i, j)
		if !succeed {
			failedRequests++
		}
		if lapse > lapseMax {
			lapseMax = lapse
		}
		if lapse < lapseMin {
			lapseMin = lapse
		}
		lapseTotal = lapseTotal + lapse
		queue <- 1
	}
	lapsechan <- Stats{
		max:   lapseMax,
		min:   lapseMin,
		total: lapseTotal,
		count: count,
	}
	failchan <- failedRequests
}

func sendRequestSharelink(i, j int) (bool, time.Duration) {
	req, err := http.NewRequest("GET", serverUrl+"?id="+strconv.Itoa(i)+","+strconv.Itoa(j), nil)
	if err != nil {
		panic(err)
	}

	ua := fmt.Sprintf("Mozilla/5.0 (Linux; Android 100.%d.%d; Nexus 5 Build/KTU84P) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/39.0.2171.93 Mobile Safari/537.36", i, j)
	req.Header.Add("User-Agent", ua)
	start := time.Now()
	cli := http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
	resp, err := cli.Do(req)
	lapse := time.Since(start)
	succeed := true
	if err != nil {
		log.Println(i, j, "GET error:", err, "resp:", resp)
		succeed = false
	} else if resp.StatusCode != http.StatusOK {
		log.Println(i, j, "Status code is not 200!", resp.StatusCode)
		succeed = false
	} else {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println(i, j, "ReadAll failed, err:", err)
			succeed = false
		} else if !strings.Contains(string(b), "deepshare-redirect.min.js") {
			log.Println(i, j, "Check failed. not contain deepshare-redirect.min.js")
			succeed = false
		} else {
			if isDebug {
				log.Println(i, j, "succeed")
			}
		}
	}
	return succeed, lapse
}

func sendRequestNi(i, j int) (bool, time.Duration) {
	body := []byte(`{
  "q": "播放周杰伦的歌",
  "model": "Le_X520",
  "tf": true,
  "device_id": "le_s2:f4fef073",
  "tz": "GMT+08:00_Asia/Shanghai",
  "enc": "O5yNcMw1w603ruRSMQvmIwANQhhVvE2Km8PiRaFNQ%2FhV4pmPUG1BRa5R%2FNsv9VeBxqZ2QiTxvhbj%0AfBsqpvUg%2FWyuT5iZjC5sCknn6BEBhJRWnbB6azgzVkg16HRv2eyoFvoban82bE2T8gbFQOvVhD3i%0AvKP8kgo7%2BbgqWcyEjIM%3D%0A",
  "app_list": {
    "com.letv.android.account2": 1,
    "cn.wps.moffice_eng2": 110
  },
  "events": [
    {
      "action": "action_perform_result",
      "timestamp": 1472465416296,
      "kvs": null
    }
  ]
}` + "\n")
	req, err := http.NewRequest("POST", serverUrl+"?id="+strconv.Itoa(i)+","+strconv.Itoa(j), bytes.NewReader(body))
	if err != nil {
		panic(err)
	}

	start := time.Now()
	cli := http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
	resp, err := cli.Do(req)
	lapse := time.Since(start)
	succeed := true
	if err != nil {
		log.Println(i, j, "POST error:", err, "resp:", resp)
		succeed = false
	} else if resp.StatusCode != http.StatusOK {
		log.Println(i, j, "Status code is not 200!", resp.StatusCode)
		succeed = false
	} else {
		b, err := ioutil.ReadAll(resp.Body)
		log.Println("~~~~~~", len(b))
		if err != nil {
			log.Println(i, j, "ReadAll failed, err:", err)
			succeed = false
		} else if len(b) < 1000 {
			log.Println(i, j, "Check failed. body is too short")
			succeed = false
		} else {
			if isDebug {
				log.Println(i, j, "succeed")
			}
		}
	}
	return succeed, lapse
}
