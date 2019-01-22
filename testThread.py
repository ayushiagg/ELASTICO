import threading

global network_nodes,n

network_nodes = []
n = 100

class Elastico:
	"""
	"""
	def __init__(self):
		self.x = 5
		self.stri = "thread"
		self.li = []
	
	def test(self):
		"""
		"""
		self.li.append('{1,2}')
		self.li.append('{1,3}')
		return self.li

def run(node):
	for i in range(2):
		response = node.test()
		print(response)


if __name__ == '__main__':
	# global network_nodes
	
	if len(network_nodes) == 0:
		for i in range(n):
			network_nodes.append(Elastico())

	t = []
	for i in range(n):
		t.append(threading.Thread(target= run, args=(network_nodes[i], )))
	for nodeIndex in range(n):
		print("thread number" , nodeIndex , "started")
		# input("dfsgtyetr")
		t[nodeIndex].start()
	for nodeIndex in range(n):
		t[nodeIndex].join()
				