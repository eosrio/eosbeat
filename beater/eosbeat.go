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
}

type Node struct {
	Name        string `json:"bp_name"`
	Org         string `json:"organisation"`
	Location    string `json:"location"`
	NodeAddress string `json:"node_addr"`
	PortHTTP    string `json:"port_http"`
	PortSSL     string `json:"port_ssl"`
	PortP2P     string `json:"port_p2p"`
	Coordinates string
	Responses   []float64
	AVR         float64
}

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


func (bt *Eosbeat) Run(b *beat.Beat) error {
	logp.Info("eosbeat is running! Hit CTRL-C to stop it.")

	filePath := "nodes.json"
	jsonFile, openerr := os.Open(filePath)
	if openerr != nil {
		fmt.Println(openerr)
	}
	fmt.Println("Successfully Opened \"" + filePath + "\"")
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	var nodeList NodeList
	json.Unmarshal(byteValue, &nodeList)

	var err error
	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}

	ticker := time.NewTicker(bt.config.Period)
	counter := 1
	for {
		select {
		case <-bt.done:
			return nil
		case <-ticker.C:
		}

		for index, element := range nodeList.Nodes {
			data := new(GetInfoResponse)
			nodeURL := "http://" + element.NodeAddress + ":" + element.PortHTTP + "/v1/chain/get_info"
			responseTime, err := getJson(nodeURL, &data)
			if err != nil {
				fmt.Println("Server is down")
			} else {
				fmt.Println("Block:", data.HeadBlockNum)
				fmt.Println("Producer:", data.HeadBlockProducer)
				respTime := float64(responseTime) / 1000000
				fmt.Println("Latency:", respTime, "ms")
				nodeList.Nodes[index].Responses = append(element.Responses, respTime)
			}

			fmt.Println("-------")

			event := beat.Event{
				Timestamp: time.Now(),
				Fields: common.MapStr{
					"type":    b.Info.Name,
					"counter": counter,
				},
			}
			bt.client.Publish(event)
			logp.Info("Event sent")
		}
		counter++
	}
}

func (bt *Eosbeat) Stop() {
	bt.client.Close()
	close(bt.done)
}
