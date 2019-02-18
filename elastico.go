// A package clause starts every source file.
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	// "reflect"
	"encoding/json"
	"math"
	"os"
	"sync" // for locks

	log "github.com/sirupsen/logrus" // for logging
	"github.com/streadway/amqp"      // for rabbitmq
)

// Elastico_States - states reperesenting the running state of the node
var Elastico_States = map[string]int{"NONE": 0, "PoW Computed": 1, "Formed Identity": 2, "Formed Committee": 3, "RunAsDirectory": 4, "RunAsDirectory after-TxnReceived": 5, "RunAsDirectory after-TxnMulticast": 6, "Receiving Committee Members": 7, "PBFT_NONE": 8, "PBFT_PRE_PREPARE": 9, "PBFT_PRE_PREPARE_SENT": 10, "PBFT_PREPARE_SENT": 11, "PBFT_PREPARED": 12, "PBFT_COMMITTED": 13, "PBFT_COMMIT_SENT": 14, "Intra Consensus Result Sent to Final": 15, "Merged Consensus Data": 16, "FinalPBFT_NONE": 17, "FinalPBFT_PRE_PREPARE": 18, "FinalPBFT_PRE_PREPARE_SENT": 19, "FinalPBFT_PREPARE_SENT": 20, "FinalPBFT_PREPARED": 21, "FinalPBFT_COMMIT_SENT": 22, "FinalPBFT_COMMITTED": 23, "PBFT Finished-FinalCommittee": 24, "CommitmentSentToFinal": 25, "FinalBlockSent": 26, "FinalBlockReceived": 27, "BroadcastedR": 28, "ReceivedR": 29, "FinalBlockSentToClient": 30, "LedgerUpdated": 31}

// shared lock among processes
var lock sync.Mutex

// shared port among the processes
var port int = 49152

// n : number of nodes
var n int64 = 66

// s - where 2^s is the number of committees
var s int = 2

// c - size of committee
var c int = 4

// D - difficulty level , leading bits of PoW must have D 0's (keep w.r.t to hex)
var D int = 6

// r - number of bits in random string
var r int64 = 4

// fin_num - final committee id
var fin_num int64 = 0

// network_nodes - list of elastico objects
var network_nodes []Elastico

func failOnError(err error, msg string) {
	// logging the error
	if err != nil {
		log.Error("see the error!")
		log.Error("%s: %s", msg, err)
		os.Exit(1)
	}
}

func getChannel(connection *amqp.Connection) *amqp.Channel {
	/*
		get channel
	*/
	channel, err := connection.Channel()         // create a channel
	failOnError(err, "Failed to open a channel") // report the error
	return channel
}

func getConnection() *amqp.Connection {
	/*
		establish a connection with RabbitMQ server
	*/
	connection, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ") // report the error
	return connection
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
	failOnError(err, "Failed to declare a queue")

	body, err := json.Marshal(msg)
	failOnError(err, "Failed to marshal")
	err = channel.Publish(
		"",         // exchange
		queue.Name, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        body,
		})

	failOnError(err, "Failed to publish a message")
}

func MulticastCommittee(commList map[int64][]Identity, identityobj Identity, txns map[int64][]Transaction) {
	/*
		each node getting views of its committee members from directory members
	*/
	// get the final committee members with the fixed committee id
	finalCommitteeMembers := commList[fin_num]
	for committee_id, commMembers := range commList {

		// find the primary identity, Take the first identity
		// ToDo: fix this, many nodes can be primary
		primaryId := commMembers[0]
		for _, memberId := range commMembers {

			// send the committee members , final committee members
			data := make(map[string]interface{})
			data["committee members"] = commMembers
			data["final Committee members"] = finalCommitteeMembers
			data["identity"] = identityobj

			// give txns only to the primary node
			if memberId == primaryId {
				data["txns"] = txns[committee_id]
			}
			// construct the msg
			msg := make(map[string]interface{})
			msg["data"] = data
			msg["type"] = "committee members views"
			// send the committee member views to nodes
			memberId.send(msg)
		}
	}
}

func BroadcastTo_Network(data map[string]interface{}, type_ string) {
	/*
		Broadcast data to the whole ntw
	*/
	// construct msg
	msg := make(map[string]interface{})
	msg["data"] = data
	msg["type"] = type_

	connection := getConnection()
	defer connection.Close() // close the connection

	channel := getChannel(connection)
	defer channel.Close() // close the channel

	for _, node := range network_nodes {
		nodePort := strconv.Itoa(node.port)
		queueName := "hello" + nodePort
		publishMsg(channel, queueName, msg) //publish the message in queue
	}
}

func random_gen(r int64) *big.Int {
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

	failOnError(err, "random number generation")
	return randomNum
}

// structure for identity of nodes
type Identity struct {
	IP               string
	PK               *rsa.PublicKey
	committee_id     int64
	PoW              map[string]interface{}
	epoch_randomness string
	port             int
}

func (i *Identity) IdentityInit() {
	i.PoW = make(map[string]interface{})
}

func (i *Identity) isEqual(identityobj *Identity) bool {
	/*
		checking two objects of Identity class are equal or not

	*/
	return i.IP == identityobj.IP && i.PK == identityobj.PK && i.committee_id == identityobj.committee_id && i.PoW["hash"] == identityobj.PoW["hash"] && i.PoW["set_of_Rs"] == identityobj.PoW["set_of_Rs"] && i.PoW["nonce"] == identityobj.PoW["nonce"] && i.epoch_randomness == identityobj.epoch_randomness && i.port == identityobj.port
}

func (i *Identity) send(msg map[string]interface{}) {
	/*
		send the msg to node based on their identity
	*/
	// establish a connection with RabbitMQ server
	connection := getConnection()
	defer connection.Close()

	// create a channel
	channel := getChannel(connection)
	// close the channel
	defer channel.Close()

	nodePort := strconv.Itoa(i.port)
	queueName := "hello" + nodePort
	publishMsg(channel, queueName, msg) // publish the msg in queue
}

type Transaction struct {
	sender   string
	receiver string
	amount   *big.Int // random_gen returns *big.Int
	// ToDo: include timestamp or not
}

func (t *Transaction) TransactionInit(sender string, receiver string, amount *big.Int) {
	t.sender = sender
	t.receiver = receiver
	t.amount = amount
}

func (t *Transaction) hexdigest() string {
	/*
		Digest of a transaction
	*/
	digest := sha256.New()
	digest.Write([]byte(t.sender))
	digest.Write([]byte(t.receiver))
	digest.Write([]byte(t.amount.String())) // convert amount(big.Int) to string

	hash_val := fmt.Sprintf("%x", digest.Sum(nil))
	return hash_val
}

func (t *Transaction) isEqual(transaction Transaction) bool {
	/*
		compare two objs are equal or not
	*/
	return t.sender == transaction.sender && t.receiver == transaction.receiver && t.amount == transaction.amount //&& t.timestamp == transaction.timestamp
}

type Elastico struct {
	connection    *amqp.Connection
	IP            string
	port          int
	key           *rsa.PrivateKey
	PoW           map[string]interface{}
	cur_directory []Identity
	identity      Identity
	committee_id  int64
	// only when this node is the member of directory committee
	committee_list map[int64][]Identity
	// only when this node is not the member of directory committee
	committee_Members []Identity
	is_directory      bool
	is_final          bool
	epoch_randomness  string
	Ri                string
	// only when this node is the member of final committee
	commitments                    map[string]bool
	txn_block                      []Transaction
	set_of_Rs                      map[string]bool
	newset_of_Rs                   map[string]bool
	CommitteeConsensusData         map[int]map[string][]string
	CommitteeConsensusDataTxns     map[int]map[string][]Transaction
	finalBlockbyFinalCommittee     map[int]map[string][]string
	finalBlockbyFinalCommitteeTxns map[int]map[string][]Transaction
	state                          int
	mergedBlock                    []Transaction
	finalBlock                     map[string]interface{}
	RcommitmentSet                 map[string]bool
	newRcommitmentSet              map[string]bool
	finalCommitteeMembers          []Identity
	// only when this is the member of the directory committee
	txn      map[int64][]Transaction
	response []Transaction
	flag     bool
	views    map[int]bool
	primary  bool
	viewId   int
	faulty   bool
	// pre_prepareMsgLog
	// prepareMsgLog
	// commitMsgLog
	// preparedData
	// committedData
	// Finalpre_prepareMsgLog
	// FinalprepareMsgLog
	// FinalcommitMsgLog
	// FinalpreparedData
	// FinalcommittedData
}

func (e *Elastico) get_key() {
	/*
		for each node, it will set key as public pvt key pair
	*/
	var err error
	// generate the public-pvt key pair
	e.key, err = rsa.GenerateKey(rand.Reader, 2048)
	failOnError(err, "key generation")
}

func (e *Elastico) get_IP() {
	/*
		for each node(processor) , get IP addr
	*/
	count := 4
	// construct the byte array of size 4
	byteArray := make([]byte, count)
	// Assigning random values to the byte array
	_, err := rand.Read(byteArray)
	failOnError(err, "reading random values error")
	// setting the IP addr from the byte array
	e.IP = fmt.Sprintf("%v.%v.%v.%v", byteArray[0], byteArray[1], byteArray[2], byteArray[3])
}

func (e *Elastico) initER() {
	/*
		initialise r-bit epoch random string
	*/

	randomnum := random_gen(r)
	// set r-bit binary string to epoch randomness
	e.epoch_randomness = fmt.Sprintf("%0"+strconv.FormatInt(r, 10)+"b\n", randomnum)
}

func (e *Elastico) get_port() {
	/*
		get port number for the process
	*/
	// acquire the lock
	lock.Lock()
	port += 1
	e.port = port
	// release the lock
	defer lock.Unlock()
}

func (e *Elastico) compute_PoW() {
	/*
		returns hash which satisfies the difficulty challenge(D) : PoW["hash"]
	*/
	zero_string := ""
	for i := 0; i < D; i++ {
		zero_string += "0"
	}
	// ToDo: type assertion for interface
	nonce := e.PoW["nonce"].(int)
	if e.state == Elastico_States["NONE"] {
		// public key
		PK := e.key.Public()
		fmt.Printf("%T", PK)
		rsaPublickey, err := PK.(*rsa.PublicKey)
		failOnError(err, "rsa Public key conversion")
		IP := e.IP
		// If it is the first epoch , randomset_R will be an empty set .
		// otherwise randomset_R will be any c/2 + 1 random strings Ri that node receives from the previous epoch
		randomset_R := make(map[string]bool)
		if len(e.set_of_Rs) > 0 {
			// ToDo: complete this for further epochs
			// e.epoch_randomness, randomset_R = e.xor_R()
		}
		// 	compute the digest
		digest := sha256.New()
		digest.Write([]byte(IP))
		digest.Write(rsaPublickey.N.Bytes())
		digest.Write([]byte(strconv.Itoa(rsaPublickey.E)))
		digest.Write([]byte(e.epoch_randomness))
		digest.Write([]byte(strconv.Itoa(nonce)))

		hash_val := fmt.Sprintf("%x", digest.Sum(nil))
		if strings.HasPrefix(hash_val, zero_string) {
			//hash starts with leading D 0's
			e.PoW["hash"] = hash_val
			e.PoW["set_of_Rs"] = randomset_R
			e.PoW["nonce"] = nonce
			// change the state after solving the puzzle
			e.state = Elastico_States["PoW Computed"]
		} else {
			// try for other nonce
			nonce += 1
			e.PoW["nonce"] = nonce
		}
	}
}

func (e *Elastico) checkCommitteeFull() {
	/*
		directory member checks whether the committees are full or not
	*/
	commList := e.committee_list
	flag := 0
	numOfCommittees := int64(math.Pow(2, float64(s)))
	// iterating over all committee ids
	for iden := int64(0); iden < numOfCommittees; iden++ {

		val, ok := commList[iden]
		if ok == false || len(commList[iden]) < c {

			log.Warn("committees not full  - bad miss id :", iden)
			flag = 1
			break
		}
	}
	if flag == 0 {

		log.Warn("committees full  - good")
		if e.state == Elastico_States["RunAsDirectory after-TxnReceived"] {

			// notify the final members
			e.notify_finalCommittee()
			// multicast the txns and committee members to the nodes
			MulticastCommittee(commList, e.identity, e.txn)
			// change the state after multicast
			e.state = Elastico_States["RunAsDirectory after-TxnMulticast"]
		}
	}
}

func (e *Elastico) receive_directoryMember(msg map[string]interface{}) {
	identityobj, _ := msg["data"].(Identity)
	// verify the PoW of the sender
	if e.verify_PoW(identityobj) {
		if len(e.cur_directory) < c {
			// check whether identityobj is already present or not
			flag := true
			for _, obj := range e.cur_directory {
				if identityobj.isEqual(obj) {
					flag = false
					break
				}
			}
			if flag {
				// append the object if not already present
				e.cur_directory = append(e.cur_directory, identityobj)
			}
		}
	} else {
		log.Error("PoW not valid of an incoming directory member", identityobj)
	}
}

func (e *Elastico) receive_newNode(msg map[string]interface{}) {
	// new node is added to the corresponding committee list if committee list has less than c members
	identityobj, _ := msg["data"].(Identity)
	// verify the PoW
	if e.verify_PoW(identityobj) {
		_, ok := e.committee_list[identityobj.committee_id]
		if ok == false {

			// Add the identity in committee
			e.committee_list[identityobj.committee_id] = []Identity{identityobj}

		} else if len(e.committee_list[identityobj.committee_id]) < c {
			// Add the identity in committee
			flag := true
			for _, obj := range e.committee_list[identityobj.committee_id] {
				if identityobj.isEqual(obj) {
					flag = false
					break
				}
			}
			if flag {
				e.committee_list[identityobj.committee_id] = append(e.committee_list[identityobj.committee_id], identityobj)
				if len(e.committee_list[identityobj.committee_id]) == c {
					// check that if all committees are full
					e.checkCommitteeFull()
				}
			}
		}

	} else {
		log.Error("PoW not valid in adding new node")
	}
}

func (e *Elastico) receive_views(msg map[string]interface{}) {

	// union of committe members views
	e.views = append(e.views, msg["data"]["identity"].port)
	commMembers := msg["data"]["committee members"]
	finalMembers := msg["data"]["final Committee members"]

	if _, ok := msg["data"]["txns"]; ok {

		// update the txn block
		// ToDo: txnblock should be ordered, not set
		e.txn_block = e.unionTxns(e.txn_block, msg["data"]["txns"])
		log.Warn("I am primary", e.port)
		e.primary = true
	}
	// ToDo: verify this union thing
	// union of committee members wrt directory member
	e.committee_Members = e.unionViews(e.committee_Members, commMembers)
	// union of final committee members wrt directory member
	e.finalCommitteeMembers = e.unionViews(e.finalCommitteeMembers, finalMembers)
	// received the members
	if e.state == Elastico_States["Formed Committee"] && len(e.views) >= c/2+1 {

		e.state = Elastico_States["Receiving Committee Members"]
	}
}

func (e *Elastico) receive_hash(msg map[string]interface{}) {

	// receiving H(Ri) by final committe members
	data := msg["data"]
	identityobj := data["identity"]
	if e.verify_PoW(identityobj) {
		e.commitments = append(e.commitments, data["Hash_Ri"])
	}
}

func (e *Elastico) receive_RandomStringBroadcast(msg map[string]interface{}) {

	data := msg["data"]
	identityobj := data["identity"]
	if e.verify_PoW(identityobj) {

		Ri := data["Ri"]
		HashRi := e.hexdigest(Ri)

		if _, ok := e.newRcommitmentSet[HashRi]; ok {

			e.newset_of_Rs = append(e.newset_of_Rs, Ri)

			if len(e.newset_of_Rs) >= c/2+1 {
				e.state = Elastico_States["ReceivedR"]
			}
		}
	}
}

func (e *Elastico) receive_finalTxnBlock(msg map[string]interface{}) {

	data := msg["data"]
	identityobj := data["identity"]
	// verify the PoW of the sender
	if e.verify_PoW(identityobj) {

		sign := data["signature"]
		received_commitmentSetList := data["commitmentSet"]
		PK := identityobj.PK
		finalTxnBlock := data["finalTxnBlock"]
		finalTxnBlock_signature := data["finalTxnBlock_signature"]
		// verify the signatures
		if e.verify_sign(sign, received_commitmentSetList, PK) && e.verify_signTxnList(finalTxnBlock_signature, finalTxnBlock, PK) {

			// list init for final txn block
			finaltxnBlockDigest := txnHexdigest(finalTxnBlock)
			if _ok := e.finalBlockbyFinalCommittee[finaltxnBlockDigest]; ok == false {

				e.finalBlockbyFinalCommittee[finaltxnBlockDigest] = []Transaction{finalTxnBlock}
			}

			// creating the object that contains the identity and signature of the final member
			identityAndSign := IdentityAndSign(finalTxnBlock_signature, identityobj)

			// check whether this combination of identity and sign already exists or not
			flag := true
			for _, idSignObj := range e.finalBlockbyFinalCommittee[finaltxnBlockDigest] {

				if idSignObj.isEqual(identityAndSign) {
					// it exists
					flag = false
					break
				}
			}
			if flag {
				// appending the identity and sign of final member
				e.finalBlockbyFinalCommittee[finaltxnBlockDigest] = append(e.finalBlockbyFinalCommittee[finaltxnBlockDigest], identityAndSign)
			}

			// block is signed by sufficient final members and when the final block has not been sent to the client yet
			if len(e.finalBlockbyFinalCommittee[finaltxnBlockDigest]) >= c/2+1 && e.state != Elastico_States["FinalBlockSentToClient"] {
				// for final members, their state is updated only when they have also sent the finalblock to ntw
				if e.isFinalMember() {

					if e.finalBlock["sent"] {

						e.state = Elastico_States["FinalBlockReceived"]
					}

				} else {

					e.state = Elastico_States["FinalBlockReceived"]
				}

				// union of commitments
				e.newRcommitmentSet |= set(received_commitmentSetList)
			}

		} else {

			log.Error("Signature invalid in final block received")
		}
	} else {
		log.Error("PoW not valid when final member send the block")
	}
}

func (e *Elastico) receive_intracommitteeblock(msg map[string]interface{}) {
	// final committee member receives the final set of txns along with the signature from the node
	data := msg["data"]
	identityobj := data["identity"]

	if e.verify_PoW(identityobj) {

		// verify the signatures
		if e.verify_signTxnList(data["sign"], data["txnBlock"], identityobj.PK) {

			if _, ok := e.CommitteeConsensusData[identityobj.committee_id]; ok == false {

				e.CommitteeConsensusData[identityobj.committee_id] = make(map[string]interface{})
				e.CommitteeConsensusDataTxns[identityobj.committee_id] = make(map[string]interface{})
			}
			TxnBlockDigest := txnHexdigest(data["txnBlock"])
			if _, ok := e.CommitteeConsensusData[identityobj.committee_id][TxnBlockDigest]; ok == false {
				// todo: see this
				e.CommitteeConsensusData[identityobj.committee_id][TxnBlockDigest] = set()
				// store the txns for this digest
				e.CommitteeConsensusDataTxns[identityobj.committee_id][TxnBlockDigest] = data["txnBlock"]
			}

			// add signatures for the txn block
			e.CommitteeConsensusData[identityobj.committee_id][TxnBlockDigest] = append(e.CommitteeConsensusData[identityobj.committee_id][TxnBlockDigest], data["sign"])

		} else {
			log.Error("signature invalid for intra committee block")
		}
	} else {
		log.Error("pow invalid for intra committee block")
	}

}

func (e *Elastico) receive(msg map[string]interface{}) {
	/*
		method to recieve messages for a node as per the type of a msg
	*/
	// new node is added in directory committee if not yet formed
	if msg["type"] == "directoryMember" {
		e.receive_directoryMember(msg)

	} else if msg["type"] == "newNode" && e.is_directory {
		e.receive_newNode(msg)

	} else if msg["type"] == "committee members views" && e.verify_PoW(msg["data"]["identity"]) && e.is_directory == false {
		if _ok := e.views[msg["data"]["identity"].port]; ok == false {
			e.receive_views(msg)
		}

	} else if msg["type"] == "hash" && e.isFinalMember() {
		e.receive_hash(msg)

	} else if msg["type"] == "RandomStringBroadcast" {
		e.receive_RandomStringBroadcast(msg)

	} else if msg["type"] == "finalTxnBlock" {
		e.receive_finalTxnBlock(msg)

	} else if msg["type"] == "intraCommitteeBlock" && e.isFinalMember() {
		e.receive_intracommitteeblock()

	} else if msg["type"] == "command to run pbft" {
		if e.is_directory == false {
			e.runPBFT(e.txn_block)
		}
	} else if msg["type"] == "command to run pbft by final committee" {
		if e.isFinalMember() {
			e.runPBFT(e.mergedBlock)
		}
	} else if msg["type"] == "send txn set and sign to final committee" {
		if e.is_directory == false {
			e.SendtoFinal()
		}
	} else if msg["type"] == "verify and merge intra consensus data" {
		if e.isFinalMember() {
			e.verifyAndMergeConsensusData()
		}
	} else if msg["type"] == "send commitments of Ris" {
		if e.isFinalMember() {
			e.sendCommitment()
		}
	} else if msg["type"] == "broadcast final set of txns to the ntw" {

		if e.isFinalMember() {
			e.BroadcastFinalTxn()
		}
	} else if msg["type"] == "notify final member" {
		log.Warn("notifying final member ", e.port)
		if e.verify_PoW(msg["data"]["identity"]) && e.committee_id == fin_num {
			e.is_final = true
		}
	} else if msg["type"] == "Broadcast Ri" {
		if e.isFinalMember() {
			e.BroadcastR()
		}
	} else if msg["type"] == "reset-all" {
		// ToDo: Add verification of pow here.
		// reset the elastico node
		e.reset()

	} else if msg["type"] == "pre-prepare" || msg["type"] == "prepare" || msg["type"] == "commit" {

		e.pbft_process_message(msg)
	} else if msg["type"] == "Finalpre-prepare" || msg["type"] == "Finalprepare" || msg["type"] == "Finalcommit" {
		e.Finalpbft_process_message(msg)
	}
}

func (e *Elastico) ElasticoInit() {
	var err error
	// create rabbit mq connection
	e.connection, err = amqp.Dial("amqp://guest:guest@localhost:5672/")
	// log if connection fails
	failOnError(err, "Failed to connect to RabbitMQ")
	// set IP
	e.get_IP()
	e.get_port()
	// set RSA
	e.get_key()
	// Initialize PoW!
	e.PoW = make(map[string]interface{})
	e.PoW["hash"] = ""
	e.PoW["set_of_Rs"] = ""
	e.PoW["nonce"] = 0

	e.cur_directory = make([]Identity, 0)

	e.committee_list = make(map[int64][]Identity)

	e.committee_Members = make([]Identity, 0)

	// for setting epoch_randomness
	e.initER()

	e.commitments = make(map[string]bool)

	e.txn_block = make([]Transaction, 0)

	e.set_of_Rs = make(map[string]bool)

	e.newset_of_Rs = make(map[string]bool)

	e.CommitteeConsensusData = make(map[int]map[string][]string)

	e.CommitteeConsensusDataTxns = make(map[int]map[string][]Transaction)

	e.finalBlockbyFinalCommittee = make(map[int]map[string][]string)

	e.finalBlockbyFinalCommitteeTxns = make(map[int]map[string][]Transaction)

	e.state = Elastico_States["NONE"]

	e.mergedBlock = make([]Transaction, 0)

	e.finalBlock = make(map[string]interface{})
	e.finalBlock["sent"] = false
	e.finalBlock["finalBlock"] = make([]Transaction, 0)

	e.RcommitmentSet = make(map[string]bool)
	e.newRcommitmentSet = make(map[string]bool)
	e.finalCommitteeMembers = make([]Identity, 0)

	e.txn = make(map[int][]Transaction)
	e.response = make([]Transaction, 0)
	e.flag = true
	e.views = make(map[int]bool)
	e.primary = false
	e.viewId = 0
	e.faulty = false
}

func (e *Elastico) reset() {
	/*
		reset some of the elastico class members
	*/
	e.get_IP()
	e.get_key()

	// channel = e.connection.channel()
	// # to delete the queue in rabbitmq for next epoch
	// channel.queue_delete(queue='hello' + str(e.port))
	// channel.close()

	// e.port = e.get_port()

	e.PoW = make(map[string]interface{})
	e.PoW["hash"] = ""
	e.PoW["set_of_Rs"] = ""
	e.PoW["nonce"] = 0

	e.cur_directory = make([]Identity, 0)
	// only when this node is the member of directory committee
	e.committee_list = make(map[int64][]Identity)
	// only when this node is not the member of directory committee
	e.committee_Members = make([]Identity, 0)

	e.identity = ""
	e.committee_id = ""
	e.Ri = ""

	e.is_directory = false
	e.is_final = false

	// only when this node is the member of final committee
	e.commitments = make(map[string]bool)
	e.txn_block = make([]Transaction, 0)
	e.set_of_Rs = e.newset_of_Rs
	e.newset_of_Rs = make(map[string]bool)
	e.CommitteeConsensusData = make(map[int]map[string][]string)
	e.CommitteeConsensusDataTxns = make(map[int]map[string][]Transaction)
	e.finalBlockbyFinalCommittee = make(map[int]map[string][]string)
	e.finalBlockbyFinalCommitteeTxns = make(map[int]map[string][]Transaction)
	e.state = Elastico_States["NONE"]
	e.mergedBlock = make([]Transaction, 0)

	e.finalBlock = make(map[string]interface{})
	e.finalBlock["sent"] = false
	e.finalBlock["finalBlock"] = make([]Transaction, 0)

	e.RcommitmentSet = e.newRcommitmentSet
	e.newRcommitmentSet = make(map[string]bool)
	e.finalCommitteeMembers = make([]Identity, 0)

	// only when this is the member of the directory committee
	e.txn = make(map[int][]Transaction)
	e.response = make([]Transaction, 0)
	e.flag = true
	e.views = make(map[int]bool)
	e.primary = false
	e.viewId = 0
	e.faulty = false

	// e.prepareMsgLog = dict()
	// e.commitMsgLog = dict()
	// e.preparedData = dict()
	// e.committedData = dict()

	// // only when this is the part of final committee
	// e.Finalpre_prepareMsgLog = dict()
	// e.FinalprepareMsgLog = dict()
	// e.FinalcommitMsgLog = dict()
	// e.FinalpreparedData = dict()
	// e.FinalcommittedData = dict()
}

func (e *Elastico) get_committeeid(PoW string) int64 {
	/*
		returns last s-bit of PoW["hash"] as Identity : committee_id
	*/
	bindigest := ""

	for i := 0; i < len(PoW); i++ {
		intVal, err := strconv.ParseInt(string(PoW[i]), 16, 0) // converts hex string to integer
		failOnError(err, "string to int conversion error")
		bindigest += fmt.Sprintf("%04b", intVal) // converts intVal to 4 bit binary value
	}
	// take last s bits of the binary digest
	identity := bindigest[len(bindigest)-s:]
	iden, err := strconv.ParseInt(identity, 2, 0) // converts binary string to integer
	failOnError(err, "binary to int conversion error")
	return iden
}

func (e *Elastico) executePoW() {
	/*
		execute PoW
	*/
	if e.flag {
		// compute Pow for good node
		e.compute_PoW()
	} else {
		// compute Pow for bad node
		e.compute_fakePoW()
	}
}

func (e *Elastico) isFinalMember() bool {
	/*
		tell whether this node is a final committee member or not
	*/
	return e.is_final
}

func (e *Elastico) runPBFT() {
	/*
		Runs a Pbft instance for the intra-committee consensus
	*/
	if e.state == Elastico_States["PBFT_NONE"] {
		if e.primary {
			// construct pre-prepare msg
			pre_preparemsg := e.construct_pre_prepare()
			// multicasts the pre-prepare msg to replicas
			// ToDo: what if primary does not send the pre-prepare to one of the nodes
			e.send_pre_prepare(pre_preparemsg)

			// change the state of primary to pre-prepared
			e.state = Elastico_States["PBFT_PRE_PREPARE_SENT"]
			// primary will log the pre-prepare msg for itself
			e.logPre_prepareMsg(pre_preparemsg)

		} else {

			// for non-primary members
			if e.is_pre_prepared() {
				e.state = Elastico_States["PBFT_PRE_PREPARE"]
			}
		}

	} else if e.state == Elastico_States["PBFT_PRE_PREPARE"] {

		if e.primary == false {

			// construct prepare msg
			// ToDo: verify whether the pre-prepare msg comes from various primaries or not
			preparemsgList := e.construct_prepare()
			// logging.warning("constructing prepares with port %s" , str(e.port))
			e.send_prepare(preparemsgList)
			e.state = Elastico_States["PBFT_PREPARE_SENT"]
		}

	} else if e.state == Elastico_States["PBFT_PREPARE_SENT"] || e.state == Elastico_States["PBFT_PRE_PREPARE_SENT"] {
		// ToDo: if, primary has not changed its state to "PBFT_PREPARE_SENT"
		if e.isPrepared() {

			// logging.warning("prepared done by %s" , str(e.port))
			e.state = Elastico_States["PBFT_PREPARED"]

		} else if e.state == Elastico_States["PBFT_PREPARED"] {

			commitMsgList := e.construct_commit()
			e.send_commit(commitMsgList)
			e.state = Elastico_States["PBFT_COMMIT_SENT"]

		} else if e.state == Elastico_States["PBFT_COMMIT_SENT"] {

			if e.isCommitted() {

				// logging.warning("committed done by %s" , str(e.port))
				e.state = Elastico_States["PBFT_COMMITTED"]
			}
		}
	}
}

func (e *Elastico) runFinalPBFT() {
	/*
		Run PBFT by final committee members
	*/
	if e.state == Elastico_States["FinalPBFT_NONE"] {

		if e.primary {

			// construct pre-prepare msg
			finalpre_preparemsg := e.construct_Finalpre_prepare()
			// multicasts the pre-prepare msg to replicas
			e.send_pre_prepare(finalpre_preparemsg)

			// change the state of primary to pre-prepared
			e.state = Elastico_States["FinalPBFT_PRE_PREPARE_SENT"]
			// primary will log the pre-prepare msg for itself
			e.logFinalPre_prepareMsg(finalpre_preparemsg)

		} else {

			// for non-primary members
			if e.is_Finalpre_prepared() {
				e.state = Elastico_States["FinalPBFT_PRE_PREPARE"]
			}
		}

	} else if e.state == Elastico_States["FinalPBFT_PRE_PREPARE"] {

		if e.primary == false {

			// construct prepare msg
			FinalpreparemsgList := e.construct_Finalprepare()
			e.send_prepare(FinalpreparemsgList)
			e.state = Elastico_States["FinalPBFT_PREPARE_SENT"]
		}
	} else if e.state == Elastico_States["FinalPBFT_PREPARE_SENT"] || e.state == Elastico_States["FinalPBFT_PRE_PREPARE_SENT"] {

		// ToDo: primary has not changed its state to "FinalPBFT_PREPARE_SENT"
		if e.isFinalPrepared() {

			e.state = Elastico_States["FinalPBFT_PREPARED"]
		}
	} else if e.state == Elastico_States["FinalPBFT_PREPARED"] {

		commitMsgList := e.construct_Finalcommit()
		e.send_commit(commitMsgList)
		e.state = Elastico_States["FinalPBFT_COMMIT_SENT"]

	} else if e.state == Elastico_States["FinalPBFT_COMMIT_SENT"] {

		if e.isFinalCommitted() {

			// for viewId in e.FinalcommittedData:
			// 	for seqnum in e.FinalcommittedData[viewId]:
			// 		msgList = e.FinalcommittedData[viewId][seqnum]
			// 		for msg in msgList:
			// 			e.finalBlock["finalBlock"] = e.unionTxns(e.finalBlock["finalBlock"], msg)
			// finalTxnBlock = e.finalBlock["finalBlock"]
			// finalTxnBlock = list(finalTxnBlock)
			// # order them! Reason : to avoid errors in signatures as sets are unordered
			// # e.finalBlock["finalBlock"] = sorted(finalTxnBlock)
			// logging.warning("final block by port %s with final block %s" , str(e.port), str(e.finalBlock["finalBlock"]))
			e.state = Elastico_States["FinalPBFT_COMMITTED"]
		}
	}
}

func (e *Elastico) compute_fakePoW() {
	/*
		bad node generates the fake PoW
	*/
	// random fakeness
	x := random_gen(32)
	_, index := x.DivMod(x, big.NewInt(3), big.NewInt(0))

	if index == 0 {

		// Random hash with initial D hex digits 0s
		digest := sha256.New()
		ranHash := fmt.Sprintf("%x", digest.Sum(nil))
		hash_val := ""
		for i := 0; i < D; i++ {
			hash_val += "0"
		}
		e.PoW["hash"] = hash_val + ranHash[D:]

	} else if index == 1 {

		// computing an invalid PoW using less number of values in digest
		randomset_R := set()
		zero_string = ""
		for i := 0; i < D; i++ {
			zero_string += "0"
		}
		// if len(e.set_of_Rs) > 0:
		// 	e.epoch_randomness, randomset_R = e.xor_R()
		for {

			digest := sha256.New()
			nonce := e.PoW["nonce"]
			digest.Write([]byte(strconv.Itoa(nonce)))
			hash_val := fmt.Sprintf("%x", digest.Sum(nil))
			if strings.HasPrefix(hash_val, zero_string) {
				e.PoW["hash"] = hash_val
				e.PoW["set_of_Rs"] = randomset_R
				e.PoW["nonce"] = nonce
			} else {
				// try for other nonce
				nonce += 1
				e.PoW["nonce"] = nonce
			}
		}
	} else if index == 2 {

		// computing a random PoW
		randomset_R := set()
		// if len(e.set_of_Rs) > 0:
		// 	e.epoch_randomness, randomset_R := e.xor_R()
		digest := sha256.New()
		ranHash := fmt.Sprintf("%x", digest.Sum(nil))
		nonce := random_gen()
		e.PoW["hash"] = ranHash
		e.PoW["set_of_Rs"] = randomset_R
		// ToDo: nonce has to be in int instead of big.Int
		e.PoW["nonce"] = nonce
	}

	log.Warn("computed fake POW ", index)
	e.state = Elastico_States["PoW Computed"]

}

func (e *Elastico) form_identity() {
	/*
		identity formation for a node
		identity consists of public key, ip, committee id, PoW, nonce, epoch randomness
	*/
	if e.state == Elastico_States["PoW Computed"] {
		// export public key
		PK := e.key.Public().(*rsa.PublicKey)

		// set the committee id acc to PoW solution
		e.committee_id = e.get_committeeid(e.PoW["hash"].(string))

		e.identity = Identity{e.IP, PK, e.committee_id, e.PoW, e.epoch_randomness, e.port}
		// changed the state after identity formation
		e.state = Elastico_States["Formed Identity"]
	}
}

func (e *Elastico) unionViews(nodeData, incomingData []Identity) []Identity {
	/*
		nodeData and incomingData are the set of identities
		Adding those identities of incomingData to nodeData that are not present in nodeData
	*/
	for _, data := range incomingData {

		flag := false
		for _, nodeId := range nodeData {

			// data is present already in nodeData
			if nodeId.isEqual(data) {
				flag = true
				break
			}
		}
		if flag == false {
			// Adding the new identity
			nodeData = append(nodeData, data)
		}
	}
	return nodeData
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

func (e *Elastico) form_committee() {
	/*
		creates directory committee if not yet created otherwise informs all the directory members
	*/
	if len(e.cur_directory) < c {

		e.is_directory = true
		// broadcast the identity to whole ntw
		BroadcastTo_Network(e.identity, "directoryMember")
		// change the state as it is the directory member
		e.state = Elastico_States["RunAsDirectory"]
	} else {

		e.Send_to_Directory()
		if e.state != Elastico_States["Receiving Committee Members"] {

			e.state = Elastico_States["Formed Committee"]
		}
	}
}

func (e *Elastico) verify_PoW(identityobj Identity) {
	/*
		verify the PoW of the node identityobj
	*/
	zero_string := ""
	for i := 0; i < D; i++ {

		zero_string += "0"
	}

	PoW := identityobj.PoW

	// length of hash in hex
	if len(PoW["hash"]) != 64 {
		return false
	}

	// Valid Hash has D leading '0's (in hex)
	if !strings.HasPrefix(Pow["hash"], zero_string) {
		return false
	}

	// check Digest for set of Ri strings
	// for Ri in PoW["set_of_Rs"]:
	// 	digest = e.hexdigest(Ri)
	// 	if digest not in e.RcommitmentSet:
	// 		return false

	// reconstruct epoch randomness
	epoch_randomness := identityobj.epoch_randomness
	// if len(PoW["set_of_Rs"]) > 0:
	// 	xor_val = 0
	// 	for R in PoW["set_of_Rs"]:
	// 		xor_val = xor_val ^ int(R, 2)
	// 	epoch_randomness = ("{:0" + str(r) +  "b}").format(xor_val)

	// recompute PoW
	PK := identityobj.PK
	IP := identityobj.IP
	nonce := PoW["nonce"].(int)

	digest := sha256.New()
	digest.Write([]byte(IP))
	digest.update(PK.encode())
	digest.update([]byte(epoch_randomness))
	digest.update(str(nonce).encode())
	hash_val = digest.hexdigest()
	if hash_val.startswith('0'*D) && hash_val == PoW["hash"] {
		// Found a valid Pow, If this doesn't match with PoW["hash"] then Doesnt verify!
		return true
	}
	return false
}

func (e *Elastico) Finalpbft_process_message(msg map[string]interface{}) {
	/*
		Process the messages related to Pbft!
	*/
	if msg["type"] == "Finalpre-prepare" {
		self.process_Finalpre_prepareMsg(msg)

	} else if msg["type"] == "Finalprepare" {
		self.process_FinalprepareMsg(msg)

	} else if msg["type"] == "Finalcommit" {
		self.process_FinalcommitMsg(msg)
	}
}

func (e *Elastico) process_commitMsg(msg map[string]interface{}) {
	/*
		process the commit msg
	*/
	// verify the commit message
	verified := e.verify_commit(msg)
	if verified {

		// Log the commit msgs!
		e.log_commitMsg(msg)
	}
}

func (e *Elastico) process_FinalcommitMsg(msg map[string]interface{}) {
	/*
		process the final commit msg
	*/
	// verify the commit message
	verified := e.verify_commit(msg)
	if verified {

		// Log the commit msgs!
		e.log_FinalcommitMsg(msg)
	}
}

func (e *Elastico) process_prepareMsg(msg map[string]interface{}) {
	/*
		process prepare msg
	*/
	// verify the prepare message
	verified := e.verify_prepare(msg)
	if verified {

		// Log the prepare msgs!
		e.log_prepareMsg(msg)
	}
}

func (e *Elastico) process_FinalprepareMsg(msg map[string]interface{}) {
	/*
		process final prepare msg
	*/
	// verify the prepare message
	verified := e.verify_Finalprepare(msg)
	if verified {

		// Log the prepare msgs!
		e.log_FinalprepareMsg(msg)
	}
}

func (e *Elastico) BroadcastR() {
	/*
		broadcast Ri to all the network, final member will do this
	*/
	if e.isFinalMember() {
		data := make(map[string]interface{})
		data["Ri"] = e.Ri
		data["identity"] = e.identity
		msg := make(map[string]interface{})
		msg["data"] = data
		msg["type"] = "RandomStringBroadcast"

		e.state = Elastico_States["BroadcastedR"]
		BroadcastTo_Network(data, "RandomStringBroadcast")

	} else {
		log.Error("non final member broadcasting R")
	}
}

func (e *Elastico) generate_randomstrings() {
	/*
		Generate r-bit random strings
	*/
	if e.isFinalMember() {
		Ri := random_gen(r)
		e.Ri = fmt.Sprintf("%0"+strconv.FormatInt(r, 10)+"b\n", Ri)
	}
}

func (e *Elastico) notify_finalCommittee() {
	/*
		notify the members of the final committee that they are the final committee members
	*/
	finalCommList := e.committee_list[fin_num]
	for _, finalMember := range finalCommList {
		data := mske(map[string]interface{})
		data["identity"] = e.identity
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
	if e.isFinalMember() {

		if e.Ri == "" {
			e.generate_randomstrings()
		}
		commitment := sha256.New()
		commitment.Write([]byte(e.Ri))
		hash_val := fmt.Sprintf("%x", commitment.Sum(nil))
		return hash_val
	}
}

func (e *Elastico) executeReset() {
	/*
		call for reset
	*/
	logging.warning("call for reset for ", e.port)
	if isinstance(e.identity, Identity) {

		// if node has formed its identity
		msg := make(map[string]interface{})
		msg["type"] = "reset-all"
		msg["data"] = e.identity
		e.identity.send(msg)
	} else {

		// this node has not computed its identity,calling reset explicitly for node
		e.reset()
	}
}

func (e *Elastico) execute(epochTxn map[int64][]Transaction) {
	/*
		executing the functions based on the running state
	*/
	// # print the current state of node for debug purpose
	// 		print(e.identity ,  list(Elastico_States.keys())[ list(Elastico_States.values()).index(e.state)], "STATE of a committee member")

	// initial state of elastico node
	if e.state == Elastico_States["NONE"] {

		e.executePoW()

	} else if e.state == Elastico_States["PoW Computed"] {

		// form identity, when PoW computed
		e.form_identity()
	} else if e.state == Elastico_States["Formed Identity"] {

		// form committee, when formed identity
		e.form_committee()

	} else if e.is_directory && e.state == Elastico_States["RunAsDirectory"] {

		log.Info("The directory member :- ", e.port)
		e.receiveTxns(epochTxn)
		// directory member has received the txns for all committees
		e.state = Elastico_States["RunAsDirectory after-TxnReceived"]

	} else if e.state == Elastico_States["Receiving Committee Members"] {
		// when a node is part of some committee
		if e.flag == false {

			// logging the bad nodes
			logging.error("member with invalid POW %s with commMembers : %s", e.identity, e.committee_Members)
		}
		// Now The node should go for Intra committee consensus
		// initial state for the PBFT
		e.state = Elastico_States["PBFT_NONE"]
		// run PBFT for intra-committee consensus
		e.runPBFT()

	} else if e.state == Elastico_States["PBFT_NONE"] || e.state == Elastico_States["PBFT_PRE_PREPARE"] || e.state == Elastico_States["PBFT_PREPARE_SENT"] || e.state == Elastico_States["PBFT_PREPARED"] || e.state == Elastico_States["PBFT_COMMIT_SENT"] || e.state == Elastico_States["PBFT_PRE_PREPARE_SENT"] {

		// run pbft for intra consensus
		e.runPBFT()
	} else if e.state == Elastico_States["PBFT_COMMITTED"] {

		// send pbft consensus blocks to final committee members
		log.Info("pbft finished by members %s", str(e.port))
		e.SendtoFinal()

	} else if e.isFinalMember() && e.state == Elastico_States["Intra Consensus Result Sent to Final"] {

		// final committee node will collect blocks and merge them
		e.checkCountForConsensusData()

	} else if e.isFinalMember() && e.state == Elastico_States["Merged Consensus Data"] {

		// final committee member runs final pbft
		e.state = Elastico_States["FinalPBFT_NONE"]
		e.runFinalPBFT()

	} else if e.state == Elastico_States["FinalPBFT_NONE"] || e.state == Elastico_States["FinalPBFT_PRE_PREPARE"] || e.state == Elastico_States["FinalPBFT_PREPARE_SENT"] || e.state == Elastico_States["FinalPBFT_PREPARED"] || e.state == Elastico_States["FinalPBFT_COMMIT_SENT"] || e.state == Elastico_States["FinalPBFT_PRE_PREPARE_SENT"] {

		e.runFinalPBFT()

	} else if e.isFinalMember() && e.state == Elastico_States["FinalPBFT_COMMITTED"] {

		// send the commitment to other final committee members
		e.sendCommitment()
		log.Warn("pbft finished by final committee %s", str(e.port))

	} else if e.isFinalMember() && e.state == Elastico_States["CommitmentSentToFinal"] {

		// broadcast final txn block to ntw
		if len(e.commitments) >= c/2+1 {
			e.BroadcastFinalTxn()
		}
	} else if e.state == Elastico_States["FinalBlockReceived"] {

		e.checkCountForFinalData()

	} else if e.isFinalMember() && e.state == Elastico_States["FinalBlockSentToClient"] {

		// broadcast Ri is done when received commitment has atleast c/2  + 1 signatures
		if len(e.newRcommitmentSet) >= c/2+1 {
			e.BroadcastR()
		}
	} else if e.state == Elastico_States["ReceivedR"] {

		e.appendToLedger()
		e.state = Elastico_States["LedgerUpdated"]

	} else if e.state == Elastico_States["LedgerUpdated"] {

		// Now, the node can be reset
		return "reset"
	}
}

func (e *Elastico) Send_to_Directory() {
	/*
		Send about new nodes to directory committee members
	*/
	// Add the new processor in particular committee list of directory committee nodes
	for _, nodeId := range e.cur_directory {
		msg := make(map[string]interafce{})
		msg["data"] = e.identity
		msg["type"] = "newNode"
		nodeId.send(msg)
	}
}

func (e *Elastico) receiveTxns(epochTxn map[int][]Transaction) {
	/*
		directory node will receive transactions from client
	*/

	// Receive txns from client for an epoch
	k := 0
	numOfCommittees := int(math.Pow(2, float64(s)))
	num = len(epochTxn) / numOfCommittees // Transactions per committee
	// loop in sorted order of committee ids
	for iden := 0; iden < numOfCommittees; iden++ {
		if iden == numOfCommittees-1 {
			// give all the remaining txns to the last committee
			e.txn[iden] = epochTxn[k:]
		} else {
			e.txn[iden] = epochTxn[k : k+num]
		}
		k = k + num
	}
}

func (e *Elastico) pbft_process_message(msg map[string]interface{}) {
	/*
		Process the messages related to Pbft!
	*/
	if msg["type"] == "pre-prepare" {

		e.process_pre_prepareMsg(msg)

	} else if msg["type"] == "prepare" {

		e.process_prepareMsg(msg)

	} else if msg["type"] == "commit" {
		e.process_commitMsg(msg)
	}
}

func (e *Elastico) process_pre_prepareMsg(msg map[string]interface{}) {
	/*
		Process Pre-Prepare msg
	*/
	// verify the pre-prepare message
	verified := e.verify_pre_prepare(msg)
	if verified {
		// Log the pre-prepare msgs!
		e.logPre_prepareMsg(msg)

	} else {
		log.Error("error in verification of process_pre_prepareMsg")
	}
}

func (e *Elastico) process_Finalpre_prepareMsg(msg map[string]interface{}) {
	/*
		Process Final Pre-Prepare msg
	*/

	// verify the Final pre-prepare message
	verified := e.verify_Finalpre_prepare(msg)
	if verified {

		// Log the final pre-prepare msgs!
		e.logFinalPre_prepareMsg(msg)

	} else {
		log.Error("error in verification of Final process_pre_prepareMsg")
	}
}

func (e *Elastico) is_pre_prepared() bool {
	/*
		if the node received the pre-prepare msg from the primary
	*/
	return len(e.pre_prepareMsgLog) > 0
}

func (e *Elastico) is_Finalpre_prepared() bool {

	return len(e.Finalpre_prepareMsgLog) > 0
}

func makeMalicious() {
	/*
		make some nodes malicious who will compute wrong PoW
	*/
	malicious_count := 0
	for i := 0; i < malicious_count; i++ {
		randomNum := random_gen(32).Int64() // converting random num big.Int to Int64
		badNodeIndex := randomNum % n
		// set the flag false for bad nodes
		network_nodes[badNodeIndex].flag = false
	}
}

func makeFaulty() {
	/*
		make some nodes faulty who will stop participating in the protocol after sometime
	*/
	// making some(4 here) nodes as faulty
	faulty_count := 0
	for i := 0; i < faulty_count; i++ {
		randomNum := random_gen(32).Int64() // converting random num big.Int to Int64
		faultyNodeIndex := randomNum % n
		// set the flag false for bad nodes
		network_nodes[faultyNodeIndex].faulty = true
	}
}

func (e *Elastico) verify_commit(msg map[string]interface{}) {
	/*
		verify commit msgs
	*/
	// verify Pow
	if !e.verify_PoW(msg["identity"]) {
		return false
	}
	// verify signatures of the received msg
	if !e.verify_sign(msg["sign"], msg["commitData"], msg["identity"].PK) {
		return false
	}

	// check the view is same or not
	if msg["commitData"]["viewId"] != e.viewId {
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
	failOnError(err, "Failed to open a channel")
	// close the channel
	defer channel.Close()

	nodeport := strconv.Itoa(e.port)
	queueName := "hello" + nodeport
	// count the number of messages that are in the queue
	Queue, err := channel.QueueInspect(queueName)

	data := make(map[string]interface{})

	failOnError(err, "error in inspect")
	// consume all the messages one by one
	for ; Queue.Messages > 0; Queue.Messages-- {

		// get the message from the queue
		msg, ok, err := channel.Get(queueName, true)
		failOnError(err, "error in get of queue")
		if ok {
			err := json.Unmarshal(msg.Body, &data)
			failOnError(err, "error in unmarshall")
			// consume the msg by taking the action in receive
			e.receive(data)
		}
	}
}

func executeSteps(nodeIndex int64, epochTxns map[int][]Transaction, sharedObj map[int]bool) {
	/*
		A process will execute based on its state and then it will consume
	*/
	node := network_nodes[nodeIndex]
	for epoch, epochTxn := range epochTxns {
		// epochTxn holds the txn for the current epoch

		// delete the entry of the node in sharedobj for the next epoch
		if _, ok := sharedobj[nodeIndex]; ok {
			delete(sharedObj, nodeIndex)
		}

		// startTime = time.time()
		for {

			// execute one step of elastico node, execution of a node is done only when it has not done reset
			if _, ok := sharedobj[nodeIndex]; ok == false {

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
			if len(sharedObj) == n {
				break
			}
			// process consume the msgs from the queue
			node.consumeMsg()
		}
		// Ensuring that all nodes are reset and sharedobj is not affected
		// time.sleep(60)
	}
}

func createTxns() []Transaction {
	/*
		create txns for an epoch
	*/
	numOfTxns := 20 // number of transactions in each epoch
	// txns is the list of the transactions in one epoch to which the committees will agree on
	txns := make([]Transaction, numOfTxns)
	for i := 0; i < numOfTxns; i++ {
		random_num := random_gen(32)                     // random amount
		transaction := Transaction{"a", "b", random_num} // create the dummy transaction
		txns[i] = transaction
	}
	return txns
}

func main() {
	// delete the file
	os.Remove("logfile.log")
	// open the logging file
	file, err := os.OpenFile("logfile.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	failOnError(err, "opening file error") // report the open file error
	numOfEpochs := 2                       // num of epochs
	epochTxns := make(map[int][]Transaction)
	for epoch := 0; epoch < numOfEpochs; epoch++ {
		epochTxns[epoch] = createTxns()
	}

	log.SetOutput(file)
	log.SetLevel(log.InfoLevel) // set the log level

	// run all the epochs
	Run(epochTxns)

}
