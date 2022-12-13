package accumulators

import (
	"errors"
	"fmt"
	"sort"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"

	earntypes "github.com/mage-coven/mage/x/earn/types"
	"github.com/mage-coven/mage/x/incentive/keeper/adapters"
	"github.com/mage-coven/mage/x/incentive/keeper/store"
	"github.com/mage-coven/mage/x/incentive/types"
)

// EarnAccumulator is an accumulator for Earn claim types. This includes
// claiming staking rewards and reward distribution for liquid mage.
type EarnAccumulator struct {
	store        store.IncentiveStore
	liquidKeeper types.LiquidKeeper
	earnKeeper   types.EarnKeeper
	adapters     adapters.SourceAdapters
}

var _ types.RewardAccumulator = EarnAccumulator{}

// NewEarnAccumulator returns a new EarnAccumulator.
func NewEarnAccumulator(
	store store.IncentiveStore,
	liquidKeeper types.LiquidKeeper,
	earnKeeper types.EarnKeeper,
	adapters adapters.SourceAdapters,
) EarnAccumulator {
	return EarnAccumulator{
		store:        store,
		liquidKeeper: liquidKeeper,
		earnKeeper:   earnKeeper,
		adapters:     adapters,
	}
}

// AccumulateRewards calculates new rewards to distribute this block and updates
// the global indexes to reflect this. The provided rewardPeriod must be valid
// to avoid panics in calculating time durations.
func (a EarnAccumulator) AccumulateRewards(
	ctx sdk.Context,
	claimType types.ClaimType,
	rewardPeriod types.MultiRewardPeriod,
) error {
	if claimType != types.CLAIM_TYPE_EARN {
		panic(fmt.Sprintf(
			"invalid claim type for earn accumulator, expected %s but got %s",
			types.CLAIM_TYPE_EARN,
			claimType,
		))
	}

	if rewardPeriod.CollateralType == "bmage" {
		return a.accumulateEarnBmageRewards(ctx, rewardPeriod)
	}

	// Non bmage vaults use the basic accumulator.
	return NewBasicAccumulator(a.store, a.adapters).AccumulateRewards(ctx, claimType, rewardPeriod)
}

// accumulateEarnBmageRewards does the same as AccumulateEarnRewards but for
// *all* bmage vaults.
func (k EarnAccumulator) accumulateEarnBmageRewards(ctx sdk.Context, rewardPeriod types.MultiRewardPeriod) error {
	// All bmage vault denoms
	bmageVaultsDenoms := make(map[string]bool)

	// bmage vault denoms from earn records (non-empty vaults)
	k.earnKeeper.IterateVaultRecords(ctx, func(record earntypes.VaultRecord) (stop bool) {
		if k.liquidKeeper.IsDerivativeDenom(ctx, record.TotalShares.Denom) {
			bmageVaultsDenoms[record.TotalShares.Denom] = true
		}

		return false
	})

	// bmage vault denoms from past incentive indexes, may include vaults
	// that were fully withdrawn.
	k.store.IterateRewardIndexesByClaimType(
		ctx,
		types.CLAIM_TYPE_EARN,
		func(reward types.TypedRewardIndexes) (stop bool) {
			if k.liquidKeeper.IsDerivativeDenom(ctx, reward.CollateralType) {
				bmageVaultsDenoms[reward.CollateralType] = true
			}

			return false
		})

	totalBmageValue, err := k.liquidKeeper.GetTotalDerivativeValue(ctx)
	if err != nil {
		return err
	}

	i := 0
	sortedBmageVaultsDenoms := make([]string, len(bmageVaultsDenoms))
	for vaultDenom := range bmageVaultsDenoms {
		sortedBmageVaultsDenoms[i] = vaultDenom
		i++
	}

	// Sort the vault denoms to ensure deterministic iteration order.
	sort.Strings(sortedBmageVaultsDenoms)

	// Accumulate rewards for each bmage vault.
	for _, bmageDenom := range sortedBmageVaultsDenoms {
		derivativeValue, err := k.liquidKeeper.GetDerivativeValue(ctx, bmageDenom)
		if err != nil {
			return err
		}

		k.accumulateBmageEarnRewards(
			ctx,
			bmageDenom,
			rewardPeriod.Start,
			rewardPeriod.End,
			GetProportionalRewardsPerSecond(
				rewardPeriod,
				totalBmageValue.Amount,
				derivativeValue.Amount,
			),
		)
	}

	return nil
}

func GetProportionalRewardsPerSecond(
	rewardPeriod types.MultiRewardPeriod,
	totalBmageSupply sdk.Int,
	singleBmageSupply sdk.Int,
) sdk.DecCoins {
	// Rate per bmage-xxx = rewardsPerSecond * % of bmage-xxx
	//                    = rewardsPerSecond * (bmage-xxx / total bmage)
	//                    = (rewardsPerSecond * bmage-xxx) / total bmage

	newRate := sdk.NewDecCoins()

	// Prevent division by zero, if there are no total shares then there are no
	// rewards.
	if totalBmageSupply.IsZero() {
		return newRate
	}

	for _, rewardCoin := range rewardPeriod.RewardsPerSecond {
		scaledAmount := rewardCoin.Amount.ToDec().
			Mul(singleBmageSupply.ToDec()).
			Quo(totalBmageSupply.ToDec())

		newRate = newRate.Add(sdk.NewDecCoinFromDec(rewardCoin.Denom, scaledAmount))
	}

	return newRate
}

func (k EarnAccumulator) accumulateBmageEarnRewards(
	ctx sdk.Context,
	collateralType string,
	periodStart time.Time,
	periodEnd time.Time,
	periodRewardsPerSecond sdk.DecCoins,
) {
	// Collect staking rewards for this validator, does not have any start/end
	// period time restrictions.
	stakingRewards := k.collectDerivativeStakingRewards(ctx, collateralType)

	// Collect incentive rewards
	// **Total rewards** for vault per second, NOT per share
	perSecondRewards := k.collectPerSecondRewards(
		ctx,
		collateralType,
		periodStart,
		periodEnd,
		periodRewardsPerSecond,
	)

	// **Total rewards** for vault per second, NOT per share
	rewards := stakingRewards.Add(perSecondRewards...)

	// Distribute rewards by incrementing indexes
	indexes, found := k.store.GetRewardIndexesOfClaimType(ctx, types.CLAIM_TYPE_EARN, collateralType)
	if !found {
		indexes = types.RewardIndexes{}
	}

	totalSourceShares := k.adapters.TotalSharesBySource(ctx, types.CLAIM_TYPE_EARN, collateralType)
	var increment types.RewardIndexes
	if totalSourceShares.GT(sdk.ZeroDec()) {
		// Divide total rewards by total shares to get the reward **per share**
		// Leave as nil if no source shares
		increment = types.NewRewardIndexesFromCoins(rewards).Quo(totalSourceShares)
	}
	updatedIndexes := indexes.Add(increment)

	if len(updatedIndexes) > 0 {
		// the store panics when setting empty or nil indexes
		k.store.SetRewardIndexes(ctx, types.CLAIM_TYPE_EARN, collateralType, updatedIndexes)
	}
}

func (k EarnAccumulator) collectDerivativeStakingRewards(ctx sdk.Context, collateralType string) sdk.DecCoins {
	rewards, err := k.liquidKeeper.CollectStakingRewardsByDenom(ctx, collateralType, types.IncentiveMacc)
	if err != nil {
		if !errors.Is(err, distrtypes.ErrNoValidatorDistInfo) &&
			!errors.Is(err, distrtypes.ErrEmptyDelegationDistInfo) {
			panic(fmt.Sprintf("failed to collect staking rewards for %s: %s", collateralType, err))
		}

		// otherwise there's no validator or delegation yet
		rewards = nil
	}
	return sdk.NewDecCoinsFromCoins(rewards...)
}

func (k EarnAccumulator) collectPerSecondRewards(
	ctx sdk.Context,
	collateralType string,
	periodStart time.Time,
	periodEnd time.Time,
	periodRewardsPerSecond sdk.DecCoins,
) sdk.DecCoins {
	previousAccrualTime, found := k.store.GetRewardAccrualTime(ctx, types.CLAIM_TYPE_EARN, collateralType)
	if !found {
		previousAccrualTime = ctx.BlockTime()
	}

	rewards, accumulatedTo := types.CalculatePerSecondRewards(
		periodStart,
		periodEnd,
		periodRewardsPerSecond,
		previousAccrualTime,
		ctx.BlockTime(),
	)

	k.store.SetRewardAccrualTime(ctx, types.CLAIM_TYPE_EARN, collateralType, accumulatedTo)

	// Don't need to move funds as they're assumed to be in the IncentiveMacc module account already.
	return rewards
}
