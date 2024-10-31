package pbft

import "emu/message"

// Define operations in a PBFT.
// This may be varied by different consensus protocols.

type ExtraOpInConsensus interface {
	// HandleInPropose mining / message generation
	HandleInPropose() (bool, *message.Request)
	// HandleInPrePrepare checking
	HandleInPrePrepare(*message.PrePrepare) bool
	// HandleInPrepare nothing necessary
	HandleInPrepare(*message.Prepare) bool
	// HandleInCommit confirming
	HandleInCommit(*message.Commit) bool
	//// HandleRequestForOldSeq do for need
	//HandleRequestForOldSeq(*message.RequestOldMessage) bool
	//// HandleForSequentialRequest do for need
	//HandleForSequentialRequest(*message.SendOldMessage) bool
}

// OpInterShards Define operations among some PBFTs.
// This may be varied by different consensus protocols.
type OpInterShards interface {
	// HandleMessageOutsidePBFT operation inter-shards
	HandleMessageOutsidePBFT(message.MessageType, []byte) bool
}
