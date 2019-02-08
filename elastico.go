package main

import (
	"fmt"
	// "crypto/sha256"
	"crypto/rsa"
	"crypto/rand"
	// "reflect"
)	

type Elastico struct{
	// connection = pika.BlockingConnection(pika.ConnectionParameters(host='localhost'))
	IP string
	port uint
	key *rsa.PrivateKey
	// PoW map[string]
	// cur_directory = set()
	// identity = ""
	// committee_id int32
	// // only when this node is the member of directory committee
	// committee_list = map[int32]
	// // only when this node is not the member of directory committee
	// committee_Members = set()
	// is_directory bool
	// is_final bool
	// epoch_randomness string
	// Ri string
	// // only when this node is the member of final committee
	// commitments = set()
	// txn_block = set()
	// set_of_Rs = set()
	// newset_of_Rs = set()
	// CommitteeConsensusData
	// finalBlockbyFinalCommittee = dict()
	// state map[string]int32
	// mergedBlock = []
	// finalBlock = {"sent" : False, "finalBlock" : set() }
	// RcommitmentSet = ""
	// newRcommitmentSet = ""
	// finalCommitteeMembers = set()
	// // only when this is the member of the directory committee
	// txn = dict()
	// response = []
	// flag bool
	// views = set()
	// primary bool
	// viewId int
	// faulty bool
	/* pre_prepareMsgLog
	// prepareMsgLog
	// commitMsgLog
	// preparedData
	// committedData
	// Finalpre_prepareMsgLog
	// FinalprepareMsgLog
	// FinalcommitMsgLog
	// FinalpreparedData
	// FinalcommittedData
	*/

}
func (e *Elastico) get_key(){
	/*
		for each node, it will set key as public pvt key pair
	*/
	var err error
	e.key, err = rsa.GenerateKey(rand.Reader, 2048)
	if err!= nil{
		fmt.Println(err.Error)
	}
}
func (e *Elastico)get_IP(){
	/*
		for each node(processor) , get IP addr
	*/
	count := 4
	byteArray := make([]byte, count)
	_, err := rand.Read(byteArray)
	if err != nil {
		fmt.Println("error:", err.Error)
	}
	e.IP= fmt.Sprintf("%v.%v.%v.%v" , byteArray[0] , byteArray[1], byteArray[2], byteArray[3])
}
func main(){

}