// A package clause starts every source file.
package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	"math/big"
	random "math/rand"
	"reflect"
	"strconv"
	"strings"
	"time"

	// "reflect"
	"encoding/base64"
	"encoding/json"
	"math"
	"os"
	"sync" // for locks

	log "github.com/sirupsen/logrus" // for logging
	"github.com/streadway/amqp"      // for rabbitmq
)

// ElasticoStates - states reperesenting the running state of the node
var ElasticoStates = map[string]int{"NONE": 0, "PoW Computed": 1, "Formed Identity": 2, "Formed Committee": 3, "RunAsDirectory": 4, "RunAsDirectory after-TxnReceived": 5, "RunAsDirectory after-TxnMulticast": 6, "Receiving Committee Members": 7, "PBFT_NONE": 8, "PBFT_PRE_PREPARE": 9, "PBFT_PRE_PREPARE_SENT": 10, "PBFT_PREPARE_SENT": 11, "PBFT_PREPARED": 12, "PBFT_COMMITTED": 13, "PBFT_COMMIT_SENT": 14, "Intra Consensus Result Sent to Final": 15, "Merged Consensus Data": 16, "FinalPBFT_NONE": 17, "FinalPBFT_PRE_PREPARE": 18, "FinalPBFT_PRE_PREPARE_SENT": 19, "FinalPBFT_PREPARE_SENT": 20, "FinalPBFT_PREPARED": 21, "FinalPBFT_COMMIT_SENT": 22, "FinalPBFT_COMMITTED": 23, "PBFT Finished-FinalCommittee": 24, "CommitmentSentToFinal": 25, "FinalBlockSent": 26, "FinalBlockReceived": 27, "BroadcastedR": 28, "ReceivedR": 29, "FinalBlockSentToClient": 30, "LedgerUpdated": 31}

var wg sync.WaitGroup

// shared lock among processes
var lock sync.Mutex

// shared Port among the processes
var Port = 49152

// n : number of nodes
var n int64 = 66

// s - where 2^s is the number of committees
var s = 2

// c - size of committee
var c = 4

// D - difficulty level , leading bits of PoW must have D 0's (keep w.r.t to hex)
var D = 3

// r - number of bits in random string
var r int64 = 4

// finNum - final committee id
var finNum int64

// networkNodes - list of elastico objects
var networkNodes []Elastico

var sharedObj map[int64]bool

// commitmentSet - set of commitments S
var commitmentSet map[string]bool

func failOnError(err error, msg string, exit bool) {
	// logging the error
	if err != nil {
		log.Error(msg, " : ", err)
		if exit {
			os.Exit(1)
		}
	}
}

func getChannel(connection *amqp.Connection) *amqp.Channel {
	/*
		get channel
	*/
	channel, err := connection.Channel()               // create a channel
	failOnError(err, "Failed to open a channel", true) // report the error
	return channel
}

func getConnection() *amqp.Connection {
	/*
		establish a connection with RabbitMQ server
	*/
	connection, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ", true) // report the error
	return connection
}

func marshalData(msg map[string]interface{}) []byte {

	body, err := json.Marshal(msg)
	// fmt.Println("marshall data", body)
	failOnError(err, "error in marshal", true)
	return body
}

func publishMsg(channel *amqp.Channel, queueName string, msg map[string]interface{}) {

	//create a hello queue to which the message will be delivered
	queue, err := channel.QueueDeclare(
		queueName, //name of the queue
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	failOnError(err, "Failed to declare a queue", true)

	body := marshalData(msg)

	err = channel.Publish(
		"",         // exchange
		queue.Name, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        body,
		})

	failOnError(err, "Failed to publish a message", true)
}

func consistencyProtocol() (bool, map[string]bool) {
	/*
		Agrees on a single set of Hash values(S)
		presently selecting random c hash of Ris from the total set of commitments
	*/
	// // ToDo: implement interactive consistency Protocol
	// for node := range networkNodes{

	// 	if node.isFinalMember(){

	// 		if len(node.commitments) <= c/2{

	// 			log.Warn("insufficientCommitments")
	// 			return false, "insufficientCommitments"
	// 		}
	// 	}
	// }

	//  if len(commitmentSet) == 0{

	// 	 flag := true
	// 	 for node := range networkNodes{

	// 		 if node.isFinalMember(){

	// 			 if flag && len(commitmentSet) == 0{

	// 				 flag = false
	// 				 commitmentSet = node.commitments
	// 			 } else{

	// 				 commitmentSet = commitmentSet.intersection(node.commitments)
	// 			 }
	// 		 }
	// 	 }
	//  }
	return true, commitmentSet
}

type ViewsMsg struct {
	CommitteeMembers      []IDENTITY
	FinalCommitteeMembers []IDENTITY
	Identity              IDENTITY
	Txns                  []Transaction
}

// MulticastCommittee :- each node getting views of its committee members from directory members
func MulticastCommittee(commList map[int64][]IDENTITY, identityobj IDENTITY, txns map[int64][]Transaction) {

	// get the final committee members with the fixed committee id
	finalCommitteeMembers := commList[finNum]
	for CommitteeID, commMembers := range commList {

		// find the primary Identity, Take the first Identity
		// ToDo: fix this, many nodes can be primary
		primaryID := commMembers[0]
		for _, memberID := range commMembers {

			// send the committee members , final committee members
			data := make(map[string]interface{})
			data["CommitteeMembers"] = commMembers
			data["FinalCommitteeMembers"] = finalCommitteeMembers
			data["Identity"] = identityobj
			// give txns only to the primary node
			if memberID.isEqual(&primaryID) {
				data["Txns"] = txns[CommitteeID]
			} else {
				data["Txns"] = make([]Transaction, 0)
			}
			// construct the msg
			msg := map[string]interface{}{"data": data, "type": "committee members views"}
			// send the committee member views to nodes
			memberID.send(msg)
		}
	}
}

// BroadcastToNetwork - Broadcast data to the whole ntw
func BroadcastToNetwork(msg map[string]interface{}) {

	connection := getConnection()
	defer connection.Close() // close the connection

	channel := getChannel(connection)
	defer channel.Close() // close the channel

	for _, node := range networkNodes {
		nodePort := strconv.Itoa(node.Port)
		queueName := "hello" + nodePort
		publishMsg(channel, queueName, msg) //publish the message in queue
	}
}

func randomGen(r int64) *big.Int {
	/*
		generate a random integer
	*/
	// n is the base, e is the exponent, creating big.Int variables
	var num, e = big.NewInt(2), big.NewInt(r)
	// taking the exponent n to the power e and nil modulo, and storing the result in n
	num.Exp(num, e, nil)
	// generates the random num in the range[0,n)
	// here Reader is a global, shared instance of a cryptographically secure random number generator.
	randomNum, err := rand.Int(rand.Reader, num)

	failOnError(err, "random number generation", true)
	return randomNum
}

// IdentityAndSign :- for signature and its Identity which can be used for verification
type IdentityAndSign struct {
	sign        string
	identityobj IDENTITY
}

// IdentityAndSignInit :- initialise the members of structure
func (is *IdentityAndSign) IdentityAndSignInit(sign string, identityobj IDENTITY) {

	is.sign = sign
	is.identityobj = identityobj
}

func (is *IdentityAndSign) isEqual(data IdentityAndSign) bool {
	/*
		compare two objects
	*/
	return is.sign == data.sign && is.identityobj.isEqual(&data.identityobj)
}

// FinalCommittedBlock :- final committed block that consists of txns and list of signatures and identities
type FinalCommittedBlock struct {
	txnList                       []Transaction
	listSignaturesAndIdentityobjs []IdentityAndSign
}

//FinalBlockInit :- initialise the members
func (fb *FinalCommittedBlock) FinalBlockInit(txnList []Transaction, listSignaturesAndIdentityobjs []IdentityAndSign) {
	fb.txnList = txnList
	fb.listSignaturesAndIdentityobjs = listSignaturesAndIdentityobjs
}

// BlockHeader :- structure for block header
type BlockHeader struct {
	prevBlockHash     string
	numAncestorBlocks int
	txnCount          int
	rootHash          string
}

// BlockHeaderInit :- init for block header
func (bh *BlockHeader) BlockHeaderInit(prevBlockHash string, numAncestorBlocks int, txnCount int, rootHash string) {
	bh.prevBlockHash = prevBlockHash
	bh.numAncestorBlocks = numAncestorBlocks
	bh.txnCount = txnCount
	bh.rootHash = rootHash
}

func (bh *BlockHeader) hexdigest() []byte {
	/*
		Digest of a block header
	*/
	digest := sha256.New()
	digest.Write([]byte(bh.prevBlockHash))
	digest.Write([]byte(strconv.Itoa(bh.numAncestorBlocks)))
	digest.Write([]byte(strconv.Itoa(bh.txnCount)))
	digest.Write([]byte(bh.rootHash))
	return digest.Sum(nil)
}

// BlockData :- block data consists of txns and merkle tree
type BlockData struct {
	transactions []Transaction
}

// BlockDataInit :- init for block data
func (bd *BlockData) BlockDataInit(transactions []Transaction) {
	bd.transactions = transactions
}

func (bd *BlockData) hexdigest() []byte {
	/*
		Digest of a block data
	*/
	digest := sha256.New()
	// take the digest of the txnBlock
	txndigest := txnHexdigest(bd.transactions)
	digest.Write([]byte(txndigest))

	return digest.Sum(nil)

}

// IDENTITY :- structure for Identity of nodes
type IDENTITY struct {
	IP              string
	PK              rsa.PublicKey
	CommitteeID     int64
	PoW             map[string]interface{}
	EpochRandomness string
	Port            int
}

// IdentityInit :- initialise of Identity members
func (i *IDENTITY) IdentityInit() {
	i.PoW = make(map[string]interface{})
}

func (i *IDENTITY) isEqual(identityobj *IDENTITY) bool {
	/*
		checking two objects of Identity class are equal or not

	*/
	// comparing uncomparable type []interface {} ie. set Of Rs

	// ToDo: compare set of Rs
	// listOfRsInI := i.PoW["setOfRs"].([]interface{})
	// setOfRsInI := make([]string, len(listOfRsInI))
	// for i := range listOfRsInI {
	// 	setOfRsInI[i] = listOfRsInI[i].(string)
	// }

	// listOfRsIniobj := identityobj.PoW["setOfRs"].([]interface{})
	// setOfRsIniobj := make([]string, len(listOfRsIniobj))
	// for i := range listOfRsIniobj {
	// 	setOfRsIniobj[i] = listOfRsIniobj[i].(string)
	// }

	return i.IP == identityobj.IP && i.PK == identityobj.PK && i.CommitteeID == identityobj.CommitteeID && i.PoW["hash"] == identityobj.PoW["hash"] && i.PoW["nonce"] == identityobj.PoW["nonce"] && i.EpochRandomness == identityobj.EpochRandomness && i.Port == identityobj.Port
}

func (i *IDENTITY) send(msg map[string]interface{}) {
	/*
		send the msg to node based on their Identity
	*/
	// establish a connection with RabbitMQ server
	connection := getConnection()
	defer connection.Close()

	// create a channel
	channel := getChannel(connection)
	// close the channel
	defer channel.Close()

	nodePort := strconv.Itoa(i.Port)
	queueName := "hello" + nodePort
	publishMsg(channel, queueName, msg) // publish the msg in queue
}

// Transaction :- structure for transaction
type Transaction struct {
	Sender   string
	Receiver string
	Amount   *big.Int // randomGen returns *big.Int
	// ToDo: include timestamp or not
}

// TransactionInit :- initialise of data members
func (t *Transaction) TransactionInit(sender string, receiver string, amount *big.Int) {
	t.Sender = sender
	t.Receiver = receiver
	t.Amount = amount
}

func (t *Transaction) hexdigest() string {
	/*
		Digest of a transaction
	*/
	digest := sha256.New()
	digest.Write([]byte(t.Sender))
	digest.Write([]byte(t.Receiver))
	digest.Write([]byte(t.Amount.String())) // convert amount(big.Int) to string

	hashVal := fmt.Sprintf("%x", digest.Sum(nil))
	return hashVal
}

func (t *Transaction) isEqual(transaction Transaction) bool {
	/*
		compare two objs are equal or not
	*/
	return t.Sender == transaction.Sender && t.Receiver == transaction.Receiver && t.Amount == transaction.Amount //&& t.timestamp == transaction.timestamp
}

type msgType struct {
	Data json.RawMessage
	Type string
}

// Elastico :- structure of elastico node
type Elastico struct {
	/*
		connection - rabbitmq connection
		IP - IP address of a node
		Port - unique number for a process
		key - public key and private key pair for a node
		PoW - dict containing 256 bit hash computed by the node, set of Rs needed for epoch randomness, and a nonce
		cur_directory - list of directory members in view of the node
		Identity - Identity consists of Public key, an IP, PoW, committee id, epoch randomness, Port
		committee_id - integer value to represent the committee to which the node belongs
		committee_list - list of nodes in all committees
		committee_Members - set of committee members in its own committee
		is_directory - whether the node belongs to directory committee or not
		is_final - whether the node belongs to final committee or not
		epoch_randomness - r-bit random string generated at the end of previous epoch
		Ri - r-bit random string
		commitments - set of H(Ri) received by final committee node members and H(Ri) is sent by the final committee node only
		txn_block - block of txns that the committee will agree on(intra committee consensus block)
		set_of_Rs - set of Ris obtained from the final committee of previous epoch
		newset_of_Rs - In the present epoch, set of Ris obtained from the final committee
		CommitteeConsensusData - a dictionary of committee ids that contains a dictionary of the txn block and the signatures
		finalBlockbyFinalCommittee - a dictionary of txn block and the signatures by the final committee members
		state - state in which a node is running
		mergedBlock - list of txns of different committees after their intra committee consensus
		finalBlock - agreed list of txns after pbft run by final committee
		RcommitmentSet - set of H(Ri)s received from the final committee after the consistency protocol [previous epoch values]
		newRcommitmentSet - For the present it contains the set of H(Ri)s received from the final committee after the consistency protocol
		finalCommitteeMembers - members of the final committee received from the directory committee
		txn- transactions stored by the directory members
		response - final block to be received by the client
		flag- to denote a bad or good node
		views - stores the ports of processes from which committee member views have been received
		primary- boolean to denote the primary node in the committee for PBFT run
		viewId - view number of the pbft
		prePrepareMsgLog - log of pre-prepare msgs received during PBFT
		prepareMsgLog - log of prepare msgs received during PBFT
		commitMsgLog - log of commit msgs received during PBFT
		preparedData - data after prepared state
		committedData - data after committed state
		Finalpre_prepareMsgLog - log of pre-prepare msgs received during PBFT run by final committee
		FinalprepareMsgLog - log of prepare msgs received during PBFT run by final committee
		FinalcommitMsgLog - log of commit msgs received during PBFT run by final committee
		FinalpreparedData - data after prepared state in final pbft run
		FinalcommittedData - data after committed state in final pbft run
		faulty - Flag denotes whether this node is faulty or not
	*/
	connection   *amqp.Connection
	IP           string
	Port         int
	key          *rsa.PrivateKey
	PoW          map[string]interface{}
	curDirectory []IDENTITY
	Identity     IDENTITY
	CommitteeID  int64
	// only when this node is the member of directory committee
	committeeList map[int64][]IDENTITY
	// only when this node is not the member of directory committee
	committeeMembers []IDENTITY
	isDirectory      bool
	isFinal          bool
	EpochRandomness  string
	Ri               string
	// only when this node is the member of final committee
	commitments                    map[string]bool
	txnBlock                       []Transaction
	setOfRs                        map[string]bool
	newsetOfRs                     map[string]bool
	CommitteeConsensusData         map[int64]map[string][]string
	CommitteeConsensusDataTxns     map[int64]map[string][]Transaction
	finalBlockbyFinalCommittee     map[string][]IdentityAndSign
	finalBlockbyFinalCommitteeTxns map[string][]Transaction
	state                          int
	mergedBlock                    []Transaction
	finalBlock                     map[string]interface{}
	RcommitmentSet                 map[string]bool
	newRcommitmentSet              map[string]bool
	finalCommitteeMembers          []IDENTITY
	// only when this is the member of the directory committee
	txn                   map[int64][]Transaction
	response              []Transaction
	flag                  bool
	views                 map[int]bool
	primary               bool
	viewID                int
	faulty                bool
	prePrepareMsgLog      map[string]PrePrepareMsg
	prepareMsgLog         map[int]map[int]map[string][]PrepareMsgData
	commitMsgLog          map[int]map[int]map[string][]CommitMsgData
	preparedData          map[int]map[int][]Transaction
	committedData         map[int]map[int][]Transaction
	FinalPrePrepareMsgLog map[string]PrePrepareMsg
	FinalPrepareMsgLog    map[int]map[int]map[string][]PrepareMsgData
	FinalcommitMsgLog     map[int]map[int]map[string][]CommitMsgData
	FinalpreparedData     map[int]map[int][]Transaction
	FinalcommittedData    map[int]map[int][]Transaction
}

type PrepareMsgData struct {
	Digest   string
	Identity IDENTITY
}

type CommitMsgData struct {
	Digest   string
	Identity IDENTITY
}

func (e *Elastico) getKey() {
	/*
		for each node, it will set key as public pvt key pair
	*/
	var err error
	// generate the public-pvt key pair
	e.key, err = rsa.GenerateKey(rand.Reader, 2048)
	failOnError(err, "key generation", true)
}

func (e *Elastico) getIP() {
	/*
		for each node(processor) , get IP addr
	*/
	count := 4
	// construct the byte array of size 4
	byteArray := make([]byte, count)
	// Assigning random values to the byte array
	_, err := rand.Read(byteArray)
	failOnError(err, "reading random values error", true)
	// setting the IP addr from the byte array
	e.IP = fmt.Sprintf("%v.%v.%v.%v", byteArray[0], byteArray[1], byteArray[2], byteArray[3])
}

func (e *Elastico) initER() {
	/*
		initialise r-bit epoch random string
	*/

	randomnum := randomGen(r)
	// set r-bit binary string to epoch randomness
	e.EpochRandomness = fmt.Sprintf("%0"+strconv.FormatInt(r, 10)+"b\n", randomnum)
}

func (e *Elastico) getPort() {
	/*
		get Port number for the process
	*/
	// acquire the lock
	lock.Lock()
	Port++
	e.Port = Port
	// release the lock
	defer lock.Unlock()
}

func sample(A []string, x int) []string {
	// randomly sample x values from list of strings A
	random.Seed(time.Now().UnixNano())
	randomize := random.Perm(len(A)) // get the random permutation of indices of A

	sampleslice := make([]string, 0)

	for _, v := range randomize[:x] {
		sampleslice = append(sampleslice, A[v])
	}
	return sampleslice
}

func xorbinary(A []string) int64 {
	// returns xor of the binary strings of A
	var xorVal int64
	for i := range A {
		intval, _ := strconv.ParseInt(A[i], 2, 0)
		xorVal = xorVal ^ intval
	}
	return xorVal
}

func (e *Elastico) xorR() (string, []string) {
	// find xor of any random c/2 + 1 r-bit strings to set the epoch randomness
	listOfRs := make([]string, 0)
	for R := range e.setOfRs {
		listOfRs = append(listOfRs, R)
	}
	randomset := sample(listOfRs, c/2+1) //get random c/2 + 1 strings from list of Rs
	xorVal := xorbinary(randomset)
	xorString := fmt.Sprintf("%0"+strconv.FormatInt(r, 10)+"b\n", xorVal) //converting xor value to r-bit string
	return xorString, randomset
}

func (e *Elastico) computePoW() {
	/*
		returns hash which satisfies the difficulty challenge(D) : PoW["hash"]
	*/
	zeroString := ""
	for i := 0; i < D; i++ {
		zeroString += "0"
	}
	if e.state == ElasticoStates["NONE"] {
		nonce := e.PoW["nonce"].(int) // type assertion
		PK := e.key.PublicKey         // public key
		IP := e.IP
		// If it is the first epoch , randomsetR will be an empty set .
		// otherwise randomsetR will be any c/2 + 1 random strings Ri that node receives from the previous epoch
		randomsetR := make([]string, 0)
		if len(e.setOfRs) > 0 {
			e.EpochRandomness, randomsetR = e.xorR()
		}
		// 	compute the digest
		digest := sha256.New()
		digest.Write([]byte(IP))
		digest.Write(PK.N.Bytes())
		digest.Write([]byte(strconv.Itoa(PK.E)))
		digest.Write([]byte(e.EpochRandomness))
		digest.Write([]byte(strconv.Itoa(nonce)))

		hashVal := fmt.Sprintf("%x", digest.Sum(nil))
		if strings.HasPrefix(hashVal, zeroString) {
			//hash starts with leading D 0's
			e.PoW["hash"] = hashVal
			e.PoW["setOfRs"] = randomsetR
			e.PoW["nonce"] = nonce
			// change the state after solving the puzzle
			e.state = ElasticoStates["PoW Computed"]
		} else {
			// try for other nonce
			nonce++
			e.PoW["nonce"] = nonce
		}
	}
}

func (e *Elastico) checkCommitteeFull() {
	/*
		directory member checks whether the committees are full or not
	*/
	commList := e.committeeList
	flag := 0
	numOfCommittees := int64(math.Pow(2, float64(s)))
	// iterating over all committee ids
	for iden := int64(0); iden < numOfCommittees; iden++ {

		_, ok := commList[iden]
		if ok == false || len(commList[iden]) < c {

			log.Warn("committees not full  - bad miss id :", iden)
			flag = 1
			break
		}
	}
	if flag == 0 {

		log.Warn("committees full  - good")
		if e.state == ElasticoStates["RunAsDirectory after-TxnReceived"] {

			// notify the final members
			e.notifyFinalCommittee()
			// multicast the txns and committee members to the nodes
			MulticastCommittee(commList, e.Identity, e.txn)
			// change the state after multicast
			e.state = ElasticoStates["RunAsDirectory after-TxnMulticast"]
		}
	}
}

func (e *Elastico) receiveViews(msg msgType) {
	var decodeMsg ViewsMsg

	err := json.Unmarshal(msg.Data, &decodeMsg)
	// fmt.Println("views msg---", decodeMsg)
	failOnError(err, "fail to decode views msg", true)

	identityobj := decodeMsg.Identity

	if e.verifyPoW(identityobj) {

		if _, ok := e.views[identityobj.Port]; ok == false {

			// union of committe members views
			e.views[identityobj.Port] = true

			commMembers := decodeMsg.CommitteeMembers
			finalMembers := decodeMsg.FinalCommitteeMembers
			Txns := decodeMsg.Txns

			// update the txn block
			// ToDo: txnblock should be ordered, not set
			if len(Txns) > 0 {

				e.txnBlock = e.unionTxns(e.txnBlock, Txns)
				log.Warn("I am primary", e.Port)
				e.primary = true
			}

			// ToDo: verify this union thing
			// union of committee members wrt directory member
			e.committeeMembers = e.unionViews(e.committeeMembers, commMembers)
			// union of final committee members wrt directory member
			e.finalCommitteeMembers = e.unionViews(e.finalCommitteeMembers, finalMembers)
			// received the members
			if e.state == ElasticoStates["Formed Committee"] && len(e.views) >= c/2+1 {

				e.state = ElasticoStates["Receiving Committee Members"]
			}
		}
	}
}

func (e *Elastico) receiveDirectoryMember(msg msgType) {
	var decodeMsg Dmsg

	err := json.Unmarshal(msg.Data, &decodeMsg)
	failOnError(err, "fail to decode directory member msg", true)

	identityobj := decodeMsg.Identity

	// verify the PoW of the sender
	if e.verifyPoW(identityobj) {
		if len(e.curDirectory) < c {
			// check whether identityobj is already present or not
			flag := true
			for _, obj := range e.curDirectory {
				if identityobj.isEqual(&obj) {
					flag = false
					break
				}
			}
			if flag {
				// append the object if not already present
				e.curDirectory = append(e.curDirectory, identityobj)
			}
		}
	} else {
		log.Error("PoW not valid of an incoming directory member", identityobj)
	}

}

func (e *Elastico) receiveNewNode(msg msgType) {
	var decodeMsg NewNodeMsg

	err := json.Unmarshal(msg.Data, &decodeMsg)
	failOnError(err, "decode error in new node msg", true)
	// new node is added to the corresponding committee list if committee list has less than c members
	identityobj := decodeMsg.Identity
	// verify the PoW
	if e.verifyPoW(identityobj) {
		_, ok := e.committeeList[identityobj.CommitteeID]
		if ok == false {

			// Add the Identity in committee
			e.committeeList[identityobj.CommitteeID] = []IDENTITY{identityobj}

		} else if len(e.committeeList[identityobj.CommitteeID]) < c {
			// Add the Identity in committee
			flag := true
			for _, obj := range e.committeeList[identityobj.CommitteeID] {
				if identityobj.isEqual(&obj) {
					flag = false
					break
				}
			}
			if flag {
				e.committeeList[identityobj.CommitteeID] = append(e.committeeList[identityobj.CommitteeID], identityobj)
				if len(e.committeeList[identityobj.CommitteeID]) == c {
					// check that if all committees are full
					e.checkCommitteeFull()
				}
			}
		}

	} else {
		log.Error("PoW not valid in adding new node")
	}
}

func (e *Elastico) receiveHash(msg msgType) {

	// receiving H(Ri) by final committe members
	var decodeMsg HashMsg

	err := json.Unmarshal(msg.Data, &decodeMsg)

	failOnError(err, "fail to decode hash msg", true)

	identityobj := decodeMsg.Identity
	HashRi := decodeMsg.HashRi
	if e.verifyPoW(identityobj) {
		e.commitments[HashRi] = true
	}
}

func (e *Elastico) receiveRandomStringBroadcast(msg msgType) {

	var decodeMsg BroadcastRmsg

	err := json.Unmarshal(msg.Data, &decodeMsg)

	failOnError(err, "fail to decode random string msg", true)

	identityobj := decodeMsg.Identity
	Ri := decodeMsg.Ri

	if e.verifyPoW(identityobj) {
		HashRi := e.hexdigest(Ri)

		if _, ok := e.newRcommitmentSet[HashRi]; ok {

			e.newsetOfRs[Ri] = true

			if len(e.newsetOfRs) >= c/2+1 {
				e.state = ElasticoStates["ReceivedR"]
			}
		}
	}
}

func unionSet(newSet map[string]bool, receivedSet map[string]bool) {
	for commitment := range receivedSet {
		newSet[commitment] = true
	}
}

func (e *Elastico) digestCommitments(receivedCommitments map[string]bool) []byte {
	digest := sha256.New()
	for commitment := range receivedCommitments {
		digest.Write([]byte(commitment))
	}
	return digest.Sum(nil)
}

func (e *Elastico) receiveFinalTxnBlock(msg map[string]interface{}) {

	data := msg["data"].(map[string]interface{})
	identityobj := data["Identity"].(IDENTITY)
	// verify the PoW of the sender
	if e.verifyPoW(identityobj) {

		sign := data["signature"].(string)
		receivedCommitments := data["commitmentSet"].(map[string]bool)
		finalTxnBlock := data["finalTxnBlock"].([]Transaction)
		finalTxnBlockSignature := data["finalTxnBlockSignature"].(string)
		// verify the signatures
		receivedCommitmentDigest := e.digestCommitments(receivedCommitments)
		PK := identityobj.PK
		if e.verifySign(sign, receivedCommitmentDigest, &PK) && e.verifySignTxnList(finalTxnBlockSignature, finalTxnBlock, &PK) {

			// list init for final txn block
			finaltxnBlockDigest := txnHexdigest(finalTxnBlock)
			if _, ok := e.finalBlockbyFinalCommittee[finaltxnBlockDigest]; ok == false {
				e.finalBlockbyFinalCommittee[finaltxnBlockDigest] = make([]IdentityAndSign, 0)
				e.finalBlockbyFinalCommitteeTxns[finaltxnBlockDigest] = finalTxnBlock
			}

			// creating the object that contains the Identity and signature of the final member
			identityAndSign := IdentityAndSign{finalTxnBlockSignature, identityobj}

			// check whether this combination of Identity and sign already exists or not
			flag := true
			for _, idSignObj := range e.finalBlockbyFinalCommittee[finaltxnBlockDigest] {

				if idSignObj.isEqual(identityAndSign) {
					// it exists
					flag = false
					break
				}
			}
			if flag {
				// appending the Identity and sign of final member
				e.finalBlockbyFinalCommittee[finaltxnBlockDigest] = append(e.finalBlockbyFinalCommittee[finaltxnBlockDigest], identityAndSign)
			}

			// block is signed by sufficient final members and when the final block has not been sent to the client yet
			if len(e.finalBlockbyFinalCommittee[finaltxnBlockDigest]) >= c/2+1 && e.state != ElasticoStates["FinalBlockSentToClient"] {
				// for final members, their state is updated only when they have also sent the finalblock to ntw
				if e.isFinalMember() {
					finalBlockSent := e.finalBlock["sent"].(bool)
					if finalBlockSent {

						e.state = ElasticoStates["FinalBlockReceived"]
					}

				} else {

					e.state = ElasticoStates["FinalBlockReceived"]
				}

				// union of commitments
				unionSet(e.newRcommitmentSet, receivedCommitments)
			}

		} else {

			log.Error("Signature invalid in final block received")
		}
	} else {
		log.Error("PoW not valid when final member send the block")
	}
}

// BroadcastFinalTxn :- final committee members will broadcast S(commitmentSet), along with final set of X(txn_block) to everyone in the network
func (e *Elastico) BroadcastFinalTxn() {
	/*
		final committee members will broadcast S(commitmentSet), along with final set of
		X(txn_block) to everyone in the network
	*/
	// boolVal , S = consistencyProtocol()
	// if boolVal == False:
	// 	return S
	// commitmentList = list(S)
	// PK = self.key.publickey().exportKey().decode()
	// data = {"commitmentSet" : commitmentList, "signature" : self.sign(commitmentList) , "Identity" : self.Identity , "finalTxnBlock" : self.finalBlock["finalBlock"] , "finalTxnBlock_signature" : self.signTxnList(self.finalBlock["finalBlock"])}
	// logging.warning("finalblock- %s" , str(self.finalBlock["finalBlock"]))
	// # final Block sent to ntw
	// self.finalBlock["sent"] = True
	// # A final node which is already in received state should not change its state
	// if self.state != ELASTICO_STATES["FinalBlockReceived"]:
	// 	logging.warning("change state to FinalBlockSent by %s" , str(self.Port))
	// 	self.state = ELASTICO_STATES["FinalBlockSent"]
	// BroadcastTo_Network(data, "finalTxnBlock")
}

func (e *Elastico) receiveIntraCommitteeBlock(msg msgType) {
	// final committee member receives the final set of txns along with the signature from the node
	var decodeMsg IntraBlockMsg
	err := json.Unmarshal(msg.Data, &decodeMsg)
	failOnError(err, "error in unmarshal intra committee block", true)

	identityobj := decodeMsg.Identity

	if e.verifyPoW(identityobj) {
		signature := decodeMsg.Sign
		TxnBlock := decodeMsg.Txnblock
		// verify the signatures
		PK := identityobj.PK
		if e.verifySignTxnList(signature, TxnBlock, &PK) {
			if _, ok := e.CommitteeConsensusData[identityobj.CommitteeID]; ok == false {

				e.CommitteeConsensusData[identityobj.CommitteeID] = make(map[string][]string)
				e.CommitteeConsensusDataTxns[identityobj.CommitteeID] = make(map[string][]Transaction)
			}
			TxnBlockDigest := txnHexdigest(TxnBlock)
			if _, ok := e.CommitteeConsensusData[identityobj.CommitteeID][TxnBlockDigest]; ok == false {
				e.CommitteeConsensusData[identityobj.CommitteeID][TxnBlockDigest] = make([]string, 0)
				// store the txns for this digest
				e.CommitteeConsensusDataTxns[identityobj.CommitteeID][TxnBlockDigest] = TxnBlock
			}

			// add signatures for the txn block
			e.CommitteeConsensusData[identityobj.CommitteeID][TxnBlockDigest] = append(e.CommitteeConsensusData[identityobj.CommitteeID][TxnBlockDigest], signature)

		} else {
			log.Error("signature invalid for intra committee block")
		}
	} else {
		log.Error("pow invalid for intra committee block")
	}

}

func (e *Elastico) verifySign(signature string, digest []byte, PublicKey *rsa.PublicKey) bool {
	/*
		verify whether signature is valid or not
	*/
	signed, err := base64.StdEncoding.DecodeString(signature) // Decode the base64 encoded signature
	failOnError(err, "Decode error of signature", true)
	err = rsa.VerifyPKCS1v15(PublicKey, crypto.SHA256, digest, signed) // verify the sign of digest
	if err != nil {
		return false
	}
	return true
}

func (e *Elastico) signTxnList(TxnBlock []Transaction) string {
	// Sign the array of Transactions
	digest := sha256.New()
	for i := 0; i < len(TxnBlock); i++ {
		txnDigest := TxnBlock[i].hexdigest() // Get the transaction digest
		digest.Write([]byte(txnDigest))
	}
	signed, err := rsa.SignPKCS1v15(rand.Reader, e.key, crypto.SHA256, digest.Sum(nil)) // sign the digest of Txn List
	failOnError(err, "Error in Signing Txn List", true)
	signature := base64.StdEncoding.EncodeToString(signed) // encode to base64
	return signature
}

func (e *Elastico) verifySignTxnList(TxnBlockSignature string, TxnBlock []Transaction, PublicKey *rsa.PublicKey) bool {
	signed, err := base64.StdEncoding.DecodeString(TxnBlockSignature) // Decode the base64 encoded signature
	failOnError(err, "Decode error of signature", true)
	// Sign the array of Transactions
	digest := sha256.New()
	for i := 0; i < len(TxnBlock); i++ {
		txnDigest := TxnBlock[i].hexdigest() // Get the transaction digest
		digest.Write([]byte(txnDigest))
	}
	err = rsa.VerifyPKCS1v15(PublicKey, crypto.SHA256, digest.Sum(nil), signed) // verify the sign of digest of Txn List
	if err != nil {
		return false
	}
	return true
}

func (e *Elastico) receive(msg msgType) {
	/*
		method to recieve messages for a node as per the type of a msg
	*/
	// new node is added in directory committee if not yet formed
	if msg.Type == "directoryMember" {
		e.receiveDirectoryMember(msg)

	} else if msg.Type == "newNode" && e.isDirectory {
		e.receiveNewNode(msg)

	} else if msg.Type == "committee members views" && e.isDirectory == false {
		e.receiveViews(msg)

	} else if msg.Type == "hash" && e.isFinalMember() {
		e.receiveHash(msg)

	} else if msg.Type == "RandomStringBroadcast" {
		e.receiveRandomStringBroadcast(msg)

	} else if msg.Type == "pre-prepare" || msg.Type == "prepare" || msg.Type == "commit" {

		e.pbftProcessMessage(msg)
	} else if msg.Type == "intraCommitteeBlock" && e.isFinalMember() {

		e.receiveIntraCommitteeBlock(msg)
	}
	else if msg.Type == "notify final member" {
		log.Warn("notifying final member ", e.Port)
		var decodeMsg NotifyFinalMsg
		err := json.Unmarshal(msg.Data , &decodeMsg)
		failOnError(err, "error in decoding final member msg" , true)
		identityobj := decodeMsg.Identity
		if e.verifyPoW(identityobj) && e.CommitteeID == finNum {
			e.isFinal = true
		}
	} 
	/* else if msg.Type == "finalTxnBlock" {
			e.receiveFinalTxnBlock(msg)
	}*/

	/*  else if msg.Type == "reset-all" {
		// ToDo: Add verification of pow here.
		// reset the elastico node
		e.reset()

	} */ /* else if msg.Type == "Finalpre-prepare" || msg.Type == "Finalprepare" || msg.Type == "Finalcommit" {
		e.FinalpbftProcessMessage(msg)
	}
	*/
}

// ElasticoInit :- initialise of data members
func (e *Elastico) ElasticoInit() {
	var err error
	// create rabbit mq connection
	e.connection, err = amqp.Dial("amqp://guest:guest@localhost:5672/")
	// log if connection fails
	failOnError(err, "Failed to connect to RabbitMQ", true)
	// set IP
	e.getIP()
	e.getPort()
	// set RSA
	e.getKey()
	// Initialize PoW!
	e.PoW = make(map[string]interface{})
	e.PoW["hash"] = ""
	e.PoW["setOfRs"] = ""
	e.PoW["nonce"] = 0

	e.curDirectory = make([]IDENTITY, 0)

	e.committeeList = make(map[int64][]IDENTITY)

	e.committeeMembers = make([]IDENTITY, 0)

	// for setting EpochRandomness
	e.initER()

	e.commitments = make(map[string]bool)

	e.txnBlock = make([]Transaction, 0)

	e.setOfRs = make(map[string]bool)

	e.newsetOfRs = make(map[string]bool)

	e.CommitteeConsensusData = make(map[int64]map[string][]string)

	e.CommitteeConsensusDataTxns = make(map[int64]map[string][]Transaction)

	e.finalBlockbyFinalCommittee = make(map[string][]IdentityAndSign)

	e.finalBlockbyFinalCommitteeTxns = make(map[string][]Transaction)

	e.state = ElasticoStates["NONE"]

	e.mergedBlock = make([]Transaction, 0)

	e.finalBlock = make(map[string]interface{})
	e.finalBlock["sent"] = false
	e.finalBlock["finalBlock"] = make([]Transaction, 0)

	e.RcommitmentSet = make(map[string]bool)
	e.newRcommitmentSet = make(map[string]bool)
	e.finalCommitteeMembers = make([]IDENTITY, 0)

	e.txn = make(map[int64][]Transaction)
	e.response = make([]Transaction, 0)
	e.flag = true
	e.views = make(map[int]bool)
	e.primary = false
	e.viewID = 0
	e.faulty = false

	e.prePrepareMsgLog = make(map[string]PrePrepareMsg)
	e.prepareMsgLog = make(map[int]map[int]map[string][]PrepareMsgData)
	e.commitMsgLog = make(map[int]map[int]map[string][]CommitMsgData)
	e.preparedData = make(map[int]map[int][]Transaction)
	e.committedData = make(map[int]map[int][]Transaction)
	e.FinalPrePrepareMsgLog = make(map[string]PrePrepareMsg)
	e.FinalPrepareMsgLog = make(map[int]map[int]map[string][]PrepareMsgData)
	e.FinalcommitMsgLog = make(map[int]map[int]map[string][]CommitMsgData)
	e.FinalpreparedData = make(map[int]map[int][]Transaction)
	e.FinalcommittedData = make(map[int]map[int][]Transaction)
}

func (e *Elastico) reset() {
	/*
		reset some of the elastico class members
	*/
	e.getIP()
	e.getKey()

	// channel = e.connection.channel()
	// # to delete the queue in rabbitmq for next epoch
	// channel.queue_delete(queue='hello' + str(e.Port))
	// channel.close()

	// e.Port = e.getPort()

	e.PoW = make(map[string]interface{})
	e.PoW["hash"] = ""
	e.PoW["setOfRs"] = ""
	e.PoW["nonce"] = 0

	e.curDirectory = make([]IDENTITY, 0)
	// only when this node is the member of directory committee
	e.committeeList = make(map[int64][]IDENTITY)
	// only when this node is not the member of directory committee
	e.committeeMembers = make([]IDENTITY, 0)

	e.Identity = IDENTITY{}
	e.CommitteeID = -1
	e.Ri = ""

	e.isDirectory = false
	e.isFinal = false

	// only when this node is the member of final committee
	e.commitments = make(map[string]bool)
	e.txnBlock = make([]Transaction, 0)
	e.setOfRs = e.newsetOfRs
	e.newsetOfRs = make(map[string]bool)
	e.CommitteeConsensusData = make(map[int64]map[string][]string)
	e.CommitteeConsensusDataTxns = make(map[int64]map[string][]Transaction)
	e.finalBlockbyFinalCommittee = make(map[string][]IdentityAndSign)
	e.finalBlockbyFinalCommitteeTxns = make(map[string][]Transaction)
	e.state = ElasticoStates["NONE"]
	e.mergedBlock = make([]Transaction, 0)

	e.finalBlock = make(map[string]interface{})
	e.finalBlock["sent"] = false
	e.finalBlock["finalBlock"] = make([]Transaction, 0)

	e.RcommitmentSet = e.newRcommitmentSet
	e.newRcommitmentSet = make(map[string]bool)
	e.finalCommitteeMembers = make([]IDENTITY, 0)

	// only when this is the member of the directory committee
	e.txn = make(map[int64][]Transaction)
	e.response = make([]Transaction, 0)
	e.flag = true
	e.views = make(map[int]bool)
	e.primary = false
	e.viewID = 0
	e.faulty = false

	e.prePrepareMsgLog = make(map[string]PrePrepareMsg)
	e.prepareMsgLog = make(map[int]map[int]map[string][]PrepareMsgData)
	e.commitMsgLog = make(map[int]map[int]map[string][]CommitMsgData)
	e.preparedData = make(map[int]map[int][]Transaction)
	e.committedData = make(map[int]map[int][]Transaction)
	e.FinalPrePrepareMsgLog = make(map[string]PrePrepareMsg)
	e.FinalPrepareMsgLog = make(map[int]map[int]map[string][]PrepareMsgData)
	e.FinalcommitMsgLog = make(map[int]map[int]map[string][]CommitMsgData)
	e.FinalpreparedData = make(map[int]map[int][]Transaction)
	e.FinalcommittedData = make(map[int]map[int][]Transaction)
}

func (e *Elastico) getCommitteeid(PoW string) int64 {
	/*
		returns last s-bit of PoW["hash"] as Identity : CommitteeID
	*/
	bindigest := ""

	for i := 0; i < len(PoW); i++ {
		intVal, err := strconv.ParseInt(string(PoW[i]), 16, 0) // converts hex string to integer
		failOnError(err, "string to int conversion error", true)
		bindigest += fmt.Sprintf("%04b", intVal) // converts intVal to 4 bit binary value
	}
	// take last s bits of the binary digest
	Identity := bindigest[len(bindigest)-s:]
	iden, err := strconv.ParseInt(Identity, 2, 0) // converts binary string to integer
	failOnError(err, "binary to int conversion error", true)
	return iden
}

func (e *Elastico) executePoW() {
	/*
		execute PoW
	*/
	if e.flag {
		// compute Pow for good node
		e.computePoW()
	} else {
		// compute Pow for bad node
		e.computeFakePoW()
	}
}

// SendtoFinal :- Each committee member sends the signed value(txn block after intra committee consensus along with signatures to final committee
func (e *Elastico) SendtoFinal() {
	// PK := e.key.Public() // public key
	// rsaPublickey := PK.(*rsa.PublicKey) //converted to rsa public key object
	for viewID := range e.committedData {
		committedDataViewID := e.committedData[viewID]
		for seqnum := range committedDataViewID {

			msgList := committedDataViewID[seqnum]

			e.txnBlock = e.unionTxns(e.txnBlock, msgList)
		}
	}
	log.Warn("size of committee members", len(e.finalCommitteeMembers))
	log.Warn("send to final---txns", e.CommitteeID, e.Port, e.txnBlock)
	for _, finalID := range e.finalCommitteeMembers {

		//  here txnBlock is a set, since sets are unordered hence can't sign them. So convert set to list for signing
		txnBlock := e.txnBlock
		data := map[string]interface{}{"Txnblock": txnBlock, "Sign": e.signTxnList(txnBlock), "Identity": e.Identity}
		msg := map[string]interface{}{"data": data, "type": "intraCommitteeBlock"}
		finalID.send(msg)
	}
	e.state = ElasticoStates["Intra Consensus Result Sent to Final"]
}

type IntraBlockMsg struct {
	Txnblock []Transaction
	Sign     string
	Identity IDENTITY
}

func (e *Elastico) isFinalMember() bool {
	/*
		tell whether this node is a final committee member or not
	*/
	return e.isFinal
}

func (e *Elastico) runPBFT() {
	/*
		Runs a Pbft instance for the intra-committee consensus
	*/
	if e.state == ElasticoStates["PBFT_NONE"] {
		if e.primary {
			prePrepareMsg := e.constructPrePrepare() //construct pre-prepare msg
			// multicasts the pre-prepare msg to replicas
			// ToDo: what if primary does not send the pre-prepare to one of the nodes
			e.sendPrePrepare(prePrepareMsg)

			// change the state of primary to pre-prepared
			e.state = ElasticoStates["PBFT_PRE_PREPARE_SENT"]
			// primary will log the pre-prepare msg for itself
			prePrepareMsgEncoded, _ := json.Marshal(prePrepareMsg["data"])

			var decodedMsg PrePrepareMsg
			err := json.Unmarshal(prePrepareMsgEncoded, &decodedMsg)
			failOnError(err, "error in unmarshal pre-prepare", true)
			e.logPrePrepareMsg(decodedMsg)

		} else {

			// for non-primary members
			if e.isPrePrepared() {
				e.state = ElasticoStates["PBFT_PRE_PREPARE"]
			}
		}

	} else if e.state == ElasticoStates["PBFT_PRE_PREPARE"] {

		if e.primary == false {

			// construct prepare msg
			// ToDo: verify whether the pre-prepare msg comes from various primaries or not
			preparemsgList := e.constructPrepare()
			// logging.warning("constructing prepares with Port %s" , str(e.Port))
			e.sendPrepare(preparemsgList)
			e.state = ElasticoStates["PBFT_PREPARE_SENT"]
		}

	} else if e.state == ElasticoStates["PBFT_PREPARE_SENT"] || e.state == ElasticoStates["PBFT_PRE_PREPARE_SENT"] {
		// ToDo: if, primary has not changed its state to "PBFT_PREPARE_SENT"
		if e.isPrepared() {

			// logging.warning("prepared done by %s" , str(e.Port))
			e.state = ElasticoStates["PBFT_PREPARED"]
		}

	} else if e.state == ElasticoStates["PBFT_PREPARED"] {

		commitMsgList := e.constructCommit()
		e.sendCommit(commitMsgList)
		e.state = ElasticoStates["PBFT_COMMIT_SENT"]

	} else if e.state == ElasticoStates["PBFT_COMMIT_SENT"] {

		if e.isCommitted() {

			// logging.warning("committed done by %s" , str(e.Port))
			e.state = ElasticoStates["PBFT_COMMITTED"]
		}
	}

}

func (e *Elastico) isPrepared() bool {
	/*
		Check if the state is prepared or not
	*/
	// collect prepared data
	preparedData := make(map[int]map[int][]Transaction)
	f := (c - 1) / 3
	// check for received request messages
	for socket := range e.prePrepareMsgLog {

		// In current View Id
		socketMap := e.prePrepareMsgLog[socket]
		prePrepareData := socketMap.PrePrepareData
		if prePrepareData.ViewID == e.viewID {

			// request msg of pre-prepare request
			requestMsg := socketMap.Message
			// digest of the message
			digest := prePrepareData.Digest
			// get sequence number of this msg
			seqnum := prePrepareData.Seq
			// find Prepare msgs for this view and sequence number
			_, ok := e.prepareMsgLog[e.viewID]

			if ok == true {
				prepareMsgLogViewID := e.prepareMsgLog[e.viewID]
				_, okk := prepareMsgLogViewID[seqnum]
				if okk == true {
					// need to find matching prepare msgs from different replicas atleast c/2 + 1
					count := 0
					prepareMsgLogSeq := prepareMsgLogViewID[seqnum]
					for replicaId := range prepareMsgLogSeq {
						prepareMsgLogReplica := prepareMsgLogSeq[replicaId]
						for _, msg := range prepareMsgLogReplica {
							checkdigest := msg.Digest
							if checkdigest == digest {
								count += 1
								break
							}
						}
						// condition for Prepared state
						if count >= 2*f{

							if _ , ok := preparedData[e.viewID] ; ok == false{

								preparedData[e.viewID] = make(map[int]interface{})
							}
							preparedViewId := preparedData[e.viewID].(map[int]interface{})
							if  _ , ok := preparedViewId[seqnum] ; ok == false{

								preparedViewId[seqnum] = make([]Transaction,0)
							}
							preparedViewId[seqnum] = append(preparedViewId[seqnum], requestMsg)
						}

					}
				}

			}
		}
		if len(preparedData) > 0{

			e.preparedData = preparedData
			return true
		}
	*/
	return false
}

func (e *Elastico) runFinalPBFT() {
	/*
		Run PBFT by final committee members
	*/
	if e.state == ElasticoStates["FinalPBFT_NONE"] {

		if e.primary {

			// construct pre-prepare msg
			finalPrePreparemsg := e.constructFinalPrePrepare()
			// multicasts the pre-prepare msg to replicas
			e.sendPrePrepare(finalPrePreparemsg)

			// change the state of primary to pre-prepared
			e.state = ElasticoStates["FinalPBFT_PRE_PREPARE_SENT"]
			// primary will log the pre-prepare msg for itself
			e.logFinalPrePrepareMsg(finalPrePreparemsg)

		} else {

			// for non-primary members
			if e.isFinalprePrepared() {
				e.state = ElasticoStates["FinalPBFT_PRE_PREPARE"]
			}
		}

	} else if e.state == ElasticoStates["FinalPBFT_PRE_PREPARE"] {

		if e.primary == false {

			// construct prepare msg
			FinalpreparemsgList := e.constructFinalPrepare()
			e.sendPrepare(FinalpreparemsgList)
			e.state = ElasticoStates["FinalPBFT_PREPARE_SENT"]
		}
	} else if e.state == ElasticoStates["FinalPBFT_PREPARE_SENT"] || e.state == ElasticoStates["FinalPBFT_PRE_PREPARE_SENT"] {

		// ToDo: primary has not changed its state to "FinalPBFT_PREPARE_SENT"
		if e.isFinalPrepared() {

			e.state = ElasticoStates["FinalPBFT_PREPARED"]
		}
	} else if e.state == ElasticoStates["FinalPBFT_PREPARED"] {

		commitMsgList := e.constructFinalCommit()
		e.sendCommit(commitMsgList)
		e.state = ElasticoStates["FinalPBFT_COMMIT_SENT"]

	} else if e.state == ElasticoStates["FinalPBFT_COMMIT_SENT"] {

		if e.isFinalCommitted() {

			// for viewID in e.FinalcommittedData:
			// 	for seqnum in e.FinalcommittedData[viewID]:
			// 		msgList = e.FinalcommittedData[viewID][seqnum]
			// 		for msg in msgList:
			// 			e.finalBlock["finalBlock"] = e.unionTxns(e.finalBlock["finalBlock"], msg)
			// finalTxnBlock = e.finalBlock["finalBlock"]
			// finalTxnBlock = list(finalTxnBlock)
			// # order them! Reason : to avoid errors in signatures as sets are unordered
			// # e.finalBlock["finalBlock"] = sorted(finalTxnBlock)
			// logging.warning("final block by Port %s with final block %s" , str(e.Port), str(e.finalBlock["finalBlock"]))
			e.state = ElasticoStates["FinalPBFT_COMMITTED"]
		}
	}
}

type PrePrepareContents struct {
	Type   string
	ViewID int
	Seq    int
	Digest string
}
type PrePrepareMsg struct {
	Message        []Transaction
	PrePrepareData PrePrepareContents
	Sign           string
	Identity       IDENTITY
}

func (e *Elastico) constructPrePrepare() map[string]interface{} {
	/*
		construct pre-prepare msg , done by primary
	*/
	txnBlockList := e.txnBlock
	// ToDo: make prePrepareContents Ordered Dict for signatures purpose
	prePrepareContents := map[string]interface{}{"type": "pre-prepare", "viewId": e.viewID, "seq": 1, "digest": txnHexdigest(txnBlockList)}
	prePrepareContentsDigest := e.digestPrePrepareMsg(prePrepareContents)
	prePrepareMsg := map[string]interface{}{"type": "pre-prepare", "message": txnBlockList, "pre-prepareData": prePrepareContents, "sign": e.Sign(prePrepareContentsDigest), "Identity": e.Identity}
	return prePrepareMsg
}

type PrepareContents struct {
	Type   string
	ViewID int
	Seq    int
	Digest string
}

type PrepareMsg struct {
	PrepareData PrepareContents
	Sign        string
	Identity    IDENTITY
}

func (e *Elastico) constructPrepare() []map[string]interface{} {
	/*
		construct prepare msg in the prepare phase
	*/
	prepareMsgList := make([]map[string]interface{}, 0)
	//  loop over all pre-prepare msgs
	for socketID := range e.prePrepareMsgLog {

		msg := e.prePrepareMsgLog[socketID].(map[string]interface{})
		prePreparedData := msg["pre-prepareData"].(map[string]interface{})
		seqnum := prePreparedData["seq"].(int)
		digest := prePreparedData["digest"].(string)
		//  make prepare_contents Ordered Dict for signatures purpose
		prepareContents := map[string]interface{}{"type": "prepare", "viewId": e.viewID, "seq": seqnum, "digest": digest}
		PrepareContentsDigest := e.digestPrepareMsg(prepareContents)
		preparemsg := map[string]interface{}{"type": "prepare", "prepareData": prepareContents, "sign": e.Sign(PrepareContentsDigest), "Identity": e.Identity}
		prepareMsgList = append(prepareMsgList, preparemsg)
	}
	return prepareMsgList

}

func (e *Elastico) constructFinalPrepare() []map[string]interface{} {
	/*
		construct prepare msg in the prepare phase
	*/
	FinalprepareMsgList := make([]map[string]interface{}, 0)
	for socketID := range e.FinalPrePrepareMsgLog {

		msg := e.FinalPrePrepareMsgLog[socketID].(map[string]interface{})
		prePreparedData := msg["pre-prepareData"].(map[string]interface{})
		seqnum := prePreparedData["seq"].(int)
		digest := prePreparedData["digest"].(string)
		//  make prepare_contents Ordered Dict for signatures purpose

		prepareContents := map[string]interface{}{"type": "Finalprepare", "viewId": e.viewID, "seq": seqnum, "digest": digest}
		PrepareContentsDigest := e.digestPrepareMsg(prepareContents)
		prepareMsg := map[string]interface{}{"type": "Finalprepare", "prepareData": prepareContents, "sign": e.Sign(PrepareContentsDigest), "Identity": e.Identity}
		FinalprepareMsgList = append(FinalprepareMsgList, prepareMsg)
	}
	return FinalprepareMsgList
}

func (e *Elastico) constructCommit() []map[string]interface{} {
	/*
		Construct commit msgs
	*/
	commitMsges := make([]map[string]interface{}, 0)
	/*
		 for viewId := range e.preparedData{

			 for seqnum := range e.preparedData[viewId] {

				 for msg :=  range e.preparedData[viewId][seqnum] {

					 digest := txnHexdigest(msg)
					// make commit_contents Ordered Dict for signatures purpose
					 commitContents := map[string]interface{}{"type" : "commit" , "viewId" : viewId , "seq" : seqnum , "digest": digest }
					 commitContentsDigest := e.digestCommitMsg(commitContents)
					 commitMsg := map[string]interface{}{"type" : "commit" , "sign" : e.Sign(commitContentsDigest) , "commitData" : commitContents, "Identity" : e.Identity}
					 commitMsges = append(commitMsges , commitMsg)
				 }
			 }
		 }
	*/
	return commitMsges
}

func (e *Elastico) constructFinalCommit() []map[string]interface{} {
	/*
		Construct commit msgs
	*/
	commitMsges := make([]map[string]interface{}, 0)
	/*
		 for viewId := range e.FinalpreparedData{

			 for seqnum := range e.FinalpreparedData[viewId]{

				 for msg := range e.FinalpreparedData[viewId][seqnum]{

					 digest := txnHexdigest(msg)
					//  make commit_contents Ordered Dict for signatures purpose
					 commitContents := map[string]interface{}{"type" : "Finalcommit" , "viewId" : viewId , "seq" : seqnum , "digest":digest }
					 commitContentsDigest := e.digestCommitments(commitContents)
					 commitMsg := map[string]interface{}{"type" : "Finalcommit" , "sign" : e.Sign(commitContentsDigest) , "commitData" : commitContents, "Identity" : e.Identity}
					 commitMsges = append(commitMsges, commitMsg)
				 }
			 }
		 }
	*/
	return commitMsges
}

func (e *Elastico) digestPrePrepareMsg(msg PrePrepareContents) []byte {
	digest := sha256.New()
	digest.Write([]byte(msg.Type))
	digest.Write([]byte(strconv.Itoa(msg.ViewID)))
	digest.Write([]byte(strconv.Itoa(msg.Seq)))
	digest.Write([]byte(msg.Digest))
	return digest.Sum(nil)
}

func (e *Elastico) digestPrepareMsg(msg map[string]interface{}) []byte {
	digest := sha256.New()
	digest.Write([]byte(msg.Type))
	digest.Write([]byte(strconv.Itoa(msg.ViewID)))
	digest.Write([]byte(strconv.Itoa(msg.Seq)))
	digest.Write([]byte(msg.Digest))
	return digest.Sum(nil)
}

func (e *Elastico) digestCommitMsg(msg map[string]interface{}) []byte {
	digest := sha256.New()
	digest.Write([]byte("type"))
	digest.Write([]byte(msg["type"].(string)))
	digest.Write([]byte("viewId"))
	digest.Write([]byte(strconv.Itoa(msg["viewId"].(int))))
	digest.Write([]byte("seq"))
	digest.Write([]byte(strconv.Itoa(msg["seq"].(int))))
	digest.Write([]byte("digest"))
	digest.Write([]byte(msg["digest"].(string)))
	return digest.Sum(nil)
}

func (e *Elastico) constructFinalPrePrepare() map[string]interface{} {
	/*
		construct pre-prepare msg , done by primary final
	*/
	txnBlockList := e.mergedBlock
	// ToDo :- make pre_prepare_contents Ordered Dict for signatures purpose
	prePrepareContents := map[string]interface{}{"type": "Finalpre-prepare", "viewId": e.viewID, "seq": 1, "digest": txnHexdigest(txnBlockList)}

	prePrepareContentsDigest := e.digestPrePrepareMsg(prePrepareContents)
	prePrepareMsg := map[string]interface{}{"type": "Finalpre-prepare", "message": txnBlockList, "pre-prepareData": prePrepareContents, "sign": e.Sign(prePrepareContentsDigest), "Identity": e.Identity}
	return prePrepareMsg
}

func (e *Elastico) sendPrePrepare(prePrepareMsg map[string]interface{}) {
	/*
		Send pre-prepare msgs to all committee members
	*/
	for _, nodeID := range e.committeeMembers {

		// dont send pre-prepare msg to self
		if e.Identity.isEqual(&nodeID) == false {

			nodeID.send(prePrepareMsg)
		}
	}
}

func (e *Elastico) sendCommit(commitMsgList []map[string]interface{}) {
	/*
		send the commit msgs to the committee members
	*/
	for _, commitMsg := range commitMsgList {

		for _, nodeID := range e.committeeMembers {

			nodeID.send(commitMsg)
		}
	}
}

func (e *Elastico) sendPrepare(prepareMsgList []map[string]interface{}) {
	/*
		send the prepare msgs to the committee members
	*/
	// send prepare msg list to committee members
	for _, preparemsg := range prepareMsgList {

		for _, nodeID := range e.committeeMembers {

			nodeID.send(preparemsg)
		}
	}
}

func (e *Elastico) checkCountForFinalData() {
	/*
		check the sufficient counts for final data
	*/
	//  collect final blocks sent by final committee and add the blocks to the response
	/*
		 for txnBlockDigest := range e.finalBlockbyFinalCommittee{

			 if len(e.finalBlockbyFinalCommittee[txnBlockDigest]) >= c/2 + 1{

				 TxnList := e.finalBlockbyFinalCommitteeTxns[txnBlockDigest]
				//  create the final committed block that contatins the txnlist and set of signatures and identities to that txn list
				 finalCommittedBlock := FinalCommittedBlock(TxnList, e.finalBlockbyFinalCommittee[txnBlockDigest])
				//  add the block to the response
				 e.response = append(e.response , finalCommittedBlock)

			 } else {

				 log.Error("less block signs : ", len(e.finalBlockbyFinalCommittee[txnBlockDigest]))
			 }
		 }
	*/
	if len(e.response) > 0 {

		log.Warn("final block sent the block to client by", e.Port)
		e.state = ElasticoStates["FinalBlockSentToClient"]
	}
}

func (e *Elastico) verifyAndMergeConsensusData() {
	/*
		each final committee member validates that the values received from the committees are signed by
		atleast c/2 + 1 members of the proper committee and takes the ordered set union of all the inputs

	*/
	// logging.warning("verify and merge %s -- %s" , str(self.Port) ,str(self.committee_id))
	// for committeeid in range(pow(2,s)):
	// 	if committeeid in self.CommitteeConsensusData:
	// 		for txnBlockDigest in self.CommitteeConsensusData[committeeid]:
	// 			if len(self.CommitteeConsensusData[committeeid][txnBlockDigest]) >= c//2 + 1:
	// 				# get the txns from the digest
	// 				txnBlock = self.CommitteeConsensusDataTxns[committeeid][txnBlockDigest]
	// 				if len(txnBlock) > 0:
	// 					# set_of_txns = ast.literal_eval(txnBlockDigest)
	// 					# self.mergedBlock.extend(set_of_txns)
	// 					self.mergedBlock = self.unionTxns(self.mergedBlock, txnBlock)
	// if len(self.mergedBlock) > 0:
	// 	self.state = ELASTICO_STATES["Merged Consensus Data"]
	// 	logging.warning("%s - Port , %s - mergedBlock" ,str(self.Port) ,  str(self.mergedBlock))
}

func (e *Elastico) logCommitMsg(msg map[string]interface{}) {
	/*
	 log the commit msg
	*/
	// viewId = msg["commitData"]["viewId"]
	// seqnum = msg["commitData"]["seq"]
	// socketId = msg["Identity"].IP +  ":" + str(msg["Identity"].Port)
	// # add msgs for this view
	// if viewId not in self.commitMsgLog:
	// 	self.commitMsgLog[viewId] = dict()

	// # add msgs for this sequence num
	// if seqnum not in self.commitMsgLog[viewId]:
	// 	self.commitMsgLog[viewId][seqnum] = dict()

	// # add all msgs from this sender
	// if socketId not in self.commitMsgLog[viewId][seqnum]:
	// 	self.commitMsgLog[viewId][seqnum][socketId] = list()

	// # log only required details from the commit msg
	// msgDetails = {"digest" : msg["commitData"]["digest"], "Identity" : msg["Identity"]}
	// # append msg
	// logging.warning("Log committed msg for view %s, seqnum %s", str(viewId), str(seqnum))
	// self.commitMsgLog[viewId][seqnum][socketId].append(msgDetails)
}

//Sign :- sign the byte array
func (e *Elastico) Sign(digest []byte) string {
	signed, err := rsa.SignPKCS1v15(rand.Reader, e.key, crypto.SHA256, digest) // sign the digest
	failOnError(err, "Error in Signing byte array", true)
	signature := base64.StdEncoding.EncodeToString(signed) // encode to base64
	return signature
}

type HashMsg struct {
	Identity IDENTITY
	HashRi   string
}

func (e *Elastico) sendCommitment() {
	/*
		send the H(Ri) to the final committe members.This is done by a final committee member
	*/
	if e.isFinalMember() == true {

		HashRi := e.getCommitment()
		for _, nodeID := range e.committeeMembers {

			log.Warn("sent the commitment by", e.Port)
			data := map[string]interface{}{"Identity": e.Identity, "HashRi": HashRi}
			msg := map[string]interface{}{"data": data, "type": "hash"}
			nodeID.send(msg)
		}
		e.state = ElasticoStates["CommitmentSentToFinal"]
	}
}

func (e *Elastico) computeFakePoW() {
	/*
		bad node generates the fake PoW
	*/
	// random fakeness
	/*
		x := randomGen(32)
		_, indexi := x.DivMod(x, big.NewInt(3), big.NewInt(0))
		index := indexi.Int64()
		if index == 0 {

			// Random hash with initial D hex digits 0s
			digest := sha256.New()
			ranHash := fmt.Sprintf("%x", digest.Sum(nil))
			hashVal := ""
			for i := 0; i < D; i++ {
				hashVal += "0"
			}
			e.PoW["hash"] = hashVal + ranHash[D:]

		} else if index == 1 {

			// computing an invalid PoW using less number of values in digest
			randomsetR := set()
			zeroString = ""
			for i := 0; i < D; i++ {
				zeroString += "0"
			}
			// if len(e.setOfRs) > 0:
			E/ 	e.EpochRandomness, randomsetR = e.xor_R()
			for {

				digest := sha256.New()
				nonce := e.PoW["nonce"]
				digest.Write([]byte(strconv.Itoa(nonce)))
				hashVal := fmt.Sprintf("%x", digest.Sum(nil))
				if strings.HasPrefix(hashVal, zeroString) {
					e.PoW["hash"] = hashVal
					e.PoW["setOfRs"] = randomsetR
					e.PoW["nonce"] = nonce
				} else {
					// try for other nonce
					nonce++
					e.PoW["nonce"] = nonce
				}
			}
		} else if index == 2 {

			// computing a random PoW
			randomsetR := set()
			// if len(e.setOfRs) > 0:
			E/ 	e.EpochRandomness, randomsetR := e.xor_R()
			digest := sha256.New()
			ranHash := fmt.Sprintf("%x", digest.Sum(nil))
			nonce := randomGen()
			e.PoW["hash"] = ranHash
			e.PoW["setOfRs"] = randomsetR
			// ToDo: nonce has to be in int instead of big.Int
			e.PoW["nonce"] = nonce
		}

		log.Warn("computed fake POW ", index)
		e.state = ElasticoStates["PoW Computed"]
	*/
}

func (e *Elastico) isFinalPrepared() bool {
	/*
		Check if the state is prepared or not
	*/
	// //  collect prepared data
	// preparedData = dict()
	// f = (c - 1)//3
	// //  check for received request messages
	// for socket in self.Finalpre_prepareMsgLog:
	// 	//  In current View Id
	// 	if self.Finalpre_prepareMsgLog[socket]["pre-prepareData"]["viewId"] == self.viewId:
	// 		//  request msg of pre-prepare request
	// 		requestMsg = self.Finalpre_prepareMsgLog[socket]["message"]
	// 		//  digest of the message
	// 		digest = self.Finalpre_prepareMsgLog[socket]["pre-prepareData"]["digest"]
	// 		//  get sequence number of this msg
	// 		seqnum = self.Finalpre_prepareMsgLog[socket]["pre-prepareData"]["seq"]
	// 		//  find Prepare msgs for this view and sequence number
	// 		if self.viewId in self.FinalprepareMsgLog and seqnum in self.FinalprepareMsgLog[self.viewId]:
	// 			//  need to find matching prepare msgs from different replicas atleast c//2 + 1
	// 			count = 0
	// 			for replicaId in self.FinalprepareMsgLog[self.viewId][seqnum]:
	// 				for msg in self.FinalprepareMsgLog[self.viewId][seqnum][replicaId]:
	// 					if msg["digest"] == digest:
	// 						count += 1
	// 						break
	// 			//  condition for Prepared state
	// 			if count >= 2*f:

	// 				if self.viewId not in preparedData:
	// 					preparedData[self.viewId] = dict()
	// 				if seqnum not in preparedData[self.viewId]:
	// 					preparedData[self.viewId][seqnum] = list()
	// 				preparedData[self.viewId][seqnum].append(requestMsg)
	// if len(preparedData) > 0:
	// 	self.FinalpreparedData = preparedData
	// 	return True
	// return False
	return true
}

func (e *Elastico) isCommitted() bool {
	/*
		Check if the state is committed or not
	*/
	// # collect committed data
	// committedData = dict()
	// f = (c - 1)//3
	// # check for received request messages
	// for socket in self.prePrepareMsgLog:
	// 	# In current View Id
	// 	if self.prePrepareMsgLog[socket]["pre-prepareData"]["viewId"] == self.viewId:
	// 		# request msg of pre-prepare request
	// 		requestMsg = self.prePrepareMsgLog[socket]["message"]
	// 		# digest of the message
	// 		digest = self.prePrepareMsgLog[socket]["pre-prepareData"]["digest"]
	// 		# get sequence number of this msg
	// 		seqnum = self.prePrepareMsgLog[socket]["pre-prepareData"]["seq"]
	// 		if self.viewId in self.preparedData and seqnum in self.preparedData[self.viewId]:
	// 			for prepareMsg in self.preparedData[self.viewId][seqnum]:
	// 				if txnHexdigest(prepareMsg) == digest:
	// 					# pre-prepared matched and prepared is also true, check for commits
	// 					if self.viewId in self.commitMsgLog:
	// 						if seqnum in self.commitMsgLog[self.viewId]:
	// 							count = 0
	// 							logging.warning("CHECK FOR COUNT IN COMMITTED BY PORT %s" , str(self.Port))
	// 							for replicaId in self.commitMsgLog[self.viewId][seqnum]:
	// 								for msg in self.commitMsgLog[self.viewId][seqnum][replicaId]:
	// 									if msg["digest"] == digest:
	// 										count += 1
	// 										break
	// 							# ToDo: condition check
	// 							if count >= 2*f + 1:
	// 								if self.viewId not in committedData:
	// 									committedData[self.viewId] = dict()
	// 								if seqnum not in committedData[self.viewId]:
	// 									committedData[self.viewId][seqnum] = list()
	// 								committedData[self.viewId][seqnum].append(requestMsg)
	// 						else:
	// 							logging.error("no seqnum found in commit msg log")
	// 					else:
	// 						logging.error("no view id found in commit msg log")
	// 				else:
	// 					logging.error("wrong digest in is committed")
	// 		else:
	// 			logging.error("view and seqnum not found in isCommitted")
	// 	else:
	// 		logging.error("wrong view in is committed")
	// if len(committedData) > 0:
	// 	e.committedData = committedData
	// 	return True
	// return False
	return true
}

func (e *Elastico) isFinalCommitted() bool {
	/*
		Check if the state is committed or not
	*/
	// // collect committed data
	// committedData = dict()
	// f = (c - 1)//3
	// // check for received request messages
	// for socket in self.Finalpre_prepareMsgLog:
	// 	// In current View Id
	// 	if self.Finalpre_prepareMsgLog[socket]["pre-prepareData"]["viewId"] == self.viewId:
	// 		// request msg of pre-prepare request
	// 		requestMsg = self.Finalpre_prepareMsgLog[socket]["message"]
	// 		// digest of the message
	// 		digest = self.Finalpre_prepareMsgLog[socket]["pre-prepareData"]["digest"]
	// 		// get sequence number of this msg
	// 		seqnum = self.Finalpre_prepareMsgLog[socket]["pre-prepareData"]["seq"]
	// 		if self.viewId in self.FinalpreparedData and seqnum in self.FinalpreparedData[self.viewId]:
	// 			for prepareMsg in self.FinalpreparedData[self.viewId][seqnum]:
	// 				if txnHexdigest(prepareMsg) == digest:
	// 					// pre-prepared matched and prepared is also true, check for commits
	// 					if self.viewId in self.FinalcommitMsgLog:
	// 						if seqnum in self.FinalcommitMsgLog[self.viewId]:
	// 							count = 0
	// 							logging.warning("CHECK FOR COUNT IN COMMITTED BY PORT %s" , str(self.Port))
	// 							for replicaId in self.FinalcommitMsgLog[self.viewId][seqnum]:
	// 								for msg in self.FinalcommitMsgLog[self.viewId][seqnum][replicaId]:
	// 									if msg["digest"] == digest:
	// 										count += 1
	// 										break
	// 							// ToDo: condition check
	// 							if count >= 2*f + 1:
	// 								if self.viewId not in committedData:
	// 									committedData[self.viewId] = dict()
	// 								if seqnum not in committedData[self.viewId]:
	// 									committedData[self.viewId][seqnum] = list()
	// 								committedData[self.viewId][seqnum].append(requestMsg)
	// 						else:
	// 							logging.error("no seqnum found in commit msg log")
	// 					else:
	// 						logging.error("no view id found in commit msg log")
	// 				else:
	// 					logging.error("wrong digest in is committed")
	// 		else:
	// 			logging.error("view and seqnum not found in isCommitted")
	// 	else:
	// 		logging.error("wrong view in is committed")
	// if len(committedData) > 0:
	// 	self.FinalcommittedData = committedData
	// 	return True
	// return False
	return true
}

func (e *Elastico) logFinalCommitMsg(msg map[string]interface{}) {
	/*
		log the final commit msg
	*/
	/*
		commitData := msg["commitData"].(map[string]interface{})
		viewID := commitData["viewId"].(int)
		seqnum := commitData["seq"].(int)
		identityobj := msg["Identity"].(IDENTITY)
		socketId = msg["Identity"].IP + ":" + strconv.Itoa(msg["Identity"].Port)
		// add msgs for this view
		_, ok := e.FinalcommitMsgLog[viewId]
		if ok == false {

			e.FinalcommitMsgLog[viewId] = dict()
		}

		// add msgs for this sequence num
		_, okk := e.FinalcommitMsgLog[viewId][seqnum]
		if okk == false {

			e.FinalcommitMsgLog[viewId][seqnum] = dict()
		}

		// add all msgs from this sender
		_, okkk := e.FinalcommitMsgLog[viewId][seqnum][socketId]
		if okkk == false {

			e.FinalcommitMsgLog[viewId][seqnum][socketId] = list()
		}

		// log only required details from the commit msg
		identityobj := msg["Identity"].(IDENTITY)
		msgDetails := map[string]interface{}{"digest": commitData["digest"], "Identity": identityobj}
		// append msg
		log.Warn("Log committed msg for view , seqnum ", viewId, seqnum)
		e.FinalcommitMsgLog[viewId][seqnum][socketId].append(msgDetails)
	*/
}

func (e *Elastico) formIdentity() {
	/*
		Identity formation for a node
		Identity consists of public key, ip, committee id, PoW, nonce, epoch randomness
	*/
	if e.state == ElasticoStates["PoW Computed"] {

		PK := e.key.PublicKey

		// set the committee id acc to PoW solution
		e.CommitteeID = e.getCommitteeid(e.PoW["hash"].(string))

		e.Identity = IDENTITY{IP: e.IP, PK: PK, CommitteeID: e.CommitteeID, PoW: e.PoW, EpochRandomness: e.EpochRandomness, Port: e.Port}
		// changed the state after Identity formation
		e.state = ElasticoStates["Formed Identity"]
	}
}

func (e *Elastico) appendToLedger() {
	/*
		append the response to the ledger
	*/
	// lock.acquire()
	// if len(self.response) >= 1:
	// 	finalCommittedBlock = self.response[0]
	// 	# extracting transactions from the final committed block
	// 	transactions = finalCommittedBlock.txnList

	// 	# create a merkle tree of the transactions
	// 	merkleTree = self.createMerkleTree(transactions)
	// 	# get the transactions count
	// 	txnCount = len(transactions)

	// 	# For the genesis block
	// 	prevBlockHash = ""
	// 	signUpdated = False
	// 	if len(ledger) > 0:
	// 		LastBlock = ledger[-1]
	// 		if LastBlock.getRootHash() == merkleTree.Get_Root_leaf():
	// 			# ToDo: Add signs here
	// 			LastBlock.addSignAndIdentities(finalCommittedBlock.listSignaturesAndIdentityobjs)
	// 			signUpdated = True
	// 		else:
	// 			prevBlockHash = LastBlock.hexdigest()
	// 	if signUpdated == False:
	// 		newBlock = Block(transactions, prevBlockHash, time.time(), len(ledger), txnCount, merkleTree)
	// 		newBlock.addSignAndIdentities(finalCommittedBlock.listSignaturesAndIdentityobjs)
	// 		ledger.append(newBlock)
	// lock.release()

	// if len(self.response) > 1:
	// 	logging.error("Multiple Blocks coming!")

}

func (e *Elastico) verifyFinalPrepare(msg map[string]interface{}) bool {
	/*
		Verify final prepare msgs
	*/
	// verify Pow
	identityobj := msg["Identity"].(IDENTITY)
	if e.verifyPoW(identityobj) == false {

		log.Warn("wrong pow in verify final prepares")
		return false
	}
	// verify signatures of the received msg
	sign := msg["sign"].(string)
	prepareData := msg["prepareData"].(map[string]interface{})
	PrepareContentsDigest := e.digestPrepareMsg(prepareData)

	PK := identityobj.PK

	if e.verifySign(sign, PrepareContentsDigest, &PK) == false {

		log.Warn("wrong sign in verify final prepares")
		return false
	}
	viewID := prepareData["viewId"].(int)
	seq := prepareData["seq"].(int)
	digest := prepareData["digest"].(string)
	// check the view is same or not
	if viewID != e.viewID {

		log.Warn("wrong view in verify final prepares")
		return false
	}

	// verifying the digest of request msg
	for socketID := range e.FinalPrePrepareMsgLog {
		prePrepareMsg := e.FinalPrePrepareMsgLog[socketID].(map[string]interface{})
		prePrepareData := prePrepareMsg["pre-prepareData"].(map[string]interface{})
		prePrepareView := prePrepareData["viewId"].(int)
		prePrepareSeq := prePrepareData["seq"].(int)
		prePrepareDigest := prePrepareData["digest"].(string)

		if prePrepareView == viewID && prePrepareSeq == seq && prePrepareDigest == digest {

			return true
		}
	}
	return false
}

func (e *Elastico) logPrepareMsg(msg map[string]interface{}) {
	/*
		log the prepare msg
	*/
	identityobj := msg["Identity"].(IDENTITY)
	prepareData := msg["prepareData"].(map[string]interface{})
	viewID := prepareData["viewId"].(int)
	seqnum := prepareData["seq"].(int)
	socketID := identityobj.IP + ":" + strconv.Itoa(identityobj.Port)
	// add msgs for this view
	if _, ok := e.prepareMsgLog[viewID]; ok == false {

		e.prepareMsgLog[viewID] = make(map[int]interface{})
	}
	prepareMsgLogViewID := e.prepareMsgLog[viewID].(map[int]interface{})
	// add msgs for this sequence num
	if _, ok := prepareMsgLogViewID[seqnum]; ok == false {

		prepareMsgLogViewID[seqnum] = make(map[string]interface{})
	}

	// add all msgs from this sender
	prepareMsgLogSeq := prepareMsgLogViewID[seqnum].(map[string]interface{})
	if _, ok := prepareMsgLogSeq[socketID]; ok == false {

		prepareMsgLogSeq[socketID] = make([]map[string]interface{}, 0)
	}

	// ToDo: check that the msg appended is dupicate or not
	// log only required details from the prepare msg
	prepareDataDigest := prepareData["digest"].(string)
	msgDetails := map[string]interface{}{"digest": prepareDataDigest, "Identity": identityobj}
	// append msg to prepare msg log
	prepareMsgLogSocket := prepareMsgLogSeq[socketID].([]map[string]interface{})
	prepareMsgLogSocket = append(prepareMsgLogSocket, msgDetails)
	// ToDo : fix this
	// e.prepareMsgLog[viewID] = [seqnum][socketID] = prepareMsgLogSocket

}

func (e *Elastico) logFinalPrepareMsg(msg map[string]interface{}) {
	/*
		log the prepare msg
	*/
	identityobj := msg["Identity"].(IDENTITY)
	prepareData := msg["prepareData"].(map[string]interface{})
	viewID := prepareData["viewId"].(int)
	seqnum := prepareData["seq"].(int)
	socketID := identityobj.IP + ":" + strconv.Itoa(identityobj.Port)

	// add msgs for this view
	if _, ok := e.FinalPrepareMsgLog[viewID]; ok == false {

		e.FinalPrepareMsgLog[viewID] = make(map[int]interface{})
	}

	prepareMsgLogViewID := e.FinalPrepareMsgLog[viewID].(map[int]interface{})
	// add msgs for this sequence num
	if _, ok := prepareMsgLogViewID[seqnum]; ok == false {

		prepareMsgLogViewID[seqnum] = make(map[string]interface{})
	}

	// add all msgs from this sender
	prepareMsgLogSeq := prepareMsgLogViewID[seqnum].(map[string]interface{})
	if _, ok := prepareMsgLogSeq[socketID]; ok == false {

		prepareMsgLogSeq[socketID] = make([]map[string]interface{}, 0)
	}

	// ToDo: check that the msg appended is dupicate or not
	// log only required details from the prepare msg
	prepareDataDigest := prepareData["digest"].(string)
	msgDetails := map[string]interface{}{"digest": prepareDataDigest, "Identity": identityobj}
	// append msg to prepare msg log
	prepareMsgLogSocket := prepareMsgLogSeq[socketID].([]map[string]interface{})
	prepareMsgLogSocket = append(prepareMsgLogSocket, msgDetails)

}

func (e *Elastico) checkCountForConsensusData() {
	/*
		check the sufficient count for consensus data
	*/

	flag := false
	var commID int64
	for commID = 0; commID < int64(math.Pow(2, float64(s))); commID++ {

		if _, ok := e.CommitteeConsensusData[commID]; ok == false {

			flag = true
			break

		} else {

			for txnBlockDigest := range e.CommitteeConsensusData[commID] {

				if len(e.CommitteeConsensusData[commID][txnBlockDigest]) <= c/2 {
					flag = true
					log.Warn("bad committee id for intra committee block", commID)
					break
				}
			}
		}
	}
	if flag == false {

		// when sufficient number of blocks from each committee are received
		log.Warn("good going for verify and merge")
		e.verifyAndMergeConsensusData()
	}

}

func (e *Elastico) unionViews(nodeData, incomingData []IDENTITY) []IDENTITY {
	/*
		nodeData and incomingData are the set of identities
		Adding those identities of incomingData to nodeData that are not present in nodeData
	*/
	for _, data := range incomingData {

		flag := false
		for _, nodeID := range nodeData {

			// data is present already in nodeData
			if nodeID.isEqual(&data) {
				flag = true
				break
			}
		}
		if flag == false {
			// Adding the new Identity
			nodeData = append(nodeData, data)
		}
	}
	return nodeData
}

func (e *Elastico) verifyPrepare(msg map[string]interface{}) bool {
	/*
	 Verify prepare msgs
	*/
	// verify Pow
	identityobj := msg["Identity"].(IDENTITY)
	if e.verifyPoW(identityobj) == false {

		log.Warn("wrong pow in verify prepares")
		return false
	}

	// verify signatures of the received msg
	sign := msg["sign"].(string)
	prepareData := msg["prepareData"].(map[string]interface{})
	PrepareContentsDigest := e.digestPrepareMsg(prepareData)
	PK := identityobj.PK
	if e.verifySign(sign, PrepareContentsDigest, &PK) == false {

		log.Warn("wrong sign in verify prepares")
		return false
	}

	// check the view is same or not
	viewID := prepareData["viewId"].(int)
	seq := prepareData["seq"].(int)
	digest := prepareData["digest"].(string)
	if viewID != e.viewID {

		log.Warn("wrong view in verify prepares")
		return false
	}
	// verifying the digest of request msg
	for socketID := range e.prePrepareMsgLog {

		prePrepareMsg := e.prePrepareMsgLog[socketID].(map[string]interface{})
		prePrepareData := prePrepareMsg["pre-prepareData"].(map[string]interface{})
		prePrepareView := prePrepareData["viewId"].(int)
		prePrepareSeq := prePrepareData["seq"].(int)
		prePrepareDigest := prePrepareData["digest"].(string)
		if prePrepareView == viewID && prePrepareSeq == seq && prePrepareDigest == digest {

			return true
		}
	}
	return false
}

func (e *Elastico) unionTxns(actualTxns, receivedTxns []Transaction) []Transaction {
	/*
		union the transactions
	*/
	for _, transaction := range receivedTxns {

		flag := true
		for _, txn := range actualTxns {
			if txn.isEqual(transaction) {

				flag = false
				break
			}
		}
		if flag {
			actualTxns = append(actualTxns, transaction)
		}
	}
	return actualTxns
}

// Dmsg :- structure for directory msg
type Dmsg struct {
	Identity IDENTITY
}

func (e *Elastico) formCommittee() {
	/*
		creates directory committee if not yet created otherwise informs all the directory members
	*/
	if len(e.curDirectory) < c {

		e.isDirectory = true

		data1 := map[string]interface{}{"Identity": e.Identity}
		msg := map[string]interface{}{"data": data1, "type": "directoryMember"}

		BroadcastToNetwork(msg)
		// change the state as it is the directory member
		e.state = ElasticoStates["RunAsDirectory"]
	} else {

		e.SendToDirectory()
		if e.state != ElasticoStates["Receiving Committee Members"] {

			e.state = ElasticoStates["Formed Committee"]
		}
	}
}

func (e *Elastico) verifyPoW(identityobj IDENTITY) bool {
	/*
		verify the PoW of the node identityobj
	*/
	zeroString := ""
	for i := 0; i < D; i++ {

		zeroString += "0"
	}
	PoW := identityobj.PoW
	// fmt.Println(PoW)
	if len(PoW) == 0 {
		return false
	}
	hash := PoW["hash"].(string)
	// length of hash in hex

	if len(hash) != 64 {
		fmt.Println("len of hash less")
		return false
	}

	// Valid Hash has D leading '0's (in hex)
	if !strings.HasPrefix(hash, zeroString) {
		fmt.Println("zero not in prefix")
		return false
	}

	// check Digest for set of Ri strings
	// for Ri in PoW["setOfRs"]:
	// 	digest = e.hexdigest(Ri)
	// 	if digest not in e.RcommitmentSet:
	// 		return false

	// reconstruct epoch randomness
	EpochRandomness := identityobj.EpochRandomness

	listOfRs := PoW["setOfRs"].([]interface{})
	setOfRs := make([]string, len(listOfRs))
	for i := range listOfRs {
		setOfRs[i] = listOfRs[i].(string)
	}

	if len(setOfRs) > 0 {
		xorVal := xorbinary(setOfRs)
		EpochRandomness = fmt.Sprintf("%0"+strconv.FormatInt(r, 10)+"b\n", xorVal)
	}

	// recompute PoW

	// public key
	rsaPublickey := identityobj.PK
	IP := identityobj.IP
	nonce := int(PoW["nonce"].(float64))

	// 	compute the digest
	digest := sha256.New()
	digest.Write([]byte(IP))
	digest.Write(rsaPublickey.N.Bytes())
	digest.Write([]byte(strconv.Itoa(rsaPublickey.E)))
	digest.Write([]byte(EpochRandomness))
	digest.Write([]byte(strconv.Itoa(nonce)))

	hashVal := fmt.Sprintf("%x", digest.Sum(nil))
	if strings.HasPrefix(hashVal, zeroString) && hashVal == hash {
		// Found a valid Pow, If this doesn't match with PoW["hash"] then Doesnt verify!
		return true
	}
	fmt.Println("prefix not found")
	return false

}

// FinalpbftProcessMessage :- Process the messages related to Pbft!
func (e *Elastico) FinalpbftProcessMessage(msg map[string]interface{}) {

	if msg["type"] == "Finalpre-prepare" {
		e.processFinalprePrepareMsg(msg)

	} else if msg["type"] == "Finalprepare" {
		e.processFinalprepareMsg(msg)

	} else if msg["type"] == "Finalcommit" {
		e.processFinalcommitMsg(msg)
	}
}

func (e *Elastico) processCommitMsg(msg map[string]interface{}) {
	/*
		process the commit msg
	*/
	// verify the commit message
	verified := e.verifyCommit(msg)
	if verified {

		// Log the commit msgs!
		e.logCommitMsg(msg)
	}
}

func (e *Elastico) processFinalcommitMsg(msg map[string]interface{}) {
	/*
		process the final commit msg
	*/
	// verify the commit message
	verified := e.verifyCommit(msg)
	if verified {

		// Log the commit msgs!
		e.logFinalCommitMsg(msg)
	}
}

func (e *Elastico) processPrepareMsg(msg map[string]interface{}) {
	/*
		process prepare msg
	*/
	// verify the prepare message
	verified := e.verifyPrepare(msg)
	if verified {

		// Log the prepare msgs!
		e.logPrepareMsg(msg)
	}
}

func (e *Elastico) processFinalprepareMsg(msg map[string]interface{}) {
	/*
		process final prepare msg
	*/
	// verify the prepare message
	verified := e.verifyFinalPrepare(msg)
	if verified {

		// Log the prepare msgs!
		e.logFinalPrepareMsg(msg)
	}
}

func (e *Elastico) verifyPrePrepare(msg map[string]interface{}) bool {
	/*
		Verify pre-prepare msgs
	*/
	// prePrepareContents := map[string]interface{}{"type": "pre-prepare", "viewId": e.viewID, "seq": 1, "digest": txnHexdigest(txnBlockList)}
	// prePrepareContentsDigest := e.digestPrePrepareMsg(prePrepareContents)
	// prePrepareMsg := map[string]interface{}{"type": "pre-prepare", "message": txnBlockList, "pre-prepareData": prePrepareContents, "sign": e.sign(prePrepareContentsDigest), "Identity": e.Identity}

	identityobj := msg["Identity"].(IDENTITY)
	prePreparedData := msg["pre-prepareData"].(map[string]interface{})
	txnBlockList := msg["message"].([]Transaction)
	// verify Pow
	if e.verifyPoW(identityobj) == false {

		log.Warn("wrong pow in  verify pre-prepare")
		return false
	}
	// verify signatures of the received msg
	sign := msg["sign"].(string)
	prePreparedDataDigest := e.digestPrePrepareMsg(prePreparedData)
	PK := identityobj.PK
	if e.verifySign(sign, prePreparedDataDigest, &PK) == false {

		log.Warn("wrong sign in  verify pre-prepare")
		return false
	}
	// verifying the digest of request msg
	prePreparedDataTxnDigest := prePreparedData["digest"].(string)
	if txnHexdigest(txnBlockList) != prePreparedDataTxnDigest {

		log.Warn("wrong digest in  verify pre-prepare")
		return false
	}
	// check the view is same or not
	prePreparedDataView := prePreparedData["viewId"].(int)
	if prePreparedDataView != e.viewID {

		log.Warn("wrong view in  verify pre-prepare")
		return false
	}
	// check if already accepted a pre-prepare msg for view v and sequence num n with different digest
	seqnum := prePreparedData["seq"].(int)
	for socket := range e.prePrepareMsgLog {

		prePrepareMsgLogSocket := e.prePrepareMsgLog[socket].(map[string]interface{})
		prePrepareMsgLogData := prePrepareMsgLogSocket["pre-prepareData"].(map[string]interface{})
		prePrepareMsgLogView := prePrepareMsgLogData["viewId"].(int)
		prePrepareMsgLogSeq := prePrepareMsgLogData["seq"].(int)
		prePrepareMsgLogTxnDigest := prePrepareMsgLogData["digest"].(string)
		if prePrepareMsgLogView == e.viewID && prePrepareMsgLogSeq == seqnum {

			if prePreparedDataTxnDigest != prePrepareMsgLogTxnDigest {

				return false
			}
		}
	}
	// If msg is discarded then what to do
	return true
}

func (e *Elastico) verifyFinalPrePrepare(msg map[string]interface{}) bool {
	/*
		Verify final pre-prepare msgs
	*/
	// txnBlockList := e.mergedBlock
	// prePrepareContents := map[string]interface{}{"type": "Finalpre-prepare", "viewId": e.viewID, "seq": 1, "digest": txnHexdigest(txnBlockList)}

	// prePrepareMsg := map[string]interface{}{"type": "Finalpre-prepare", "message": txnBlockList, "pre-prepareData": prePrepareContents, "sign": e.sign(prePrepareContents), "Identity": e.Identity}

	identityobj := msg["Identity"].(IDENTITY)
	prePreparedData := msg["pre-prepareData"].(map[string]interface{})
	txnBlockList := msg["message"].([]Transaction)

	// verify Pow
	if e.verifyPoW(identityobj) == false {

		log.Warn("wrong pow in  verify final pre-prepare")
		return false
	}
	// verify signatures of the received msg
	sign := msg["sign"].(string)
	prePreparedDataDigest := e.digestPrePrepareMsg(prePreparedData)
	PK := identityobj.PK
	if e.verifySign(sign, prePreparedDataDigest, &PK) == false {

		log.Warn("wrong sign in  verify final pre-prepare")
		return false
	}

	// verifying the digest of request msg
	prePreparedDataTxnDigest := prePreparedData["digest"].(string)
	if txnHexdigest(txnBlockList) != prePreparedDataTxnDigest {

		log.Warn("wrong digest in  verify final pre-prepare")
		return false
	}
	// check the view is same or not
	prePreparedDataView := prePreparedData["viewId"].(int)
	if prePreparedDataView != e.viewID {

		log.Warn("wrong view in  verify final pre-prepare")
		return false
	}
	// check if already accepted a pre-prepare msg for view v and sequence num n with different digest
	seqnum := prePreparedData["seq"].(int)
	for socket := range e.FinalPrePrepareMsgLog {

		prePrepareMsgLogSocket := e.prePrepareMsgLog[socket].(map[string]interface{})
		prePrepareMsgLogData := prePrepareMsgLogSocket["pre-prepareData"].(map[string]interface{})
		prePrepareMsgLogView := prePrepareMsgLogData["viewId"].(int)
		prePrepareMsgLogSeq := prePrepareMsgLogData["seq"].(int)
		prePrepareMsgLogTxnDigest := prePrepareMsgLogData["digest"].(string)

		if prePrepareMsgLogView == e.viewID && prePrepareMsgLogSeq == seqnum {

			if prePreparedDataTxnDigest != prePrepareMsgLogTxnDigest {

				return false
			}
		}
	}
	return true

}

func (e *Elastico) logPrePrepareMsg(msg map[string]interface{}) {
	/*
		log the pre-prepare msg
	*/
	identityobj := msg["Identity"].(IDENTITY)
	IP := identityobj.IP
	Port := identityobj.Port
	// create a socket
	socket := IP + ":" + strconv.Itoa(Port)
	e.prePrepareMsgLog[socket] = msg
}

func (e *Elastico) logFinalPrePrepareMsg(msg map[string]interface{}) {
	/*
		log the pre-prepare msg
	*/
	identityobj := msg["Identity"].(IDENTITY)
	IP := identityobj.IP
	Port := identityobj.Port
	// create a socket
	socket := IP + ":" + strconv.Itoa(Port)
	e.FinalPrePrepareMsgLog[socket] = msg

}

// BroadcastRmsg :- structure for Broadcast R msg
type BroadcastRmsg struct {
	Ri       string
	Identity IDENTITY
}

// BroadcastR :- broadcast Ri to all the network, final member will do this
func (e *Elastico) BroadcastR() {

	if e.isFinalMember() {

		data := map[string]interface{}{"Ri": e.Ri, "Identity": e.Identity}

		msg := map[string]interface{}{"data": data, "type": "RandomStringBroadcast"}

		e.state = ElasticoStates["BroadcastedR"]

		BroadcastToNetwork(msg)

	} else {
		log.Error("non final member broadcasting R")
	}
}

func (e *Elastico) generateRandomstrings() {
	/*
		Generate r-bit random strings
	*/
	if e.isFinalMember() {
		Ri := randomGen(r)
		e.Ri = fmt.Sprintf("%0"+strconv.FormatInt(r, 10)+"b\n", Ri)
	}
}

func (e *Elastico) notifyFinalCommittee() {
	/*
		notify the members of the final committee that they are the final committee members
	*/
	finalCommList := e.committeeList[finNum]
	for _, finalMember := range finalCommList {
		data := make(map[string]interface{})
		data["Identity"] = e.Identity
		// construct the msg
		msg := make(map[string]interface{})
		msg["data"] = data
		msg["type"] = "notify final member"
		finalMember.send(msg)
	}
}

func (e *Elastico) getCommitment() string {
	/*
		generate commitment for random string Ri. This is done by a final committee member
	*/

	if e.Ri == "" {
		e.generateRandomstrings()
	}
	commitment := sha256.New()
	commitment.Write([]byte(e.Ri))
	hashVal := fmt.Sprintf("%x", commitment.Sum(nil))
	return hashVal
}

func (e *Elastico) executeReset() {
	/*
		call for reset
	*/
	log.Warn("call for reset for ", e.Port)
	if reflect.TypeOf(e.Identity) == reflect.TypeOf(IDENTITY{}) {

		// if node has formed its Identity
		msg := make(map[string]interface{})
		msg["type"] = "reset-all"
		msg["data"] = e.Identity
		e.Identity.send(msg)
	} else {

		// this node has not computed its Identity,calling reset explicitly for node
		e.reset()
	}
}

func (e *Elastico) execute(epochTxn []Transaction) string {
	/*
		executing the functions based on the running state
	*/
	// initial state of elastico node
	if e.state == ElasticoStates["NONE"] {
		e.executePoW()
	} else if e.state == ElasticoStates["PoW Computed"] {

		// form Identity, when PoW computed
		e.formIdentity()
	} else if e.state == ElasticoStates["Formed Identity"] {

		// form committee, when formed Identity
		e.formCommittee()
	} else if e.isDirectory && e.state == ElasticoStates["RunAsDirectory"] {

		log.Info("The directory member :- ", e.Port)
		e.receiveTxns(epochTxn)
		// directory member has received the txns for all committees
		e.state = ElasticoStates["RunAsDirectory after-TxnReceived"]
	} else if e.state == ElasticoStates["Receiving Committee Members"] {

		// when a node is part of some committee
		if e.flag == false {

			// logging the bad nodes
			log.Error("member with invalid POW %s with commMembers : %s", e.Identity, e.committeeMembers)
		}
		// Now The node should go for Intra committee consensus
		// initial state for the PBFT
		e.state = ElasticoStates["PBFT_NONE"]
		// run PBFT for intra-committee consensus
		e.runPBFT()

	} else if e.state == ElasticoStates["PBFT_NONE"] || e.state == ElasticoStates["PBFT_PRE_PREPARE"] || e.state == ElasticoStates["PBFT_PREPARE_SENT"] || e.state == ElasticoStates["PBFT_PREPARED"] || e.state == ElasticoStates["PBFT_COMMIT_SENT"] || e.state == ElasticoStates["PBFT_PRE_PREPARE_SENT"] {

		// run pbft for intra consensus
		e.runPBFT()
	} else if e.state == ElasticoStates["PBFT_COMMITTED"] {

		// send pbft consensus blocks to final committee members
		log.Info("pbft finished by members %s", e.Port)
		e.SendtoFinal()

	} else if e.isFinalMember() && e.state == ElasticoStates["Intra Consensus Result Sent to Final"] {

		// final committee node will collect blocks and merge them
		e.checkCountForConsensusData()

	} else if e.isFinalMember() && e.state == ElasticoStates["Merged Consensus Data"] {

		// final committee member runs final pbft
		e.state = ElasticoStates["FinalPBFT_NONE"]
		e.runFinalPBFT()

	} else if e.state == ElasticoStates["FinalPBFT_NONE"] || e.state == ElasticoStates["FinalPBFT_PRE_PREPARE"] || e.state == ElasticoStates["FinalPBFT_PREPARE_SENT"] || e.state == ElasticoStates["FinalPBFT_PREPARED"] || e.state == ElasticoStates["FinalPBFT_COMMIT_SENT"] || e.state == ElasticoStates["FinalPBFT_PRE_PREPARE_SENT"] {

		e.runFinalPBFT()

	} else if e.isFinalMember() && e.state == ElasticoStates["FinalPBFT_COMMITTED"] {

		// send the commitment to other final committee members
		e.sendCommitment()
		log.Warn("pbft finished by final committee", e.Port)

	} else if e.isFinalMember() && e.state == ElasticoStates["CommitmentSentToFinal"] {

		// broadcast final txn block to ntw
		if len(e.commitments) >= c/2+1 {
			e.BroadcastFinalTxn()
		}
	} else if e.state == ElasticoStates["FinalBlockReceived"] {

		e.checkCountForFinalData()

	} else if e.isFinalMember() && e.state == ElasticoStates["FinalBlockSentToClient"] {

		// broadcast Ri is done when received commitment has atleast c/2  + 1 signatures
		if len(e.newRcommitmentSet) >= c/2+1 {
			e.BroadcastR()
		}
	} else if e.state == ElasticoStates["ReceivedR"] {

		e.appendToLedger()
		e.state = ElasticoStates["LedgerUpdated"]

	} else if e.state == ElasticoStates["LedgerUpdated"] {

		// Now, the node can be reset
		return "reset"
	}
	return ""
}

type NewNodeMsg struct {
	Identity IDENTITY
}

// SendToDirectory :- Send about new nodes to directory committee members
func (e *Elastico) SendToDirectory() {

	// Add the new processor in particular committee list of directory committee nodes
	for _, nodeID := range e.curDirectory {

		data := map[string]interface{}{"Identity": e.Identity}

		msg := map[string]interface{}{"data": data, "type": "newNode"}

		nodeID.send(msg)
	}
}

func (e *Elastico) receiveTxns(epochTxn []Transaction) {
	/*
		directory node will receive transactions from client
	*/

	// Receive txns from client for an epoch
	var k int64
	numOfCommittees := int64(math.Pow(2, float64(s)))
	var num int64
	num = int64(len(epochTxn)) / numOfCommittees // Transactions per committee
	// loop in sorted order of committee ids
	var iden int64
	for iden = 0; iden < numOfCommittees; iden++ {
		if iden == numOfCommittees-1 {
			// give all the remaining txns to the last committee
			e.txn[iden] = epochTxn[k:]
		} else {
			e.txn[iden] = epochTxn[k : k+num]
		}
		k = k + num
	}
}

func (e *Elastico) pbftProcessMessage(msg map[string]interface{}) {
	/*
		Process the messages related to Pbft!
	*/
	if msg["type"] == "pre-prepare" {

		e.processPrePrepareMsg(msg)

	} else if msg["type"] == "prepare" {

		e.processPrepareMsg(msg)

	} else if msg["type"] == "commit" {
		e.processCommitMsg(msg)
	}
}

func (e *Elastico) processPrePrepareMsg(msg map[string]interface{}) {
	/*
		Process Pre-Prepare msg
	*/
	// verify the pre-prepare message
	verified := e.verifyPrePrepare(msg)
	if verified {
		// Log the pre-prepare msgs!
		e.logPrePrepareMsg(msg)

	} else {
		log.Error("error in verification of processPrePrepareMsg")
	}
}

func (e *Elastico) processFinalprePrepareMsg(msg map[string]interface{}) {
	/*
		Process Final Pre-Prepare msg
	*/

	// verify the Final pre-prepare message
	verified := e.verifyFinalPrePrepare(msg)
	if verified {

		// Log the final pre-prepare msgs!
		e.logFinalPrePrepareMsg(msg)

	} else {
		log.Error("error in verification of Final processPrePrepareMsg")
	}
}

func (e *Elastico) isPrePrepared() bool {
	/*
		if the node received the pre-prepare msg from the primary
	*/
	return len(e.prePrepareMsgLog) > 0
}

func (e *Elastico) isFinalprePrepared() bool {

	return len(e.FinalPrePrepareMsgLog) > 0
}

func makeMalicious() {
	/*
		make some nodes malicious who will compute wrong PoW
	*/
	maliciousCount := 0
	for i := 0; i < maliciousCount; i++ {
		randomNum := randomGen(32).Int64() // converting random num big.Int to Int64
		badNodeIndex := randomNum % n
		// set the flag false for bad nodes
		networkNodes[badNodeIndex].flag = false
	}
}

func makeFaulty() {
	/*
		make some nodes faulty who will stop participating in the protocol after sometime
	*/
	// making some(4 here) nodes as faulty
	faultyCount := 0
	for i := 0; i < faultyCount; i++ {
		randomNum := randomGen(32).Int64() // converting random num big.Int to Int64
		faultyNodeIndex := randomNum % n
		// set the flag false for bad nodes
		networkNodes[faultyNodeIndex].faulty = true
	}
}

func (e *Elastico) verifyCommit(msg map[string]interface{}) bool {
	/*
		verify commit msgs
	*/
	// verify Pow
	identityobj := msg["Identity"].(IDENTITY)
	if !e.verifyPoW(identityobj) {
		return false
	}
	// verify signatures of the received msg

	sign := msg["sign"].(string)
	commitData := msg["commitData"].(map[string]interface{})
	digestCommitData := e.digestCommitMsg(commitData)
	PK := identityobj.PK
	if !e.verifySign(sign, digestCommitData, &PK) {
		return false
	}

	// check the view is same or not
	viewID := commitData["viewID"].(int)
	if viewID != e.viewID {
		return false
	}
	return true
}

func (e *Elastico) consumeMsg() {
	/*
		consume the msgs for this node
	*/

	// create a channel
	channel, err := e.connection.Channel()
	failOnError(err, "Failed to open a channel", true)
	// close the channel
	defer channel.Close()

	nodeport := strconv.Itoa(e.Port)
	queueName := "hello" + nodeport
	// count the number of messages that are in the queue
	Queue, err := channel.QueueInspect(queueName)
	// failOnError(err, "error in inspect", false)

	var decoded msgType
	if err == nil {
		// consume all the messages one by one
		for ; Queue.Messages > 0; Queue.Messages-- {

			// get the message from the queue
			msg, ok, err := channel.Get(Queue.Name, true)
			failOnError(err, "error in get of queue", true)
			if ok {
				err := json.Unmarshal(msg.Body, &decoded)
				failOnError(err, "error in unmarshall", true)
				// consume the msg by taking the action in receive
				e.receive(decoded)
			}
		}
	}
}

// txnHexdigest - Hex digest of txn List
func txnHexdigest(txnList []Transaction) string {
	/*
		return hexdigest for a list of transactions
	*/
	// ToDo : Sort the Txns based on hash value
	digest := sha256.New()
	for i := 0; i < len(txnList); i++ {
		txnDigest := txnList[i].hexdigest()
		digest.Write([]byte(txnDigest))
	}
	hashVal := fmt.Sprintf("%x", digest.Sum(nil)) // hash of the list of txns
	return hashVal
}

func executeSteps(nodeIndex int64, epochTxns map[int][]Transaction, sharedObj map[int64]bool) {
	/*
		A process will execute based on its state and then it will consume
	*/
	defer wg.Done()
	node := networkNodes[nodeIndex]
	for _, epochTxn := range epochTxns {
		// epochTxn holds the txn for the current epoch

		// delete the entry of the node in sharedobj for the next epoch
		if _, ok := sharedObj[nodeIndex]; ok {
			delete(sharedObj, nodeIndex)
		}

		// startTime = time.time()
		for {

			// execute one step of elastico node, execution of a node is done only when it has not done reset
			if _, ok := sharedObj[nodeIndex]; ok == false {

				response := node.execute(epochTxn)
				if response == "reset" {
					// now reset the node
					node.executeReset()
					// adding the value reset for the node in the sharedobj
					sharedObj[nodeIndex] = true
				}
			}
			// stop the faulty node in between
			if node.faulty == true { //and time.time() - startTime >= 60:
				log.Warn("bye bye!")
				break
			}
			// All the elastico objects has done their reset
			if int64(len(sharedObj)) == n {
				break
			}
			// process consume the msgs from the queue
			node.consumeMsg()
		}
		// Ensuring that all nodes are reset and sharedobj is not affected
		// time.sleep(60)
	}
}

func (e *Elastico) hexdigest(data string) string {
	/*
		Digest of data
	*/
	digest := sha256.New()
	digest.Write([]byte(data))

	hashVal := fmt.Sprintf("%x", digest.Sum(nil)) // convert to hexdigest
	return hashVal
}

func createTxns() []Transaction {
	/*
		create txns for an epoch
	*/
	numOfTxns := 20 // number of transactions in each epoch
	// txns is the list of the transactions in one epoch to which the committees will agree on
	txns := make([]Transaction, numOfTxns)
	for i := 0; i < numOfTxns; i++ {
		randomNum := randomGen(32)                                                // random amount
		transaction := Transaction{Sender: "a", Receiver: "b", Amount: randomNum} // create the dummy transaction
		txns[i] = transaction
	}
	return txns
}

func createNodes() {
	/*
		create the elastico nodes
	*/
	// network_nodes is the list of elastico objects
	if len(networkNodes) == 0 {
		networkNodes = make([]Elastico, n)
		for i := int64(0); i < n; i++ {
			networkNodes[i].ElasticoInit() //initialise elastico nodes
		}
	}
}

func createRoutines(epochTxns map[int][]Transaction, sharedObj map[int64]bool) {
	/*
		create a Go Routine for each elastico node
	*/

	for nodeIndex := int64(0); nodeIndex < n; nodeIndex++ {
		go executeSteps(nodeIndex, epochTxns, sharedObj) // start thread
	}
}

// Run :- run all the epochs
func Run(epochTxns map[int][]Transaction) {
	// # Manager for managing the shared variable among the processes
	// manager = Manager()
	sharedObj = make(map[int64]bool)

	createNodes() // create the elastico nodes

	// make some nodes malicious and faulty
	makeMalicious()
	makeFaulty()

	// create the threads
	createRoutines(epochTxns, sharedObj)

	// log.Warn("LEDGER- , length - ", ledger, len(ledger))
	// for block in ledger:
	// 	logging.warning("txns : %s", block.data.transactions)
}

func main() {

	wg.Add(int(n))

	os.Remove("logfile.log") // delete the file
	// open the logging file
	file, err := os.OpenFile("logfile.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	failOnError(err, "opening file error", true) // report the open file error

	numOfEpochs := 2 // num of epochs
	epochTxns := make(map[int][]Transaction)
	for epoch := 0; epoch < numOfEpochs; epoch++ {
		epochTxns[epoch] = createTxns()
	}

	log.SetOutput(file)
	log.SetLevel(log.InfoLevel) // set the log level

	// run all the epochs
	Run(epochTxns)

	wg.Wait()
}
