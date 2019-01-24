import sys
import time

import zmq

def sync(connect_to):
    # use connect socket + 1
    sync_with = ':'.join(connect_to.split(':')[:-1] +
                         [str(int(connect_to.split(':')[-1]) + 1)]
                        )
    ctx = zmq.Context.instance()
    s = ctx.socket(zmq.REQ)
    s.connect(sync_with)
    s.send_string('READY')
    s.recv()

def main():
    try:
        connect_to = "tcp://127.0.0.1:49153"
    except Exception as e:
        raise e

    ctx = zmq.Context()
    s = ctx.socket(zmq.SUB)
    s.connect(connect_to)
    s.setsockopt_string(zmq.SUBSCRIBE,'')

    # sync(connect_to)


    print("Receiving arrays...")
    msg = s.recv_pyobj()
    msg = s.recv_pyobj()
    msg = s.recv_pyobj()
    msg = s.recv_pyobj()
    print("msg-" , msg)
    print("Done.")

if __name__ == "__main__":
    main()