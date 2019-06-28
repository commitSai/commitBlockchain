package cli

import (
	"os"
	
	"github.com/comdex-blockchain/client"
	"github.com/comdex-blockchain/client/context"
	"github.com/comdex-blockchain/client/utils"
	sdk "github.com/comdex-blockchain/types"
	"github.com/comdex-blockchain/wire"
	authcmd "github.com/comdex-blockchain/x/auth/client/cli"
	context2 "github.com/comdex-blockchain/x/auth/client/context"
	"github.com/comdex-blockchain/x/bank/client/cli"
	"github.com/comdex-blockchain/x/ibc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// IBCBuyerExecuteOrder :
func IBCBuyerExecuteOrder(cdc *wire.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buyerExecuteOrder",
		Short: "ibc buyer execute order",
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
			
			buyerAddressString := viper.GetString(cli.FlagBuyerAddress)
			buyerAddress, err := sdk.AccAddressFromBech32(buyerAddressString)
			if err != nil {
				return err
			}
			
			sellerAddressString := viper.GetString(cli.FlagSellerAddress)
			sellerAddress, err := sdk.AccAddressFromBech32(sellerAddressString)
			if err != nil {
				return err
			}
			pegHashStr := viper.GetString(flagPegHash)
			pegHashHex, err := sdk.GetAssetPegHashHex(pegHashStr)
			
			fiatPeg := sdk.BaseFiatPeg{
				PegHash: pegHashHex,
			}
			if err != nil {
				return err
			}
			
			fiatProofHashStr := viper.GetString(cli.FlagFiatProofHash)
			destinationChainID := viper.GetString(flagChain)
			sourceChainID := viper.GetString(client.FlagChainID)
			
			msg := ibc.BuildBuyerExecuteOrder(from, buyerAddress, sellerAddress, pegHashHex, fiatProofHashStr, sdk.FiatPegWallet{fiatPeg}, sourceChainID, destinationChainID)
			
			return utils.SendTx(txCtx, cliCtx, []sdk.Msg{msg})
		},
	}
	cmd.Flags().String(flagChain, "", "destination chainID")
	cmd.Flags().String(cli.FlagFiatProofHash, "", "fiat proof hash")
	cmd.Flags().String(cli.FlagBuyerAddress, "", "buyerAddress")
	cmd.Flags().String(cli.FlagSellerAddress, "", "sellerAddress")
	cmd.Flags().String(flagPegHash, "", "pegHash")
	
	return cmd
}