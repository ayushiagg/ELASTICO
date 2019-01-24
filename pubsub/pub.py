import sys
import time

import zmq

def sync(bind_to):
    # use bind socket + 1
    sync_with = ':'.join(bind_to.split(':')[:-1] +
                         [str(int(bind_to.split(':')[-1]) + 1)])
    ctx = zmq.Context.instance()
    s = ctx.socket(zmq.REP)
    s.bind(sync_with)
    print("Waiting for subscriber to connect...")
    s.recv()
    print("   Done.")
    s.send_string('GO')

def main():
    # if len (sys.argv) != 4:
    #     print('usage: publisher <bind-to> <array-size> <array-count>')
    #     sys.exit (1)
    try:
        bind_to = "tcp://127.0.0.1:49153"
    except Exception as e:
        raise e

    ctx = zmq.Context()
    s = ctx.socket(zmq.PUB)
    s.bind(bind_to)

    # sync(bind_to)

    print("Sending arrays...")
    msg = {"data" : "ayushi" , "type" : "type"}
    s.send_pyobj(msg)
    s.send_pyobj(msg)
    s.send_pyobj(msg)
    s.send_pyobj(msg)
    print("Done.")

if __name__ == "__main__":
    main()
