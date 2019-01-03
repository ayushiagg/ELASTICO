from hashlib import sha256
from subprocess import check_output
from Crypto.PublicKey import RSA
from Crypto.Signature import PKCS1_v1_5
from Crypto.Hash import SHA256
import json
import random

# network_nodes - All objects of nodes in the network
global network_nodes, n, s, c, D, r
# n : number of processors
n = 10
# s - where 2^s is the number of committees
s = 20
# c - size of committee
c = 3
# D - difficulty level , leading bits of PoW must have D 0's (keep w.r.t to hex)
D = 5
# r - number of bits in random string 
r = 5


# class Network:
# 	"""
# 		class for networking between nodes
# 	"""

def BroadcastTo_Network(data, type_):
	"""
		Broadcast to the whole ntw
	"""
	# broadcast data obj as directory member to the network nodes
        # comment: no if statements
        # for each instance of Elastico, create a receive(self, msg) method
        # this function will just call node.receive(msg)
        # inside msg, you will need to have a message type, and message data.
	if type_ == "directoryMember":
		for node in network_nodes:
			if len(node.cur_directory) < c:
				node.cur_directory.append( data )


def Send_to_Directory(data):
	"""
		Send about new nodes to directory committee members
	"""
	# Todo : Extract processor identifying information from data in identity and committee_id

	# Add the new processor in particular committee list of curent directory nodes
	for node in data.cur_directory:
		if len(node.commitee_list[data.identity]) < c:
			node.commitee_list[data.identity].append(data)


	for node in data.cur_directory:
		commList = node.commitee_list
		flag = 0
		for iden in commList:
			val = commList[iden]
			if len(val) < c:
				flag = 1
				break

		if flag == 0:
			# Send commList[iden] to members of commList[iden]
			MulticastCommittee(commList)


def BroadcastTo_Committee(node, data , type_):
	"""
		Broadcast to the particular committee id
	"""
	pass


def MulticastCommittee(commList):
	"""
		each node getting views of its committee members from directory members
	"""
	for iden in commList:
		commMembers = commList[iden]
		for member in commMembers:
			# union of committe members views
			member.committee_Members |= set(commMembers)


class Elastico:
	"""
		class members: 
			node - single processor
			identity - committee to which a processor(node) belongs to
			txn_block - block of txns that the committee will agree on
			commitee_list - list of nodes in all committees
			directory_committee - list of nodes in the directory committee
			final_committee - list of nodes in the final committee
			is_direcotory - whether the node belongs to directory committee or not
			is_final - whether the node belongs to final committee or not
			epoch_randomness - r-bit random string generated at the end of previous epoch
			committee_Members - set of committee members in its own committee
			IP - IP address of a node
			key - public key and private key pair for a node
			cur_directory - list of directory members in view of the node
			PoW - 256 bit hash computed by the node
			
	"""

	def __init__(self):
		self.IP = self.get_IP()
		self.key = self.get_key()
		self.PoW = ""
		self.cur_directory = []
		self.identity = ""
		# only when this node is the member of directory committee
		self.committee_list = dict()
		# only when this node is not the member of directory committee
		self.committee_Members = set()
		self.is_directory = False
		self.is_final = False
		# ToDo : Correctly setup epoch Randomness from step 5 of the protocol
		self.epoch_randomness = self.initER()


	def initER(self):
		"""
			initialise r-bit epoch random string
		"""
                # minor comment: this must be cryptographically secure, but this is not.
                # might want to replace this with reads from /dev/urandom.
		randomnum = random.randint(0,2**r-1)
		return ("{:0" + str(r) +  "b}").format(randomnum)


	def get_IP(self):
		"""
			for each node(processor) , get IP addr
			will return IP
		"""
                # comment: use random strings instead of IP address.
                # you could even have the strings be generated in the IP addr format.
		ips = check_output(['hostname', '--all-ip-addresses'])
		ips = ips.decode()
		return ips.split(' ')[0]


	def get_key(self):
		"""
			for each node, get public key
			will return PK
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
			data =  {"IP" : IP , "PK" : PK , "epoch_randomness" : epoch_randomness , "nonce" : nonce}
                        # minor comment: create a sha256 object by calling hashlib.sha256()
                        # then repeatedly call sha256.update(...) with the things that need to be hashed together.
                        # finally extract digest by calling sha256.digest()
                        # don't convert to json and then to string
			data_string = json.dumps(data, sort_keys = True)
			hash_val = sha256(data_string.encode()).hexdigest()
			if hash_val.startswith('0' * D):
				self.PoW = hash_val
				return hash_val
			nonce += 1


	def get_committeeid(self, PoW):
		"""
			returns last s-bit of PoW as Identity : committee_id
		"""	
		bindigest = ''
		for hashdig in PoW:
			bindigest += "{:04b}".format(int(hashdig, 16))
		identity = bindigest[-s:]
		self.identity = int(identity, 2)
		return int(identity, 2)


	def form_committee(self, PoW, committee_id):
		"""
			creates directory committee if not yet created otherwise informs all
			the directory members
		"""	
		# ToDo : Implement verification of PoW(valid or not)
		if len(self.cur_directory) < c:
			self.is_directory = True
			# ToDo : Discuss regarding data
			BroadcastTo_Network(self, "directoryMember")
		else:
			# ToDo : Send the above data only
			Send_to_Directory(self)


	def runPBFT():
		"""
			Runs a Pbft instance for the intra-committee consensus
		"""
		pass


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


	def generate_randomstrings(r):
		"""
			Generate r-bit random strings
		"""

		pass


	def getCommitment(R):
		"""
			generate commitment for random string R
		"""
		commitment = SHA256.new()
		commitment.update(R.encode())
		return commitment.hexdigest()


	def consistencyProtocol(set_of_Rs):
		"""
			Agrees on a single set of Hash values(S)
		"""
		pass


	def BroadcastR(Ri):
		"""
			broadcast Ri to all the network
		"""
		pass


	def xor_R(set_of_Rs):
		"""
			find xor of any random c/2 + 1 r-bit strings
		"""
		# ToDo: set_of_Rs must be atleast c/2 + 1, so make sure this
		randomset = random.sample(set_of_Rs , c//2 + 1)
		xor_val = 0
		for R in randomset:
			xor_val = xor_val ^ int(R, 2)

		return ("{:0" + str(r) +  "b}").format(xor_val)


def Run():
	"""
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

