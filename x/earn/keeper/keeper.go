package keeper

import (
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/kava-labs/kava/x/earn/types"

	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

// Keeper keeper for the earn module
type Keeper struct {
	key           sdk.StoreKey
	cdc           codec.Codec
	paramSubspace paramtypes.Subspace
	hooks         types.EarnHooks
	accountKeeper types.AccountKeeper
	bankKeeper    types.BankKeeper
	liquidKeeper  types.LiquidKeeper

	// Keepers used for strategies
	hardKeeper    types.HardKeeper
	savingsKeeper types.SavingsKeeper

	// name of module account the community pool deposit/withdraw proposals use
	communityPoolMaccName string
}

// NewKeeper creates a new keeper
func NewKeeper(
	cdc codec.Codec,
	key sdk.StoreKey,
	paramstore paramtypes.Subspace,
	accountKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
	liquidKeeper types.LiquidKeeper,
	hardKeeper types.HardKeeper,
	savingsKeeper types.SavingsKeeper,
	communityPoolMaccName string,
) Keeper {
	if !paramstore.HasKeyTable() {
		paramstore = paramstore.WithKeyTable(types.ParamKeyTable())
	}

	return Keeper{
		key:           key,
		cdc:           cdc,
		paramSubspace: paramstore,
		accountKeeper: accountKeeper,
		bankKeeper:    bankKeeper,
		liquidKeeper:  liquidKeeper,
		hardKeeper:    hardKeeper,
		savingsKeeper: savingsKeeper,

		communityPoolMaccName: communityPoolMaccName,
	}
}

// SetHooks adds hooks to the keeper.
func (k *Keeper) SetHooks(sh types.EarnHooks) *Keeper {
	if k.hooks != nil {
		panic("cannot set earn hooks twice")
	}
	k.hooks = sh
	return k
}

// ClearHooks clears the hooks on the keeper
func (k *Keeper) ClearHooks() {
	k.hooks = nil
}
