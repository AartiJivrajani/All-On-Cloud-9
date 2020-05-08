package application

import (
	"All-On-Cloud-9/common"
	"All-On-Cloud-9/config"
	"All-On-Cloud-9/messenger"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/Sirupsen/logrus"

	"github.com/nats-io/nats.go"
)

var (
	manufacturer                *Manufacturer
	sendClientRequestToAppsChan = make(chan *common.Transaction)
)

type Manufacturer struct {
	MsgChannel    chan *nats.Msg
	ContractValid chan bool
	IsPrimary     bool
}

func (m *Manufacturer) subToInterAppNats(ctx context.Context, nc *nats.Conn, serverId string, serverNumId int) {
	var (
		err error
	)
	err = messenger.SubscribeToInbox(ctx, nc, common.NATS_MANUFACTURER_INBOX, m.MsgChannel)
	if err != nil {
		log.WithFields(log.Fields{
			"error":       err.Error(),
			"application": config.APP_MANUFACTURER,
			"topic":       common.NATS_MANUFACTURER_INBOX,
		}).Error("error subscribing to the nats topic")
	}
	go func() {
		for {
			select {
			// send the client request to the target application
			case txn := <-sendClientRequestToAppsChan:
				txn.FromId = serverId
				txn.FromApp = config.APP_MANUFACTURER
				txn.ToId = fmt.Sprintf(config.NODE_NAME, txn.ToApp, 1)

				// TODO: Fill these fields correctly
				msg := common.Message{
					ToApp:       txn.ToApp,
					FromApp:     config.APP_MANUFACTURER,
					MessageType: "",
					Timestamp:   0,
					FromNodeId:  serverId,
					FromNodeNum: serverNumId,
					Txn:         txn,
					Digest:      "",
					PKeySig:     "",
				}

				jMsg, _ := json.Marshal(msg)
				toNatsInbox := fmt.Sprintf("NATS_%s_INBOX", txn.ToApp)
				messenger.PublishNatsMessage(ctx, nc, toNatsInbox, jMsg)
			}
		}
	}()
}

func (m *Manufacturer) processTxn(ctx context.Context, msg *common.Message) {

}

type ManufacturerClientRequest struct {
	ToApp               string `json:"to_application"`
	NumUnitsToSell      int    `json:"num_units_to_sell"`
	AmountToBeCollected int    `json:"amount_to_be_collected"`
}

// handleManufacturerRequest expects a json message body containing a
// SINGLE transaction
func handleManufacturerRequest(w http.ResponseWriter, r *http.Request) {
	var (
		mTxn *ManufacturerClientRequest
		txn  *common.Transaction
	)
	_ = json.NewDecoder(r.Body).Decode(&mTxn)
	jTxn, _ := json.Marshal(mTxn)

	txn = &common.Transaction{
		TxnBody: jTxn,
		FromApp: config.APP_MANUFACTURER,
		ToApp:   mTxn.ToApp,
		ToId:    "",
		FromId:  "",
		TxnType: "",
		Clock:   nil,
	}
	sendClientRequestToAppsChan <- txn
}

func (m *Manufacturer) startClient(ctx context.Context) {
	http.HandleFunc("/app/manufacturer", handleManufacturerRequest)
	_ = http.ListenAndServe(":8080", nil)
}

func StartManufacturerApplication(ctx context.Context, nc *nats.Conn, serverId string,
	serverNumId int, isPrimary bool) {
	manufacturer = &Manufacturer{
		ContractValid: make(chan bool),
		MsgChannel:    make(chan *nats.Msg),
		IsPrimary:     isPrimary,
	}
	// has to be a go-routine cause the http handler is a blocking call
	go manufacturer.startClient(ctx)
	// all the other app-specific business logic can come here.
	manufacturer.subToInterAppNats(ctx, nc, serverId, serverNumId)
	// following logic has to be taken care of here -
	// 1. Listen to the NATS channel
	// 2. once a message is received, send it to the main AppServer object which establishes consensus
	// 3. Once consensus is reached, a message is sent back to the manufacturer object
	// 4. Once the object receives the consensus results, and if the result is true, run the manufacturer
	//    smart contract.
	// 5. Listen to the smart contract channel as well, and if the result is false, tell the main AppServer that
	//    addition of the block to the blockchain cannot be performed.
	go startInterAppNatsListener(ctx, manufacturer.MsgChannel)
}
