# from multiprocessing import Process
# import pika
# import threading, pickle , logging
# global nodes
# nodes = []

# class Node:
# 	def __init__(self,x,y,z):
# 		self.x = x
# 		self.y = y
# 		self.z = z

# 	def func(self, data):
# 		self.x = "lol"
# 		self.y = data

# 	def callback(self, ch, method, properties, body):
# 		# print(" [x] Received %r" % body)
# 		try:
# 			print("callback here")
# 			logging.info("queue message consumed")
# 			data = pickle.loads(body)
# 			self.func(data["msg"])
# 		except Exception as e:
# 			logging.error("callback error", exc_info=e)
# 			raise e 

# 	def serve(self, nodeId, channel, connection):

# 		global nodes
# 		# print("thread here")
# 		try:
# 			# connection = pika.BlockingConnection(pika.ConnectionParameters(host='localhost'))

# 			# channel = connection.channel()
# 			# # channel.basic_qos(prefetch_size=0 )
			
# 			# queue = channel.queue_declare( queue='hello' + str(nodeId))
# 			print(nodeId, "serving")
# 			method_frame, header_frame, body = channel.basic_get(queue = 'hello' + str(nodeId))        
# 			if method_frame.NAME == 'Basic.GetEmpty':
# 				connection.close()
# 				return ''
# 			else:            
# 				channel.basic_ack(delivery_tag=method_frame.delivery_tag)
# 				# connection.close() 
# 				return body

# 			# print("msg count : ", queue.method.message_count)
# 			# # response  = channel.queue_declare(queue='hello' + str(nodeId), passive=True)
# 			# # # response = channel.queue_declare(‘queue-name’, passive=True)
# 			# # print('The queue has {0} messages'.format(response.message_count))

# 			# channel.basic_consume(self.callback, queue='hello' + str(nodeId), no_ack=False)

# 			# print(' [*] Waiting for messages. To exit press CTRL+C')

# 			# channel.start_consuming()

# 		except Exception as e:
# 			logging.error('Error in  execute server ', exc_info=e)
# 			raise e


# 	def produce(self, nodeId):
# 		"""
# 		"""
# 		try:
# 			connection = pika.BlockingConnection(pika.ConnectionParameters(host='localhost'))
# 			channel = connection.channel()

# 			# create a hello queue to which the message will be delivered
# 			channel.queue_declare( queue= 'hello' + str(1 - nodeId) )

# 			msg_data = {"msg" : str(nodeId)*5}

# 			serialized_data = pickle.dumps(msg_data)
			
# 			channel.basic_publish(exchange='', routing_key='hello' + str(1 - nodeId), body= serialized_data)
# 			print(" [x] Sent 'Hello World!'")

# 			# close the connection
# 			connection.close()

# 		except Exception as e:
# 			logging.error('Error in  producing', exc_info=e)
# 			raise e

# def execute(nodeId):
# 	"""

# 	"""
# 	print("process here")
	
# 	global nodes
# 	node = nodes[nodeId]
	
# 	node.produce(nodeId)

# 	connection = pika.BlockingConnection(pika.ConnectionParameters(host='localhost'))

# 	channel = connection.channel()
# 			# channel.basic_qos(prefetch_size=0 )
			
# 	queue = channel.queue_declare( queue='hello' + str(nodeId))
	
# 	print("msg count : ", queue.method.message_count)
# 	count = queue.method.message_count
	
# 	while count > 0:
# 		print("count" , count)
# 		data = node.serve(nodeId, channel , connection)
# 		data = pickle.loads(data)
# 		print("msg-" , data)
# 		count -= 1
# 		# print("count" , count)
	
# 	print(node.x, node.y, node.z)
# 	print("p ends")


# if __name__ == '__main__':
# 	logging.basicConfig(filename='test.log',filemode='w',level=logging.INFO)
# 	n = 2

# 	for nodeId in range(n):
# 		nodes.append(Node(1  , 2  , 3 ))

# 	p = []
# 	for nodeId in range(n):
# 		p.append(Process(target=execute,args=(nodeId,)))
		
# 	for nodeId in range(n):
# 		process = p[nodeId]
# 		process.start()
	
# 	for nodeId in range(n):
# 		process = p[nodeId]
# 		process.join()

# 	for nodeId in range(n):
# 		node = nodes[nodeId]
# # 		print(node.x, node.y, node.z)

# class Elastic:
# 	def __init__(self, IP, PK, committee_id, PoW, epoch_randomness, port):
# 		self.IP = IP
# 		self.PK = PK
# 		self.committee_id = committee_id
# 		self.PoW = PoW
# 		self.epoch_randomness = epoch_randomness
# 		self.partOfNtw = False
# 		self.port = port

# identityobj = Elastic("ip" , "pk" , "id", "pow" , "epoch" , "port")
# d = identityobj.__dict__
# print(d["port"])		

from rmq_elastico import Transaction
import time
t = Transaction('a' , 'b' , 40 , time.time())
u = Transaction('a' , 'b' , 20 , time.time())
v = Transaction('a' , 'b' , 230 , time.time())
l = [t,u,v]
l = sorted(l, key=lambda txn: txn.timestamp, reverse = True)
print(l[0].amount)
print(type(u) is Transaction)