package pbft

import (
	"All-On-Cloud-9/common"
	"time"

	log "github.com/Sirupsen/logrus"
)

type pbftState struct {
	viewNumber        int
	failureTolerance  int
	totalNodes        int
	currentTimestamp  int
	viewChangeCounter int
	timerIsRunning    bool
	candidateNumber   int
	suffix            string
	counter           map[reducedMessage]int
	timeoutTimer      *time.Timer
	//localLog          []common.Message
	messageOut chan common.Transaction
}

func newPbftState(failureTolerance int, totalNodes int, suffix string) *pbftState {
	newState := pbftState{
		viewNumber:        0,
		failureTolerance:  failureTolerance,
		totalNodes:        totalNodes,
		currentTimestamp:  -1,
		viewChangeCounter: 0,
		timerIsRunning:    false,
		candidateNumber:   1,
		suffix:            suffix,
		counter:           make(map[reducedMessage]int),
		timeoutTimer:      nil,
		//localLog:          make([]common.Message, 0),
		messageOut: make(chan common.Transaction),
	}

	return &newState
}

func (state *pbftState) setTimer() {
	if !state.timerIsRunning {
		state.timeoutTimer.Reset(TIMEOUT * time.Second)
		state.timerIsRunning = true
	}
}

func (state *pbftState) stopTimer() {
	state.timeoutTimer.Stop()
	state.timerIsRunning = false
}

func (state *pbftState) handleMessage(
	message common.Message,
	broadcast func(common.Message),
	isLeader func() bool,
	isSuggestedLeader func(int) bool,
	getId func() int,
) {
	_txn := *message.Txn
	_message := message
	_message.Txn = &_txn

	switch _message.MessageType {
	case NEW_VIEW:
		state.viewChangeCounter = 0
		state.stopTimer()
		state.viewNumber = _message.Timestamp
		state.candidateNumber = state.viewNumber + 1
	case VIEW_CHANGE:
		if isSuggestedLeader(_message.Timestamp) {
			state.viewChangeCounter++
			//println("inside VIEW_CHANGE", getId(), state.viewChangeCounter, 2*state.failureTolerance+1)
			if state.viewChangeCounter == 2*state.failureTolerance+1 {
				//println("broadcasting NEW_VIEW")
				go broadcast(common.Message{
					MessageType: NEW_VIEW,
					Timestamp:   _message.Timestamp,
					FromNodeNum: getId(),
					Txn:         &dummyTxn,
				})
			}
		}
	case NEW_MESSAGE:
		if isLeader() {
			state.currentTimestamp++
			go broadcast(common.Message{
				MessageType: PRE_PREPARE,
				Timestamp:   state.currentTimestamp,
				FromNodeNum: getId(),
				Txn:         _message.Txn,
			})
		} else {
			state.setTimer()
		}
	case PRE_PREPARE:
		// TODO: Check if is from leader
		state.stopTimer()
		go broadcast(common.Message{
			MessageType: PREPARE,
			Timestamp:   _message.Timestamp,
			FromNodeNum: getId(),
			Txn:         _message.Txn,
		})
	case PREPARE:
		reduced := reducedMessage{
			messageType: PREPARE,
			Txn:         newReducedTransaction(*_message.Txn),
		}

		state.counter[reduced]++
		if state.totalNodes-state.counter[reduced] == state.failureTolerance {
			go broadcast(common.Message{
				MessageType: COMMIT,
				Timestamp:   _message.Timestamp,
				FromNodeNum: getId(),
				Txn:         _message.Txn,
			})
		} else if state.totalNodes-state.counter[reduced] < state.failureTolerance {
			log.WithFields(log.Fields{
				"phase":         "PREPARE",
				"state counter": state.counter[reduced],
				//"reduced":           reduced,
				//"failure tolerance": state.failureTolerance,
			}).Debug("PREPARE")
		}
	case COMMIT:
		reduced := reducedMessage{
			messageType: COMMIT,
			Txn:         newReducedTransaction(*_message.Txn),
		}

		state.counter[reduced]++
		if state.totalNodes-state.counter[reduced] == state.failureTolerance {
			go broadcast(common.Message{
				MessageType: COMMITED,
				Timestamp:   _message.Timestamp,
				FromNodeNum: getId(),
				Txn:         _message.Txn,
			})
		} else if state.totalNodes-state.counter[reduced] < state.failureTolerance {
			//println("COMMIT") //
		}
	case COMMITED:
		reduced := reducedMessage{
			messageType: COMMITED,
			Txn:         newReducedTransaction(*_message.Txn),
		}

		state.counter[reduced]++
		if state.totalNodes-state.counter[reduced] == 2*state.failureTolerance {
			state.currentTimestamp = _message.Timestamp
			state.messageOut <- *_message.Txn
		} else if state.totalNodes-state.counter[reduced] < 2*state.failureTolerance {
			//println("COMMITTED")
		}
	}
}
