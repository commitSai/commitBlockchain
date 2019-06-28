package simulation

import (
	"encoding/json"
	"math/rand"
	"testing"
	
	"github.com/tendermint/tendermint/crypto"
	
	sdk "github.com/comdex-blockchain/types"
	"github.com/comdex-blockchain/x/bank"
	"github.com/comdex-blockchain/x/mock"
	"github.com/comdex-blockchain/x/mock/simulation"
)

func TestBankWithRandomMessages(t *testing.T) {
	mapp := mock.NewApp()
	
	bank.RegisterWire(mapp.Cdc)
	mapper := mapp.AccountMapper
	coinKeeper := bank.NewKeeper(mapper)
	mapp.Router().AddRoute("bank", bank.NewHandler(coinKeeper))
	
	err := mapp.CompleteSetup([]*sdk.KVStoreKey{})
	if err != nil {
		panic(err)
	}
	
	appStateFn := func(r *rand.Rand, keys []crypto.PrivKey, accs []sdk.AccAddress) json.RawMessage {
		mock.RandomSetGenesis(r, mapp, accs, []string{"stake"})
		return json.RawMessage("{}")
	}
	
	simulation.Simulate(
		t, mapp.BaseApp, appStateFn,
		[]simulation.Operation{
			SimulateSingleInputMsgSend(mapper),
		},
		[]simulation.RandSetup{},
		[]simulation.Invariant{
			NonnegativeBalanceInvariant(mapper),
			TotalCoinsInvariant(mapper, func() sdk.Coins { return mapp.TotalCoinsSupply }),
		},
		30, 30,
		false,
	)
}