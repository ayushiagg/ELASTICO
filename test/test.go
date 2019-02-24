package main

import (
	"crypto/rsa"
	"fmt"

	// "crypto/sha256"
	"crypto/rand"
	// "reflect"
	"encoding/json"
	"math/big"
	"os"
	"strconv"

	log "github.com/sirupsen/logrus" // for logging
	"github.com/streadway/amqp"      // for rabbitmq
)

var ELASTICO_STATES = map[string]int{"NONE": 0, "PoW Computed": 1, "Formed Identity": 2, "Formed Committee": 3, "RunAsDirectory": 4, "RunAsDirectory after-TxnReceived": 5, "RunAsDirectory after-TxnMulticast": 6, "Receiving Committee Members": 7, "PBFT_NONE": 8, "PBFT_PRE_PREPARE": 9, "PBFT_PRE_PREPARE_SENT": 10, "PBFT_PREPARE_SENT": 11, "PBFT_PREPARED": 12, "PBFT_COMMITTED": 13, "PBFT_COMMIT_SENT": 14, "Intra Consensus Result Sent to Final": 15, "Merged Consensus Data": 16, "FinalPBFT_NONE": 17, "FinalPBFT_PRE_PREPARE": 18, "FinalPBFT_PRE_PREPARE_SENT": 19, "FinalPBFT_PREPARE_SENT": 20, "FinalPBFT_PREPARED": 21, "FinalPBFT_COMMIT_SENT": 22, "FinalPBFT_COMMITTED": 23, "PBFT Finished-FinalCommittee": 24, "CommitmentSentToFinal": 25, "FinalBlockSent": 26, "FinalBlockReceived": 27, "BroadcastedR": 28, "ReceivedR": 29, "FinalBlockSentToClient": 30, "LedgerUpdated": 31}

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
	connection, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
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
		"hello"+port, //name of the queue
		false,        // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	failOnError(err, "Failed to declare a queue")

	body, err := json.Marshal(msg)
	failOnError(err, "Failed to marshal")
	err = ch.Publish(
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

type Elastico struct {
	key *rsa.PrivateKey
	IP  string
	cur []Identity
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func random_gen(r int64) *big.Int {
	/*
		generate a random integer
	*/
	// n is the base, e is the exponent, creating big.Int variables
	var n, e = big.NewInt(2), big.NewInt(r)
	// taking the exponent n to the power e and nil modulo, and storing the result in n
	n.Exp(n, e, nil)
	// generates the random num in the range[0,n)
	// here Reader is a global, shared instance of a cryptographically secure random number generator.
	randomNum, _ := rand.Int(rand.Reader, n)
	return randomNum
}

func main() {
	os.Remove("rus.log")
	file, _ := os.OpenFile("rus.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)

	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	h := make(map[string]interface{})
	h["dsadsdfsds"] = "dfawhuwguwerwghftuquiqejeiqw"
	msg1 := make(map[string]interface{})
	msg1["data"] = "nidwghhhhhhhhhhhhhhhhyerrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrhhhhhhhhhhhhhhhhhhhhhhhhhhh"
	msg1["features"] = h
	i := Identity{}
	i.send(msg1)

	defer conn.Close()

	log.SetOutput(file)
	log.SetLevel(log.InfoLevel)

	// create a channel
	channel, er := conn.Channel()
	failOnError(er, "Failed to open a channel")
	// close the channel
	defer channel.Close()
	nodeport := strconv.Itoa(0)
	queueName := "hello" + nodeport
	// count the number of messages that are in the queue
	Queue, err := channel.QueueInspect(queueName)
	msg2 := make(map[string]interface{})
	fmt.Println(Queue)
	// consume all the messages one by one
	for ; Queue.Messages > 0; Queue.Messages-- {

		// get the message from the queue
		msg, ok, _ := channel.Get(queueName, true)
		fmt.Println(msg, ok)
		if ok {

			_ = json.Unmarshal(msg.Body, &msg2)
			// consume the msg by taking the action in receive
			fmt.Println(msg2, "see")
		}
	}
}

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"math/big"
)

type msgType struct {
	Data json.RawMessage
	Type string
}

func main() {
	fmt.Println("Start!")
	rsakey, errKey := rsa.GenerateKey(rand.Reader, 2048)
	if errKey == nil {
		publicKey := rsakey.Public()               // get the public key from the rsa key
		rsaPublicKey := publicKey.(*rsa.PublicKey) // convert to rsa publick key type
		// fmt.Println("N : ", rsaPublicKey.N)
		// fmt.Println("E : ", rsaPublicKey.E)
		// send N
		msg := map[string]interface{}{"data": rsaPublicKey.N, "type": "publicKey"}
		encoded, errMarshal := json.Marshal(msg) // encode the N param
		if errMarshal == nil {
			var decoded msgType
			errUnmarshall := json.Unmarshal(encoded, &decoded)
			if errUnmarshall == nil && decoded.Type == "publicKey" {
				receivedPublic := big.NewInt(0)
				errUnmarshallkey := json.Unmarshal(decoded.Data, &receivedPublic)
				if errUnmarshallkey == nil {
					fmt.Println(receivedPublic.Cmp(rsaPublicKey.N) == 0) // To check got the correct N or not
				} else {
					fmt.Println("Error : ", errUnmarshallkey)
				}
			} else {
				fmt.Println("UnMarshal error", errUnmarshall)
			}
		} else {
			fmt.Println("Marshal error", errMarshal)
		}
	} else {
		fmt.Println("Rsa key generation Error!", errKey)
	}
}

