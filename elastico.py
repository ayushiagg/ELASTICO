from subprocess import check_output
from Crypto.PublicKey import RSA
from Crypto.Signature import PKCS1_v1_5
from Crypto.Hash import SHA256
from secrets import SystemRandom

# network_nodes - All objects of nodes in the network
global network_nodes, n, s, c, D, r, identityNodeMap, fin_num, commitmentSet, ledger, NtwParticipatingNodes
# n : number of processors
n = 150
# s - where 2^s is the number of committees
s = 4
# c - size of committee
c = 2
# D - difficulty level , leading bits of PoW must have D 0's (keep w.r.t to hex)
D = 1 
# r - number of bits in random string 
r = 5
# fin_num - final committee id
fin_num = 0
# identityNodeMap- mapping of identity object to Elastico node
identityNodeMap = dict()
# commitmentSet - set of commitments S
commitmentSet = set()
# ledger - ledger is the database that contains the set of blocks where each block comes after an epoch
ledger = []
# NtwParticipatingNodes - list of nodes those are the part of some committee
NtwParticipatingNodes = []
# network_nodes - list of all nodes 
network_nodes = []
# ELASTICO_STATES - states reperesenting the running state of the node
ELASTICO_STATES = {"NONE": 0, "PoW Computed": 1, "Formed Identity" : 2,"Formed Committee": 3, "RunAsDirectory": 4 ,"Receiving Committee Members" : 5,"Committee full" : 6 , "PBFT Finished" : 7, "Intra Consensus Result Sent to Final" : 8, "Final Committee in PBFT" : 9, "FinalBlockSent" : 10, "FinalBlockReceived" : 11, "RunAsDirectory after-TxnReceived" : 12, "RunAsDirectory after-TxnMulticast" : 13, "Final PBFT Start" : 14, "Merged Consensus Data" : 15, "PBFT Finished-FinalCommittee" : 16 , "CommitmentSentToFinal" : 17, "BroadcastedR" : 18, "ReceivedR" :  19, "FinalBlockSentToClient" : 20}

# class Network:
# 	"""
# 		class for networking between nodes
# 	"""

def consistencyProtocol():
	"""
		Agrees on a single set of Hash values(S)
		presently selecting random c hash of Ris from the total set of commitments
	"""
	global network_nodes, commitmentSet

	for node in network_nodes:
		if node.isFinalMember():
			if len(node.commitments) <= c//2:
				input("insufficientCommitments")
				return False, "insufficientCommitments"

	# ToDo: Discuss with sir about intersection.
	if len(commitmentSet) == 0:
		flag = True
		for node in network_nodes:
			if node.isFinalMember():
				if flag and len(commitmentSet) == 0:
					flag = False
					commitmentSet = node.commitments
				else:	
					commitmentSet = commitmentSet.intersection(node.commitments)
	return True,commitmentSet


def random_gen(size=32):
	"""
		generates the size-bit random number
		size denotes the number of bits
	"""
	# with open("/dev/urandom", 'rb') as f:
	# 	return int.from_bytes(f.read(4), 'big')
	random_num = SystemRandom().getrandbits(size)
	return random_num


def BroadcastTo_Network(data, type_):
	"""
		Broadcast data to the whole ntw
	"""
		# comment: no if statements
		# for each instance of Elastico, create a receive(self, msg) method
		# this function will just call node.receive(msg)
		# inside msg, you will need to have a message type, and message data.

	global identityNodeMap
	print("---Broadcast to network---")
	msg = {"type" : type_ , "data" : data}
	# ToDo: directly accessing of elastico objects should be removed
	for node in network_nodes:
		node.receive(msg)


def BroadcastTo_Committee(committee_id, data , type_):
	"""
		Broadcast to the particular committee id
	"""
	msg = {"type" : type_ , "data" : data}

	pass


def MulticastCommittee(commList, identityobj, txns):
	"""
		each node getting views of its committee members from directory members
	"""

	print("---multicast committee list to committee members---")
	
	# print(len(commList), commList)
	finalCommitteeMembers = commList[fin_num]
	for committee_id in commList:
		commMembers = commList[committee_id]
		for memberId in commMembers:
			# union of committe members views
			data = {"committee members" : commMembers , "final Committee members"  : finalCommitteeMembers , "txns" : txns[committee_id] ,"identity" : identityobj}
			msg = {"data" : data , "type" : "committee members views"}
			memberId.send(msg)



class Identity:
	"""
		class for the identity of nodes
	"""
	def __init__(self, IP, PK, committee_id, PoW, epoch_randomness):
		self.IP = IP
		self.PK = PK
		self.committee_id = committee_id
		self.PoW = PoW
		self.epoch_randomness = epoch_randomness
		self.partOfNtw = False


	def isEqual(self, identityobj):
		"""
			checking two objects of Identity class are equal or not
		"""
		return self.IP == identityobj.IP and self.PK == identityobj.PK and self.committee_id == identityobj.committee_id \
		and self.PoW == identityobj.PoW and self.epoch_randomness == identityobj.epoch_randomness and self.partOfNtw == identityobj.partOfNtw

	def send(self, msg):
		"""
			send the msg to node based on their identity
		"""
		global identityNodeMap
		# print("--send to node--")
		node = identityNodeMap[self]
		response = node.receive(msg)
		return response

class Elastico:
	"""
		class members: 
			node - single processor
			identity - identity consists of Public key, an IP, PoW, committee id, epoch randomness
			txn_block - block of txns that the committee will agree on(intra committee consensus block)
			committee_list - list of nodes in all committees
			final_committee - list of nodes in the final committee
			is_directory - whether the node belongs to directory committee or not
			is_final - whether the node belongs to final committee or not
			epoch_randomness - r-bit random string generated at the end of previous epoch
			committee_Members - set of committee members in its own committee
			IP - IP address of a node
			key - public key and private key pair for a node
			cur_directory - list of directory members in view of the node
			PoW - dict containing 256 bit hash computed by the node, set of Rs needed for epoch randomness, and a nonce
			Ri - r-bit random string
			commitments - set of H(Ri) received by final committee node members and H(Ri) is sent by the final committee node only
			set_of_Rs - set of Ris obtained from the final committee
			committee_id - integer value to represent the committee to which the node belongs
			final_committee_id - committee id of final committee
			CommitteeConsensusData - a dictionary of committee ids that contains a dictionary of the txn block and the signatures
			finalBlockbyFinalCommittee - a dictionary of txn block and the signatures by the final committee members
			state - state in which a node is running
			mergedBlock - list of txns of different committees after their intra committee consensus
			finalBlock - agreed list of txns after pbft run by final committee
			RcommitmentSet - set of H(Ri)s received from the final committee after the consistency protocol
			finalCommitteeMembers - members of the final committee received from the directory committee
			txn- transactions stored by the directory members
			ConsensusMsgCount - count of intra consensus blocks of each committee received by the final committee
	"""

	def __init__(self):
		print("---Constructor of elastico class---")
		self.IP = self.get_IP()
		self.key = self.get_key()
		self.PoW = {"hash" : "", "set_of_Rs" : "", "nonce" : 0}
		self.cur_directory = []
		self.identity = ""
		self.committee_id = ""
		# only when this node is the member of directory committee
		self.committee_list = dict()
		# only when this node is not the member of directory committee
		self.committee_Members = set()
		self.is_directory = False
		self.is_final = False
		self.epoch_randomness = self.initER()
		self.Ri = ""
		# only when this node is the member of final committee
		self.commitments = set()
		self.txn_block = set()
		self.set_of_Rs = set()
		self.newset_of_Rs = set()
		self.CommitteeConsensusData = dict()
		self.finalBlockbyFinalCommittee = dict()
		self.state = ELASTICO_STATES["NONE"]
		self.mergedBlock = []
		self.finalBlock = {"sent" : False, "finalBlock" : [] }
		self.RcommitmentSet = ""
		self.newRcommitmentSet = ""
		self.finalCommitteeMembers = set()
		# only when this node is the member of final committee
		self.ConsensusMsgCount = dict()
		# only when this is the member of the directory committee
		self.txn = dict()

	def reset(self):
		"""
			reset some of the elastico class members
		"""
		self.IP = self.get_IP()
		self.key = self.get_key()
		self.PoW = {"hash" : "", "set_of_Rs" : "", "nonce" : 0}
		self.cur_directory = []
		self.identity = ""
		self.committee_id = ""
		# only when this node is the member of directory committee
		self.committee_list = dict()
		# only when this node is not the member of directory committee
		self.committee_Members = set()
		self.is_directory = False
		self.is_final = False
		self.Ri = ""
		# only when this node is the member of final committee
		self.commitments = set()
		self.txn_block = set()
		self.set_of_Rs = self.newset_of_Rs
		self.newset_of_Rs = set()
		self.CommitteeConsensusData = dict()
		self.finalBlockbyFinalCommittee = dict()
		self.state = ELASTICO_STATES["NONE"]
		self.mergedBlock = []
		self.finalBlock = {"sent" : False, "finalBlock" : [] }
		self.RcommitmentSet = self.newRcommitmentSet
		self.newRcommitmentSet = ""
		self.finalCommitteeMembers = set()
		# only when this node is the member of final committee
		self.ConsensusMsgCount = dict()
		# only when this is the member of the directory committee
		self.txn = dict()

	def initER(self):
		"""
			initialise r-bit epoch random string
		"""
				# minor comment: this must be cryptographically secure, but this is not.
				# might want to replace this with reads from /dev/urandom.
		print("---initial epoch randomness for a node---")
		randomnum = random_gen(r)
		return ("{:0" + str(r) +  "b}").format(randomnum)


	def get_IP(self):
		"""
			for each node(processor) , get IP addr
			will return IP
		"""
		# ips = check_output(['hostname', '--all-ip-addresses'])
		# ips = ips.decode()
		# return ips.split(' ')[0]

		print("---get IP address---")
		ip=""
		for i in range(4):
			ip += str(random_gen(8))
			ip += "."
		ip = ip[ : -1]
		return ip


	def get_key(self):
		"""
			for each node, it will return public pvt key pair
		"""
		print("---get public pvt key pair---")
		key = RSA.generate(2048)
		return key


	def compute_PoW(self):
		"""
			returns hash which satisfies the difficulty challenge(D) : PoW["hash"]
		"""
		# print("---PoW computation started---")
		if self.state == ELASTICO_STATES["NONE"]:
			PK = self.key.publickey().exportKey().decode()
			IP = self.IP
			# If it is the first epoch , randomset_R will be an empty set .
			# otherwise randomset_R will be any c/2 + 1 random strings Ri that node receives from the previous epoch
			randomset_R = set()
			if len(self.set_of_Rs) > 0:
				self.epoch_randomness, randomset_R = self.xor_R()
							# minor comment: create a sha256 object by calling hashlib.sha256()
							# then repeatedly call sha256.update(...) with the things that need to be hashed together.
							# finally extract digest by calling sha256.digest()
							# don't convert to json and then to string
							# bug is possible in this, find and fix it.
			digest = SHA256.new()
			digest.update(IP.encode())
			digest.update(PK.encode())
			digest.update(self.epoch_randomness.encode())
			digest.update(str(self.PoW["nonce"]).encode())
			hash_val = digest.hexdigest()
			if hash_val.startswith('0' * D):
				# ToDo: Put the nonce here in Pow
				nonce = self.PoW["nonce"]
				self.PoW = {"hash" : hash_val, "set_of_Rs" : randomset_R, "nonce" : nonce}
				print("---PoW computation end---")
				self.state = ELASTICO_STATES["PoW Computed"]
				return hash_val
			self.PoW["nonce"] += 1


	def notify_finalCommittee(self):
		"""
			notify the members of the final committee that they are the final committee members
		"""
		# ToDo: Ensure that the final committee has c members 
		
		finalCommList = self.committee_list[fin_num]
		for finalMember in finalCommList:
			data = {"identity" : self.identity}
			msg = {"data" : data , "type" : "notify final member"}
			finalMember.send(msg)

	def get_committeeid(self, PoW):
		"""
			returns last s-bit of PoW["hash"] as Identity : committee_id
		"""	
		bindigest = ''
		for hashdig in PoW:
			bindigest += "{:04b}".format(int(hashdig, 16))
		identity = bindigest[-s:]
		return int(identity, 2)


	def form_identity(self):
		"""
			identity formation for a node
			identity consists of public key, ip, committee id, PoW, nonce, epoch randomness
		"""
		if self.state == ELASTICO_STATES["PoW Computed"]:
			global identityNodeMap
			print("---form identity---")
			# export public key
			PK = self.key.publickey().exportKey().decode()
			# set the committee id acc to PoW solution
			self.committee_id = self.get_committeeid(self.PoW["hash"])
			self.identity = Identity(self.IP, PK, self.committee_id, self.PoW, self.epoch_randomness)
			# mapped identity object to the elastico object
			identityNodeMap[self.identity] = self
			self.state = ELASTICO_STATES["Formed Identity"]
			return self.identity


	def is_OwnIdentity(self, identityobj):
		"""
			Checking whether the identityobj is the Elastico node's identity or not
		"""
		if self.identity == "":
			self.form_identity()
		return self.identity.isEqual(identityobj)


	def form_committee(self):
		"""
			creates directory committee if not yet created otherwise informs all
			the directory members
		"""	
		if len(self.cur_directory) < c:
			self.is_directory = True
			print("---not seen c members yet, so broadcast to ntw---")
			BroadcastTo_Network(self.identity, "directoryMember")
			self.state = ELASTICO_STATES["RunAsDirectory"]
		else:
			print("---seen c members---")
			# track previous state before adding in committee
			prevState = self.state
			
			self.Send_to_Directory()
			# ToDo : check state assignment order
			if prevState == ELASTICO_STATES["Formed Identity"] and self.state == ELASTICO_STATES["Receiving Committee Members"]:
				msg = {"data" : self.identity ,"type" : "Committee full"}
				BroadcastTo_Network(msg["data"] , msg["type"])
			else: 
				self.state = ELASTICO_STATES["Formed Committee"]
				# broadcast committee full state notification to all nodes when the present state is "Received Committee members"


	def Send_to_Directory(self):
		"""
			Send about new nodes to directory committee members
		"""
		# Add the new processor in particular committee list of directory committee nodes
		print("---Send to directory---")
		for nodeId in self.cur_directory:
			msg = {"data" : self.identity, "type" : "newNode"}
			nodeId.send(msg)



	def checkCommitteeFull(self):
		"""
			directory member checks whether the committees are full or not
		"""
		commList = self.committee_list
		flag = 0
		for iden in range(pow(2,s)):
			if iden not in commList or len(commList[iden]) < c:
				flag = 1
				break
		if flag == 0:
			# Send commList[iden] to members of commList[iden]
			print("----------committees full----------------")
			if self.state == ELASTICO_STATES["RunAsDirectory"]:
				# directory member has not yet received the epochTxn
				pass
			if self.state == ELASTICO_STATES["RunAsDirectory after-TxnReceived"]:
				MulticastCommittee(commList, self.identity, self.txn)
				self.state = ELASTICO_STATES["RunAsDirectory after-TxnMulticast"]
				self.notify_finalCommittee()
				# ToDo: transition of state to committee full 


	def receive(self, msg):
		"""
			method to recieve messages for a node as per the type of a msg
		"""
		# new node is added in directory committee if not yet formed
		if msg["type"] == "directoryMember":
			# verify the PoW of the sender
			identityobj = msg["data"]
			if self.verify_PoW(identityobj):
				if len(self.cur_directory) < c:
					self.cur_directory.append(identityobj)
			else:
		 		print("$$$$$$$ PoW not valid $$$$$$")

		# new node is added to the corresponding committee list if committee list has less than c members
		elif msg["type"] == "newNode" and self.is_directory:
			identityobj = msg["data"]
			if self.verify_PoW(identityobj):
				if identityobj.committee_id not in self.committee_list:
					self.committee_list[identityobj.committee_id] = [identityobj]
				elif len(self.committee_list[identityobj.committee_id]) < c:
					self.committee_list[identityobj.committee_id].append(identityobj)
					# Once each committee contains at least c identities each, directory members multicast the committee list to each committee member
					if len(self.committee_list[identityobj.committee_id]) == c:
						self.checkCommitteeFull()
			else:
				print("$$$$$$$ PoW not valid 22222 $$$$$$")

		# union of committe members views
		elif msg["type"] == "committee members views" and self.verify_PoW(msg["data"]["identity"]):
			# data = {"committee members" : commMembers , "final Committee members"  : finalCommitteeMembers , "txns" : self.txn[committee_id] ,"identity" : self.identity}
			commMembers = msg["data"]["committee members"]
			finalMembers  = msg["data"]["final Committee members"]
			self.txn_block |= set(msg["data"]["txns"])
			self.committee_Members |= set(commMembers)
			self.finalCommitteeMembers |= set(finalMembers)
			self.state  = ELASTICO_STATES["Receiving Committee Members"]
			print("commMembers for committee id - " , self.committee_id, "is :-", self.committee_Members)


		elif msg["type"] == "Committee full" and self.verify_PoW(msg["data"]):
			if self.state == ELASTICO_STATES["Receiving Committee Members"]:
				self.state = ELASTICO_STATES["Committee full"]

		# receiving H(Ri) by final committe members
		elif msg["type"] == "hash" and self.isFinalMember():
			data = msg["data"]
			identityobj = data["identity"]
			if self.verify_PoW(identityobj):
				self.commitments.add(data["Hash_Ri"])

		elif msg["type"] == "RandomStringBroadcast":
			data = msg["data"]
			identityobj = data["identity"]
			if self.verify_PoW(identityobj):
				Ri = data["Ri"]
				HashRi = self.hexdigest(Ri)
				if HashRi in self.newRcommitmentSet:
					self.newset_of_Rs.add(Ri)
					if len(self.newset_of_Rs) >= c//2 + 1:
						self.state = ELASTICO_STATES["ReceivedR"]

		elif msg["type"] == "finalTxnBlock":
			data = msg["data"]
			# data = {"commitmentSet" : S, "signature" : self.sign(S) , "finalTxnBlock" : self.txn_block}
			identityobj = data["identity"]
			if self.verify_PoW(identityobj):
				sign = data["signature"]
				received_commitmentSet = data["commitmentSet"]
				PK = identityobj.PK
				finalTxnBlock = data["finalTxnBlock"]
				finalTxnBlock_signature = data["finalTxnBlock_signature"]
				if self.verify_sign(sign, received_commitmentSet, PK) and self.verify_sign(finalTxnBlock_signature, finalTxnBlock, PK):
					# ToDo : to take step regarding this
					if str(finalTxnBlock) not in self.finalBlockbyFinalCommittee:
						self.finalBlockbyFinalCommittee[str(finalTxnBlock)] = set()
					self.finalBlockbyFinalCommittee[str(finalTxnBlock)].add(finalTxnBlock_signature)
					# ToDo : Check this, It is overwritten here
					if len(self.finalBlockbyFinalCommittee[str(finalTxnBlock)]) >= c//2 + 1:
						# for final members, their state is updated only when they have also sent the finalblock
						if self.isFinalMember():
							if self.finalBlock["sent"]:
								self.state = ELASTICO_STATES["FinalBlockReceived"]
							pass
						else:
							self.state = ELASTICO_STATES["FinalBlockReceived"]
					# ToDo : Check this, It is overwritten here or need to be union of commitments
					if self.newRcommitmentSet == "":
						self.newRcommitmentSet = set()
					self.newRcommitmentSet |= received_commitmentSet

				else:
					print("Signature invalid")
					input()
			else:
				print("PoW not valid")
				input()

		elif msg["type"] == "getCommitteeMembers":
			if self.is_directory == False:
				return False , set()
			data = msg["data"]
			identityobj = data["identity"]
			if self.verify_PoW(identityobj):
				committeeid = data["committee_id"]
				print("final comid :-" , committeeid)
				return True, self.committee_list[committeeid]

		# final committee member receives the final set of txns along with the signature from the node
		elif msg["type"] == "intraCommitteeBlock" and self.isFinalMember():
			data = msg["data"]
			identityobj = data["identity"]
			print("txnBlock : - " , data["txnBlock"])
			print("commid - " , identityobj.committee_id)
			if self.verify_PoW(identityobj):
				# data = {"txnBlock" = self.txn_block , "sign" : self.sign(self.txn_block), "identity" : self.identity}
				if self.verify_sign(data["sign"], data["txnBlock"] , identityobj.PK):
					if identityobj.committee_id not in self.CommitteeConsensusData:
						self.CommitteeConsensusData[identityobj.committee_id] = dict()
					# add signatures for the txn block 
					if str(data["txnBlock"]) not in self.CommitteeConsensusData[identityobj.committee_id]:
						self.CommitteeConsensusData[identityobj.committee_id][ str(data["txnBlock"]) ] = set()
					self.CommitteeConsensusData[identityobj.committee_id][ str(data["txnBlock"]) ].add( data["sign"] )
					if identityobj.committee_id not in self.ConsensusMsgCount:
						self.ConsensusMsgCount[identityobj.committee_id	] = 1
					else:	
						self.ConsensusMsgCount[identityobj.committee_id] += 1
		elif msg["type"] == "request committee list from directory member":
			if self.is_directory == False:
				return False , dict()
			else:
				commList = self.committee_list
				return True , commList

		elif msg["type"] == "command to run pbft":
			if self.is_directory == False:
				self.runPBFT(self.txn_block, msg["data"]["instance"])

		elif msg["type"] == "command to run pbft by final committee":
			if self.isFinalMember():
				self.runPBFT(self.mergedBlock, msg["data"]["instance"])


		elif msg["type"] == "send txn set and sign to final committee":
			if self.is_directory == False:
				self.SendtoFinal()

		elif msg["type"] == "verify and merge intra consensus data":
			if self.isFinalMember():
				self.verifyAndMergeConsensusData()	

		elif msg["type"] == "send commitments of Ris":
			if self.isFinalMember():
				self.sendCommitment()

		elif msg["type"] == "broadcast final set of txns to the ntw":
			if self.isFinalMember():
				self.BroadcastFinalTxn()

		elif msg["type"] == "notify final member":
			if self.verify_PoW(msg["data"]["identity"]):
				self.is_final = True

		elif msg["type"] == "Broadcast Ri":
			if self.isFinalMember():
				self.BroadcastR()

		elif msg["type"] == "append to ledger":
			if self.is_directory == False and len(self.committee_Members) == c:
				response = []
				for txnBlock in self.finalBlockbyFinalCommittee:
					if len(self.finalBlockbyFinalCommittee[txnBlock]) >= c//2 + 1:
						response.append(txnBlock)
				return response		

		elif msg["type"] == "reset-all" and self.verify_PoW(msg["data"]):
			# reset the elastico node
			self.reset()

	def verifyAndMergeConsensusData(self):
		"""
			each final committee member validates that the values received from the committees are signed by 
			atleast c/2 + 1 members of the proper committee and takes the ordered set union of all the inputs
		"""
		print("--verify And Merge--")
		for committeeid in range(pow(2,s)):
			print("comm id : -" , committeeid)
			if committeeid in self.CommitteeConsensusData:
				for txnBlock in self.CommitteeConsensusData[committeeid]:
					if len(self.CommitteeConsensusData[committeeid][txnBlock]) >= c//2 + 1:
						print(type(txnBlock) , txnBlock)
						# input()
						try:
							# ToDo: Check where is empty block coming from
							if len(txnBlock) > 0:
								set_of_txns = eval(txnBlock)
						except Exception as e:
							print("excepton:" , txnBlock , "  ", len(txnBlock), " ", type(txnBlock))
							raise e
						self.mergedBlock.extend(set_of_txns)
		self.state = ELASTICO_STATES["Merged Consensus Data"]
		print(self.mergedBlock)
		input("Check merged block above!")


	def runPBFT(self , txnBlock, instance):
		"""
			Runs a Pbft instance for the intra-committee consensus
		"""
		txn_set = set()
		for txn in txnBlock:
			txn_set.add(txn)
		if instance == "final committee consensus":
			self.finalBlock["finalBlock"] = txn_set
			self.state = ELASTICO_STATES["PBFT Finished-FinalCommittee"]
		elif instance == "intra committee consensus":
			self.txn_block = txn_set
			self.state = ELASTICO_STATES["PBFT Finished"]

	def isFinalMember(self):
		"""
			tell whether this node is a final committee member or not
		"""
		return self.is_final

	def sign(self,data):
		"""
			Sign the data i.e. signature
		"""
		# make sure that data is string or not
		if type(data) is not str:
			data = str(data)
		digest = SHA256.new()
		digest.update(data.encode())
		signer = PKCS1_v1_5.new(self.key)
		signature = signer.sign(digest)
		return signature


	def verify_sign(self, signature, data, publickey):
		"""
			verify whether signature is valid or not 
			if public key is not key object then create a key object
		"""
		# print("---verify_sign func---")
		if type(publickey) is str:
			publickey = publickey.encode()
		if type(data) is not str:
			data = str(data)
		if type(publickey) is bytes:
			publickey = RSA.importKey(publickey)
		digest = SHA256.new()
		digest.update(data.encode())
		verifier = PKCS1_v1_5.new(publickey)
		return verifier.verify(digest,signature)


	def BroadcastFinalTxn(self):
		"""
			final committee members will broadcast S(commitmentSet), along with final set of 
			X(txn_block) to everyone in the network
		"""
		boolVal , S = consistencyProtocol()
		if boolVal == False:
			return S
		data = {"commitmentSet" : S, "signature" : self.sign(S) , "identity" : self.identity , "finalTxnBlock" : self.finalBlock["finalBlock"] , "finalTxnBlock_signature" : self.sign(self.finalBlock["finalBlock"])}
		print("finalblock-" , self.finalBlock)
		# final Block sent to ntw
		self.finalBlock["sent"] = True
		BroadcastTo_Network(data, "finalTxnBlock")
		if self.state != ELASTICO_STATES["FinalBlockReceived"]:
			self.state = ELASTICO_STATES["FinalBlockSent"]

	def getCommittee_members(committee_id):
		"""
			Returns all members which have this committee id : committee_list[committee_id]
		"""
		pass


	def SendtoFinal(self):
		"""
			Each committee member sends the signed value(txn block after intra committee consensus)
			along with signatures to final committee
		"""
		for finalId in self.finalCommitteeMembers:
			# here txn_block is a set
			data = {"txnBlock" : self.txn_block , "sign" : self.sign(self.txn_block), "identity" : self.identity}
			msg = {"data" : data, "type" : "intraCommitteeBlock" }
			finalId.send(msg)
		self.state = ELASTICO_STATES["Intra Consensus Result Sent to Final"]



	def union(data):
		"""
			Takes ordered set union of agreed values of committees
		"""
		pass


	def validate_signs(signatures):
		"""
			validate the signatures, should be atleast c/2 + 1 signs
		"""
		pass



	def generate_randomstrings(self):
		"""
			Generate r-bit random strings
		"""
		if self.isFinalMember() == True:
			Ri = random_gen(r)
			self.Ri = ("{:0" + str(r) +  "b}").format(Ri)


	def hexdigest(self, msg):
		"""
			returns the digest for a msg
		"""
		commitment = SHA256.new()
		commitment.update(msg.encode())
		return commitment.hexdigest()


	def getCommitment(self):
		"""
			generate commitment for random string Ri. This is done by a
			final committee member
		"""
		if self.isFinalMember() == True:
			if self.Ri == "":
				self.generate_randomstrings()
			commitment = SHA256.new()
			commitment.update(self.Ri.encode())
			return commitment.hexdigest()


	def sendCommitment(self):
		"""
			send the H(Ri) to the final committe members.This is done by a
			final committee member
		"""		
		if self.isFinalMember() == True:
			Hash_Ri = self.getCommitment()
			for nodeId in self.committee_Members:
				data = {"identity" : self.identity , "Hash_Ri"  : Hash_Ri}
				msg = {"data" : data , "type" : "hash"}
				nodeId.send(msg)
			self.state = ELASTICO_STATES["CommitmentSentToFinal"]


	def addCommitment(self, finalBlock):
		"""
			ToDo: Check where to use this
			include H(Ri) ie. commitment in final block
		"""
		Hash_Ri = self.getCommitment()
		finalBlock["hash"] = Hash_Ri


	def BroadcastR(self):
		"""
			broadcast Ri to all the network
		"""
		# if self.Ri == "":
		# 	self.generate_randomstrings()
		data = {"Ri" : self.Ri, "identity" : self.identity}
		msg = {"data" : data , "type" : "RandomStringBroadcast"}
		# for nodeId in NtwParticipatingNodes:
		# 	nodeId.send(msg)
		self.state = ELASTICO_STATES["BroadcastedR"]
		BroadcastTo_Network(data, "RandomStringBroadcast")


	def xor_R(self):
		"""
			find xor of any random c/2 + 1 r-bit strings to set the epoch randomness
		"""
		# ToDo: set_of_Rs must be atleast c/2 + 1, so make sure this
		randomset = SystemRandom().sample(self.set_of_Rs , c//2 + 1)
		xor_val = 0
		for R in randomset:
			xor_val = xor_val ^ int(R, 2)
		self.epoch_randomness = ("{:0" + str(r) +  "b}").format(xor_val)
		return ("{:0" + str(r) +  "b}").format(xor_val) , randomset

	# verify the PoW of the sender
	def verify_PoW(self, identityobj):
		"""
			verify the PoW of the node identityobj
		"""
		# PoW = {"hash" : hash_val, "set_of_Rs" : randomset_R}
		PoW = identityobj.PoW
		# Valid Hash has D leading '0's (in hex)
		if not PoW["hash"].startswith('0' * D):
			return False
		
		# check Digest for set of Ri strings
		for Ri in PoW["set_of_Rs"]:
			digest = self.hexdigest(Ri)
			if digest not in self.RcommitmentSet:
				return False

		# reconstruct epoch randomness

		epoch_randomness = identityobj.epoch_randomness
		if len(PoW["set_of_Rs"]) > 0:
			xor_val = 0
			for R in PoW["set_of_Rs"]:
				xor_val = xor_val ^ int(R, 2)
			epoch_randomness = ("{:0" + str(r) +  "b}").format(xor_val)
		PK = identityobj.PK
		IP = identityobj.IP
		
		# recompute PoW 
		nonce = PoW["nonce"]
		 
		digest = SHA256.new()
		digest.update(IP.encode())
		digest.update(PK.encode())
		digest.update(epoch_randomness.encode())
		digest.update(str(nonce).encode())
		hash_val = digest.hexdigest()
		if hash_val.startswith('0' * D) and hash_val == PoW["hash"]:
			# Found a valid Pow, If this doesn't match with PoW["hash"] then Doesnt verify!
			return True
		return False

	def appendToLedger(self):
		"""
		"""
		pass


	def execute(self, epochTxn):
		"""
			executing the functions based on the running state
		"""
		print(self.identity ,  list(ELASTICO_STATES.keys())[ list(ELASTICO_STATES.values()).index(self.state)], "STATE of a committee member")
		if self.state == ELASTICO_STATES["NONE"]:
			self.compute_PoW()
		elif self.state == ELASTICO_STATES["PoW Computed"]:
			self.form_identity()
		elif self.state == ELASTICO_STATES["Formed Identity"]:
			self.form_committee()
		elif self.is_directory and self.state == ELASTICO_STATES["RunAsDirectory"]:
			# Receive txns from client for an epoch
			k = 0
			num = len(epochTxn) // pow(2,s) 
			# loop in sorted order of committee ids
			for iden in range(pow(2,s)):
				if iden == pow(2,s)-1:
					self.txn[iden] = epochTxn[ k : ]
				else:
					self.txn[iden] = epochTxn[ k : k + num]
				k = k + num
			self.state  = ELASTICO_STATES["RunAsDirectory after-TxnReceived"]
		elif self.state == ELASTICO_STATES["Committee full"]:
			# Now The node should go for Intra committee consensus
			if self.is_directory == False:
				self.runPBFT(self.txn_block, "intra committee consensus")

		elif self.state == ELASTICO_STATES["Formed Committee"]:
			# These Nodes are not part of network
			pass	
		elif self.state == ELASTICO_STATES["PBFT Finished"]:
			self.SendtoFinal()
		
		elif self.isFinalMember() and self.state == ELASTICO_STATES["Intra Consensus Result Sent to Final"]:
			flag = False
			for commId in self.ConsensusMsgCount:
				if self.ConsensusMsgCount[commId] <= c//2:
					flag = True
					break
			if flag == False:
				self.state = ELASTICO_STATES["Final PBFT Start"]
				self.verifyAndMergeConsensusData()
			
		elif self.isFinalMember() and self.state == ELASTICO_STATES["Merged Consensus Data"]:
			self.runPBFT(self.mergedBlock, "final committee consensus")

		elif self.isFinalMember() and self.state == ELASTICO_STATES["PBFT Finished-FinalCommittee"]:
			self.sendCommitment()

		elif self.isFinalMember() and self.state == ELASTICO_STATES["CommitmentSentToFinal"]:
			if len(self.commitments) >= c//2 + 1:
				self.BroadcastFinalTxn()

		elif self.state == ELASTICO_STATES["FinalBlockReceived"] and len(self.committee_Members) == c and self.is_directory == False:
			# input("welcome")
			response = []
			for txnBlock in self.finalBlockbyFinalCommittee:
				if len(self.finalBlockbyFinalCommittee[txnBlock]) >= c//2 + 1:
					response.append(txnBlock)
				else:
					print("less block signs : ", len(self.finalBlockbyFinalCommittee[txnBlock]))
			if len(response) > 0:
				self.state = ELASTICO_STATES["FinalBlockSentToClient"]
				return response
		
		elif self.isFinalMember() and self.state == ELASTICO_STATES["FinalBlockSentToClient"]:
			# broadcast Ri is done when received commitment has atleast c/2  + 1 signatures 
			if len(self.newRcommitmentSet) >= c//2 + 1:
				self.BroadcastR()
		
		elif self.state == ELASTICO_STATES["FinalBlockReceived"]:
			# input("welcome all")
			pass
					
		elif self.state == ELASTICO_STATES["ReceivedR"]:
			# self.reset()
			return "reset"


def Run(epochTxn):
	"""
		runs for one epoch
	"""
	global network_nodes,ledger
	E = []
	if len(network_nodes) == 0:
		network_nodes = E
		# E is the list of elastico objects
		for i in range(n):
			print( "---Running for processor number---" , i + 1)
			E.append(Elastico())
	else:
		E = network_nodes

	epochBlock = set()

	while True:
		resetcount = 0
		for node in network_nodes:
			response = []
			response = node.execute(epochTxn)
			if response == "reset":
				resetcount += 1
				pass
			elif response != None and len(response) != 0:
				for txnBlock in response:
					print(txnBlock)
					# input("ayushiiiiiiiii")
					epochBlock |= eval(txnBlock)
				ledger.append(epochBlock)
		if resetcount == n:
			# ToDo: discuss with sir - earlier I was using broadcast, but it had a problem that anyone can send "reset-all" as msg[type]
			for node in network_nodes:
				msg = {"type": "reset-all", "data" : node.identity}
				if isinstance(node.identity, Identity):
					node.identity.send(msg)
				else:
					print("illegal call")
					node.reset()
			break

	print("ledger block" , ledger)
	input("ledger updated!!")

if __name__ == "__main__":
	# epochTxns - dictionary that maps the epoch number to the list of transactions
	epochTxns = dict()
	for i in range(5):
		# txns is the list of the transactions in one epoch to which the committees will agree on
		txns = []
		for j in range(200):
			random_num = random_gen(32)
			txns.append(random_num)
		epochTxns[i] = txns
	for epoch in epochTxns:
		print("epoch number :-" , epoch + 1 , "started")
		Run(epochTxns[epoch])

