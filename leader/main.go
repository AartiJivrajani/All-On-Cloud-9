package main

import (
	"All-On-Cloud-9/leader/node"
	"All-On-Cloud-9/common"
	"github.com/nats-io/nats.go"
	"fmt"
	"encoding/json"
)

func main() {
	l := leadernode.NewLeader(0)
	go func(leader *leadernode.Leader) {
		socket := common.Socket{}
		_ = socket.Connect(nats.DefaultURL)
		socket.Subscribe(common.DepsToLeader, func(m *nats.Msg) {
			fmt.Println("Received")
			received_json := string(m.Data)
			fmt.Println(received_json)
			data := common.MessageEvent{}
			json.Unmarshal([]byte(received_json), &data)
			l.AddToMessages(&data)
			if l.GetMessagesLen() > common.F {
				newMessageEvent := l.HandleReceiveDeps()
				fmt.Printf("%T\n", newMessageEvent)
				fmt.Printf("%T\n", newMessageEvent.VertexId)
				fmt.Printf("%T\n", newMessageEvent.Deps)
				// fmt.Println(newMessageEvent.Deps[1])
				// fmt.Printf("%+v\n",newMessageEvent)
				sentMessage, err := json.Marshal(&newMessageEvent)
				if err != nil {
					fmt.Println("i can publish a message")
					socket.Publish(common.LeaderToProposer, sentMessage)
				}else{
					fmt.Println("json marshal failed")
				}
				// should we flush when it fails?
				l.FlushMessages()
			}
		})
	}(&l)
	common.HandleInterrupt()
}