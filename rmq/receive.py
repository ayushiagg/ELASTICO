import pika

connection = pika.BlockingConnection(pika.ConnectionParameters(host='localhost'))
channel = connection.channel()


channel.queue_declare(queue='hello')

class A:
    def __init__(self):
        self.x = 5
    
    def callback(self, ch, method, properties, body):
        print(self.x)
        print(ch)
        print(method)
        print(properties)
        print(" [x] Received %r" % body)

a = A()
channel.basic_consume(a.callback, queue='hello', no_ack=True)

print(' [*] Waiting for messages. To exit press CTRL+C')
channel.start_consuming()
