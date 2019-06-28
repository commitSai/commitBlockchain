package rest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	
	"github.com/asaskevich/govalidator"
	cliclient "github.com/comdex-blockchain/client"
	"github.com/comdex-blockchain/client/context"
	"github.com/comdex-blockchain/client/utils"
	"github.com/comdex-blockchain/crypto/keys"
	"github.com/comdex-blockchain/rest"
	sdk "github.com/comdex-blockchain/types"
	"github.com/comdex-blockchain/wire"
	"github.com/comdex-blockchain/x/acl"
	aclTypes "github.com/comdex-blockchain/x/acl"
	context2 "github.com/comdex-blockchain/x/auth/client/context"
	"github.com/comdex-blockchain/x/negotiation"
)

// ConfirmBuyerBidRequestHandlerFn : handler for confirm buyer bid request
func ConfirmBuyerBidRequestHandlerFn(cdc *wire.Codec, kb keys.Keybase, cliCtx context.CLIContext, kafka bool, kafkaState rest.KafkaState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		var msg negotiation.ConfirmBuyerBidBody
		
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		
		err = json.Unmarshal(body, &msg)
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		
		_, err = govalidator.ValidateStruct(msg)
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		
		txCtx := context2.TxContext{
			Codec:         cdc,
			Gas:           msg.Gas,
			ChainID:       msg.ChainID,
			AccountNumber: msg.AccountNumber,
			Sequence:      msg.Sequence,
		}
		
		adjustment, ok := utils.ParseFloat64OrReturnBadRequest(w, msg.GasAdjustment, cliclient.DefaultGasAdjustment)
		if !ok {
			return
		}
		cliCtx = cliCtx.WithGasAdjustment(adjustment)
		cliCtx = cliCtx.WithFromAddressName(msg.From)
		cliCtx.JSON = true
		
		from, err := cliCtx.GetFromAddress()
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		
		res, err := cliCtx.QueryStore(acl.AccountStoreKey(from), "acl")
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("couldn't query account. Error: %s", err.Error()))
			return
		}
		
		// the query will return empty if there is no data for this account
		if len(res) == 0 {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Unauthorized transaction. ACL is not defined for Buyer"))
			return
		}
		// decode the value
		decoder := aclTypes.GetACLAccountDecoder(cdc)
		account, err := decoder(res)
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("couldn't parse query result. Result: %s. Error: %s", res, err.Error()))
			return
		}
		if !account.GetACL().ConfirmBuyerBid {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Unauthorized transaction"))
			return
		}
		
		to, err := sdk.AccAddressFromBech32(msg.To)
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		
		pegHashHex, err := sdk.GetAssetPegHashHex(msg.PegHash)
		negotiationID := sdk.NegotiationID(append(append(from.Bytes(), to.Bytes()...), pegHashHex.Bytes()...))
		
		SignBytes := negotiation.NewSignNegotiationBody(from, to, pegHashHex, msg.Bid, msg.Time)
		signature, _, err := kb.Sign(msg.From, msg.Password, SignBytes.GetSignBytes())
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		proposedNegotiation := &sdk.BaseNegotiation{
			NegotiationID:   negotiationID,
			BuyerAddress:    from,
			SellerAddress:   to,
			PegHash:         pegHashHex,
			Bid:             msg.Bid,
			Time:            msg.Time,
			BuyerSignature:  signature,
			SellerSignature: nil,
		}
		
		confirmBuyerBid := negotiation.BuildMsgConfirmBuyerBid(proposedNegotiation)
		
		if kafka == true {
			ticketID := rest.TicketIDGenerator("NCFB")
			
			jsonResponse := rest.SendToKafka(rest.NewKafkaMsgFromRest(confirmBuyerBid, ticketID, txCtx, cliCtx, msg.Password), kafkaState, cdc)
			w.WriteHeader(http.StatusAccepted)
			w.Write(jsonResponse)
		} else {
			output, err := utils.SendTxWithResponse(txCtx, cliCtx, []sdk.Msg{confirmBuyerBid}, msg.Password)
			if err != nil {
				utils.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			
			w.Write(utils.ResponseBytesToJSON(output))
		}
	}
}