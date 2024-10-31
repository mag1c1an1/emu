package committee

import "emu/message"

type CommitteeModule interface {
	HandleBlockInfo(*message.BlockInfoMsg)
	MsgSendingControl()
	HandleOtherMessage([]byte)
}
