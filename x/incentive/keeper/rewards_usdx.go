package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	cdptypes "github.com/mage-coven/mage/x/cdp/types"
	"github.com/mage-coven/mage/x/incentive/types"
)

// AccumulateFUSDMintingRewards calculates new rewards to distribute this block and updates the global indexes to reflect this.
// The provided rewardPeriod must be valid to avoid panics in calculating time durations.
func (k Keeper) AccumulateFUSDMintingRewards(ctx sdk.Context, rewardPeriod types.RewardPeriod) {
	previousAccrualTime, found := k.GetPreviousFUSDMintingAccrualTime(ctx, rewardPeriod.CollateralType)
	if !found {
		previousAccrualTime = ctx.BlockTime()
	}

	factor, found := k.GetFUSDMintingRewardFactor(ctx, rewardPeriod.CollateralType)
	if !found {
		factor = sdk.ZeroDec()
	}
	// wrap in RewardIndexes for compatibility with Accumulator
	indexes := types.RewardIndexes{}.With(types.FUSDMintingRewardDenom, factor)

	acc := types.NewAccumulator(previousAccrualTime, indexes)

	totalSource := k.getFUSDTotalSourceShares(ctx, rewardPeriod.CollateralType)

	acc.Accumulate(types.NewMultiRewardPeriodFromRewardPeriod(rewardPeriod), totalSource, ctx.BlockTime())

	k.SetPreviousFUSDMintingAccrualTime(ctx, rewardPeriod.CollateralType, acc.PreviousAccumulationTime)

	factor, found = acc.Indexes.Get(types.FUSDMintingRewardDenom)
	if !found {
		panic("could not find factor that should never be missing when accumulating fusd rewards")
	}
	k.SetFUSDMintingRewardFactor(ctx, rewardPeriod.CollateralType, factor)
}

// getFUSDTotalSourceShares fetches the sum of all source shares for a fusd minting reward.
// In the case of fusd minting, this is the total debt from all cdps of a particular type, divided by the cdp interest factor.
// This gives the "pre interest" value of the total debt.
func (k Keeper) getFUSDTotalSourceShares(ctx sdk.Context, collateralType string) sdk.Dec {
	totalPrincipal := k.cdpKeeper.GetTotalPrincipal(ctx, collateralType, cdptypes.DefaultStableDenom)

	cdpFactor, found := k.cdpKeeper.GetInterestFactor(ctx, collateralType)
	if !found {
		// assume nothing has been borrowed so the factor starts at it's default value
		cdpFactor = sdk.OneDec()
	}
	// return debt/factor to get the "pre interest" value of the current total debt
	return totalPrincipal.ToDec().Quo(cdpFactor)
}

// InitializeFUSDMintingClaim creates or updates a claim such that no new rewards are accrued, but any existing rewards are not lost.
// this function should be called after a cdp is created. If a user previously had a cdp, then closed it, they shouldn't
// accrue rewards during the period the cdp was closed. By setting the reward factor to the current global reward factor,
// any unclaimed rewards are preserved, but no new rewards are added.
func (k Keeper) InitializeFUSDMintingClaim(ctx sdk.Context, cdp cdptypes.CDP) {
	claim, found := k.GetFUSDMintingClaim(ctx, cdp.Owner)
	if !found { // this is the owner's first fusd minting reward claim
		claim = types.NewFUSDMintingClaim(cdp.Owner, sdk.NewCoin(types.FUSDMintingRewardDenom, sdk.ZeroInt()), types.RewardIndexes{})
	}

	globalRewardFactor, found := k.GetFUSDMintingRewardFactor(ctx, cdp.Type)
	if !found {
		globalRewardFactor = sdk.ZeroDec()
	}
	claim.RewardIndexes = claim.RewardIndexes.With(cdp.Type, globalRewardFactor)

	k.SetFUSDMintingClaim(ctx, claim)
}

// SynchronizeFUSDMintingReward updates the claim object by adding any accumulated rewards and updating the reward index value.
// this should be called before a cdp is modified.
func (k Keeper) SynchronizeFUSDMintingReward(ctx sdk.Context, cdp cdptypes.CDP) {
	claim, found := k.GetFUSDMintingClaim(ctx, cdp.Owner)
	if !found {
		return
	}

	sourceShares, err := cdp.GetNormalizedPrincipal()
	if err != nil {
		panic(fmt.Sprintf("during fusd reward sync, could not get normalized principal for %s: %s", cdp.Owner, err.Error()))
	}

	claim = k.synchronizeSingleFUSDMintingReward(ctx, claim, cdp.Type, sourceShares)

	k.SetFUSDMintingClaim(ctx, claim)
}

// synchronizeSingleFUSDMintingReward synchronizes a single rewarded cdp collateral type in a fusd minting claim.
// It returns the claim without setting in the store.
// The public methods for accessing and modifying claims are preferred over this one. Direct modification of claims is easy to get wrong.
func (k Keeper) synchronizeSingleFUSDMintingReward(ctx sdk.Context, claim types.FUSDMintingClaim, ctype string, sourceShares sdk.Dec) types.FUSDMintingClaim {
	globalRewardFactor, found := k.GetFUSDMintingRewardFactor(ctx, ctype)
	if !found {
		// The global factor is only not found if
		// - the cdp collateral type has not started accumulating rewards yet (either there is no reward specified in params, or the reward start time hasn't been hit)
		// - OR it was wrongly deleted from state (factors should never be removed while unsynced claims exist)
		// If not found we could either skip this sync, or assume the global factor is zero.
		// Skipping will avoid storing unnecessary factors in the claim for non rewarded denoms.
		// And in the event a global factor is wrongly deleted, it will avoid this function panicking when calculating rewards.
		return claim
	}

	userRewardFactor, found := claim.RewardIndexes.Get(ctype)
	if !found {
		// Normally the factor should always be found, as it is added when the cdp is created in InitializeFUSDMintingClaim.
		// However if a cdp type is not rewarded then becomes rewarded (ie a reward period is added to params), existing cdps will not have the factor in their claims.
		// So assume the factor is the starting value for any global factor: 0.
		userRewardFactor = sdk.ZeroDec()
	}

	newRewardsAmount, err := k.CalculateSingleReward(userRewardFactor, globalRewardFactor, sourceShares)
	if err != nil {
		// Global reward factors should never decrease, as it would lead to a negative update to claim.Rewards.
		// This panics if a global reward factor decreases or disappears between the old and new indexes.
		panic(fmt.Sprintf("corrupted global reward indexes found: %v", err))
	}
	newRewardsCoin := sdk.NewCoin(types.FUSDMintingRewardDenom, newRewardsAmount)

	claim.Reward = claim.Reward.Add(newRewardsCoin)
	claim.RewardIndexes = claim.RewardIndexes.With(ctype, globalRewardFactor)

	return claim
}

// SimulateFUSDMintingSynchronization calculates a user's outstanding FUSD minting rewards by simulating reward synchronization
func (k Keeper) SimulateFUSDMintingSynchronization(ctx sdk.Context, claim types.FUSDMintingClaim) types.FUSDMintingClaim {
	for _, ri := range claim.RewardIndexes {
		_, found := k.GetFUSDMintingRewardPeriod(ctx, ri.CollateralType)
		if !found {
			continue
		}

		globalRewardFactor, found := k.GetFUSDMintingRewardFactor(ctx, ri.CollateralType)
		if !found {
			globalRewardFactor = sdk.ZeroDec()
		}

		// the owner has an existing fusd minting reward claim
		index, hasRewardIndex := claim.HasRewardIndex(ri.CollateralType)
		if !hasRewardIndex { // this is the owner's first fusd minting reward for this collateral type
			claim.RewardIndexes = append(claim.RewardIndexes, types.NewRewardIndex(ri.CollateralType, globalRewardFactor))
		}
		userRewardFactor := claim.RewardIndexes[index].RewardFactor
		rewardsAccumulatedFactor := globalRewardFactor.Sub(userRewardFactor)
		if rewardsAccumulatedFactor.IsZero() {
			continue
		}

		claim.RewardIndexes[index].RewardFactor = globalRewardFactor

		cdp, found := k.cdpKeeper.GetCdpByOwnerAndCollateralType(ctx, claim.GetOwner(), ri.CollateralType)
		if !found {
			continue
		}
		newRewardsAmount := rewardsAccumulatedFactor.Mul(cdp.GetTotalPrincipal().Amount.ToDec()).RoundInt()
		if newRewardsAmount.IsZero() {
			continue
		}
		newRewardsCoin := sdk.NewCoin(types.FUSDMintingRewardDenom, newRewardsAmount)
		claim.Reward = claim.Reward.Add(newRewardsCoin)
	}

	return claim
}

// SynchronizeFUSDMintingClaim updates the claim object by adding any rewards that have accumulated.
// Returns the updated claim object
func (k Keeper) SynchronizeFUSDMintingClaim(ctx sdk.Context, claim types.FUSDMintingClaim) (types.FUSDMintingClaim, error) {
	for _, ri := range claim.RewardIndexes {
		cdp, found := k.cdpKeeper.GetCdpByOwnerAndCollateralType(ctx, claim.Owner, ri.CollateralType)
		if !found {
			// if the cdp for this collateral type has been closed, no updates are needed
			continue
		}
		claim = k.synchronizeRewardAndReturnClaim(ctx, cdp)
	}
	return claim, nil
}

// this function assumes a claim already exists, so don't call it if that's not the case
func (k Keeper) synchronizeRewardAndReturnClaim(ctx sdk.Context, cdp cdptypes.CDP) types.FUSDMintingClaim {
	k.SynchronizeFUSDMintingReward(ctx, cdp)
	claim, _ := k.GetFUSDMintingClaim(ctx, cdp.Owner)
	return claim
}

// ZeroFUSDMintingClaim zeroes out the claim object's rewards and returns the updated claim object
func (k Keeper) ZeroFUSDMintingClaim(ctx sdk.Context, claim types.FUSDMintingClaim) types.FUSDMintingClaim {
	claim.Reward = sdk.NewCoin(claim.Reward.Denom, sdk.ZeroInt())
	k.SetFUSDMintingClaim(ctx, claim)
	return claim
}
