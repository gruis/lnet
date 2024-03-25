package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	probing "github.com/prometheus-community/pro-bing"
)

const ClearLine = "\033[2K"

// https://github.com/prometheus-community/pro-bing/blob/main/cmd/ping/ping.go
// https://pkg.go.dev/github.com/prometheus-community/pro-bing#Pinger
func pingHost(host string, times int) *probing.Statistics {
	pinger, err := probing.NewPinger(host)
	if err != nil {
		panic(err)
	}
	pinger.Count = times
	pinger.Timeout = time.Second * 5
	err = pinger.Run() // Blocks until finished.
	if err != nil {
		panic(err)
	}
	return pinger.Statistics() // get send/receive/duplicate/rtt stats
}

func getHosts() (hosts []string) {
	return append(parseHostsList("ping-hosts.txt"), parseHostsList("ping-hosts.local.txt")...)
}

func parseHostsList(filePath string) (hosts []string) {
	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// optionally, resize scanner's capacity for lines over 64K, see next example
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 0 {
			if line[0:1] != "#" {
				hPc := strings.Split(line, "#")
				if len(hPc[0]) > 0 {
					host := strings.TrimSpace(hPc[0])
					hosts = append(hosts, host)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	return hosts
}

func spinReprint(prefix string, cb func()) {
	spin(prefix, cb, true)
}

func spinNoReprint(prefix string, cb func()) {
	spin(prefix, cb, false)
}

func spin(prefix string, cb func(), reprint bool) {
	spn := spinner.New(spinner.CharSets[35], 300*time.Millisecond)
	spn.Color("blue", "bold")
	fmt.Printf(ClearLine)
	spn.Prefix = prefix
	spn.Start()
	cb()
	spn.Stop()
	if reprint {
		fmt.Printf(prefix)
	}
}

func main() {
	var maxRTT time.Duration
	var pktSent int
	var pktLost int
	maxPkts := 100

	if len(os.Args) > 1 && len(os.Args[1]) > 0 {
		var err error
		maxPkts, err = strconv.Atoi(os.Args[1])
		if err != nil {
			panic(err)
		}
	}

	// TODO: calculate width of number in parens
	const statPreamble = "\r%15s (%3d) - "

	spn := spinner.New(spinner.CharSets[35], 300*time.Millisecond)
	spn.Color("blue", "bold")

	for pktSent < maxPkts {
		// re-read every time to support hot-reloading
		hosts := getHosts()

		// unless this is our first time through the entire list, pause for a bit
		if pktSent > 0 {
			secs := rand.IntN(30)
			spinNoReprint(fmt.Sprintf("pausing %d  ", secs), func() { time.Sleep(time.Second * time.Duration(secs)) })
		}

		for i := 0; i < len(hosts); i++ {
			host := hosts[i]
			var stats *probing.Statistics

			spinReprint(fmt.Sprintf(statPreamble, host, pktSent),
				func() { stats = pingHost(host, 1) })

			if stats.MaxRtt > maxRTT {
				maxRTT = stats.MaxRtt
			}
			pktSent = pktSent + stats.PacketsSent

			fmt.Printf("tx: %d, loss:%v%%  ", stats.PacketsSent, stats.PacketLoss)
			if stats.PacketLoss > 0 {
				pktLost = pktLost + (stats.PacketsSent - stats.PacketsRecv)
				// persist the last stat line to show the packet loss
				fmt.Printf(" %s", time.Now())
				fmt.Println()
			} else {
				// TODO: do print if packet loss is not 100%
				fmt.Printf("min:%v, avg:%v, max:%v", stats.MinRtt, stats.AvgRtt, stats.MaxRtt)
			}
			if pktSent >= maxPkts {
				break
			}
			secs := rand.IntN(3)
			time.Sleep(time.Second * time.Duration(secs))
		}
	}
	fmt.Printf(ClearLine)
	fmt.Printf("\ntx: %d, lost: %d, maxRTT: %v\n", pktSent, pktLost, maxRTT)
}
