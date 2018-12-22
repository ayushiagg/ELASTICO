from hashlib import sha256
from subprocess import check_output
from Crypto.PublicKey import RSA
import json


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


	def form_committee(committee_id):
		"""
			creates directory committee if not yet created otherwise informs all
			the directory members
		"""	
		pass


	def BroadcastTo_Network(network_nodes , data , type):
		"""
			Broadcast to the whole ntw
		"""
		pass


	def BroadcastTo_Committee(committee_id, data , type):
		"""
			Broadcast to the particular committee id
		"""	
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


	def BroadcastR(Ri , network_nodes):
		"""
			broadcast Ri to all the network
		"""
		pass

