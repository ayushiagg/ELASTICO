package main

import (
	"fmt"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/rand"
	"reflect"
	"math/big"
	"strconv"
	"github.com/streadway/amqp" // for rabbitmq
	"log"
)

var ELASTICO_STATES = map[string]int{"NONE": 0, "PoW Computed": 1, "Formed Identity" : 2, "Formed Committee": 3, "RunAsDirectory": 4 ,"RunAsDirectory after-TxnReceived" : 5,  "RunAsDirectory after-TxnMulticast" : 6, "Receiving Committee Members" : 7,"PBFT_NONE" : 8 , "PBFT_PRE_PREPARE" : 9, "PBFT_PRE_PREPARE_SENT"  : 10, "PBFT_PREPARE_SENT" : 11, "PBFT_PREPARED" : 12, "PBFT_COMMITTED" : 13, "PBFT_COMMIT_SENT" : 14,  "Intra Consensus Result Sent to Final" : 15,  "Merged Consensus Data" : 16, "FinalPBFT_NONE" : 17,  "FinalPBFT_PRE_PREPARE" : 18, "FinalPBFT_PRE_PREPARE_SENT"  : 19,  "FinalPBFT_PREPARE_SENT" : 20 , "FinalPBFT_PREPARED" : 21, "FinalPBFT_COMMIT_SENT" : 22, "FinalPBFT_COMMITTED" : 23, "PBFT Finished-FinalCommittee" : 24 , "CommitmentSentToFinal" : 25, "FinalBlockSent" : 26, "FinalBlockReceived" : 27,"BroadcastedR" : 28, "ReceivedR" :  29, "FinalBlockSentToClient" : 30,   "LedgerUpdated" : 31}

type Identity struct{
	IP string
	PK string
	committee_id int
	PoW map[string]interface{}
	epoch_randomness string
	port int
}

type Elastico struct{
	key *rsa.PrivateKey
	IP string
	cur []Identity
}

func failOnError(err error, msg string) {
  if err != nil {
    log.Fatalf("%s: %s", msg, err)
  }
}

func get_committeeid(PoW string){
	/*
		returns last s-bit of PoW["hash"] as Identity : committee_id
	*/ 
	s := 4
	bindigest := ""
	for i:=0 ; i < len(PoW); i++ {
		intVal, _ := strconv.ParseUint( string(PoW[i]) ,16,0)
		bindigest += fmt.Sprintf("%04b", intVal)
	}
	fmt.Println(bindigest)
	identity := bindigest[len(bindigest)-s:]
	fmt.Println(identity)
	iden, _ := strconv.ParseUint(identity, 2 , 0)
	fmt.Println(iden)
}
func main() {
	e:= Elastico{}
	var err error 
	e.key,err = rsa.GenerateKey(rand.Reader, 2048)
	if err!= nil{
		fmt.Println(e.key)
	}
	fmt.Println(reflect.TypeOf(e.key))

	c := 4
	i := make([]byte, c)
	_, err = rand.Read(i)
	if err != nil {
		fmt.Println("error:", err.Error)
	}
	e.IP= fmt.Sprintf("%v.%v.%v.%v" , i[0] , i[1], i[2], i[3])
	fmt.Println(e.IP)
	n:= 2345
	str:= strconv.Itoa(n)
	var prime1, _ = new(big.Int).SetString(str, 10)
	// Generate random numbers in range [0..prime1]
	// Ignore error values
	x, _ := rand.Int(rand.Reader, prime1)

	mape := make(map[string]interface{})
	mape["hash"] = ""
	mape["set_of_Rs"] = ""
	mape["nonce"] = 0
	fmt.Printf("x: %v\n", x)

	a:= make(map[int]map[string]string)
	a[0] = make(map[string]string)
	a[0]["d"] = "dfsgf"
	a[0]["de"] = "dfsgf"

	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()
	digest := sha256.New()
	digest.Write([]byte("051"))

	p := digest.Sum(nil)

	hash_val := string(p)

	hash := ""
	for i := 0; i < len(p); i++{
		hash += string(p[i])
	}
	// fmt.Println(hash_val)
	// fmt.Println(hash)
	lelo := fmt.Sprintf("%x", p)
	get_committeeid(hash)
	get_committeeid(hash_val)
	get_committeeid(lelo)
}
