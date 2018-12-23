from hashlib import sha256
from subprocess import check_output
from Crypto.PublicKey import RSA
import json

global committee_list
commitee_list = dict()

global network_nodes

n = 10

class Network:
	"""
		class for networking between nodes
	"""
	def BroadcastTo_Network(self, data , type_):
		"""
			Broadcast to the whole ntw
		"""
		if type_ == "directoryMember":
			# ToDo : Add to all members of network
			for node in network_nodes:
				if len(node.cur_directory) < c:
					node.cur_directory.append( data )
		pass


	def Send_to_Directory(self, data):
		"""
			Send about new nodes to directory committee members
		"""
		# Todo : Extract processor identifying information from data in identity and committee_id
		info = ""
		committee_id = ""
		commitee_list[committee_id].append(info)
		if len(commitee_list[committee_id]) >= c:
			for i in  commitee_list[committee_id]:
				BroadcastTo_Committee(i , commitee_list[committee_id] , type_)
		pass

	def BroadcastTo_Committee(self, node, data , type_):
		"""
			Broadcast to the particular committee id
		"""
		pass



class Elastico:
	"""
		class members: 
			node - single processor
			committee_id - committee to which a processor(node) belongs to
			txn_block - block of txns that the committee will agree on
			commitee_list - list of nodes in all committees
			directory_committee - list of nodes in the directory committee
			final_committee - list of nodes in the final committee
			is_direcotory - whether the node belongs to directory committee or not
			is_final - whether the node belongs to final committee or not
			global network_nodes - total nodes in the network
			committee_size - number of nodes in a committee
			global r - number of bits in random string 
			epoch_randomness - r-bit random string generated at the end of previous epoch
			global D - difficulty level , leading bits of O must have D 0's	(keep w.r.t to hex)
			IP - IP addr of a node
			key - public key and pvt key pair for a node
			global s - where 2^s is the number of committees
			
	"""

	def __init__(self):
		self.D = 20
		self.IP = get_IP()
		self.key = get_key()
		self.s = 20
		self.c = 3
		self.PoW = ""
		self.cur_directory = []


	def get_IP(self):
		"""
			for each node(processor) , get IP addr
			will return IP
		"""
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


	def compute_PoW(self , epoch_randomness , IP , key):
		"""
			returns hash which satisfies the difficulty challenge(D) : PoW
		"""
		PK = key.publickey().exportKey().decode()
		nonce = 0
		while True: 
			data =  {"IP" : IP , "PK" : PK , "epoch_randomness" : epoch_randomness , "nonce" : nonce}
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
		identity = bindigest[-self.s:]
		return int(identity, 2)


	def form_committee(self, PoW, committee_id):
		"""
			creates directory committee if not yet created otherwise informs all
			the directory members
		"""	
		# ToDo : Implement verification of PoW(valid or not)
		if len(self.cur_directory) < c:
			self.is_direcotory = True
			# ToDo : Discuss regarding data
			BroadcastTo_Network(self, "directoryMember")
		else:
			# ToDo : Send the above data only
			Send_to_Directory()
		pass


	def FormDirectory_Committee():
		"""
			Create the first committee: returns directory_committee is_direcotory
		"""
		pass


	def runPBFT():
		"""
			Runs a Pbft instance for the intra-committee consensus
		"""
		pass


	def sign(data):
		"""
			Sign the data i.e. signature
		"""
		pass


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
		pass


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


		
