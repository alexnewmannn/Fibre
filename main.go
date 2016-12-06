package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	apiUrl         = "https://api.superfastmaps.co.uk/openreach/1.2/ajax/check.ajax.php"
	unwantedStatus = "highdemand"
)

var (
	interval    = flag.Duration("interval", time.Minute*15, "polling interval")
	notifyError = flag.Bool("notifyerr", false, "whether to send notification on error")
	slackUrl    = os.Getenv("SLACK_URL")
	available   bool
	lastTime    = time.Now()
)

func main() {
	listen := fmt.Sprintf(":%s", os.Getenv("PORT"))
	go http.ListenAndServe(listen, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)

		msg := fmt.Sprintf("available: `%t` last polled %s ago \n\n(%s)", available, time.Since(lastTime), time.Now().String())
		w.Write([]byte(msg))
	}))

	for {
		check()
		time.Sleep(*interval)
	}
}

func check() {
	fmt.Printf("polling...")

	available, err := poll()
	if err != nil {
		fmt.Printf("error: %v\n", err)
		if *notifyError {
			notify(fmt.Sprintf("openreach error: %v", err))
		}
		return
	}

	if available {
		fmt.Printf("available, sending notification\n")
		notify("openreach broadband is available")
	} else {
		fmt.Printf("not available\n")
	}
}

func poll() (bool, error) {
	form := url.Values{}
	form.Add("input", os.Getenv("POSTCODE"))
	form.Add("address", os.Getenv("ADDRESS"))
	form.Add("latlng", os.Getenv("LATLNG"))

	req, err := http.NewRequest("POST", apiUrl, strings.NewReader(form.Encode()))
	if err != nil {
		fmt.Println("failed creating req")
		return false, err
	}

	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Origin", "https://api.superfastmaps.co.uk")
	req.Header.Set("Accept-Language", "en-US,en;q=0.8")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/54.0.2840.98 Safari/537.36")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Referer", "https://api.superfastmaps.co.uk/openreach/1.2/?simple=yes")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("failed req")
		return false, err
	}

	var status struct {
		Cabinet struct {
			Status string `json:"status"`
		} `json:"cabinet"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&status); err != nil {
		fmt.Println("failed decoding req")
		return false, err
	}

	available = status.Cabinet.Status != unwantedStatus
	lastTime = time.Now()

	return available, nil
}

func notify(message string) {
	form := url.Values{}
	form.Add("payload", fmt.Sprintf(`{"text": "%s"}`, message))
	http.PostForm(slackUrl, form)
}
