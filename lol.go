diff --git a/elastico.go b/elastico.go
index 5a3c45a..0ec82b5 100644
--- a/elastico.go
+++ b/elastico.go
@@ -35,8 +35,8 @@ var wg sync.WaitGroup
 // shared lock among processes
 var lock sync.Mutex
 
-// shared port among the processes
-var port = 49152
+// shared Port among the processes
+var Port = 49152
 
 // n : number of nodes
 var n int64 = 66
@@ -92,6 +92,30 @@ func getConnection() *amqp.Connection {
 	return connection
 }
 
+func (d directoryMemberMsg) MarshalJSON() ([]byte, error) {
+	return json.Marshal(d.identity)
+}
+
+func marshalData(msg map[string]interface{}) []byte {
+	if msg["type"] == "directoryMember" {
+		var data directoryMemberMsg
+		data = msg["data"].(directoryMemberMsg)
+		body1, _ := json.Marshal(data)
+		fmt.Println("bod1 marshall :- ", body1)
+		var data1 directoryMemberMsg
+		_ = json.Unmarshal(body1, &data1)
+		fmt.Println("bod1 unmarshall :- ", data1)
+	}
+	body, err := json.Marshal(msg)
+	// fmt.Println(body)
+	var d msgType
+	_ = json.Unmarshal(body, &d)
+	fmt.Println("msg : dd ", d)
+
+	failOnError(err, "Failed to marshal", true)
+	return body
+}
+
 func publishMsg(channel *amqp.Channel, queueName string, msg map[string]interface{}) {
 
 	//create a hello queue to which the message will be delivered
@@ -106,9 +130,8 @@ func publishMsg(channel *amqp.Channel, queueName string, msg map[string]interfac
 	failOnError(err, "Failed to declare a queue", true)
 
 	fmt.Println("publish msg", msg)
-	body, err := json.Marshal(msg)
-	fmt.Println(body)
-	failOnError(err, "Failed to marshal", true)
+	body := marshalData(msg)
+
 	err = channel.Publish(
 		"",         // exchange
 		queue.Name, // routing key
@@ -166,7 +189,7 @@ func MulticastCommittee(commList map[int64][]Identity, identityobj Identity, txn
 
 	// get the final committee members with the fixed committee id
 	finalCommitteeMembers := commList[finNum]
-	for committeeID, commMembers := range commList {
+	for CommitteeID, commMembers := range commList {
 
 		// find the primary identity, Take the first identity
 		// ToDo: fix this, many nodes can be primary
@@ -181,7 +204,7 @@ func MulticastCommittee(commList map[int64][]Identity, identityobj Identity, txn
 
 			// give txns only to the primary node
 			if memberID.isEqual(&primaryID) {
-				data["txns"] = txns[committeeID]
+				data["txns"] = txns[CommitteeID]
 			}
 			// construct the msg
 			msg := make(map[string]interface{})
@@ -195,11 +218,6 @@ func MulticastCommittee(commList map[int64][]Identity, identityobj Identity, txn
 
 // BroadcastToNetwork - Broadcast data to the whole ntw
 func BroadcastToNetwork(msg map[string]interface{}) {
-	// construct msg
-	// msg := make(map[string]interface{})
-	// msg["data"] = data
-	// msg["type"] = _type
-
 	connection := getConnection()
 	defer connection.Close() // close the connection
 
@@ -207,7 +225,7 @@ func BroadcastToNetwork(msg map[string]interface{}) {
 	defer channel.Close() // close the channel
 
 	for _, node := range networkNodes {
-		nodePort := strconv.Itoa(node.port)
+		nodePort := strconv.Itoa(node.Port)
 		queueName := "hello" + nodePort
 		publishMsg(channel, queueName, msg) //publish the message in queue
 	}
@@ -314,12 +332,12 @@ func (bd *BlockData) hexdigest() []byte {
 
 // Identity :- structure for identity of nodes
 type Identity struct {
-	IP              string
-	PK              bytes.Buffer
-	committeeID     int64
-	PoW             map[string]interface{}
-	epochRandomness string
-	port            int
+	IP              string                 `json:"ip"`
+	PK              bytes.Buffer           `json:"pk"`
+	CommitteeID     int64                  `json:"committeeid"`
+	PoW             map[string]interface{} `json:"pow"`
+	EpochRandomness string                 `json:"epochrandomness"`
+	Port            int                    `json:"Port"`
 }
 
 // IdentityInit :- initialise of Identity members
@@ -334,7 +352,7 @@ func (i *Identity) isEqual(identityobj *Identity) bool {
 	*/
 	iPK := i.getPK()
 	identityobjPK := identityobj.getPK()
-	return i.IP == identityobj.IP && iPK == identityobjPK && i.committeeID == identityobj.committeeID && i.PoW["hash"] == identityobj.PoW["hash"] && i.PoW["setOfRs"] == identityobj.PoW["setOfRs"] && i.PoW["nonce"] == identityobj.PoW["nonce"] && i.epochRandomness == identityobj.epochRandomness && i.port == identityobj.port
+	return i.IP == identityobj.IP && iPK == identityobjPK && i.CommitteeID == identityobj.CommitteeID && i.PoW["hash"] == identityobj.PoW["hash"] && i.PoW["setOfRs"] == identityobj.PoW["setOfRs"] && i.PoWE"nonce"] == identityobj.PoW["nonce"] && i.epochRandomness == identityobj.epochRandomness && i.Port == identityobj.Port
 }
 
 func (i *Identity) send(msg map[string]interface{}) {
@@ -350,7 +368,7 @@ func (i *Identity) send(msg map[string]interface{}) {
 	// close the channel
 	defer channel.Close()
 
-	nodePort := strconv.Itoa(i.port)
+	nodePort := strconv.Itoa(i.Port)
 	queueName := "hello" + nodePort
 	publishMsg(channel, queueName, msg) // publish the msg in queue
 }
@@ -400,11 +418,11 @@ type Elastico struct {
 	/*
 		connection - rabbitmq connection
 		IP - IP address of a node
-		port - unique number for a process
+		Port - unique number for a process
 		key - public key and private key pair for a node
 		PoW - dict containing 256 bit hash computed by the node, set of Rs needed for epoch randomness, and a nonce
 		cur_directory - list of directory members in view of the node
-		identity - identity consists of Public key, an IP, PoW, committee id, epoch randomness, port
+		identity - identity consists of Public key, an IP, PoW, committee id, epoch randomness, Port
 		committee_id - integer value to represent the committee to which the node belongs
 		committee_list - list of nodes in all committees
 		committee_Members - set of committee members in its own committee
@@ -444,19 +462,19 @@ type Elastico struct {
 	*/
 	connection   *amqp.Connection
 	IP           string
-	port         int
+	Port         int
 	key          *rsa.PrivateKey
 	PoW          map[string]interface{}
 	curDirectory []Identity
 	identity     Identity
-	committeeID  int64
+	CommitteeID  int64
 	// only when this node is the member of directory committee
 	committeeList map[int64][]Identity
 	// only when this node is not the member of directory committee
 	committeeMembers []Identity
 	isDirectory      bool
 	isFinal          bool
-	epochRandomness  string
+	EpochRandomness  string
 	Ri               string
 	// only when this node is the member of final committee
 	commitments                    map[string]bool
@@ -524,17 +542,17 @@ func (e *Elastico) initER() {
 
 	randomnum := randomGen(r)
 	// set r-bit binary string to epoch randomness
-	e.epochRandomness = fmt.Sprintf("%0"+strconv.FormatInt(r, 10)+"b\n", randomnum)
+	E.epochRandomness = fmt.Sprintf("%0"+strconv.FormatInt(r, 10)+"b\n", randomnum)
 }
 
 func (e *Elastico) getPort() {
 	/*
-		get port number for the process
+		get Port number for the process
 	*/
 	// acquire the lock
 	lock.Lock()
-	port++
-	e.port = port
+	Port++
+	e.Port = Port
 	// release the lock
 	defer lock.Unlock()
 }
@@ -591,14 +609,14 @@ func (e *Elastico) computePoW() {
 		// otherwise randomsetR will be any c/2 + 1 random strings Ri that node receives from the previous epoch
 		randomsetR := make([]string, 0)
 		if len(e.setOfRs) > 0 {
-			e.epochRandomness, randomsetR = e.xorR()
+			E.epochRandomness, randomsetR = e.xorR()
 		}
 		// 	compute the digest
 		digest := sha256.New()
 		digest.Write([]byte(IP))
 		digest.Write(rsaPublickey.N.Bytes())
 		digest.Write([]byte(strconv.Itoa(rsaPublickey.E)))
-		digest.Write([]byte(e.epochRandomness))
+		Eigest.Write([]byte(e.epochRandomness))
 		digest.Write([]byte(strconv.Itoa(nonce)))
 
 		hashVal := fmt.Sprintf("%x", digest.Sum(nil))
@@ -687,24 +705,24 @@ func (e *Elastico) receiveNewNode(msg msgType) {
 	identityobj := decodeMsg.data
 	// verify the PoW
 	if e.verifyPoW(identityobj) {
-		_, ok := e.committeeList[identityobj.committeeID]
+		_, ok := e.committeeList[identityobj.CommitteeID]
 		if ok == false {
 
 			// Add the identity in committee
-			e.committeeList[identityobj.committeeID] = []Identity{identityobj}
+			e.committeeList[identityobj.CommitteeID] = []Identity{identityobj}
 
-		} else if len(e.committeeList[identityobj.committeeID]) < c {
+		} else if len(e.committeeList[identityobj.CommitteeID]) < c {
 			// Add the identity in committee
 			flag := true
-			for _, obj := range e.committeeList[identityobj.committeeID] {
+			for _, obj := range e.committeeList[identityobj.CommitteeID] {
 				if identityobj.isEqual(&obj) {
 					flag = false
 					break
 				}
 			}
 			if flag {
-				e.committeeList[identityobj.committeeID] = append(e.committeeList[identityobj.committeeID], identityobj)
-				if len(e.committeeList[identityobj.committeeID]) == c {
+				e.committeeList[identityobj.CommitteeID] = append(e.committeeList[identityobj.CommitteeID], identityobj)
+				if len(e.committeeList[identityobj.CommitteeID]) == c {
 					// check that if all committees are full
 					e.checkCommitteeFull()
 				}
@@ -720,7 +738,7 @@ func (e *Elastico) receiveViews(msg map[string]interface{}) {
 	data := msg["data"].(map[string]interface{})
 	identityobj := data["identity"].(Identity)
 	// union of committe members views
-	e.views[identityobj.port] = true
+	e.views[identityobj.Port] = true
 
 	commMembers := data["committee members"].([]Identity)
 	finalMembers := data["final Committee members"].([]Identity)
@@ -731,7 +749,7 @@ func (e *Elastico) receiveViews(msg map[string]interface{}) {
 		// ToDo: txnblock should be ordered, not set
 		txns := data["txns"].([]Transaction)
 		e.txnBlock = e.unionTxns(e.txnBlock, txns)
-		log.Warn("I am primary", e.port)
+		log.Warn("I am primary", e.Port)
 		e.primary = true
 	}
 	// ToDo: verify this union thing
@@ -877,7 +895,7 @@ func (e *Elastico) BroadcastFinalTxn() {
 	// self.finalBlock["sent"] = True
 	// # A final node which is already in received state should not change its state
 	// if self.state != ELASTICO_STATES["FinalBlockReceived"]:
-	// 	logging.warning("change state to FinalBlockSent by %s" , str(self.port))
+	// 	logging.warning("change state to FinalBlockSent by %s" , str(self.Port))
 	// 	self.state = ELASTICO_STATES["FinalBlockSent"]
 	// BroadcastTo_Network(data, "finalTxnBlock")
 }
@@ -893,22 +911,22 @@ func (e *Elastico) receiveIntraCommitteeBlock(msg map[string]interface{}) {
 		// verify the signatures
 		PK := identityobj.getPK()
 		if e.verifySignTxnList(signature, TxnBlock, &PK) {
-			if _, ok := e.CommitteeConsensusData[identityobj.committeeID]; ok == false {
+			if _, ok := e.CommitteeConsensusData[identityobj.CommitteeID]; ok == false {
 
-				e.CommitteeConsensusData[identityobj.committeeID] = make(map[string][]string)
-				e.CommitteeConsensusDataTxns[identityobj.committeeID] = make(map[string][]Transaction)
+				e.CommitteeConsensusData[identityobj.CommitteeID] = make(map[string][]string)
+				e.CommitteeConsensusDataTxns[identityobj.CommitteeID] = make(map[string][]Transaction)
 			}
 			txnBlock := data["txnBlock"].([]Transaction)
 			TxnBlockDigest := txnHexdigest(txnBlock)
-			if _, ok := e.CommitteeConsensusData[identityobj.committeeID][TxnBlockDigest]; ok == false {
-				e.CommitteeConsensusData[identityobj.committeeID][TxnBlockDigest] = make([]string, 0)
+			if _, ok := e.CommitteeConsensusData[identityobj.CommitteeID][TxnBlockDigest]; ok == false {
+				e.CommitteeConsensusData[identityobj.CommitteeID][TxnBlockDigest] = make([]string, 0)
 				// store the txns for this digest
-				e.CommitteeConsensusDataTxns[identityobj.committeeID][TxnBlockDigest] = txnBlock
+				e.CommitteeConsensusDataTxns[identityobj.CommitteeID][TxnBlockDigest] = txnBlock
 			}
 
 			// add signatures for the txn block
 			signature := data["sign"].(string)
-			e.CommitteeConsensusData[identityobj.committeeID][TxnBlockDigest] = append(e.CommitteeConsensusData[identityobj.committeeID][TxnBlockDigest], signature)
+			e.CommitteeConsensusData[identityobj.CommitteeID][TxnBlockDigest] = append(e.CommitteeConsensusData[identityobj.CommitteeID][TxnBlockDigest], signature)
 
 		} else {
 			log.Error("signature invalid for intra committee block")
@@ -977,7 +995,7 @@ func (e *Elastico) receive(msg msgType) {
 		identityobj := data["identity"].(Identity)
 		if e.verifyPoW(identityobj) {
 
-			if _, ok := e.views[identityobj.port]; ok == false {
+			if _, ok := e.views[identityobj.Port]; ok == false {
 				e.receiveViews(msg)
 			}
 		}
@@ -1020,10 +1038,10 @@ func (e *Elastico) receive(msg msgType) {
 			e.BroadcastFinalTxn()
 		}
 	} else if msg.Type == "notify final member" {
-		log.Warn("notifying final member ", e.port)
+		log.Warn("notifying final member ", e.Port)
 		data := msg["data"].(map[string]interface{})
 		identityobj := data["identity"].(Identity)
-		if e.verifyPoW(identityobj) && e.committeeID == finNum {
+		if e.verifyPoW(identityobj) && e.CommitteeID == finNum {
 			e.isFinal = true
 		}
 	} else if msg.Type == "Broadcast Ri" {
@@ -1068,7 +1086,7 @@ func (e *Elastico) ElasticoInit() {
 
 	e.committeeMembers = make([]Identity, 0)
 
-	// for setting epochRandomness
+	E/ for setting epochRandomness
 	e.initER()
 
 	e.commitments = make(map[string]bool)
@@ -1128,10 +1146,10 @@ func (e *Elastico) reset() {
 
 	// channel = e.connection.channel()
 	// # to delete the queue in rabbitmq for next epoch
-	// channel.queue_delete(queue='hello' + str(e.port))
+	// channel.queue_delete(queue='hello' + str(e.Port))
 	// channel.close()
 
-	// e.port = e.getPort()
+	// e.Port = e.getPort()
 
 	e.PoW = make(map[string]interface{})
 	e.PoW["hash"] = ""
@@ -1145,7 +1163,7 @@ func (e *Elastico) reset() {
 	e.committeeMembers = make([]Identity, 0)
 
 	e.identity = Identity{}
-	e.committeeID = -1
+	e.CommitteeID = -1
 	e.Ri = ""
 
 	e.isDirectory = false
@@ -1194,7 +1212,7 @@ func (e *Elastico) reset() {
 
 func (e *Elastico) getCommitteeid(PoW string) int64 {
 	/*
-		returns last s-bit of PoW["hash"] as Identity : committeeID
+		returns last s-bit of PoW["hash"] as Identity : CommitteeID
 	*/
 	bindigest := ""
 
@@ -1240,7 +1258,7 @@ func (e *Elastico) SendtoFinal() {
 		}
 	}
 	log.Warn("size of committee members", len(e.finalCommitteeMembers))
-	log.Warn("send to final---txns", e.committeeID, e.port, e.txnBlock)
+	log.Warn("send to final---txns", e.CommitteeID, e.Port, e.txnBlock)
 	for _, finalID := range e.finalCommitteeMembers {
 
 		//  here txnBlock is a set, since sets are unordered hence can't sign them. So convert set to list for signing
@@ -1290,7 +1308,7 @@ func (e *Elastico) runPBFT() {
 			// construct prepare msg
 			// ToDo: verify whether the pre-prepare msg comes from various primaries or not
 			preparemsgList := e.constructPrepare()
-			// logging.warning("constructing prepares with port %s" , str(e.port))
+			// logging.warning("constructing prepares with Port %s" , str(e.Port))
 			e.sendPrepare(preparemsgList)
 			e.state = ElasticoStates["PBFT_PREPARE_SENT"]
 		}
@@ -1299,7 +1317,7 @@ func (e *Elastico) runPBFT() {
 		// ToDo: if, primary has not changed its state to "PBFT_PREPARE_SENT"
 		if e.isPrepared() {
 
-			// logging.warning("prepared done by %s" , str(e.port))
+			// logging.warning("prepared done by %s" , str(e.Port))
 			e.state = ElasticoStates["PBFT_PREPARED"]
 
 		} else if e.state == ElasticoStates["PBFT_PREPARED"] {
@@ -1312,7 +1330,7 @@ func (e *Elastico) runPBFT() {
 
 			if e.isCommitted() {
 
-				// logging.warning("committed done by %s" , str(e.port))
+				// logging.warning("committed done by %s" , str(e.Port))
 				e.state = ElasticoStates["PBFT_COMMITTED"]
 			}
 		}
@@ -1451,7 +1469,7 @@ func (e *Elastico) runFinalPBFT() {
 			// finalTxnBlock = list(finalTxnBlock)
 			// # order them! Reason : to avoid errors in signatures as sets are unordered
 			// # e.finalBlock["finalBlock"] = sorted(finalTxnBlock)
-			// logging.warning("final block by port %s with final block %s" , str(e.port), str(e.finalBlock["finalBlock"]))
+			// logging.warning("final block by Port %s with final block %s" , str(e.Port), str(e.finalBlock["finalBlock"]))
 			e.state = ElasticoStates["FinalPBFT_COMMITTED"]
 		}
 	}
@@ -1679,7 +1697,7 @@ func (e *Elastico) checkCountForFinalData() {
 	*/
 	if len(e.response) > 0 {
 
-		log.Warn("final block sent the block to client by", e.port)
+		log.Warn("final block sent the block to client by", e.Port)
 		e.state = ElasticoStates["FinalBlockSentToClient"]
 	}
 }
@@ -1690,7 +1708,7 @@ func (e *Elastico) verifyAndMergeConsensusData() {
 		atleast c/2 + 1 members of the proper committee and takes the ordered set union of all the inputs
 
 	*/
-	// logging.warning("verify and merge %s -- %s" , str(self.port) ,str(self.committee_id))
+	// logging.warning("verify and merge %s -- %s" , str(self.Port) ,str(self.committee_id))
 	// for committeeid in range(pow(2,s)):
 	// 	if committeeid in self.CommitteeConsensusData:
 	// 		for txnBlockDigest in self.CommitteeConsensusData[committeeid]:
@@ -1703,7 +1721,7 @@ func (e *Elastico) verifyAndMergeConsensusData() {
 	// 					self.mergedBlock = self.unionTxns(self.mergedBlock, txnBlock)
 	// if len(self.mergedBlock) > 0:
 	// 	self.state = ELASTICO_STATES["Merged Consensus Data"]
-	// 	logging.warning("%s - port , %s - mergedBlock" ,str(self.port) ,  str(self.mergedBlock))
+	// 	logging.warning("%s - Port , %s - mergedBlock" ,str(self.Port) ,  str(self.mergedBlock))
 }
 
 func (e *Elastico) logCommitMsg(msg map[string]interface{}) {
@@ -1712,7 +1730,7 @@ func (e *Elastico) logCommitMsg(msg map[string]interface{}) {
 	*/
 	// viewId = msg["commitData"]["viewId"]
 	// seqnum = msg["commitData"]["seq"]
-	// socketId = msg["identity"].IP +  ":" + str(msg["identity"].port)
+	// socketId = msg["identity"].IP +  ":" + str(msg["identity"].Port)
 	// # add msgs for this view
 	// if viewId not in self.commitMsgLog:
 	// 	self.commitMsgLog[viewId] = dict()
@@ -1749,7 +1767,7 @@ func (e *Elastico) sendCommitment() {
 		HashRi := e.getCommitment()
 		for _, nodeID := range e.committeeMembers {
 
-			log.Warn("sent the commitment by", e.port)
+			log.Warn("sent the commitment by", e.Port)
 			data := map[string]interface{}{"identity": e.identity, "HashRi": HashRi}
 			msg := map[string]interface{}{"data": data, "type": "hash"}
 			nodeID.send(msg)
@@ -1787,7 +1805,7 @@ func (e *Elastico) computeFakePoW() {
 				zeroString += "0"
 			}
 			// if len(e.setOfRs) > 0:
-			// 	e.epochRandomness, randomsetR = e.xor_R()
+			E/ 	e.epochRandomness, randomsetR = e.xor_R()
 			for {
 
 				digest := sha256.New()
@@ -1809,7 +1827,7 @@ func (e *Elastico) computeFakePoW() {
 			// computing a random PoW
 			randomsetR := set()
 			// if len(e.setOfRs) > 0:
-			// 	e.epochRandomness, randomsetR := e.xor_R()
+			E/ 	e.epochRandomness, randomsetR := e.xor_R()
 			digest := sha256.New()
 			ranHash := fmt.Sprintf("%x", digest.Sum(nil))
 			nonce := randomGen()
@@ -1889,7 +1907,7 @@ func (e *Elastico) isCommitted() bool {
 	// 					if self.viewId in self.commitMsgLog:
 	// 						if seqnum in self.commitMsgLog[self.viewId]:
 	// 							count = 0
-	// 							logging.warning("CHECK FOR COUNT IN COMMITTED BY PORT %s" , str(self.port))
+	// 							logging.warning("CHECK FOR COUNT IN COMMITTED BY PORT %s" , str(self.Port))
 	// 							for replicaId in self.commitMsgLog[self.viewId][seqnum]:
 	// 								for msg in self.commitMsgLog[self.viewId][seqnum][replicaId]:
 	// 									if msg["digest"] == digest:
@@ -1943,7 +1961,7 @@ func (e *Elastico) isFinalCommitted() bool {
 	// 					if self.viewId in self.FinalcommitMsgLog:
 	// 						if seqnum in self.FinalcommitMsgLog[self.viewId]:
 	// 							count = 0
-	// 							logging.warning("CHECK FOR COUNT IN COMMITTED BY PORT %s" , str(self.port))
+	// 							logging.warning("CHECK FOR COUNT IN COMMITTED BY PORT %s" , str(self.Port))
 	// 							for replicaId in self.FinalcommitMsgLog[self.viewId][seqnum]:
 	// 								for msg in self.FinalcommitMsgLog[self.viewId][seqnum][replicaId]:
 	// 									if msg["digest"] == digest:
@@ -1982,7 +2000,7 @@ func (e *Elastico) logFinalCommitMsg(msg map[string]interface{}) {
 		viewID := commitData["viewId"].(int)
 		seqnum := commitData["seq"].(int)
 		identityobj := msg["identity"].(Identity)
-		socketId = msg["identity"].IP + ":" + strconv.Itoa(msg["identity"].port)
+		socketId = msg["identity"].IP + ":" + strconv.Itoa(msg["identity"].Port)
 		// add msgs for this view
 		_, ok := e.FinalcommitMsgLog[viewId]
 		if ok == false {
@@ -2023,11 +2041,11 @@ func (e *Elastico) formIdentity() {
 		PK := e.key.PublicKey
 
 		// set the committee id acc to PoW solution
-		e.committeeID = e.getCommitteeid(e.PoW["hash"].(string))
+		e.CommitteeID = e.getCommitteeid(e.PoW["hash"].(string))
 		var network bytes.Buffer        // Stand-in for a network connection
 		enc := gob.NewEncoder(&network) // Will write to network.
 		enc.Encode(&PK)
-		e.identity = Identity{e.IP, network, e.committeeID, e.PoW, e.epochRandomness, e.port}
+		E.identity = Identity{e.IP, network, e.CommitteeID, e.PoW, e.epochRandomness, e.Port}
 		// changed the state after identity formation
 		e.state = ElasticoStates["Formed Identity"]
 	}
@@ -2127,7 +2145,7 @@ func (e *Elastico) logPrepareMsg(msg map[string]interface{}) {
 	prepareData := msg["prepareData"].(map[string]interface{})
 	viewID := prepareData["viewId"].(int)
 	seqnum := prepareData["seq"].(int)
-	socketID := identityobj.IP + ":" + strconv.Itoa(identityobj.port)
+	socketID := identityobj.IP + ":" + strconv.Itoa(identityobj.Port)
 	// add msgs for this view
 	if _, ok := e.prepareMsgLog[viewID]; ok == false {
 
@@ -2167,7 +2185,7 @@ func (e *Elastico) logFinalPrepareMsg(msg map[string]interface{}) {
 	prepareData := msg["prepareData"].(map[string]interface{})
 	viewID := prepareData["viewId"].(int)
 	seqnum := prepareData["seq"].(int)
-	socketID := identityobj.IP + ":" + strconv.Itoa(identityobj.port)
+	socketID := identityobj.IP + ":" + strconv.Itoa(identityobj.Port)
 
 	// add msgs for this view
 	if _, ok := e.FinalPrepareMsgLog[viewID]; ok == false {
@@ -2335,7 +2353,7 @@ func (e *Elastico) unionTxns(actualTxns, receivedTxns []Transaction) []Transacti
 }
 
 type directoryMemberMsg struct {
-	identity Identity
+	identity Identity `json:"identity"`
 }
 
 func (e *Elastico) formCommittee() {
@@ -2392,11 +2410,11 @@ func (e *Elastico) verifyPoW(identityobj Identity) bool {
 	// 		return false
 
 	// reconstruct epoch randomness
-	epochRandomness := identityobj.epochRandomness
+	EpochRandomness := identityobj.epochRandomness
 	setOfRs := PoW["setOfRs"].([]string)
 	if len(setOfRs) > 0 {
 		xorVal := xorbinary(setOfRs)
-		epochRandomness = fmt.Sprintf("%0"+strconv.FormatInt(r, 10)+"b\n", xorVal)
+		EpochRandomness = fmt.Sprintf("%0"+strconv.FormatInt(r, 10)+"b\n", xorVal)
 	}
 
 	// recompute PoW
@@ -2411,7 +2429,7 @@ func (e *Elastico) verifyPoW(identityobj Identity) bool {
 	digest.Write([]byte(IP))
 	digest.Write(rsaPublickey.N.Bytes())
 	digest.Write([]byte(strconv.Itoa(rsaPublickey.E)))
-	digest.Write([]byte(epochRandomness))
+	Eigest.Write([]byte(epochRandomness))
 	digest.Write([]byte(strconv.Itoa(nonce)))
 
 	hashVal := fmt.Sprintf("%x", digest.Sum(nil))
@@ -2621,9 +2639,9 @@ func (e *Elastico) logPrePrepareMsg(msg map[string]interface{}) {
 	*/
 	identityobj := msg["identity"].(Identity)
 	IP := identityobj.IP
-	port := identityobj.port
+	Port := identityobj.Port
 	// create a socket
-	socket := IP + ":" + strconv.Itoa(port)
+	socket := IP + ":" + strconv.Itoa(Port)
 	e.prePrepareMsgLog[socket] = msg
 }
 
@@ -2633,9 +2651,9 @@ func (e *Elastico) logFinalPrePrepareMsg(msg map[string]interface{}) {
 	*/
 	identityobj := msg["identity"].(Identity)
 	IP := identityobj.IP
-	port := identityobj.port
+	Port := identityobj.Port
 	// create a socket
-	socket := IP + ":" + strconv.Itoa(port)
+	socket := IP + ":" + strconv.Itoa(Port)
 	e.FinalPrePrepareMsgLog[socket] = msg
 
 }
@@ -2713,7 +2731,7 @@ func (e *Elastico) executeReset() {
 	/*
 		call for reset
 	*/
-	log.Warn("call for reset for ", e.port)
+	log.Warn("call for reset for ", e.Port)
 	if reflect.TypeOf(e.identity) == reflect.TypeOf(Identity{}) {
 
 		// if node has formed its identity
@@ -2746,7 +2764,7 @@ func (e *Elastico) execute(epochTxn []Transaction) string {
 		e.formCommittee()
 	} else if e.isDirectory && e.state == ElasticoStates["RunAsDirectory"] {
 
-		log.Info("The directory member :- ", e.port)
+		log.Info("The directory member :- ", e.Port)
 		e.receiveTxns(epochTxn)
 		// directory member has received the txns for all committees
 		e.state = ElasticoStates["RunAsDirectory after-TxnReceived"]
@@ -2771,7 +2789,7 @@ func (e *Elastico) execute(epochTxn []Transaction) string {
 	} else if e.state == ElasticoStates["PBFT_COMMITTED"] {
 
 		// send pbft consensus blocks to final committee members
-		log.Info("pbft finished by members %s", e.port)
+		log.Info("pbft finished by members %s", e.Port)
 		e.SendtoFinal()
 
 	} else if e.isFinalMember() && e.state == ElasticoStates["Intra Consensus Result Sent to Final"] {
@@ -2793,7 +2811,7 @@ func (e *Elastico) execute(epochTxn []Transaction) string {
 
 		// send the commitment to other final committee members
 		e.sendCommitment()
-		log.Warn("pbft finished by final committee", e.port)
+		log.Warn("pbft finished by final committee", e.Port)
 
 	} else if e.isFinalMember() && e.state == ElasticoStates["CommitmentSentToFinal"] {
 
@@ -2991,11 +3009,11 @@ func (e *Elastico) consumeMsg() {
 	// close the channel
 	defer channel.Close()
 
-	nodeport := strconv.Itoa(e.port)
+	nodeport := strconv.Itoa(e.Port)
 	queueName := "hello" + nodeport
 	// count the number of messages that are in the queue
 	Queue, err := channel.QueueInspect(queueName)
-	failOnError(err, "error in inspect", false)
+	// failOnError(err, "error in inspect", false)
 
 	var decoded msgType
 	if err == nil {
@@ -3008,7 +3026,7 @@ func (e *Elastico) consumeMsg() {
 			if ok {
 				err := json.Unmarshal(msg.Body, &decoded)
 				failOnError(err, "error in unmarshall", true)
-				fmt.Println(decoded)
+				// fmt.Println(decoded)
 				// consume the msg by taking the action in receive
 				e.receive(decoded)
 			}
