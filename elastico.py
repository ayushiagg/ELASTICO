from hashlib import sha256


class Elastico:
	"""
		class members: 
			node - single processor
			committee_id - committee to which a processor(node) belongs to
			txn_block - block of txns that the committee will agree on
			same_committee - list of nodes in the same committee
			directory_committee - list of nodes in the directory committee
			final_committee - list of nodes in the final committee
			is_direcotory - whether the node belongs to directory committee or not
			is_final - whether the node belongs to final committee or not
			network_nodes - total nodes in the network
			committee_size - number of nodes in a committee
			global r - number of bits in random string 
			epoch_randomness - r-bit random string generated at the end of previous epoch
			global D - difficulty level , leading bits of O must have D 0's	
			IP - IP addr of a node
			PK - public key of a node

			
	"""
	def get_IP():
		"""
			for each node(processor) , get IP addr
		"""
		pass


	def get_PK():
		"""
			for each node, get public key
		"""
		pass

	
	def search_nonce(epoch_randomness , IP, PK , D):
		"""
			returns hash which satisfies the difficulty challenge
		"""
		pass


	def get_committeeid(PoW):
		"""
			returns last s-bit of PoW as Identity
		"""	
		pass

	def form_committee():
		"""
			creates directory committee if not yet created otherwise informs all
			the directory members
		"""	
		pass


	def BroadcastTo_Network():
		"""
			Broadcast to the whole ntw
		"""
		pass

	
	def BroadcastTo_Committee(committee_id):
		"""
			Broadcast to the particular committee id
		"""	
		pass


	def FormDirectory_Committee():
		"""
			Create the first committee
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
			Returns all members which have this committee id
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


	def BroadcastR():
		"""
		"""
		pass

