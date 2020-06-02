package application

import (
	"All-On-Cloud-9/common"
	"All-On-Cloud-9/config"
	"All-On-Cloud-9/messenger"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/nats-io/nats.go"
)

var (
	// Define some arbitrary shipping rates for each carrier
	Rates                   = map[string]int{"USPS": 1, "FEDEX": 2, "UPS": 3}
	ManufacturerCostPerUnit = 5 // default value
	SupplierCostPerUnit     = 5 // Default value

	AppAgentChan                = make(chan *common.Message)
	sendClientRequestToAppsChan = make(chan *common.Transaction)
)

func startInterAppNatsListener(ctx context.Context, msgChan chan *nats.Msg) {
	var (
		msg *common.Message
	)
	for {
		select {
		case natsMsg := <-msgChan:
			_ = json.Unmarshal(natsMsg.Data, &msg)
			fmt.Println(msg.Clock)
			fmt.Println(msg)
			common.UpdateGlobalClock(msg.Clock.Clock, false)
			AppAgentChan <- msg
		}
	}
}

func startClient(ctx context.Context, addr string, port string, handler func(http.ResponseWriter, *http.Request)) error {
	http.HandleFunc(addr, handler)
	err := http.ListenAndServe(":"+port, nil)
	return err
}

func advertiseTransactionMessage(ctx context.Context, nc *nats.Conn,
	fromApp string, serverId string, serverNumId int) {
	log.Info("adverstising....")
	var (
		txn *common.Transaction
	)
	// This requires all transaction struct to have a ToApp field
	for {
		select {
		// send the client request to the target application
		case txn = <-sendClientRequestToAppsChan:
			txn.FromId = serverId
			txn.FromApp = fromApp
			txn.ToId = fmt.Sprintf(config.NODE_NAME, txn.ToApp, 0)
			// TODO: Fill these fields correctly
			msg := common.Message{
				ToApp:       txn.ToApp,
				FromApp:     fromApp,
				MessageType: "",
				Timestamp:   0,
				FromNodeId:  serverId,
				FromNodeNum: serverNumId,
				Txn:         txn,
				Digest:      "",
				PKeySig:     "",
				Clock:       txn.Clock,
			}

			jMsg, _ := json.Marshal(msg)
			toNatsInbox := fmt.Sprintf("NATS_%s_INBOX", txn.ToApp)
			log.Info("ready to send a message to nats")
			messenger.PublishNatsMessage(ctx, nc, toNatsInbox, jMsg)
		}
	}
}
