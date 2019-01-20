# Import socket module 
import socket
import json

def sendMsg(socketconn, data):
	"""
	"""
	serialized_data = json.dumps(data)
	encoded_data = serialized_data.encode()
	socketconn.send(encoded_data)


# Create a socket object 
s = socket.socket()

# Define the port on which you want to connect 
port = 12346

# connect to the server on local computer 
try:
	s.connect(('127.0.0.1', port))
	# receive data from the server
	print( s.recv(1024) )
	msg = {"type" : "msgtype", "data" : "msgdata"*1000}
	s.send(json.dumps(msg).encode())
	# close the connection
	s.close()
except Exception as e:
	print("Connection not possible")
