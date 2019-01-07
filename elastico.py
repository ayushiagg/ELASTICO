from subprocess import check_output
from Crypto.PublicKey import RSA
from Crypto.Signature import PKCS1_v1_5
from Crypto.Hash import SHA256
from secrets import SystemRandom

# network_nodes - All objects of nodes in the network
global network_nodes, n, s, c, D, r, identityNodeMap, fin_num, commitmentSet
# n : number of processors
n = 48
# s - where 2^s is the number of committees
s = 4
# c - size of committee
c = 3
# D - difficulty level , leading bits of PoW must have D 0's (keep w.r.t to hex)
D = 4 
# r - number of bits in random string 
r = 5
# fin_num - final committee id
fin_num = ""
# identityNodeMap- mapping of identity object to Elastico node
identityNodeMap = dict()
# commitmentSet - set of commitments S
commitmentSet = set()


# class Network:
# 	"""
# 		class for networking between nodes
# 	"""

def consistencyProtocol():
	"""
		Agrees on a single set of Hash values(S)
		presently selecting random c hash of Ris from the total set of commitments
	"""
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


def random_gen(size):
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
		Broadcast to the whole ntw
	"""
		# comment: no if statements
		# for each instance of Elastico, create a receive(self, msg) method
		# this function will just call node.receive(msg)
		# inside msg, you will need to have a message type, and message data.
	msg = {"type" : type_ , "data" : data}
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
	for committee_id in commList:
		commMembers = commList[committee_id]
		for memberId in commMembers:
			# union of committe members views
			msg = {"data" : commMembers , "type" : "committee members views"}
			send(memberId, msg)


def send(nodeIdentity, msg):
	"""
		send the info to the nodes based on their identity
	"""
	node = identityNodeMap[nodeIdentity]
	response = node.receive(msg)
	return response

class Identity:
	"""
		class for the identity of nodes
	"""
	def __init__(self, IP, PK, committee_id, PoW):
		self.IP = IP
		self.PK = PK
		self.identity = committee_id
		self.PoW = PoW


	def isEqual(self, identityobj):
		"""
			checking two objects of Identity class are equal or not
		"""
		return self.IP == identityobj.IP and self.PK == identityobj.PK and self.identity == identityobj.identity and self.PoW == identityobj.PoW


class Elastico:
	"""
		class members: 
			node - single processor
			identity - identity consists of Public key, an IP, PoW, committee id
			txn_block - block of txns that the committee will agree on
			commitee_list - list of nodes in all committees
			final_committee - list of nodes in the final committee
			is_direcotory - whether the node belongs to directory committee or not
			is_final - whether the node belongs to final committee or not
			epoch_randomness - r-bit random string generated at the end of previous epoch
			committee_Members - set of committee members in its own committee
			IP - IP address of a node
			key - public key and private key pair for a node
			cur_directory - list of directory members in view of the node
			PoW - 256 bit hash computed by the node
			Ri - r-bit random string
			commitments - set of H(Ri) received by final committee node members
			set_of_Rs - set of Ris obtained from the final committee
			committee_id - integer value to represent the committee to which the node belongs
			final_committee_id - committee id of final committee
	"""

	def __init__(self):
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

	def initER(self):
		"""
			initialise r-bit epoch random string
		"""
				# minor comment: this must be cryptographically secure, but this is not.
				# might want to replace this with reads from /dev/urandom.
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
		key = RSA.generate(2048)
		return key


	def compute_PoW(self):
		"""
			returns hash which satisfies the difficulty challenge(D) : PoW
		"""
		PK = self.key.publickey().exportKey().decode()
		IP = self.IP
		epoch_randomness = self.epoch_randomness
		nonce = 0
		while True: 
						# minor comment: create a sha256 object by calling hashlib.sha256()
						# then repeatedly call sha256.update(...) with the things that need to be hashed together.
						# finally extract digest by calling sha256.digest()
						# don't convert to json and then to string
						# bug is possible in this, find and fix it.			
			digest = SHA256.new()
			digest.update(IP.encode())
			digest.update(PK.encode())
			digest.update(epoch_randomness.encode())
			digest.update(str(nonce).encode())
			hash_val = digest.hexdigest()
			if hash_val.startswith('0' * D):
				self.PoW = hash_val
				return hash_val
			nonce += 1


	def form_finalCommittee(self):
		if self.is_directory == True and fin_num == "":
			fin_num = random_gen(s)
		self.final_committee_id = fin_num


	def get_committeeid(self, PoW):
		"""
			returns last s-bit of PoW as Identity : committee_id
		"""	
		bindigest = ''
		for hashdig in PoW:
			bindigest += "{:04b}".format(int(hashdig, 16))
		identity = bindigest[-s:]
		self.committee_id = int(identity, 2)
		return int(identity, 2)


	def form_identity(self):
		"""
			identity formation for a node
			identity consists of public key, ip, committee id, PoW
		"""
		if self.identity == "":
			PK = self.key.publickey().exportKey().decode()
			if self.committee_id == "":
				if self.PoW == "":
					self.compute_PoW()
				self.get_committeeid(self.PoW)
			self.identity = Identity(self.IP, PK, self.committee_id, self.PoW)
		identityNodeMap[self.Identity] = self
		return self.identity


	def is_OwnIdentity(self, identityobj):
		"""
			Checking whether the identityobj is the Elastico node's identity or not
		"""
		if self.identity == "":
			self.form_identity()
		return self.identity.isEqual(identityobj)


	def form_committee(self, PoW, committee_id):
		"""
			creates directory committee if not yet created otherwise informs all
			the directory members
		"""	
		# ToDo : Implement verification of PoW(valid or not)
		if len(self.cur_directory) < c:
			self.is_directory = True
			# ToDo : Discuss regarding data
			BroadcastTo_Network(self.identity, "directoryMember")
		else:
			# ToDo : Send the above data only
			Send_to_Directory()


	def Send_to_Directory(self):
		"""
			Send about new nodes to directory committee members
		"""
		# Todo : Extract processor identifying information from data in identity and committee_id

		# Add the new processor in particular committee list of curent directory nodes
		
		for nodeId in self.cur_directory:
			msg = {"data" : self.identity, "type" : "newNode"}
			send(nodeId, msg)


		for nodeId in self.cur_directory:
			msg = {"data" : "" , "type" : "checkCommitteeFull"}
			send(nodeId, msg)
			# commList = node.commitee_list
			# flag = 0
			# for iden in commList:
			# 	val = commList[iden]
			# 	if len(val) < c:
			# 		flag = 1
			# 		break

			# if flag == 0:
			# 	# Send commList[iden] to members of commList[iden]
			# 	MulticastCommittee(commList)



	def receive(self, msg):
		"""
			method to recieve messages for a node
		"""
		if msg["type"] == "directoryMember":
			if len(self.cur_directory) < c:
				self.cur_directory.append(msg["data"])

		# new node is added to the corresponding committee list
		if msg["type"] == "newNode" and self.is_direcotory:
			identityobj = msg["data"]
			if len(self.commitee_list[identityobj.committee_id]) < c:
				self.commitee_list[identityobj.committee_id].append(identityobj)

		if msg["type"] == "checkCommitteeFull":
			commList = self.commitee_list
			flag = 0
			for iden in commList:
				val = commList[iden]
				if len(val) < c:
					flag = 1
					break

			if flag == 0:
				# Send commList[iden] to members of commList[iden]
				MulticastCommittee(commList) 

		# union of committe members views
		if msg["type"] == "committee members views":
			commMembers = msg["data"]
			self.committee_Members |= set(commMembers)

		# receiving H(Ri) by final committe members
		if msg["type"] == "hash" and self.isFinalMember():
			self.commitments.add(msg["data"])

		if msg["type"] == "RandomStringBroadcast":
			Ri = msg["data"]
			HashRi = self.hexdigest(Ri)
			if HashRi in commitmentSet:
				self.set_of_Rs.add(msg["data"])

		if msg["type"] == "finalTxnBlock":
			data = msg["data"]
			# data = {"commitmentSet" : S, "signature" : self.sign(S) , "finalTxnBlock" : self.txn_block}			
			sign = data["signature"]
			received_commitmentSet = data["commitmentSet"]
			identity = data["identity"]
			PK = identity.PK
			finalTxnBlock = data["finalTxnBlock"]
			finalTxnBlock_signature = data["finalTxnBlock_signature"]
			if self.verify_sign(sign, received_commitmentSet, PK) and self.verify_sign(finalTxnBlock_signature, finalTxnBlock, PK):
				# ToDo : to take step regarding this
				pass
				

	def runPBFT():
		"""
			Runs a Pbft instance for the intra-committee consensus
		"""
		pass

	def isFinalMember(self):
		"""
			tell whether this node is a final committee member or not
		"""
		if self.committee_id == fin_num:
			self.is_final = True
			return True
		return False


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
		if self.isFinalMember():
			S = consistencyProtocol()
			# ToDo: Add signatures on S
			data = {"commitmentSet" : S, "signature" : self.sign(S) , "identity" : self.identity , "finalTxnBlock" : self.txn_block , "finalTxnBlock_signature" : self.sign(self.txn_block)}
			BroadcastTo_Network(data, "finalTxnBlock")		


	def getCommittee_members(committee_id):
		"""
			Returns all members which have this committee id : commitee_list[committee_id]
		"""
		pass


	def Sendto_final(committee_id, signatures, txn_set, final_committee_id):
		"""
			Each committee member sends the signed value along with signatures to final committee
		"""
		pass


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


	def Broadcast_signature(signature, signedVal):
		"""
			Send the signature after vaidation to complete network
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
			for nodeId in committee_Members:
				msg = {"data" : Hash_Ri , "type" : "hash"}
				send(nodeId , msg)


	def addCommitment(self, finalBlock):
		"""
			include H(Ri) ie. commitment in final block
		"""
		Hash_Ri = self.getCommitment()
		finalBlock["hash"] = Hash_Ri


	def BroadcastR(self):
		"""
			broadcast Ri to all the network
		"""
		if self.Ri == "":
			self.generate_randomstrings()
		BroadcastTo_Network(self.Ri, "RandomStringBroadcast")


	def xor_R(self):
		"""
			find xor of any random c/2 + 1 r-bit strings
		"""
		# ToDo: set_of_Rs must be atleast c/2 + 1, so make sure this
		randomset = SystemRandom().sample(self.set_of_Rs , c//2 + 1)
		xor_val = 0
		for R in randomset:
			xor_val = xor_val ^ int(R, 2)
		self.epoch_randomness = ("{:0" + str(r) +  "b}").format(xor_val)
		return ("{:0" + str(r) +  "b}").format(xor_val) , randomset


def Run(txns):
	"""
		each run is one epoch
		run processors to compute PoW and form committees
	"""
	
	E = [[] for i in range(n)]
	network_nodes = E
	h = [[] for i in range(n)]
	for i in range(n):
		E[i] = Elastico()
		h[i] = E[i].compute_PoW()
		identity = E[i].get_committeeid(h[i])
		E[i].form_committee(h[i] , identity)

