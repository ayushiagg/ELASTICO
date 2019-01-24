import pika

# establish a connection with RabbitMQ server
connection = pika.BlockingConnection(pika.ConnectionParameters(host='localhost'))
channel = connection.channel()

port = 12345

# create a hello queue to which the message will be delivered
channel.queue_declare( queue= 'hello' + str(port) )

channel.basic_publish(exchange='', routing_key='hello', body='Hello World!')
print(" [x] Sent 'Hello World!'")

# close the connection
connection.close()