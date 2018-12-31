from math import floor

PBFT_TOTAL_NODES = 10
PBFT_F = floor((PBFT_TOTAL_NODES - 1) / 3)
PBFT_STATES = {"NONE": 0, "PRE-PREPARE": 1, "PREPARE": 2, "COMMIT": 3}


class PBFT:
    """
        Pbft class for implementing PBFT protocol, for consensus
        within a committee.
    """

    def __init__(self, blockChain):
        """
        """
        self.blockChain = blockChain
        self.node = blockChain.node
        self.pendingBlocks = {}
        self.prepareInfo = None
        self.commitInfo = {}
        self.state = PBFT_STATES["NONE"]
        pass

    def has_block(self, block_hash):
        if block_hash in self.pendingBlocks:
            return True
        else:
            return False

    def is_busy(self):
        if self.state == PBFT_STATES["None"]:
            return False
        else:
            return True

    def add_block(self, block):
        # add block hash to pending blocks
        blockhash = block.get_hash()
        self.pendingBlocks[blockhash] = block

        if self.state == PBFT_STATES["None"]:
            # Do PRE-PREPARE
            pass

    def verify_request(self, msg):
        """
            Verify the request messgar
        """
        pass

    def Pre_Prepare(self, msg):
        """
            Send Pre-Prepare msgs
        """
        pass

    def verify_pre_prepare(self, msg):
        """
        """
        pass

    def Prepare(self, msg):
        """
            Send Prepare msgs
        """
        pass

    def verify_prepare(self, msg):
        """
        """
        pass
