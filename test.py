from multiprocessing import Process
import pika
import threading
global nodes, pickle
nodes = []

class Node:
	def __init__(self,x,y,z):
		self.x = x
		self.y = y
		self.z = z

	def func(self, data):
		self.x = "lol"
		self.y = data

	def callback(self, ch, method, properties, body):
        # print(" [x] Received %r" % body)
        try:
            logging.info("queue message consumed")
            data = pickle.loads(body)
            self.func(data["msg"])
        except Exception as e:
            logging.error("callback error", exc_info=e)
            raise e	

	def serve(nodeId, i):

		global nodes
		print("thread here")
		try:
	        connection = pika.BlockingConnection(pika.ConnectionParameters(host='localhost'))
	        channel = connection.channel()


	        channel.queue_declare(queue='hello' + str(nodeId))


	        channel.basic_consume(self.callback, queue='hello' + str(nodeId), no_ack=True)

	        print(' [*] Waiting for messages. To exit press CTRL+C')
	        channel.start_consuming()

	    except Exception as e:
            logging.error('Error in  execute server ', exc_info=e)
            raise e


    def produce(self, nodeId):
    	"""
    	"""
    	try:
	    	connection = pika.BlockingConnection(pika.ConnectionParameters(host='localhost'))
	        channel = connection.channel()

	        # create a hello queue to which the message will be delivered
	        channel.queue_declare( queue= 'hello' + str(1 - nodeId) )

	        msg_data = {"sendto" : self, "msg" : str(nodeId)*5}

	        serialized_data = pickle.dumps(msg_data)
	        
	        channel.basic_publish(exchange='', routing_key='hello' + str(1 - nodeId), body= serialized_data)
	        print(" [x] Sent 'Hello World!'")

	        # close the connection
	        connection.close()

        except Exception as e:
            logging.error('Error in  producing', exc_info=e)
            raise e

def execute(nodeId):
	"""

	"""
	print("process here")
	
	global nodes
	node = nodes[nodeId]
	node.produce(nodeId)

	thread = threading.Thread(target= node.serve,args=(nodeId,1))
	thread.start()
	
	print(node.x, node.y, node.z)

	print("p ends")


if __name__ == '__main__':

	n = 2

	for nodeId in range(n):
		nodes.append(Node(1  , 2  , 3 ))

	p = []
	for nodeId in range(n):
		p.append(Process(target=execute,args=(nodeId,)))
		
	for nodeId in range(n):
		process = p[nodeId]
		process.start()
	
	for nodeId in range(n):
		process = p[nodeId]
		process.join()

	for nodeId in range(n):
		node = nodes[nodeId]
		print(node.x, node.y, node.z)
