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
fin_num = ""
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
ELASTICO_STATES = {"NONE": 0, "PoW Computed": 1, "Formed Identity" : 2,"Formed Committee": 3, "RunAsDirectory": 4 , "InPBFT" : 5, "Consensus Sent" : 6, "Final Committee in PBFT" : 7, "Sent Final Block" : 8, "Received Final Block" : 9 }



# class Network:
# 	"""
# 		class for networking between nodes
# 	"""

def consistencyProtocol():
	"""
		Agrees on a single set of Hash values(S)
		presently selecting random c hash of Ris from the total set of commitments
	"""
	# ToDo : Re-check before meeting!
	global network_nodes, commitmentSet
	if len(commitmentSet) == 0:
		flag = True
		for node in network_nodes:
			if node.isFinalMember():
				if flag and len(commitmentSet) == 0:
					flag = False
					commitmentSet = node.commitments
				else:	
					commitmentSet = commitmentSet.intersection(node.commitments)
	return commitmentSet


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


def MulticastCommittee(commList):
	"""
		each node getting views of its committee members from directory members
	"""

	print("---multicast committee list to committee members---")
	
	# print(len(commList), commList)
	
	for committee_id in commList:
		commMembers = commList[committee_id]
		for memberId in commMembers:
			# union of committe members views
			msg = {"data" : commMembers , "type" : "committee members views"}
			memberId.send(msg)
			# input()


class Identity:
	"""
		class for the identity of nodes
	"""
	def __init__(self, IP, PK, committee_id, PoW, nonce, epoch_randomness):
		self.IP = IP
		self.PK = PK
		self.committee_id = committee_id
		self.PoW = PoW
		self.nonce = nonce
		self.epoch_randomness = epoch_randomness
		self.partOfNtw = False


	def isEqual(self, identityobj):
		"""
			checking two objects of Identity class are equal or not
		"""
		return self.IP == identityobj.IP and self.PK == identityobj.PK and self.committee_id == identityobj.committee_id \
		and self.PoW == identityobj.PoW and self.nonce == identityobj.nonce and self.epoch_randomness == identityobj.epoch_randomness and self.partOfNtw == identityobj.partOfNtw

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
			identity - identity consists of Public key, an IP, PoW, committee id
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
			PoW - 256 bit hash computed by the node
			Ri - r-bit random string
			commitments - set of H(Ri) received by final committee node members and H(Ri) is sent by the final committee node only
			set_of_Rs - set of Ris obtained from the final committee
			committee_id - integer value to represent the committee to which the node belongs
			final_committee_id - committee id of final committee
			CommitteeConsensusData - a dictionary of committee ids that contains a dictionary of the txn block and the signatures
			finalBlockbyFinalCommittee - a dictionary of txn block and the signatures by the final committee members
			nonce - a number that each processor searches to get a  valid PoW
			state - state in which a node is running
			mergedBlock - list of txns of different committees after their intra committee consensus
			finalBlock - agreed list of txns after pbft run by final committee
	"""

	def __init__(self):
		print("---Constructor of elastico class---")
		self.IP = self.get_IP()
		self.key = self.get_key()
		self.PoW = ""
		self.cur_directory = []
		self.identity = ""
		self.committee_id = ""
		# only when this node is the member of directory committee
		self.committee_list = dict()
		# only when this node is not the member of directory committee
		self.committee_Members = set()
		self.is_directory = False
		self.is_final = False
		# ToDo : Correctly setup epoch Randomness from step 5 of the protocol
		self.epoch_randomness = self.initER()
		self.final_committee_id = ""
		self.Ri = ""
		# only when this node is the member of final committee
		self.commitments = set()
		self.txn_block = ""
		self.set_of_Rs = set()
		self.CommitteeConsensusData = dict()
		self.finalBlockbyFinalCommittee = dict()
		self.nonce = 0
		self.state = ELASTICO_STATES["NONE"]
		self.mergedBlock = []
		self.finalBlock = []
		self.RcommitmentSet = ""

	def reset(self):
		"""
			reset some of the elastico class members
		"""
		self.IP = self.get_IP()
		self.key = self.get_key()
		self.PoW = ""
		self.cur_directory = []
		self.identity = ""
		self.committee_id = ""
		# only when this node is the member of directory committee
		self.committee_list = dict()
		# only when this node is not the member of directory committee
		self.committee_Members = set()
		self.is_directory = False
		self.is_final = False
		# ToDo : Correctly setup epoch Randomness from step 5 of the protocol
		self.final_committee_id = ""
		self.Ri = ""
		# only when this node is the member of final committee
		self.commitments = set()
		self.txn_block = ""
		self.CommitteeConsensusData = dict()
		self.finalBlockbyFinalCommittee = dict()
		self.nonce = 0
		self.state = ELASTICO_STATES["NONE"]
		self.mergedBlock = []
		self.finalBlock = []
		# self.RcommitmentSet = ""


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
				# comment: use random strings instead of IP address.
				# you could even have the strings be generated in the IP addr format.
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
			digest.update(str(self.nonce).encode())
			hash_val = digest.hexdigest()
			if hash_val.startswith('0' * D):
				self.PoW = {"hash" : hash_val, "set_of_Rs" : randomset_R}
				print("---PoW computation end---")
				self.state = ELASTICO_STATES["PoW Computed"]
				return hash_val
			self.nonce += 1


	def form_finalCommittee(self):
		"""
			Select a committee number as final committee id
		"""
		# ToDo: Ensure that the final committee has c members 
		print("---form final committee---")
		global fin_num
		if self.is_directory == True and fin_num == "":
			fin_num = random_gen(s)
			finalCommList = self.committee_list[fin_num]
			# notify the members of the final committee that they are the final committee members
			for finalMember in finalCommList:
				data = {"final member" : fin_num , "identity" : self.identity}
				msg = {"data" : data , "type" : "notify final member"}
				finalMember.send(msg)
		self.final_committee_id = fin_num
		



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
		global identityNodeMap
		print("---form identity---")
		# export public key
		PK = self.key.publickey().exportKey().decode()
		# set the committee id acc to PoW solution
		self.committee_id = self.get_committeeid(self.PoW["hash"])
		self.identity = Identity(self.IP, PK, self.committee_id, self.PoW, self.nonce, self.epoch_randomness)
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
			# input()
			print("---seen c members---")
			self.Send_to_Directory()
			self.state = ELASTICO_STATES["Formed Committee"]	


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
		# for iden in commList:
		# 	val = commList[iden]
		# 	if len(val) < c:
		# 		flag = 1
		# 		break

		for iden in range(pow(2,s)):
			if iden not in commList or len(commList[iden]) < c:
				flag = 1
				break
		if flag == 0:
			# Send commList[iden] to members of commList[iden]
			print("----------committees full----------------")
			MulticastCommittee(commList)
			self.form_finalCommittee()

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
		elif msg["type"] == "committee members views":
			commMembers = msg["data"]
			self.committee_Members |= set(commMembers)
			print("commMembers for committee id - " , self.committee_id, "is :-", self.committee_Members)

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
				if HashRi in self.RcommitmentSet:
					self.set_of_Rs.add(Ri)

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
					self.RcommitmentSet = received_commitmentSet
				
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


		elif msg["type"] == "set_of_txns":
			# ToDo: Add verify pow step in this
			data = msg["data"]
			# txn_block is a list of txns
			self.txn_block = data["txn_block"]		

		elif msg["type"] == "request committee list from directory member":
			if self.is_directory == False:
				return False , dict()
			else:
				commList = self.committee_list
				return True , commList

		elif msg["type"] == "command to run pbft":
			if self.is_directory == False:
				self.runPBFT(self.txn_block)

		elif msg["type"] == "command to run pbft by final committee":
			if self.isFinalMember():
				self.runPBFT(self.mergedBlock)


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
			# ToDo:verify the data 
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
		print(self.mergedBlock)
		input("Check merged block above!")

	
	def runPBFT(self , txnBlock):
		"""
			Runs a Pbft instance for the intra-committee consensus
		"""
		# ToDo : Final committee presently doesnt participate in intracommittee consensus, fix this
		# print("run pbft")
		txn_set = set()
		for txn in txnBlock:
			txn_set.add(txn)
		if self.isFinalMember():
			self.finalBlock = txn_set
		else:	
			self.txn_block = txn_set	

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
		S = consistencyProtocol()
		data = {"commitmentSet" : S, "signature" : self.sign(S) , "identity" : self.identity , "finalTxnBlock" : self.finalBlock , "finalTxnBlock_signature" : self.sign(self.finalBlock)}
		print("finalblock-" , self.finalBlock)
		msg = {"data" : data , "type" : "finalTxnBlock"}
		# for nodeId in NtwParticipatingNodes:
		# 	nodeId.send(msg)
		BroadcastTo_Network(data, "finalTxnBlock")		


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
		if self.final_committee_id == "":
			self.form_finalCommittee()
		# ToDo : Any better way? Think!
		# to get final committee members, we need a directory committee node
		nodeId = self.cur_directory[0]
		data = {"committee_id" : self.final_committee_id , "identity" : self.identity}
		msg = {"data" : data , "type" : "getCommitteeMembers"}
		# response has final committee members
		boolval, response = nodeId.send(msg)
		for finalId in response:
			# here txn_block is a set
			data = {"txnBlock" : self.txn_block , "sign" : self.sign(self.txn_block), "identity" : self.identity}
			msg = {"data" : data, "type" : "intraCommitteeBlock" }
			finalId.send(msg)
		


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
		nonce = identityobj.nonce
		 
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



def Run(epochTxn):
	"""
		runs for one epoch
	"""
	global network_nodes, fin_num, identityNodeMap
	E = []
	if len(network_nodes) == 0:
		network_nodes = E
		# E is the list of elastico objects
		for i in range(n):
			print( "---Running for processor number---" , i + 1)
			E.append(Elastico())
	else:
		E = network_nodes
		for i in range(n):
			print( "---Running for processor number---" , i + 1)
			E[i].reset() 	
	# Id - identity of the nodes
	Id = [[] for i in range(n)]
	fin_num = ""
	identityNodeMap = dict()
	# commitmentSet = set()
	objIndex = set(range(n))
	while True:
		flag = False
		for i in objIndex.copy():
			# if the state is NONE, then each node has to compute its PoW
			if E[i].state == ELASTICO_STATES["NONE"]:
				E[i].compute_PoW()
				flag = True
			# if the PoW computed for a node, then each processor will be assigned to a committee based on its identity
			elif E[i].state == ELASTICO_STATES["PoW Computed"]:
				Id[i] = E[i].form_identity()
				E[i].form_committee()
				objIndex.remove(i)
		if flag == False:
			break
	print("\n\n")
	print("########### STEP 1 Done ###########")
	print("-----------------------------------------------------------------------------------------------")
	print("########### STEP 2 Done ###########")
	print("-----------------------------------------------------------------------------------------------")
	print("\n\n")
	# input()

	# ToDo: We are communicating Id, some Identity objects may not be the part of the network. So, fix this.
	# NtwParticipatingNodes - list of nodes those are the part of some committee
	global NtwParticipatingNodes
	NtwParticipatingNodes = []

	# finalMembers - list of identity objects of the final committee members
	finalMembers = []
	for node in Id:
		data = {"identity" :  node}
		type_ = "request committee list from directory member"
		msg = {"data" : data , "type" : type_}
		is_directory , commList = node.send(msg)
		if is_directory == True:
			k = 0
			# loop in sorted order of committee ids
			finalMembers = commList[fin_num]
			print("finalMembers - " , finalMembers)
			# input("See the final Members")
			for iden in commList:
				txn = epochTxn[ k : k + 8]
				k = k + 8
				commMembers = commList[iden]
				for commMemberId in commMembers:
					# partOfNtw is set to true when the nodes are participating in the ntw(part of any committee)
					commMemberId.partOfNtw = True
					NtwParticipatingNodes.append(commMemberId)
					data = {"txn_block" : txn}
					msg = {"data" : data , "type" : "set_of_txns"}
					commMemberId.send(msg)
			break

	print("No. of NtwParticipatingNodes : ", len(NtwParticipatingNodes))

	print("\n")
	print("---set of txns added to each committee---")
	print("\n")

	# ToDo: To run the real pbft implementation
	for node in NtwParticipatingNodes:
		data = {"identity" :  node}
		type_ = "command to run pbft"
		msg = {"data" : data , "type" : type_}
		node.send(msg)
	
	print("\n")
	print("---PBFT FINISH---")
	print("\n")

	for node in NtwParticipatingNodes:
		data = {"identity" :  node}
		type_ = "send txn set and sign to final committee"
		msg = {"data" : data , "type" : type_}
		node.send(msg)

	print("\n\n")
	print("########### STEP 3 Done ###########")	
	print("-----------------------------------------------------------------------------------------------")
	print("\n\n")

	# for node in Id:
	# 	data = {"identity" :  node , "committee_id" : fin_num}
	# 	type_ = "getCommitteeMembers"
	# 	msg = {"data" : data , "type" : type_}
	# 	is_directory , commList = node.send(msg)
	# 	if is_directory == True:
	# 		for commNodeId in commList:
	# 			data = {"identity" :  commNodeId}
	# 			type_ = "verify and merge intra consensus data"
	# 			msg = {"data" : data , "type" : type_}
	# 			commNodeId.send(msg)
	# 		break
	for finalNode in finalMembers:
		data = {"identity" :  finalNode}
		type_ = "verify and merge intra consensus data"
		msg = {"data" : data , "type" : type_}
		finalNode.send(msg)

	for node in finalMembers:
		data = {"identity" :  node}
		type_ = "command to run pbft by final committee"
		msg = {"data" : data , "type" : type_}
		node.send(msg)	

	print("\n")
	print("---PBFT FINISH BY FINAL COMMITTEE---")
	print("\n")
	
	for node in finalMembers:
		# ToDo: try to do this only for final committee members not for whole ntw
		data = {"identity" :  node}
		type_ = "send commitments of Ris"
		msg = {"data" : data , "type" : type_}
		node.send(msg)


	for node in finalMembers:
		data = {"identity" :  node}
		type_ = "broadcast final set of txns to the ntw"
		msg = {"data" : data , "type" : type_}
		node.send(msg)


	print("\n\n")
	print("########### STEP 4 Done ###########")	
	print("-----------------------------------------------------------------------------------------------")
	print("\n\n")


	for node in finalMembers:
		# each final committee member broadcasts the random string Ri to everyone on the ntw
		data = {"identity" : node}
		type_= "Broadcast Ri"
		msg = {"data" : data , "type" : type_}
		node.send(msg)

	finalSet = []
	for node in NtwParticipatingNodes:
		data = {"identity" : node}
		msg = {"data" : data, "type" :  "append to ledger"}
		response = node.send(msg)
		# print("response" , type(response) , " ", response , " type of eval" , type(eval(response)))
		finalSet.append(response)
		# print(response)
		# input()

	epochBlock = set()
	for blocklist in finalSet:
		for block in blocklist:
			fin_block = eval(block)
			epochBlock |= fin_block

	
	ledger.append(epochBlock)
	print("ledger block" , ledger)	
	input()
	print("\n\n")
	print("########### STEP 5 Done ###########")
	print("-----------------------------------------------------------------------------------------------")
	print("\n\n")


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

