from subprocess import check_output
from Crypto.PublicKey import RSA
from Crypto.Signature import PKCS1_v1_5
from Crypto.Hash import SHA256
from secrets import SystemRandom
import socket
import json, pika, threading, pickle
# for creating logs
import logging
# for multi-processing
from multiprocessing import Process, Lock, Manager
import time

global network_nodes, n, s, c, D, r, identityNodeMap, fin_num, commitmentSet, ledger,  epochBlock, port, lock

lock = Lock()
# n : number of nodes
n = 30
# s - where 2^s is the number of committees
s = 2
# c - size of committee
c = 2
# D - difficulty level , leading bits of PoW must have D 0's (keep w.r.t to hex)
D = 3
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
# network_nodes - list of all nodes 
network_nodes = []
# final block in an epoch
epochBlock = []
# port - avaliable ports start from here
port = 49152 

# ELASTICO_STATES - states reperesenting the running state of the node
ELASTICO_STATES = {"NONE": 0, "PoW Computed": 1, "Formed Identity" : 2,"Formed Committee": 3, "RunAsDirectory": 4 ,"Receiving Committee Members" : 5,"Committee full" : 6 , "PBFT Finished" : 7, "Intra Consensus Result Sent to Final" : 8, "FinalBlockSent" : 9, "FinalBlockReceived" : 10, "RunAsDirectory after-TxnReceived" : 11, "RunAsDirectory after-TxnMulticast" : 12, "Final PBFT Start" : 13, "Merged Consensus Data" : 14, "PBFT Finished-FinalCommittee" : 15 , "CommitmentSentToFinal" : 16, "BroadcastedR" : 17, "ReceivedR" :  18, "FinalBlockSentToClient" : 19}

def consistencyProtocol():
	"""
		Agrees on a single set of Hash values(S)
		presently selecting random c hash of Ris from the total set of commitments
	"""
	# ToDo: fix this 
	global network_nodes, commitmentSet

	for node in network_nodes:
		if node.isFinalMember():
			if len(node.commitments) <= c//2:
				logging.warning("insufficientCommitments")
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
	#   return int.from_bytes(f.read(4), 'big')
	random_num = SystemRandom().getrandbits(size)
	return random_num


def BroadcastTo_Network(data, type_):
	"""
		Broadcast data to the whole ntw
	"""

	global network_nodes

	msg = { "data" : data , "type" : type_ }

	for node in network_nodes:
		try:
			connection = pika.BlockingConnection(pika.ConnectionParameters(host='localhost'))
			channel = connection.channel()

			port = node.port

			# create a hello queue to which the message will be delivered
			channel.queue_declare( queue= 'hello' + str(port) )
			
			serialized_data = pickle.dumps(msg)
			channel.basic_publish(exchange='', routing_key='hello' + str(port), body=serialized_data)
			
			# close the connection
			connection.close()

		except Exception as e:
			logging.error("error in broadcast to network" , exc_info=e)
			if isinstance(e, ConnectionRefusedError):
				logging.error("ConnectionRefusedError at port : %s", str(node.port))
			raise e


def MulticastCommittee(commList, identityobj_dict, txns):
	"""
		each node getting views of its committee members from directory members
	"""
	try:
		finalCommitteeMembers = commList[fin_num]
		for committee_id in commList:
			commMembers = commList[committee_id]
			for memberId in commMembers:
				data = {"committee members" : commMembers , "final Committee members"  : finalCommitteeMembers , "txns" : txns[committee_id] ,"identity" : identityobj_dict}
				msg = {"data" : data , "type" : "committee members views"}
				memberId.send(msg)
	except Exception as e:
		logging.error("error in multicast committees list", exc_info=e)
		raise e


class Identity:
	"""
		class for the identity of nodes
	"""
	def __init__(self, IP, PK, committee_id, PoW, epoch_randomness, port):
		self.IP = IP
		self.PK = PK
		self.committee_id = committee_id
		self.PoW = PoW
		self.epoch_randomness = epoch_randomness
		self.partOfNtw = False
		self.port = port


	def isEqual(self, identityobj):
		"""
			checking two objects of Identity class are equal or not
		"""
		return self.IP == identityobj.IP and self.PK == identityobj.PK and self.committee_id == identityobj.committee_id \
		and self.PoW == identityobj.PoW and self.epoch_randomness == identityobj.epoch_randomness and self.partOfNtw == identityobj.partOfNtw and self.port == identityobj.port


	def send(self, msg):
		"""
			send the msg to node based on their identity
		"""
		try:
			logging.info("sending msg - %s" , str(msg))

			# establish a connection with RabbitMQ server
			connection = pika.BlockingConnection(pika.ConnectionParameters(host='localhost'))
			channel = connection.channel()

			port = self.port

			# create a hello queue to which the message will be delivered
			channel.queue_declare( queue= 'hello' + str(port) )
			serialized_data = pickle.dumps(msg)
			if channel.basic_publish(exchange='', routing_key='hello' + str(port), body= serialized_data, properties=pika.BasicProperties(delivery_mode=1)):
				pass
			else:
				logging.error("messgae not published %s" , str(msg))	

			# close the connection
			connection.close()
		except Exception as e:
			logging.error("error at send msg ", exc_info=e)
			raise e

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
			set_of_Rs - set of Ris obtained from the final committee of previous epoch
			newset_of_Rs - In the present epoch, set of Ris obtained from the final committee
			committee_id - integer value to represent the committee to which the node belongs
			final_committee_id - committee id of final committee
			CommitteeConsensusData - a dictionary of committee ids that contains a dictionary of the txn block and the signatures
			finalBlockbyFinalCommittee - a dictionary of txn block and the signatures by the final committee members
			state - state in which a node is running
			mergedBlock - list of txns of different committees after their intra committee consensus
			finalBlock - agreed list of txns after pbft run by final committee
			RcommitmentSet - set of H(Ri)s received from the final committee after the consistency protocol [previous epoch values]
			newRcommitmentSet - For the present it contains the set of H(Ri)s received from the final committee after the consistency protocol
			finalCommitteeMembers - members of the final committee received from the directory committee
			txn- transactions stored by the directory members
			ConsensusMsgCount - count of intra consensus blocks of each committee received by the final committee
			flag- to denote a bad or good node
	"""

	def __init__(self):
		print("---Constructor of elastico class---")
		self.IP = self.get_IP()
		self.port = self.get_port()
		self.key = self.get_key()
		self.PoW = {"hash" : "", "set_of_Rs" : "", "nonce" : 0}
		self.cur_directory = set()
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
		# self.socketConn = self.get_socket()
		self.response = []
		self.flag = True
		# self.serve = False
		self.views = set()

	def reset(self):
		"""
			reset some of the elastico class members
		"""
		try:
			self.IP = self.get_IP()
			self.key = self.get_key()
			connection = pika.BlockingConnection(pika.ConnectionParameters(host='localhost'))
			channel = connection.channel()
			channel.queue_delete(queue='hello' + str(self.port))
			connection.close()
			self.port = self.get_port()
			self.PoW = {"hash" : "", "set_of_Rs" : "", "nonce" : 0}
			self.cur_directory = set()
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
			# self.socketConn = self.get_socket()
			self.flag = True
			# self.serve = False
			self.views = set()
		except Exception as e:
			logging.error("error in reset", exc_info=e)
			raise e

	def initER(self):
		"""
			initialise r-bit epoch random string
		"""
				# minor comment: this must be cryptographically secure, but this is not.
				# might want to replace this with reads from /dev/urandom.
		randomnum = random_gen(r)
		return ("{:0" + str(r) +  "b}").format(randomnum)

	def get_port(self):
		"""
		"""
		try:
			lock.acquire()
			global port
			port += 1
		except Exception as e:
			logging.error("error in acquiring port lock" , exc_info=e)
			raise e
		finally:
			lock.release()
			return port 
		
		


	# def get_socket(self):
	# 	"""
	# 	"""
	# 	s = socket.socket()
	# 	print ("Socket successfully created")
	# 	# Modified 
	# 	s.bind(('', self.port))
	# 	print ("socket binded to %s" %(port) )
	# 	return s


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
				# logging.warning("set of Rs greater than zero")
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
				nonce = self.PoW["nonce"]
				self.PoW = {"hash" : hash_val, "set_of_Rs" : randomset_R, "nonce" : nonce}
				# print("---PoW computation end---")
				self.state = ELASTICO_STATES["PoW Computed"]
				return hash_val
			self.PoW["nonce"] += 1


	def notify_finalCommittee(self):
		"""
			notify the members of the final committee that they are the final committee members
		"""

		finalCommList = self.committee_list[fin_num]
		for finalMember in finalCommList:
			data = {"identity" : self.identity.__dict__}
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
			
			# export public key
			PK = self.key.publickey().exportKey().decode()
			
			# set the committee id acc to PoW solution
			self.committee_id = self.get_committeeid(self.PoW["hash"])
			
			self.identity = Identity(self.IP, PK, self.committee_id, self.PoW, self.epoch_randomness,self.port)
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
			# logging.warning( "checking %s - %s" , str(self.IP) , str(self.identity.IP) )

			# logging.warning(" %s - %s - %s -  %s- not seen c members yet, so broadcast to ntw---" , str(self.port)  ,str(self.identity) , str(self.committee_id) , str(self.IP))
			# ToDo: do all broadcast asynchronously
			BroadcastTo_Network(self.identity.__dict__, "directoryMember")
			self.state = ELASTICO_STATES["RunAsDirectory"]
		else:
			# track previous state before adding in committee
			# prevState = self.state
			
			self.Send_to_Directory()
			# ToDo : check state assignment order
			# if prevState == ELASTICO_STATES["Formed Identity"] and self.state == ELASTICO_STATES["Receiving Committee Members"]:
			# if self.state == ELASTICO_STATES["Receiving Committee Members"]:
			# 	msg = {"data" : self.identity ,"type" : "Committee full"}
			# 	BroadcastTo_Network(msg["data"] , msg["type"])
			if self.state != ELASTICO_STATES["Receiving Committee Members"]: 
				self.state = ELASTICO_STATES["Formed Committee"]
				# broadcast committee full state notification to all nodes when the present state is "Received Committee members"


	def Send_to_Directory(self):
		"""
			Send about new nodes to directory committee members
		"""
		# Add the new processor in particular committee list of directory committee nodes
		print("---Send to directory---")
		for nodeId in self.cur_directory:
			msg = {"data" : self.identity.__dict__, "type" : "newNode"}
			nodeId.send(msg)



	def checkCommitteeFull(self):
		"""
			directory member checks whether the committees are full or not
		"""
		commList = self.committee_list
		flag = 0
		for iden in range(pow(2,s)):
			if iden not in commList or len(commList[iden]) < c:
				logging.warning("committees not full  - bad miss id : %s", str(iden))
				flag = 1
				break
		if flag == 0:
			# Send commList[iden] to members of commList[iden]
			logging.warning("committees full  - good")
			if self.state == ELASTICO_STATES["RunAsDirectory"]:
				logging.error("directory member has not yet received the epochTxn")
				# directory member has not yet received the epochTxn
				pass
			if self.state == ELASTICO_STATES["RunAsDirectory after-TxnReceived"]:
				self.notify_finalCommittee()
				MulticastCommittee(commList, self.identity.__dict__, self.txn)
				self.state = ELASTICO_STATES["RunAsDirectory after-TxnMulticast"]
				# ToDo: transition of state to committee full 


	def unionViews(self, nodeData, incomingData):
		"""
		"""
		for data in incomingData:
			flag = False
			for nodeId in nodeData:
				if nodeId.isEqual(data):
					flag = True
					break
			if flag == False:
				nodeData.add(data)
		return nodeData

						

	def receive(self, msg):
		"""
			method to recieve messages for a node as per the type of a msg
		"""
		logging.info("call to receive method with type - %s ",str(msg["type"]))
		try:
			# new node is added in directory committee if not yet formed
			if msg["type"] == "directoryMember":
				identityobj = msg["data"]
				# logging.warning("directory member to be appended %s" , str(identityobj))
				# logging.warning(" %s - %s - %s -  %s- directory member to get appended" , str(identityobj.port)  ,str(identityobj) , str(identityobj.committee_id) , str(identityobj.IP) )
				# verify the PoW of the sender
				if self.verify_PoW(identityobj):
					if len(self.cur_directory) < c:
						logging.info("incoming receive call with msg type %s" , str(msg["type"]))
						idenobj = Identity(identityobj["IP"] , identityobj["PK"] , identityobj["committee_id"], identityobj["PoW"], identityobj["epoch_randomness"] , identityobj["port"])
						flag = True
						for obj in self.cur_directory:
							if idenobj.isEqual(obj):
								flag = False
								break
						if flag:
							self.cur_directory.add(idenobj)		
						# self.cur_directory.add(identityobj)
				else:
					logging.error("%s  PoW not valid of an incoming directory member " , str(identityobj) )

			# new node is added to the corresponding committee list if committee list has less than c members
			elif msg["type"] == "newNode" and self.is_directory:
				identityobj = msg["data"]
				if self.verify_PoW(identityobj):
					idenobj = Identity(identityobj["IP"] , identityobj["PK"] ,identityobj["committee_id"], identityobj["PoW"], identityobj["epoch_randomness"] , identityobj["port"])
					if identityobj["committee_id"] not in self.committee_list:
						# Add the identity in committee
						self.committee_list[identityobj["committee_id"]] = [idenobj]

					elif len(self.committee_list[identityobj["committee_id"]]) < c:
						# Add the identity in committee
						flag = True
						for obj in self.committee_list[identityobj["committee_id"]]:
							if idenobj.isEqual(obj):
								flag = False
								break
						if flag:
							# self.cur_directory.add(idenobj)
							self.committee_list[identityobj["committee_id"]].append(idenobj)
							if len(self.committee_list[identityobj["committee_id"]]) == c:
								# check that if all committees are full
								self.checkCommitteeFull()
				else:
					logging.error("PoW not valid in adding new node")

			# union of committe members views
			elif msg["type"] == "committee members views" and self.verify_PoW(msg["data"]["identity"]) and self.is_directory == False and msg["data"]["identity"]["port"] not in self.views:
				# logging.warning("committee member views taken by committee id - %s" , str(self.committee_id))
				self.views.add(msg["data"]["identity"]["port"])
				logging.warning("receiving views")
				commMembers = msg["data"]["committee members"]
				finalMembers  = msg["data"]["final Committee members"]
				# update the txn block
				self.txn_block |= set(msg["data"]["txns"])
				# union of committee members wrt directory member
				self.committee_Members = self.unionViews(self.committee_Members, commMembers)
				# union of final committee members wrt directory member
				self.finalCommitteeMembers = self.unionViews(self.finalCommitteeMembers , finalMembers)
				# received the members
				# ToDo : Check and ensure that states are not overwritten
				if self.state == ELASTICO_STATES["Formed Committee"] and len(self.views) >= c //2 + 1:
					self.state = ELASTICO_STATES["Receiving Committee Members"]
				else:
					logging.error("Wrong state : %s", str(self.state))


			elif msg["type"] == "Committee full" and self.verify_PoW(msg["data"]):
				if self.state == ELASTICO_STATES["Receiving Committee Members"]:
					# all committee members have received their member views
					logging.warning("change to committee full")
					self.state = ELASTICO_STATES["Committee full"]
				else:
					logging.warning("change to committee full failure")

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
							logging.warning("received r by %s--%s" , str(self.port) , str(self.committee_id))
							self.state = ELASTICO_STATES["ReceivedR"]

			elif msg["type"] == "finalTxnBlock":
				data = msg["data"]
				identityobj = data["identity"]

				if self.verify_PoW(identityobj):
					sign = data["signature"]
					received_commitmentSet = data["commitmentSet"]
					PK = data["PK"]
					finalTxnBlock = data["finalTxnBlock"]
					finalTxnBlock_signature = data["finalTxnBlock_signature"]
					# verify the signatures
					# if self.verify_sign(sign, received_commitmentSet, PK) and self.verify_sign(finalTxnBlock_signature, finalTxnBlock, PK):

					if str(finalTxnBlock) not in self.finalBlockbyFinalCommittee:
						self.finalBlockbyFinalCommittee[str(finalTxnBlock)] = set()

					self.finalBlockbyFinalCommittee[str(finalTxnBlock)].add(finalTxnBlock_signature)

					if len(self.finalBlockbyFinalCommittee[str(finalTxnBlock)]) >= c//2 + 1:
						logging.warning("condition fulfilled")
						# for final members, their state is updated only when they have also sent the finalblock
						if self.isFinalMember():
							if self.finalBlock["sent"] and self.state != ELASTICO_STATES["FinalBlockSentToClient"]:
								logging.warning("changing state of final member to FinalBlockReceived")
								self.state = ELASTICO_STATES["FinalBlockReceived"]
							pass
						else:
							self.state = ELASTICO_STATES["FinalBlockReceived"]

					if self.newRcommitmentSet == "":
						self.newRcommitmentSet = set()
					# union of commitments 
					self.newRcommitmentSet |= received_commitmentSet
					logging.warning("new r commit set %s", str(self.newRcommitmentSet))

					# else:
					# 	logging.error("Signature invalid in final block received")
				else:
					logging.error("PoW not valid when final member send the block")

			# final committee member receives the final set of txns along with the signature from the node
			elif msg["type"] == "intraCommitteeBlock" and self.isFinalMember():
				data = msg["data"]
				identityobj = data["identity"]

				logging.warning("%s received the intra committee block from commitee id - %s- %s", str(self.port) , str(identityobj["committee_id"]) , str(identityobj["port"]))	
				if self.verify_PoW(identityobj):
					# verify the signatures
					# if self.verify_sign( data["sign"], data["txnBlock"] , data["PK"]):
					if identityobj["committee_id"] not in self.CommitteeConsensusData:
						self.CommitteeConsensusData[identityobj["committee_id"]] = dict()

					if str(data["txnBlock"]) not in self.CommitteeConsensusData[identityobj["committee_id"]]:
						self.CommitteeConsensusData[identityobj["committee_id"]][ str(data["txnBlock"]) ] = set()

					# add signatures for the txn block 
					self.CommitteeConsensusData[identityobj["committee_id"]][ str(data["txnBlock"]) ].add( data["sign"] )
					# to verify the number of txn blocks received from each committee
					# if identityobj["committee_id"] not in self.ConsensusMsgCount:
					# 	self.ConsensusMsgCount[identityobj.committee_id ] = 1
					# else:
					# 	self.ConsensusMsgCount[identityobj.committee_id] += 1
					logging.warning("intra committee block received by state - %s -%s- %s- receiver port%s" , str(self.state) ,str( identityobj["committee_id"]) , str(identityobj["port"]) , str(self.port))	
					# else:
					# 	logging.error("signature invalid for intra committee block")		
				else:
					logging.error("pow invalid for intra committee block")
			# ToDo: add verify of pows if reqd in below ifs
			
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
				logging.warning("notifying final member %s" , str(self.port))
				if self.verify_PoW(msg["data"]["identity"]):
					self.is_final = True

			elif msg["type"] == "Broadcast Ri":
				if self.isFinalMember():
					self.BroadcastR()

			# ToDo: Add verification of pow here.
			elif msg["type"] == "reset-all":
				# reset the elastico node
				self.reset()

		except Exception as e:
			# log the raised exception
			logging.error('Error at receive step ', exc_info=e)
			if isinstance(e, ConnectionRefusedError):
				logging.info("ConnectionRefusedError at port : %s", "!")
			raise e


	def verifyAndMergeConsensusData(self):
		"""
			each final committee member validates that the values received from the committees are signed by 
			atleast c/2 + 1 members of the proper committee and takes the ordered set union of all the inputs
		"""
		logging.warning("verify and merge %s -- %s" , str(self.port) ,str(self.committee_id))
		for committeeid in range(pow(2,s)):
			if committeeid in self.CommitteeConsensusData:
				for txnBlock in self.CommitteeConsensusData[committeeid]:
					if len(self.CommitteeConsensusData[committeeid][txnBlock]) >= c//2 + 1:
						if len(txnBlock) > 0:
							set_of_txns = eval(txnBlock)
							self.mergedBlock.extend(set_of_txns)
		if len(self.mergedBlock) > 0:
			self.state = ELASTICO_STATES["Merged Consensus Data"]
			logging.warning("%s - port , %s - mergedBlock" ,str(self.port) ,  str(self.mergedBlock))


	def runPBFT(self , txnBlock, instance):
		"""
			Runs a Pbft instance for the intra-committee consensus
		"""
		txn_set = set()
		for txn in txnBlock:
			txn_set.add(txn)
		# for final committee consensus 
		if instance == "final committee consensus":
			self.finalBlock["finalBlock"] = txn_set
			self.state = ELASTICO_STATES["PBFT Finished-FinalCommittee"]
		# for intra committee consensus 
		elif instance == "intra committee consensus":
			self.txn_block = txn_set
			logging.warning("%s changing state to pbft finished" , str(self.port))
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
		# ToDo: check this S, discuss with sir
		boolVal , S = consistencyProtocol()
		if boolVal == False:
			return S
		PK = self.key.publickey().exportKey().decode()	
		data = {"commitmentSet" : S, "signature" : self.sign(S) , "identity" : self.identity.__dict__ , "finalTxnBlock" : self.finalBlock["finalBlock"] , "finalTxnBlock_signature" : self.sign(self.finalBlock["finalBlock"]) , "PK" : PK}
		logging.warning("finalblock- %s" , str(self.finalBlock["finalBlock"]))
		# final Block sent to ntw
		self.finalBlock["sent"] = True
		# A final node which is already in received state should not change its state
		if self.state != ELASTICO_STATES["FinalBlockReceived"]:
			logging.warning("change state to FinalBlockSent by %s" , str(self.port))
			self.state = ELASTICO_STATES["FinalBlockSent"]
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
		PK = self.key.publickey().exportKey().decode()

		logging.warning("size of committee members %s" , str(len(self.finalCommitteeMembers)))
		logging.warning("send to final %s - %s--txns %s", str(self.committee_id) , str(self.port) , str(self.txn_block))
		for finalId in self.finalCommitteeMembers:
			# here txn_block is a set
			data = {"txnBlock" : self.txn_block , "sign" : self.sign(self.txn_block), "identity" : self.identity.__dict__, "PK" : PK}
			msg = {"data" : data, "type" : "intraCommitteeBlock" }
			finalId.send(msg)
		self.state = ELASTICO_STATES["Intra Consensus Result Sent to Final"]


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
				logging.warning("sent the commitment by %s" , str(self.port))
				data = {"identity" : self.identity.__dict__ , "Hash_Ri"  : Hash_Ri}
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
			broadcast Ri to all the network, final member will do this
		"""
		if self.isFinalMember():
			data = {"Ri" : self.Ri, "identity" : self.identity.__dict__}
			msg = {"data" : data , "type" : "RandomStringBroadcast"}
			self.state = ELASTICO_STATES["BroadcastedR"]
			BroadcastTo_Network(data, "RandomStringBroadcast")
		else:
			logging.error("non final member broadcasting R")    


	def xor_R(self):
		"""
			find xor of any random c/2 + 1 r-bit strings to set the epoch randomness
		"""
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
		PoW = identityobj["PoW"]

		# length of hash in hex
		if len(PoW["hash"]) != 64:
			return False

		# Valid Hash has D leading '0's (in hex)
		if not PoW["hash"].startswith('0' * D):
			return False

		# check Digest for set of Ri strings
		for Ri in PoW["set_of_Rs"]:
			digest = self.hexdigest(Ri)
			if digest not in self.RcommitmentSet:
				print("pow failed due to RcommitmentSet")
				return False

		# reconstruct epoch randomness
		epoch_randomness = identityobj["epoch_randomness"]
		if len(PoW["set_of_Rs"]) > 0:
			xor_val = 0
			for R in PoW["set_of_Rs"]:
				xor_val = xor_val ^ int(R, 2)
			epoch_randomness = ("{:0" + str(r) +  "b}").format(xor_val)

		# recompute PoW 
		PK = identityobj["PK"]
		IP = identityobj["IP"]
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

	def compute_fakePoW(self):
		"""
			bad node generates the fake PoW
		"""
		logging.info("computing fake POW")
		# random fakeness
		index = random_gen(32)%2
		if index == 0:
			digest = SHA256.new()
			ranHash = digest.hexdigest()
			self.PoW["hash"] = D*'0' + ranHash[D:]
		# todo : fix this
		elif index == 1:
			randomset_R = set()
			if len(self.set_of_Rs) > 0:
				self.epoch_randomness, randomset_R = self.xor_R()    
			while True:
				digest = SHA256.new()
				digest.update(str(self.PoW["nonce"]).encode())
				hash_val = digest.hexdigest()
				if hash_val.startswith('0' * D):
					nonce = self.PoW["nonce"]
					self.PoW = {"hash" : hash_val, "set_of_Rs" : randomset_R, "nonce" : nonce}
					break
				self.PoW["nonce"] += 1

		self.state = ELASTICO_STATES["PoW Computed"]


	def execute(self, epochTxn):
		"""
			executing the functions based on the running state
		"""
		try:
			# print the current state of node for debug purpose
			print(self.identity ,  list(ELASTICO_STATES.keys())[ list(ELASTICO_STATES.values()).index(self.state)], "STATE of a committee member")

			# initial state of elastico node
			if self.state == ELASTICO_STATES["NONE"]:
				if self.flag == True:
					# compute Pow for good node
					self.compute_PoW()
				else:
					logging.warning("wrong pow computing")
					# compute Pow for bad node
					self.compute_fakePoW()

			elif self.state == ELASTICO_STATES["PoW Computed"]:
				# form identity, when PoW computed
				self.form_identity()

			elif self.state == ELASTICO_STATES["Formed Identity"]:
				# form committee, when formed identity
				self.form_committee()

			elif self.is_directory and self.state == ELASTICO_STATES["RunAsDirectory"]:
				logging.warning("%s is the directory member" , str(self.port))
				# directory node will receive transactions
				# Receive txns from client for an epoch
				k = 0
				num = len(epochTxn) // pow(2,s) 
				# loop in sorted order of committee ids
				for iden in range(pow(2,s)):
					if iden == pow(2,s)-1:
						# give all the remaining txns to the last committee
						self.txn[iden] = epochTxn[ k : ]
					else:
						self.txn[iden] = epochTxn[ k : k + num]
					k = k + num
				# directory member has received the txns for all committees 
				self.state  = ELASTICO_STATES["RunAsDirectory after-TxnReceived"]

			elif self.state == ELASTICO_STATES["Receiving Committee Members"]:
				logging.warning("changing to committee full %s" , str(self.port))
				self.state = ELASTICO_STATES["Committee full"]
			
			# when a node is part of some committee
			elif self.state == ELASTICO_STATES["Committee full"]:
				logging.warning("welcome to committee full - %s -- %s", str(self.port) , str(self.committee_id))
				if self.flag == False:
					# logging the bad nodes
					logging.error("member with invalid POW %s with commMembers : %s", self.identity , self.committee_Members)
				
				# Now The node should go for Intra committee consensus
				if self.is_directory == False:
					self.runPBFT(self.txn_block, "intra committee consensus")
				else:
					# directory member should not change its state to committee full
					logging.warning("directory member state changed to Committee full(unwanted state)")


			elif self.state == ELASTICO_STATES["Formed Committee"]:
				# nodes who are not the part of any committee
				pass

			elif self.state == ELASTICO_STATES["PBFT Finished"]:
				# send pbft consensus blocks to final committee members
				logging.warning("pbft finished by memebrs %s" , str(self.port))
				self.SendtoFinal()
			
			elif self.isFinalMember() and self.state == ELASTICO_STATES["Intra Consensus Result Sent to Final"]:
				# final committee node will collect blocks and merge them
				logging.warning("final member sent the block to final")
				flag = False
				for commId in range(pow(2,s)):
					if commId not in self.CommitteeConsensusData:
						flag = True
						logging.warning("bad committee id lol %s" , str(commId))
						break
					else:
						for txnBlock in self.CommitteeConsensusData[commId]:
							if len(self.CommitteeConsensusData[commId][txnBlock]) <= c//2:
								flag = True
								logging.warning("bad committee id for intra committee block %s" , str(commId))
								break
				if flag == False:
					# when sufficient number of blocks from each committee are received
					self.verifyAndMergeConsensusData()

			elif self.isFinalMember() and self.state == ELASTICO_STATES["Merged Consensus Data"]:
				# final committee member runs final pbft
				logging.warning("merged consensus data")
				self.runPBFT(self.mergedBlock, "final committee consensus")

			elif self.isFinalMember() and self.state == ELASTICO_STATES["PBFT Finished-FinalCommittee"]:
				# send the commitment to other final committee members
				logging.warning("pbft finished by final committee %s" , str(self.port))
				self.sendCommitment()

			elif self.isFinalMember() and self.state == ELASTICO_STATES["CommitmentSentToFinal"]:
				# broadcast final txn block to ntw
				if len(self.commitments) >= c//2 + 1:
					logging.warning("got sufficient commitments")
					self.BroadcastFinalTxn()

			elif self.state == ELASTICO_STATES["FinalBlockReceived"] and self.isFinalMember():
				# collect final blocks sent by final committee and send to client.
				# Todo : check this send to client
				for txnBlock in self.finalBlockbyFinalCommittee:
					if len(self.finalBlockbyFinalCommittee[txnBlock]) >= c//2 + 1:
						self.response.append(txnBlock)
						logging.warning("adding response by %s-- %s" , str(self.port) , str(self.response))
					else:
						logging.error("less block signs : %s", str(len(self.finalBlockbyFinalCommittee[txnBlock])))

				if len(self.response) > 0:
					logging.warning("final block sent the block to client by %s", str(self.port))
					self.state = ELASTICO_STATES["FinalBlockSentToClient"]

			elif self.isFinalMember() and self.state == ELASTICO_STATES["FinalBlockSentToClient"]:
				# broadcast Ri is done when received commitment has atleast c/2  + 1 signatures
				# ToDo: check this constraint 
				if len(self.newRcommitmentSet) >= c//2 + 1:
					logging.warning("R Broadcasted by Final member")
					self.BroadcastR()
				else:
					logging.warning("insufficient RCommitments")

			elif self.state == ELASTICO_STATES["FinalBlockReceived"]:
				if self.isFinalMember():
					logging.warning("wrong state of final committee member")

			elif self.state == ELASTICO_STATES["ReceivedR"]:
				# Now, the node can be reset
				logging.warning("call for reset BY %s - %s" , str(self.port), str(self.committee_id))
				return "reset"

		except Exception as e:
			# log the raised exception
			logging.error('Error at execute step ', exc_info=e)
			if isinstance(e, ConnectionRefusedError):
				logging.info("ConnectionRefusedError at port : %s", str(self.port))
			raise e

	
	def serve(self, nodeId, channel, connection):

		try:
			method_frame, header_frame, body = channel.basic_get(queue = 'hello' + str(self.port))        
			if method_frame.NAME == 'Basic.GetEmpty':
				# connection.close()
				return ""
			else:            
				channel.basic_ack(delivery_tag=method_frame.delivery_tag)
				# connection.close() 
				return body
		except Exception as e:
			logging.error('Error in consumer ', exc_info=e)
			raise e		

def executeSteps(nodeIndex, epochTxns , sharedObj):
	"""
		A process will execute the elastico node
	"""
	global network_nodes

	try:
		for epoch in epochTxns:
			node = network_nodes[nodeIndex]
			
			if nodeIndex in sharedObj:
				sharedObj.pop(nodeIndex)
			epochTxn = epochTxns[epoch]
			while True:
				# execute one step of elastico node
				if nodeIndex not in sharedObj:
					response = node.execute(epochTxn)
					if response == "reset":
						# now reset the node
						logging.warning("call for reset for  %s" , str(node.port))

						if isinstance(node.identity, Identity):
							# identity obj exists for this node
							msg = {"type": "reset-all", "data" : node.identity.__dict__}
							node.identity.send(msg)
						else:
							# this node has not computed its identity
							print("illegal call")
							# calling reset explicitly for node
							node.reset()
						sharedObj[nodeIndex] = "reset"
				if len(sharedObj) == n:
					break
				else:
					pass
				
				# connect to rabbitmq server
				connection = pika.BlockingConnection(pika.ConnectionParameters(host='localhost'))
				# create a channel
				channel = connection.channel()
				# specify the queue 
				queue = channel.queue_declare( queue='hello' + str(node.port))
				count = queue.method.message_count
				while count:
					method_frame, header_frame, body = channel.basic_get('hello' + str(node.port))
					if method_frame:
						channel.basic_ack(method_frame.delivery_tag)
						data = pickle.loads(body)
						node.receive(data)
					else:
						logging.error('No message returned %s' , str(count))
						logging.warning("%s - method_frame , %s - header frame , %s - body" , str(method_frame)  , str(header_frame) , str(body))
					count -= 1
			# ToDo: Ensure that all nodes are reset and sharedobj is not affect
			time.sleep(60)

	except Exception as e:
		# log any error raised in the above try block
		logging.error('Error in  execute steps ', exc_info=e)
		raise e
	



def Run(epochTxns):
	"""
		runs for one epoch
	"""
	global network_nodes, ledger, commitmentSet, epochBlock
	
	try:
		if len(network_nodes) == 0:
			# network_nodes is the list of elastico objects
			for i in range(n):
				print( "---Running for processor number---" , i + 1)
				network_nodes.append(Elastico())

		# making some(5 here) nodes as malicious
		malicious_count = 0
		for i in range(malicious_count):
			badNodeIndex = random_gen(32)%n
			# set the flag false for bad nodes
			network_nodes[badNodeIndex].flag = False

		epochBlock = set()
		commitmentSet = set()
		# Manager for managing the shared variable among the processes
		manager = Manager()
		sharedObj = manager.dict()
		
		# list of processes
		processes = []
		for nodeIndex in range(n):
			# create a process
			process = Process(target= executeSteps, args=(nodeIndex, epochTxns, sharedObj))
			# add to the list of processes
			processes.append(process)

		for nodeIndex in range(n):
			print("process number" , nodeIndex , "started")
			# start the process
			processes[nodeIndex].start()

		for nodeIndex in range(n):
			# waits for the process to finish
			processes[nodeIndex].join()

		logging.warning("processes finished")

		# All processes are over. Computing response in each node to update ledger
		# for nodeIndex in range(n):
		# 	response = network_nodes[nodeIndex].response
		# 	if len(response) > 0:
		# 		logging.warning("taking response by member")
		# 		for txnBlock in response:
		# 			# ToDo: remove eval
		# 			epochBlock |= eval(txnBlock)
		# 		# reset the response 
		# 		network_nodes[nodeIndex].response = []

		# Append the block in ledger
		ledger.append(epochBlock)
		print("ledger block" , ledger)
		# input("ledger updated!!")
	except Exception as e:
		logging.error("error in run step" , exc_info=e)
		raise e


if __name__ == "__main__":
	try:
		
		# logging module configured, will log in elastico.log file for each execution
		logging.basicConfig(filename='elastico.log',filemode='w',level=logging.WARNING)

		# epochTxns - dictionary that maps the epoch number to the list of transactions
		epochTxns = dict()
		numOfEpochs = 1
		for i in range(numOfEpochs):
			# txns is the list of the transactions in one epoch to which the committees will agree on
			txns = []
			# number of transactions in each epoch
			numOfTxns = 200
			for j in range(numOfTxns):
				random_num = random_gen()
				txns.append(random_num)
			epochTxns[i] = txns

		# run all the epochs 
		# for epoch in epochTxns:
		# 	logging.info("epoch number :- %s started" , str(epoch + 1) )
		# 	Run(epochTxns[epoch])
		Run(epochTxns)

	except Exception as e:
		# log the exception raised
		logging.error('Error in  main ', exc_info=e)
		raise e
	
