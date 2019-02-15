package main

import (
	"fmt"
	"crypto/sha256"
	"crypto/rsa"
	"crypto/rand"
	"math/big"
	"strconv"
	"strings"
	// "reflect"
	"sync"
	"github.com/streadway/amqp" // for rabbitmq
	"log"
)

// ELASTICO_STATES - states reperesenting the running state of the node
var ELASTICO_STATES = map[string]int{"NONE": 0, "PoW Computed": 1, "Formed Identity" : 2, "Formed Committee": 3, "RunAsDirectory": 4 ,"RunAsDirectory after-TxnReceived" : 5,  "RunAsDirectory after-TxnMulticast" : 6, "Receiving Committee Members" : 7,"PBFT_NONE" : 8 , "PBFT_PRE_PREPARE" : 9, "PBFT_PRE_PREPARE_SENT"  : 10, "PBFT_PREPARE_SENT" : 11, "PBFT_PREPARED" : 12, "PBFT_COMMITTED" : 13, "PBFT_COMMIT_SENT" : 14,  "Intra Consensus Result Sent to Final" : 15,  "Merged Consensus Data" : 16, "FinalPBFT_NONE" : 17,  "FinalPBFT_PRE_PREPARE" : 18, "FinalPBFT_PRE_PREPARE_SENT"  : 19,  "FinalPBFT_PREPARE_SENT" : 20 , "FinalPBFT_PREPARED" : 21, "FinalPBFT_COMMIT_SENT" : 22, "FinalPBFT_COMMITTED" : 23, "PBFT Finished-FinalCommittee" : 24 , "CommitmentSentToFinal" : 25, "FinalBlockSent" : 26, "FinalBlockReceived" : 27,"BroadcastedR" : 28, "ReceivedR" :  29, "FinalBlockSentToClient" : 30,   "LedgerUpdated" : 31}

// shared lock among processes
var lock sync.Mutex

var port uint = 49152

// n : number of nodes
var n int = 66 
// s - where 2^s is the number of committees
var s int = 2
// c - size of committee
var c int = 4
// D - difficulty level , leading bits of PoW must have D 0's (keep w.r.t to hex)
var D int = 6
// r - number of bits in random string
var r int64 = 4
// fin_num - final committee id
var fin_num int = 0

func failOnError(err error, msg string) {
	// logging the error
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}


func random_gen(r int64) (*big.Int) {
	/*
		generate a random integer
	*/
	// n is the base, e is the exponent, creating big.Int variables
	var n,e = big.NewInt(2) , big.NewInt(r)
	// taking the exponent n to the power e, and storing the result in n
	n.Exp(n, e, nil)
	// generates the random num in the range[0,n)
	randomNum, err := rand.Int(rand.Reader, n)

	if err != nil {
		fmt.Println("error:", err.Error)
	}
	return randomNum
}


type Identity struct{
	IP string
	PK string
	committee_id int
	PoW map[string]interface{}
	epoch_randomness string
	port uint
}

func (e *Elastico)get_committeeid int{
	/*
		returns last s-bit of PoW["hash"] as Identity : committee_id
	*/ 
	bindigest = ''
	for hashdig in PoW:
		bindigest += "{:04b}".format(int(hashdig, 16))
	identity = bindigest[-s:]
	return int(identity, 2)
}

func (e *Elastico)form_identity{
	/*
		identity formation for a node
		identity consists of public key, ip, committee id, PoW, nonce, epoch randomness
	*/	
	if e.state == ELASTICO_STATES["PoW Computed"]{
		// export public key
		PK := e.key.Public()

		// set the committee id acc to PoW solution
		e.committee_id = e.get_committeeid(e.PoW["hash"])

		e.identity = Identity{e.IP, PK, e.committee_id, e.PoW, e.epoch_randomness,e.port}
		// changed the state after identity formation
		e.state = ELASTICO_STATES["Formed Identity"]
		return e.identity
	}
}


type Transaction struct{
	sender string
	receiver string
	amount float32	
}

type Elastico struct{

	connection *amqp.Connection
	IP string
	port uint
	key *rsa.PrivateKey
	PoW map[string]interface{}
	cur_directory []Identity
	identity Identity
	committee_id int
	// only when this node is the member of directory committee
	committee_list map[int][]Identity
	// only when this node is not the member of directory committee
	committee_Members []Identity
	is_directory bool
	is_final bool
	epoch_randomness string
	Ri string
	// only when this node is the member of final committee
	commitments map[string]bool
	txn_block []Transaction
	set_of_Rs map[string]bool
	newset_of_Rs map[string]bool
	CommitteeConsensusData map[int]map[string][]string
	CommitteeConsensusDataTxns map[int]map[string][]Transaction
	finalBlockbyFinalCommittee map[int]map[string][]string
	finalBlockbyFinalCommitteeTxns map[int]map[string][]Transaction
	state int
	mergedBlock []Transaction
	finalBlock map[string]interface{}
	RcommitmentSet map[string]bool
	newRcommitmentSet map[string]bool
	finalCommitteeMembers []Identity
	// only when this is the member of the directory committee
	txn map[int][]Transaction
	response []Transaction
	flag bool
	views map[int]bool
	primary bool
	viewId int
	faulty bool
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


func (e *Elastico) get_key(){
	/*
		for each node, it will set key as public pvt key pair
	*/
	var err error
	// generate the public-pvt key pair
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
	// construct the byte array of size 4
	byteArray := make([]byte, count)
	// Assigning random values to the byte array
	_, err := rand.Read(byteArray)
	if err != nil {
		fmt.Println("error:", err.Error)
	}
	// setting the IP addr from the byte array
	e.IP = fmt.Sprintf("%v.%v.%v.%v" , byteArray[0] , byteArray[1], byteArray[2], byteArray[3])
}


func (e *Elastico) initER(){
		/*
			initialise r-bit epoch random string
		*/

		randomnum := random_gen(r)
		// set r-bit binary string to epoch randomness
		e.epoch_randomness = fmt.Sprintf("%0"+ strconv.FormatInt(r, 10) + "b\n", randomnum)
}

func (e *Elastico)get_port(){

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

			
		// if self.state == ELASTICO_STATES["NONE"]:
		// 	PK = self.key.publickey().exportKey().decode()
		// 	IP = self.IP
		// 	

func (e* Elastico) compute_PoW(){
	/*	
		returns hash which satisfies the difficulty challenge(D) : PoW["hash"]
	*/
	zero_string := ""
	for i:=0 ; i < D; i ++ {
		zero_string += "0"
	}
	nonce :=  e.PoW["nonce"].(int)
	if e.state == ELASTICO_STATES["NONE"] {
		// public key
		PK := e.key.Public()
		rsaPublickey, _ := PK.(*rsa.PublicKey)
		// fmt.Println("%T", reflect.TypeOf(PK) )
		IP := e.IP
		// If it is the first epoch , randomset_R will be an empty set .
		// otherwise randomset_R will be any c/2 + 1 random strings Ri that node receives from the previous epoch
		randomset_R := make(map[string]bool)
		if len(e.set_of_Rs) > 0 {
			// e.epoch_randomness, randomset_R = e.xor_R()
		}
		// 	compute the digest 
		digest := sha256.New()
		digest.Write([]byte(IP))
		digest.Write(rsaPublickey.N.Bytes())
		digest.Write([]byte(strconv.Itoa(rsaPublickey.E)))
		digest.Write([]byte(e.epoch_randomness))
		digest.Write([]byte(strconv.Itoa(nonce)))

		hash_val := fmt.Sprintf("%x" , digest.Sum(nil))
		if strings.HasPrefix(hash_val, zero_string){
			//hash starts with leading D 0's
			e.PoW["hash"] = hash_val
			e.PoW["set_of_Rs"] =  randomset_R
			e.PoW["nonce"] = nonce
			// change the state after solving the puzzle
			e.state = ELASTICO_STATES["PoW Computed"]
		} else {
			// try for other nonce
			nonce += 1 
			e.PoW["nonce"] = nonce
		}
	}
}



func (e* Elastico) init() {
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
	
	e.committee_list = make(map[int][]Identity)

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

	e.state = ELASTICO_STATES["NONE"]

	e.mergedBlock = make([]Transaction, 0)

	e.finalBlock = make(map[string]interface{})
	e.finalBlock["sent"] = false
	e.finalBlock["finalBlock"] = make([]Transaction,0)

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

func main(){
	e := &Elastico{}
	e.init()
	for e.state == 0{
		e.compute_PoW()
	}
	fmt.Println(e.state, e.PoW)
	// pk := e.key.Public()

	// rsaPublickey, _ := pk.(*rsa.PublicKey)
	// fmt.Println(rsaPublickey.N.Bytes())
	// fmt.Println(rsaPublickey.E.Bytes())

}