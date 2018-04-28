package beater

import (
	"fmt"
	"time"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/eosrio/eosbeat/config"
	"net/http"
	"encoding/json"
	"os"
	"io/ioutil"
	"net"
	"log"
	"net/http/httptrace"
	"strings"
	"crypto/tls"
)

type Eosbeat struct {
	done   chan struct{}
	config config.Config
	client beat.Client
}

// Creates beater
func New(b *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	c := config.DefaultConfig
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	bt := &Eosbeat{
		done:   make(chan struct{}),
		config: c,
	}
	return bt, nil
}

type GetInfoResponse struct {
	ServerVersion            string `json:"server_version"`
	HeadBlockNum             int    `json:"head_block_num"`
	HeadBlockProducer        string `json:"head_block_producer"`
	HeadBlockTime            string `json:"head_block_time"`
	HeadBlockID              string `json:"head_block_id"`
	LastIrreversibleBlockNum int    `json:"last_irreversible_block_num"`
}

type DetailedTraceResponse struct {
	Dns  float64
	Tls  float64
	Conn float64
	Resp float64
	Full float64
}

type NodeList struct {
	Nodes   []Node `json:"blockProducerList"`
	Network string `json:"network"`
}

type Node struct {
	Name         string `json:"bp_name"`
	Org          string `json:"organisation"`
	Location     string `json:"location"`
	NodeAddress  string `json:"node_addr"`
	PortHTTP     string `json:"port_http"`
	PortSSL      string `json:"port_ssl"`
	PortP2P      string `json:"port_p2p"`
	ProducerName string `json:"bp_name"`
	Coordinates  string
	Responses    []float64
	AVR          float64
}

// Define client with 5 second timeout
var httpClient = &http.Client{Timeout: 5 * time.Second}

func findPublicIP(server string) string {
	resp, err := httpClient.Get(server)
	if err != nil {
		os.Stderr.WriteString(err.Error())
		os.Stderr.WriteString("\n")
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		bodyString := string(bodyBytes)
		return bodyString
	} else {
		return "none"
	}
}

func trace(url string, target interface{}) (DetailedTraceResponse, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}

	var startDNS time.Time
	var dnsTime time.Duration

	var startCONN time.Time
	var timeCONN time.Duration

	var refFRB time.Time
	var timeFRB time.Duration

	var refTLS time.Time
	var timeTLS time.Duration

	trace := &httptrace.ClientTrace{
		DNSStart: func(dnsInfo httptrace.DNSStartInfo) {
			startDNS = time.Now()
		},
		DNSDone: func(dnsDoneInfo httptrace.DNSDoneInfo) {
			dnsTime = time.Since(startDNS)
		},
		ConnectStart: func(network, addr string) {
			startCONN = time.Now()
		},
		ConnectDone: func(network, addr string, err error) {
			timeCONN = time.Since(startCONN)
		},
		TLSHandshakeStart: func() {
			refTLS = time.Now()
		},
		TLSHandshakeDone: func(state tls.ConnectionState, e error) {
			timeTLS = time.Since(refTLS)
		},
		GotConn: func(info httptrace.GotConnInfo) {
			refFRB = time.Now()
		},
		GotFirstResponseByte: func() {
			timeFRB = time.Since(refFRB)
		},
	}
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	transport := &http.Transport{
		DialContext:         (dialer).DialContext,
		TLSHandshakeTimeout: 2 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(5000) * time.Millisecond,
	}
	now := time.Now()
	res, err := client.Do(req)
	cost := time.Since(now)
	if err != nil {
		fmt.Println(err)
		return DetailedTraceResponse{}, err
	}
	defer res.Body.Close()

	response := DetailedTraceResponse{
		Dns:  dnsTime.Seconds() * 1000,
		Tls:  timeTLS.Seconds() * 1000,
		Conn: timeCONN.Seconds() * 1000,
		Resp: timeFRB.Seconds() * 1000,
		Full: cost.Seconds() * 1000,
	}

	fmt.Printf("DNS Resolution: %.2f ms\n", response.Dns)
	fmt.Printf("TLS Handshake: %.2f ms\n", response.Tls)
	fmt.Printf("Connection Time: %.2f ms\n", response.Conn)
	fmt.Printf("Response Time: %.2f ms\n", response.Resp)
	fmt.Printf("Client Cost: %.2f ms\n", response.Full)

	// Decode JSON
	if res.StatusCode == 200 {
		decErr := json.NewDecoder(res.Body).Decode(target)
		if decErr == nil {
			return response, err
		} else {
			fmt.Println("JSON Decode Error: ", decErr)
			return DetailedTraceResponse{}, err
		}
	} else {
		fmt.Println("HTTP Status Code: ", res.StatusCode)
		return DetailedTraceResponse{}, err
	}
}

var externalIP string
var proto = "http://"
var endpoint = "/v1/chain/get_info"
var testnetName string

func genEvent(listOfNodes NodeList, index int) beat.Event {
	element := listOfNodes.Nodes[index]
	nodeURL := element.NodeAddress + ":" + element.PortHTTP
	data := new(GetInfoResponse)
	// Call HTTP Tracer
	response, traceErr := trace(proto+nodeURL+endpoint, &data)
	if traceErr != nil {
		fmt.Println("Server (" + nodeURL + ") is down")
		return beat.Event{}
	} else {
		fmt.Println("Target: "+element.NodeAddress+" | Latency:", response.Full, "ms")

		// Create and submit beat event
		return beat.Event{
			Timestamp: time.Now(),
			Fields: common.MapStr{
				"network":          testnetName,
				"target":           element.NodeAddress,
				"source":           externalIP,
				"latency_full":     response.Full,
				"latency_dns":      response.Dns,
				"latency_conn":     response.Conn,
				"latency_resp":     response.Resp,
				"block":            data.HeadBlockNum,
				"current_producer": data.HeadBlockProducer,
				"bp_name":          element.ProducerName,
				"lastirrb":         data.LastIrreversibleBlockNum,
				"org":              element.Org,
				"location":         element.Location,
			},
		}
	}
}

func (bt *Eosbeat) Run(b *beat.Beat) error {
	logp.Info("eosbeat is running! Hit CTRL-C to stop it.")

	filePath := "nodes.json"
	jsonFile, openerr := os.Open(filePath)
	if openerr != nil {
		fmt.Println(openerr)
	}
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	var nodeList NodeList
	json.Unmarshal(byteValue, &nodeList)
	testnetName = nodeList.Network
	fmt.Println("Active Testnet: ", testnetName)
	fmt.Println("Node count: ", len(nodeList.Nodes))
	externalIP = findPublicIP("https://api.ipify.org")
	externalIP = strings.TrimSuffix(externalIP, "\n")
	fmt.Println("Firing requests from:", externalIP)

	var err error
	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}

	ticker := time.NewTicker(bt.config.Period)
	var index int
	nodeCount := len(nodeList.Nodes)
	index = 0
	for {
		select {
		case <-bt.done:
			return nil
		case <-ticker.C:
		}
		fmt.Println("\n--------------------", index+1, "/", nodeCount, "--------------------")
		evt := genEvent(nodeList, index)
		if evt.Fields != nil {
			bt.client.Publish(evt)
		}
		index++
		if index >= len(nodeList.Nodes) {
			index = 0
		}
	}
}

func (bt *Eosbeat) Stop() {
	bt.client.Close()
	close(bt.done)
}
