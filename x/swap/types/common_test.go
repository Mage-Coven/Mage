package types_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/mage-coven/mage/app"
)

func init() {
	mageConfig := sdk.GetConfig()
	app.SetBech32AddressPrefixes(mageConfig)
	app.SetBip44CoinType(mageConfig)
	mageConfig.Seal()
}
