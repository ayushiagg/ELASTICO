import socket

UDP_IP = "127.0.0.1"
UDP_PORT = 5005

sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM) 
sock.bind((UDP_IP, UDP_PORT))

while True:
  msg = b''
  data, client = sock.recvfrom(1024) # buffer size is 1024 bytes
  while data:
    msg+=data
    data = client.recv(1024)
  print("received message- ",msg.decode())