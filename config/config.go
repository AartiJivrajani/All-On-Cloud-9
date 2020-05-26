package config

import (
	"All-On-Cloud-9/common"
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"strconv"

	log "github.com/Sirupsen/logrus"
)

const (
	APP_MANUFACTURER = "MANUFACTURER"
	APP_SUPPLIER     = "SUPPLIER"
	APP_BUYER        = "BUYER"
	APP_CARRIER      = "CARRIER"

	NODE_NAME = "%s_%d"
)

var (
	SystemConfig *Config
)

type Servers struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type Applications struct {
	AppManufacturer *ApplicationInstance `json:"MANUFACTURER,omitempty"`
	AppBuyer        *ApplicationInstance `json:"BUYER,omitempty"`
	AppSupplier     *ApplicationInstance `json:"SUPPLIER,omitempty"`
	AppCarrier      *ApplicationInstance `json:"CARRIER,omitempty"`
}

type ApplicationInstance struct {
	Servers []*Servers `json:"servers"`
}

type Orderers struct {
	Servers []*Servers `json:"servers"`
}

type NatsServers struct {
	Servers []string `json:"servers"`
}

type Config struct {
	AppInstance         *Applications `json:"application_instance"`
	Orderers            *Orderers     `json:"orderers"`
	Nats                *NatsServers  `json:"nats"`
	GlobalConsensusAlgo string        `json:"global_consensus_algorithm"`
	Consensus           string        `json:"consensus"`
}

func GetGlobalConsensusMethod() int {
	switch SystemConfig.GlobalConsensusAlgo {
	case common.GLOBAL_CONSENSUS_ALGO_ORDERER:
		return 1
	case common.GLOBAL_CONSENSUS_ALGO_HEIRARCHICAL:
		return 2
	case common.GLOBAL_CONSENSUS_ALGO_SLPBFT:
		return 3
	}
	return 1
}

func IsByzantineTolerant(appName string) bool { //For now, we better put it in the config file
	return GetAppId(appName) < 2
}

func getAppNum(appName string) int {
	switch appName {
	case APP_BUYER:
		return 0
	case APP_CARRIER:
		return 1
	case APP_MANUFACTURER:
		return 2
	case APP_SUPPLIER:
		return 3
	}
	panic("no such app: " + appName)
}

func GetAppCnt() int { //How many applications we have in total
	return 4
}

func GetAppId(appName string) int {
	if appName == "" {
		panic("fill FromApp")
	}
	appId, err := strconv.Atoi(appName)
	if err != nil {
		appId = getAppNum(appName)
	}

	return appId
}

func GetAppNodeCnt(appName string) int {
	appId := GetAppId(appName)
	return GetAppNodeCntInt(appId)
}

func GetAppNodeCntInt(appId int) int {
	switch appId {
	case 0:
		return len(SystemConfig.AppInstance.AppBuyer.Servers)
	case 1:
		return len(SystemConfig.AppInstance.AppCarrier.Servers)
	case 2:
		return len(SystemConfig.AppInstance.AppManufacturer.Servers)
	case 3:
		return len(SystemConfig.AppInstance.AppSupplier.Servers)
	}

	panic("no such app: " + strconv.Itoa(appId))
}

func LoadConfig(ctx context.Context, filepath string) {
	//initNodeIds()
	jsonFile, err := os.Open(filepath)
	if err != nil {
		log.WithFields(log.Fields{
			"err":  err.Error(),
			"path": filepath,
		}).Error("error opening config file")
	}
	file, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err.Error(),
		}).Error("error reading config file")
	}
	err = json.Unmarshal(file, &SystemConfig)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err.Error(),
		}).Error("error unmarshalling config into the conf object")
	}
}

//// STUB: For testing only
//func main() {
//	LoadConfig(nil, "/Users/aartij17/go/src/All-On-Cloud-9/config/config.json")
//}
