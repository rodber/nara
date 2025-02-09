package main

import (
	"errors"
	"fmt"
	"github.com/go-ping/ping"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/load"
	"github.com/sirupsen/logrus"
	"net"
	"runtime"
	"strings"
	"time"
)

func measurePingForever() {
	for {
		measureAndStorePing("google", "8.8.8.8")

		for name, nara := range neighbourhood {
			if me.Status.Observations[nara.Name].Online != "ONLINE" {
				delete(me.Status.PingStats, name)
				continue
			}

			measureAndStorePing(name, nara.Ip)
		}
		ts := chattinessRate(*me, 30, 120)
		time.Sleep(time.Duration(ts) * time.Second)
	}
}

func measureAndStorePing(name string, dest string) {
	ping, err := measurePing(name, dest)
	if err == nil && ping > 0 {
		me.Status.PingStats[name] = ping
	} else {
		// logrus.Println("problem when pinging", dest, err)
		delete(me.Status.PingStats, name)
	}
}

func measurePing(name string, dest string) (float64, error) {
	logrus.Debug("pinging", name, dest)
	pinger, err := ping.NewPinger(dest)
	if err != nil {
		return 0, err
	}
	pinger.Count = 5
	pinger.Timeout = time.Second * 10
	pinger.SetPrivileged(true)
	err = pinger.Run() // blocks until finished
	if err != nil {
		return 0, err
	}
	stats := pinger.Statistics() // get send/receive/rtt stats
	return float64(stats.AvgRtt/time.Microsecond) / 1000, nil
}

func updateHostStatsForever() {
	for {
		updateHostStats()
		time.Sleep(5 * time.Second)
	}
}

func updateHostStats() {
	uptime, _ := host.Uptime()
	me.Status.HostStats.Uptime = uptime

	load, _ := load.Avg()
	loadavg := load.Load1 / float64(runtime.NumCPU())
	me.Status.HostStats.LoadAvg = loadavg

	if forceChattiness >= 0 && forceChattiness <= 100 {
		me.Status.Chattiness = int64(forceChattiness)
	} else {
		if loadavg < 1 {
			me.Status.Chattiness = int64((1 - loadavg) * 100)
		} else {
			me.Status.Chattiness = 0
		}
	}
}

func pingBetween(a Nara, b Nara) float64 {
	a_ping, a_ping_present := a.Status.PingStats[b.Name]
	b_ping, b_ping_present := b.Status.PingStats[a.Name]
	if a_ping_present && b_ping_present {
		return (a_ping + b_ping) / 2
	} else if a_ping_present {
		return a_ping
	}
	return b_ping
}

func pingBetweenMs(a Nara, b Nara) string {
	ping := pingBetween(a, b)
	if ping == 0 {
		return ""
	}
	return fmt.Sprintf("%.2fms", ping)
}

// https://stackoverflow.com/questions/23558425/how-do-i-get-the-local-ip-address-in-go
func externalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}

			// skip non-tailscale IPs
			if !strings.HasPrefix(ip.String(), "100.") {
				continue
			}

			return ip.String(), nil
		}
	}
	return "", errors.New("are you connected to the network?")
}
