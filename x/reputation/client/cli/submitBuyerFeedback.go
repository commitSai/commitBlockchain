package cli

import (
	"os"
	
	"github.com/comdex-blockchain/client/context"
	"github.com/comdex-blockchain/client/utils"
	sdk "github.com/comdex-blockchain/types"
	"github.com/comdex-blockchain/wire"
	authcmd "github.com/comdex-blockchain/x/auth/client/cli"
	context2 "github.com/comdex-blockchain/x/auth/client/context"
	"github.com/comdex-blockchain/x/reputation"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// SubmitBuyerFeedbackCmd : gives rating to address
func SubmitBuyerFeedbackCmd(cdc *wire.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buyerFeedback",
		Short: "buyer gives feedback to the seller",
		RunE: func(cmd *cobra.Command, args []string) error {
			txCtx := context2.NewTxContextFromCLI().WithCodec(cdc)
			cliCtx := context.NewCLIContext().WithCodec(cdc).WithLogger(os.Stdout).WithAccountDecoder(authcmd.GetAccountDecoder(cdc))
			
			if err := cliCtx.EnsureAccountExists(); err != nil {
				return err
			}
			
			from, err := cliCtx.GetFromAddress()
			if err != nil {
				return err
			}
			
			to := viper.GetString(FlagTo)
			toAddress, err := sdk.AccAddressFromBech32(to)
			if err != nil {
				return err
			}
			
			pegHashStr := viper.GetString(FlagPegHash)
			pegHashHex, err := sdk.GetAssetPegHashHex(pegHashStr)
			if err != nil {
				return err
			}
			
			rating := viper.GetInt64(FlagRating)
			
			msg := reputation.BuildBuyerFeedbackMsg(from, toAddress, pegHashHex, rating)
			return utils.SendTx(txCtx, cliCtx, []sdk.Msg{msg})
		},
	}
	
	cmd.Flags().AddFlagSet(fsPeghash)
	cmd.Flags().AddFlagSet(fsRating)
	cmd.Flags().AddFlagSet(fsTo)
	return cmd
}