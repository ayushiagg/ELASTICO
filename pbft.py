from math import floor

PBFT_TOTAL_NODES = 10
PBFT_F = floor((PBFT_TOTAL_NODES - 1) / 3)
PBFT_STATES = {"NONE": 0, "PRE-PREPARE": 1, "PREPARE": 2, "COMMIT": 3}


class PBFT:
    """
        Pbft class for implementing PBFT protocol, for consensus
        within a committee.
    """

    def __init__(self):
        """
        """

    pass
