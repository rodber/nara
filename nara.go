package main

import (
	"flag"
	"fmt"
	"github.com/bugsnag/bugsnag-go"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/shirou/gopsutil/host"
	"github.com/sirupsen/logrus"
	"math/rand"
	// "strconv"
	"time"

	"os"
	"os/signal"
	"runtime"
	"syscall"
)

type Nara struct {
	Name     string
	Hostname string
	Ip       string
	Status   NaraStatus
}

type NaraStatus struct {
	PingStats    map[string]float64
	Barrio       string
	HostStats    HostStats
	Chattiness   int64
	Observations map[string]NaraObservation
}

type HostStats struct {
	Uptime  uint64
	LoadAvg float64
}

type NaraObservation struct {
	Online      string
	StartTime   int64
	Restarts    int64
	LastSeen    int64
	LastRestart int64
	ClusterName string
}

var me = &Nara{}

var forceChattiness int

func main() {
	rand.Seed(time.Now().UnixNano())

	bugsnag.Configure(bugsnag.Configuration{
		APIKey:          "0bd8e595fccf5f1befe9151c3a32ea61",
		ProjectPackages: []string{"main"},
	})

	mqttHostPtr := flag.String("mqtt-host", "tcp://hass.eljojo.casa:1883", "mqtt server hostname")
	mqttUserPtr := flag.String("mqtt-user", "my_username", "mqtt server username")
	mqttPassPtr := flag.String("mqtt-pass", "my_password", "mqtt server password")
	naraIdPtr := flag.String("nara-id", "raspberry", "nara id")
	showNeighboursPtr := flag.Bool("show-neighbours", true, "show table with neighbourhood")
	showNeighboursSpeedPtr := flag.Int("refresh-rate", 60, "refresh rate in seconds for neighbourhood table")
	forceChattinessPtr := flag.Int("force-chattiness", -1, "specific chattiness to force, -1 for auto (default)")
	verbosePtr := flag.Bool("verbose", false, "log debug stuff")

	flag.Parse()
	forceChattiness = *forceChattinessPtr

	me.Name = *naraIdPtr
	me.Status.PingStats = make(map[string]float64)
	me.Status.Observations = make(map[string]NaraObservation)
	updateHostStats()

	ip, err := externalIP()
	if err == nil {
		me.Ip = ip
		logrus.Println("local ip", ip)
	} else {
		logrus.Panic(err)
	}

	hostinfo, _ := host.Info()
	me.Hostname = hostinfo.Hostname

	client := connectMQTT(*mqttHostPtr, *mqttUserPtr, *mqttPassPtr, *naraIdPtr)
	go announceForever(client)
	go measurePingForever()
	go updateHostStatsForever()
	go formOpinion()
	go observationMaintenance()
	if *showNeighboursPtr {
		go printNeigbourhoodForever(*showNeighboursSpeedPtr)
	}

	if *verbosePtr {
		logrus.SetLevel(logrus.DebugLevel)
	}

	SetupCloseHandler(client)
	defer chau(client)

	for {
		time.Sleep(10 * time.Millisecond)
		runtime.Gosched() // https://blog.container-solutions.com/surprise-golang-thread-scheduling
	}
}

func SetupCloseHandler(client mqtt.Client) {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("babaayyy")
		chau(client)
		os.Exit(0)
	}()
}
