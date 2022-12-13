package keeper_test

import (
	"time"

	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"

	"github.com/mage-coven/mage/x/incentive/types"
)

func (suite *HandlerTestSuite) TestPayoutFUSDClaim() {
	userAddr, receiverAddr := suite.addrs[0], suite.addrs[1]

	authBulder := suite.authBuilder().
		WithSimpleAccount(userAddr, cs(c("bnb", 1e12))).
		WithSimpleAccount(receiverAddr, nil)

	incentBuilder := suite.incentiveBuilder().
		WithSimpleFUSDRewardPeriod("bnb-a", c(types.FUSDMintingRewardDenom, 1e6))

	suite.SetupWithGenState(authBulder, incentBuilder)

	// mint some fusd
	err := suite.DeliverMsgCreateCDP(userAddr, c("bnb", 1e9), c("fusd", 1e7), "bnb-a")
	suite.NoError(err)
	// accumulate some rewards
	suite.NextBlockAfter(7 * time.Second)

	preClaimBal := suite.GetBalance(userAddr)

	msg := types.NewMsgClaimFUSDMintingReward(userAddr.String(), "large")

	// Claim a single denom
	err = suite.DeliverIncentiveMsg(&msg)
	suite.NoError(err)

	// Check rewards were paid out
	expectedRewards := cs(c(types.FUSDMintingRewardDenom, 7*1e6))
	suite.BalanceEquals(userAddr, preClaimBal.Add(expectedRewards...))

	suite.VestingPeriodsEqual(userAddr, []vestingtypes.Period{
		{Length: (17+31+28+31+30+31+30+31+31+30+31+30+31)*secondsPerDay - 7, Amount: expectedRewards},
	})
	// Check that claimed coins have been removed from a claim's reward
	suite.FUSDRewardEquals(userAddr, c(types.FUSDMintingRewardDenom, 0))
}
