package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"time"
)

type request struct {
	PID           int
	RequestNumber int
	Duration      time.Duration
	Domain        string
}

type session struct {
	PID      int
	Duration time.Duration
}

type logLine struct {
	Time          time.Time
	PID           int
	RequestNumber int
	SourceIP      string
	SourcePort    int
	Action        string // forwarded|reply|query
	Domain        string
	Extra1        string // is|from|to
	Extra2        string // 127.0.0.1#10053|NXDOMAIN
	Extra3        string // Mostly empty.
}

func main() {
	log.Print("analyze-logs v0.1")

	// 2023-10-10T10:02:36+02:00	I1010 08:02:36.571446       1 nanny.go:146] dnsmasq[4811]: 315207 10.0.0.83/56382 forwarded metadata.google.internal.cluster.local to 127.0.0.1#10053
	re, err := regexp.Compile(`.*\tI1010 (\d+:\d+:\d+.\d+)\s+1 nanny.go:146] dnsmasq\[(\d+)\]: (\d+) ([0-9\.]+)/(\d+) (\S+) (\S+) (\S+) (\S+)`)
	if err != nil {
		fmt.Println(err)
	}

	// State
	logLinesbyPid := make(map[int][]logLine)
	logLinesbyTX := make(map[int][]logLine)

	// Read file
	readFile, err := os.Open("tmp/logs-full.txt")

	if err != nil {
		fmt.Println(err)
	}
	fileScanner := bufio.NewScanner(readFile)

	fileScanner.Split(bufio.ScanLines)

	timeLayout := "2006-01-02T15:04:05.99"

	i := 0
	for fileScanner.Scan() {
		i++

		line := re.FindStringSubmatch(fileScanner.Text())

		t, _ := time.Parse(timeLayout, fmt.Sprintf("2023-10-10T%s", line[1]))
		if err != nil {
			fmt.Println(err)
		}

		pid, err := strconv.Atoi(line[2])
		if err != nil {
			fmt.Println(err)
		}

		reqNumber, err := strconv.Atoi(line[3])
		if err != nil {
			fmt.Println(err)
		}

		sPort, err := strconv.Atoi(line[5])
		if err != nil {
			fmt.Println(err)
		}

		ll := logLine{
			Time:          t,
			PID:           pid,
			RequestNumber: reqNumber,
			SourceIP:      line[4],
			SourcePort:    sPort,
			Action:        line[6],
			Domain:        line[7],
			Extra1:        line[8],
		}

		if len(line) >= 10 {
			ll.Extra2 = line[9]
		}

		if len(line) >= 11 {
			ll.Extra3 = line[9]
		}

		logLinesbyPid[pid] = append(logLinesbyPid[pid], ll)

		logLinesbyTX[reqNumber] = append(logLinesbyTX[reqNumber], ll)

		if i > 5000 {
			break
		}

	}

	readFile.Close()

	// By transaction/request
	var requests []request

	for _, req := range logLinesbyTX {

		r := request{
			PID:           req[0].PID,
			RequestNumber: req[0].RequestNumber,
			Domain:        req[0].Domain,
			Duration:      req[len(req)-1].Time.Sub(req[0].Time),
		}

		requests = append(requests, r)
	}

	for _, r := range requests {
		if r.Duration < time.Duration(time.Millisecond*2) {
			continue
		}
		fmt.Printf("Request: PID %v RequestNumber %v Domain %v Duration %v\n", r.PID, r.RequestNumber, r.Domain, r.Duration)
	}

	// By session
	var sessions []session

	for pid, ses := range logLinesbyPid {

		s := session{
			PID:      pid,
			Duration: ses[len(ses)-1].Time.Sub(ses[0].Time),
		}

		sessions = append(sessions, s)
	}

	for _, s := range sessions {
		if s.Duration < time.Duration(time.Millisecond*500) {
			continue
		}
		fmt.Printf("Session: PID %v Duration %v\n", s.PID, s.Duration)
	}

	//json, err := json.MarshalIndent(requests, "", "  ")
	//if err != nil {
	//	fmt.Println(err)
	//}
	//fmt.Println(string(json))

}
