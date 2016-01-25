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

func main() {
	fs := flag.NewFlagSet("stress-test", flag.ExitOnError)
	clientNum := fs.Int("client-num", 1, "specify the clients num to run concurrently")
	testUrl := fs.String("url", "https://xxx/d/54378fd28a6c81e3/2cbKCNg2pq", "specify the addr to test")
	runs := fs.Int("runs", 10, "specify how many runs")
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
	log.Println("url:", *testUrl)
	log.Println("client-num:", *clientNum)
	log.Println("runs:", *runs)
	url := *testUrl
	failchan := make(chan int)
	lapses := make(chan [3]time.Duration)
	for i := 0; i < *clientNum; i++ {
		go func() {
			fails := 0
			lapseMax := time.Duration(0)
			lapseMin := time.Duration(100) * time.Minute
			lapseTotal := time.Duration(0)
			defer func() {
				failchan <- fails
				lapses <- [3]time.Duration{lapseMax, lapseMin, lapseTotal}
			}()
			for j := 0; j < *runs; j++ {
				succeed := false
				defer func() {
					if !succeed {
						fails++
					}
				}()
				req, err := http.NewRequest("GET", url, nil)
				if err != nil {
					panic(err)
				}
				req.Header.Add("User-Agent", "Mozilla/5.0 (Linux; Android 4.4.4; Nexus 5 Build/KTU84P) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/39.0.2171.93 Mobile Safari/537.36")
				start := time.Now()
				resp, err := http.DefaultClient.Do(req)
				lapse := time.Since(start)
				if lapseMax < lapse {
					lapseMax = lapse
				}
				if lapseMin > lapse {
					lapseMin = lapse
				}
				lapseTotal += lapse
				if err != nil {
					log.Println("GET error:", err, "resp:", resp)
					return
				}
				if resp.StatusCode != http.StatusOK {
					log.Println("Status code is not 200!", resp.StatusCode)
					return
				}
				b, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					log.Println("ReadAll failed, err:", err)
				}
				if !strings.Contains(string(b), "deepshare-redirect.min.js") {
					log.Println("Check failed. not contain deepshare-redirect.min.js")
					return
				}
				succeed = true
			}
		}()
	}

	fails := make([]int, *clientNum)
	lapseMax := time.Duration(0)
	lapseMin := time.Duration(100) * time.Minute
	lapseTotal := time.Duration(0)
	failedClients := 0
	for i := 0; i < *clientNum; i++ {
		n := <-failchan
		a := <-lapses
		if n > 0 {
			failedClients++
			fails[i] = n
		}
		if lapseMax < a[0] {
			lapseMax = a[0]
		}
		if lapseMin > a[1] {
			lapseMin = a[1]
		}
		lapseTotal += a[2]
	}
	lapseMean := lapseTotal.Nanoseconds() / int64(*clientNum) / int64(*runs) / 1000000
	log.Printf("Lapse: \n	max	%v	\n	min	%v	\n	mean	%dms\n", lapseMax, lapseMin, lapseMean)

	log.Printf("Tested with (%d clients * %d runs), %d failed, \n\n\n", *clientNum, *runs, failedClients)
	if failedClients > 0 {
		log.Printf("fails: %v!\n", fails)
	}
}
