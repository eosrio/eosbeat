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

type NodeList struct {
	Nodes []Node `json:"blockProducerList"`
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
	NodeIP       string
	Responses    []float64
	AVR          float64
}

// Define client with 5 second timeout
var httpClient = &http.Client{Timeout: 5 * time.Second}

func getJson(url string, target interface{}) (int64, error) {
	t := int64(0)
	start := time.Now()
	fmt.Println("Fetching data from", url)
	r, err := httpClient.Get(url)
	if err != nil {
		return t, err
	}
	defer r.Body.Close()
	elapsed := time.Since(start).Nanoseconds()
	t = elapsed
	return t, json.NewDecoder(r.Body).Decode(target)
}

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

func trace(url string) string {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}
	var ip net.IP
	trace := &httptrace.ClientTrace{
		DNSDone: func(dnsInfo httptrace.DNSDoneInfo) {
			ip = dnsInfo.Addrs[0].IP
		},
		GotConn: func(connInfo httptrace.GotConnInfo) {
			fmt.Printf("Got Conn: %+v\n", connInfo)
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	if _, err := http.DefaultTransport.RoundTrip(req); err != nil {
		log.Println(err)
		return "_"
	}
	log.Println("trace done!")
	return ip.String()
}

func measureConn(host, port string) {
	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		log.Println(err)
	} else {
		defer conn.Close()
		conn.Write([]byte("GET / HTTP/1.0\r\n\r\n"))

		// start := time.Now()
		oneByte := make([]byte, 1)
		_, err = conn.Read(oneByte)
		if err != nil {
			log.Println(err)
		} else {
			// log.Println("First byte:", time.Since(start))

			_, err = ioutil.ReadAll(conn)
			if err != nil {
				log.Println(err)
			} else {
				// log.Println("Everything:", time.Since(start))
			}
		}
	}
}

var externalIP string
var proto = "http://"
var endpoint = "/v1/chain/get_info"
var testnetName string

func genEvent(element Node, listOfNodes NodeList, index int) beat.Event {
	nodeURL := element.NodeAddress + ":" + element.PortHTTP
	measureConn(element.NodeAddress, element.PortHTTP)
	var target string
	if element.NodeIP == "" {
		// fmt.Println("Requesting:", nodeURL)
		remoteIP := trace(proto + nodeURL + endpoint)
		if len(remoteIP) > 6 {
			listOfNodes.Nodes[index].NodeIP = remoteIP
			target = remoteIP
		}
		// fmt.Println("Remote IP:", remoteIP)
	} else {
		nodeURL = element.NodeIP + ":" + element.PortHTTP
		target = element.NodeIP
		// fmt.Println("Requesting:", nodeURL)
		trace(proto + nodeURL + endpoint)
	}

	data := new(GetInfoResponse)
	var respTime float64
	responseTime, err := getJson(proto+nodeURL+endpoint, &data)
	if err != nil {
		fmt.Println("Server is down")
		return beat.Event{}
	} else {
		respTime = float64(responseTime) / 1000000
		listOfNodes.Nodes[index].Responses = append(element.Responses, respTime)
		if target == "" {
			target = listOfNodes.Nodes[index].NodeAddress
		}
		fmt.Println("Target:", target)
		fmt.Println("Latency:", respTime, "ms")
		return beat.Event{
			Timestamp: time.Now(),
			Fields: common.MapStr{
				"network":          testnetName,
				"target":           target,
				"source":           externalIP,
				"latency":          respTime,
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
	fmt.Println("Node list loaded from \"" + filePath + "\"")
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	var nodeList NodeList
	json.Unmarshal(byteValue, &nodeList)

	testnetName = nodeList.Network

	for _, elem := range nodeList.Nodes {
		fmt.Println(elem.NodeAddress + ":" + elem.PortHTTP)
	}

	externalIP = findPublicIP("http://myexternalip.com/raw")
	externalIP = strings.TrimSuffix(externalIP, "\n")
	fmt.Println("Firing requests from:", externalIP)

	var err error
	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}

	ticker := time.NewTicker(bt.config.Period)
	var index int
	for {
		select {
		case <-bt.done:
			return nil
		case <-ticker.C:
		}

		fmt.Println(string(index+1) + "/" + string(len(nodeList.Nodes)))

		// Beat routine
		evt := genEvent(nodeList.Nodes[index], nodeList, index)
		if evt.Fields != nil {
			bt.client.Publish(evt)
		}
		// End of routine

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
