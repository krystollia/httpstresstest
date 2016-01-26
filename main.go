package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
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
	testUrl := fs.String("url", "https://fds.so/d/54378fd28a6c81e3/2cbKCNg2pq", "specify the url to test")
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
			failedRequests++
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
	j := 0
	failedRequests := 0
	lapseMax := time.Duration(0)
	lapseMin := time.Duration(100) * time.Minute
	lapseTotal := time.Duration(0)
	for {
		if finish {
			break
		}
		<-queue

		req, err := http.NewRequest("GET", serverUrl, nil)
		if err != nil {
			panic(err)
		}
		req.Header.Add("User-Agent", "Mozilla/5.0 (Linux; Android 4.4.4; Nexus 5 Build/KTU84P) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/39.0.2171.93 Mobile Safari/537.36")
		start := time.Now()
		resp, err := http.DefaultClient.Do(req)
		lapse := time.Since(start)
		j++
		if err != nil {
			log.Println(i, j, "GET error:", err, "resp:", resp)
			failedRequests++
		} else if resp.StatusCode != http.StatusOK {
			log.Println(i, j, "Status code is not 200!", resp.StatusCode)
			failedRequests++
		} else {
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Println(i, j, "ReadAll failed, err:", err)
				failedRequests++
			} else if !strings.Contains(string(b), "deepshare-redirect.min.js") {
				log.Println(i, j, "Check failed. not contain deepshare-redirect.min.js")
				failedRequests++
			} else {
				if isDebug {
					log.Println(i, j, "succeed")
				}
			}
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
		count: j,
	}
	failchan <- failedRequests
}
