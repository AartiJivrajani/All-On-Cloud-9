package nodes

import (
	"All-On-Cloud-9/common"
	"All-On-Cloud-9/config"
	"All-On-Cloud-9/server/application"
	"All-On-Cloud-9/server/blockchain"
	"context"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/terraform/dag"
)

func (server *Server) listenToContractChannel(c chan bool, resp chan bool) {
	for {
		select {
		case r := <-c:
			resp <- r
			return
		}
	}
}

func (server *Server) InvokeSmartContract(block *blockchain.Block) bool {
	respChan := make(chan bool)
	contractValid := make(chan bool)
	log.WithFields(log.Fields{
		"app": block.Transaction.FromApp,
	}).Info("invoking smart contract")
	switch block.Transaction.FromApp {
	case config.APP_MANUFACTURER:
		go server.listenToContractChannel(contractValid, respChan)
		application.RunManufacturerContract(block, contractValid)
	case config.APP_SUPPLIER:
		go server.listenToContractChannel(contractValid, respChan)
		application.RunSupplierContract(block, contractValid)
	case config.APP_BUYER:
		go server.listenToContractChannel(contractValid, respChan)
		application.RunBuyerContract(block, contractValid)
	case config.APP_CARRIER:
		go server.listenToContractChannel(contractValid, respChan)
		application.RunCarrierContract(block, contractValid)
	}

	for {
		select {
		case r := <-respChan:
			return r
		}
	}
}

func (server *Server) InitiateAddBlock(ctx context.Context, message *common.Message) {
	var (
		newBlock *blockchain.Block
		blockId  string
		isGlobal = false
		isLocal  = false
	)

	server.MapLock.Lock()
	defer server.MapLock.Unlock()

	log.WithFields(log.Fields{
		"fromApp":  message.FromApp,
		"toApp":    message.ToApp,
		"txn type": message.Txn.TxnType,
	}).Info("message details for the new block")
	if _, OK := server.PIDMap[message.Clock.PID]; OK {
		log.WithFields(log.Fields{
			"pid": message.Clock.PID,
		}).Warn("block already present in blockchain")
		return
	}
	if message.Txn.TxnType == common.LOCAL_TXN {
		if server.LastAddedLocalBlock.V.(*blockchain.Block).Clock.Clock > message.Clock.Clock {
			log.WithFields(log.Fields{
				"lastAddedTS": server.LastAddedLocalBlock.V.(*blockchain.Block).Clock.Clock,
				"incomingTS":  message.Clock.Clock,
			}).Warn("not going to add an older local block, skipping..")
			return
		}
		if server.LastAddedLocalBlock.V.(*blockchain.Block).Clock.PID == message.Clock.PID {
			log.WithFields(log.Fields{
				"messageClockId": message.Clock.PID,
			}).Warn("local block already added to the blockchain, nothing to do")
			return
		}
	} else if message.Txn.TxnType == common.GLOBAL_TXN {
		if server.LastAddedGlobalBlock != nil &&
			server.LastAddedGlobalBlock.V.(*blockchain.Block).Clock.Clock > message.Clock.Clock {
			log.WithFields(log.Fields{
				"lastAddedTS": server.LastAddedGlobalBlock.V.(*blockchain.Block).Clock.Clock,
				"incomingTS":  message.Clock.Clock,
			}).Warn("not going to add an older global block, skipping..")
			return
		}
		if server.LastAddedGlobalBlock != nil &&
			server.LastAddedGlobalBlock.V.(*blockchain.Block).Clock.PID == message.Clock.PID {
			log.WithFields(log.Fields{
				"messageClockId": message.Clock.PID,
			}).Warn("global block already added to the blockchain, nothing to do")
			return
		}
	}

	// if this is a local txn, the message should be intended for the current application
	if message.Txn.TxnType == common.LOCAL_TXN && server.AppName != message.ToApp {
		log.WithFields(log.Fields{
			"current app": server.AppName,
			"block app":   message.Txn.FromApp,
		}).Info("local block received, but not intended for this application")
		return
	}
	if message.Txn.TxnType == common.LOCAL_TXN && server.AppName == message.ToApp {
		blockchain.LocalSeqNumber += 1
		blockId = fmt.Sprintf(common.LOCAL_BLOCK_NUM, server.AppName, server.ServerNumId,
			blockchain.LocalSeqNumber)
		isLocal = true
	} else if message.Txn.TxnType == common.GLOBAL_TXN {
		// in case of a global transaction, increment both the local and the global sequence numbers
		blockchain.LocalSeqNumber += 1
		blockchain.GlobalSeqNumber += 1
		isGlobal = true

		blockId = fmt.Sprintf(common.GLOBAL_BLOCK_NUM, server.AppName, server.ServerNumId, blockchain.LocalSeqNumber,
			blockchain.GlobalSeqNumber)
	}

	newBlock = &blockchain.Block{
		IsGenesis:     false,
		CryptoHash:    "",
		Transaction:   message.Txn,
		InitiatorNode: message.FromNodeId,
		Clock:         message.Clock,
		ViewType:      message.Txn.TxnType,
	}

	result := server.InvokeSmartContract(newBlock)
	if !result {
		log.Error("smart contract violated, skipping the block addition")
		return
	} else {
		log.Info("moving on......, contract check done!")
	}

	if isGlobal {
		newBlock.BlockId = blockId
		newBlock.ViewType = common.GLOBAL_TXN
	} else if isLocal {
		newBlock.BlockId = blockId
		newBlock.ViewType = common.LOCAL_TXN
	}
	newVertex := &blockchain.Vertex{
		VertexId: blockId,
		V:        dag.Vertex(newBlock),
	}
	blockchain.Blockchain.Add(dag.Vertex((newVertex)))
	if isGlobal && server.LastAddedGlobalBlock != nil {
		edgeGlobal := dag.BasicEdge(dag.Vertex(newVertex), dag.Vertex(server.LastAddedGlobalBlock))
		blockchain.Blockchain.Connect(edgeGlobal)
		log.WithFields(log.Fields{
			"fromVertex": newVertex.VertexId,
			"toVertex":   server.LastAddedGlobalBlock.VertexId,
		}).Info("added new edge for global block")
	}
	// if this is the first global block, add an edge b/w this block and the lambda block as well
	if isGlobal && blockchain.GlobalSeqNumber == 1 {
		lamdbaBlock := server.VertexMap[common.LAMBDA_BLOCK]
		edgeGlobal := dag.BasicEdge(dag.Vertex(newVertex), dag.Vertex(lamdbaBlock))
		blockchain.Blockchain.Connect(edgeGlobal)
	}
	// check if the new local block to be added has a previously added global block of the same app
	if message.FromNodeNum == server.LastAddedGlobalNodeId {
		edge := dag.BasicEdge(dag.Vertex(newVertex), dag.Vertex(server.LastAddedGlobalBlock))
		blockchain.Blockchain.Connect(edge)
	}
	// irrespective of it being a local/global block, the new vertex HAS to point to
	// the last added local block
	edgeLocal := dag.BasicEdge(dag.Vertex(newVertex), dag.Vertex(server.LastAddedLocalBlock))
	blockchain.Blockchain.Connect(edgeLocal)

	server.VertexMap[blockId] = newVertex
	server.PIDMap[message.Clock.PID] = true

	if isLocal {
		server.LastAddedLocalBlock = newVertex
		server.LastAddedLocalNodeId = message.FromNodeNum
	} else if isGlobal {
		server.LastAddedGlobalBlock = newVertex
		server.LastAddedGlobalNodeId = message.FromNodeNum
	}
	log.WithFields(log.Fields{
		"fromVertex": newVertex.VertexId,
		"toVertex":   server.LastAddedLocalBlock.VertexId,
		"pid":        message.Clock.PID,
	}).Info("added new edge for local block")

	blockchain.PrintBlockchain()
	return
}
