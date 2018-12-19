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


	def get_committeeid(O):
		"""
			returns last s-bit
		"""	
		pass

		


