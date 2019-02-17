package main

import (
	"fmt"
	"crypto/rsa"
	// "crypto/sha256"
	"crypto/rand"
	// "reflect"
	"math/big"
	"strconv"
	"github.com/streadway/amqp" // for rabbitmq
	log "github.com/sirupsen/logrus" // for logging
	"os"
	"encoding/json"
)

var ELASTICO_STATES = map[string]int{"NONE": 0, "PoW Computed": 1, "Formed Identity" : 2, "Formed Committee": 3, "RunAsDirectory": 4 ,"RunAsDirectory after-TxnReceived" : 5,  "RunAsDirectory after-TxnMulticast" : 6, "Receiving Committee Members" : 7,"PBFT_NONE" : 8 , "PBFT_PRE_PREPARE" : 9, "PBFT_PRE_PREPARE_SENT"  : 10, "PBFT_PREPARE_SENT" : 11, "PBFT_PREPARED" : 12, "PBFT_COMMITTED" : 13, "PBFT_COMMIT_SENT" : 14,  "Intra Consensus Result Sent to Final" : 15,  "Merged Consensus Data" : 16, "FinalPBFT_NONE" : 17,  "FinalPBFT_PRE_PREPARE" : 18, "FinalPBFT_PRE_PREPARE_SENT"  : 19,  "FinalPBFT_PREPARE_SENT" : 20 , "FinalPBFT_PREPARED" : 21, "FinalPBFT_COMMIT_SENT" : 22, "FinalPBFT_COMMITTED" : 23, "PBFT Finished-FinalCommittee" : 24 , "CommitmentSentToFinal" : 25, "FinalBlockSent" : 26, "FinalBlockReceived" : 27,"BroadcastedR" : 28, "ReceivedR" :  29, "FinalBlockSentToClient" : 30,   "LedgerUpdated" : 31}

// structure for identity of nodes
type Identity struct{
	IP string
	PK *rsa.PublicKey
	committee_id int64
	PoW map[string]interface{}
	epoch_randomness string
	port int
}

func (i *Identity) IdentityInit(){
	i.PoW = make(map[string]interface{})
}


func (i *Identity)isEqual(identityobj *Identity) bool{
	/*
		checking two objects of Identity class are equal or not
		
	*/
	return i.IP == identityobj.IP && i.PK == identityobj.PK && i.committee_id == identityobj.committee_id && i.PoW["hash"] == identityobj.PoW["hash"] && i.PoW["set_of_Rs"] == identityobj.PoW["set_of_Rs"] && i.PoW["nonce"] == identityobj.PoW["nonce"] &&i.epoch_randomness == identityobj.epoch_randomness && i.port == identityobj.port
}


func(i *Identity)send(msg map[string]interface{}){
	/*
		send the msg to node based on their identity
	*/
	// establish a connection with RabbitMQ server
	connection , err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	// report the error
	failOnError(err, "Failed to connect to RabbitMQ")
	// close the connection
	defer connection.Close()
	// create a channel
	ch, er := connection.Channel()
	// report the error
	failOnError(er, "Failed to open a channel")
	// close the channel
	defer ch.Close()
	port := strconv.Itoa(i.port)

	//create a hello queue to which the message will be delivered
	queue, err := ch.QueueDeclare(
		"hello" + port,	//name of the queue
		false,	// durable
		false,	// delete when unused
		false,	// exclusive
		false,	// no-wait
		nil,	// arguments
	)
	failOnError(err, "Failed to declare a queue")

	body, err := json.Marshal(msg)
	failOnError(err, "Failed to marshal")
	err = ch.Publish(
		"",				// exchange
		queue.Name,		// routing key
		false,			// mandatory
		false,			// immediate
		amqp.Publishing {
		ContentType: "text/plain",
		Body:		body,
	})

	failOnError(err, "Failed to publish a message")
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

func random_gen(r int64) (*big.Int) {
	/*
		generate a random integer
	*/
	// n is the base, e is the exponent, creating big.Int variables
	var n,e = big.NewInt(2) , big.NewInt(r)
	// taking the exponent n to the power e and nil modulo, and storing the result in n
	n.Exp(n, e, nil)
	// generates the random num in the range[0,n)
	// here Reader is a global, shared instance of a cryptographically secure random number generator.
	randomNum, err := rand.Int(rand.Reader, n)

	if err != nil {
		fmt.Println("error:", err.Error)
	}
	return randomNum
}


func main() {
	os.Remove("rus.log")
	file, _ := os.OpenFile("rus.log",  os.O_CREATE|os.O_APPEND | os.O_WRONLY , 0666)

	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()
	
	log.SetOutput(file)
    log.SetLevel(log.InfoLevel)
    
}
