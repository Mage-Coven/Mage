package magedist

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/mage-coven/mage/x/magedist/keeper"
)

func BeginBlocker(ctx sdk.Context, k keeper.Keeper) {
	err := k.MintPeriodInflation(ctx)
	if err != nil {
		panic(err)
	}
}
