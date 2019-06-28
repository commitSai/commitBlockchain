package slashing

import (
	"github.com/comdex-blockchain/wire"
)

// Register concrete types on wire codec
func RegisterWire(cdc *wire.Codec) {
	cdc.RegisterConcrete(MsgUnjail{}, "comdex-blockchain/MsgUnjail", nil)
}

var cdcEmpty = wire.NewCodec()