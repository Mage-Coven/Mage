package types_test

import (
	"fmt"
	"testing"

	types "github.com/mage-coven/mage/x/swap/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// create a new umage coin from int64
func umage(amount int64) sdk.Coin {
	return sdk.NewCoin("umage", sdk.NewInt(amount))
}

// create a new fusd coin from int64
func fusd(amount int64) sdk.Coin {
	return sdk.NewCoin("fusd", sdk.NewInt(amount))
}

// create a new hard coin from int64
func hard(amount int64) sdk.Coin {
	return sdk.NewCoin("hard", sdk.NewInt(amount))
}

func TestDenominatedPool_NewDenominatedPool_Validation(t *testing.T) {
	testCases := []struct {
		reservesA   sdk.Coin
		reservesB   sdk.Coin
		expectedErr string
	}{
		{umage(0), fusd(1e6), "reserves must have two denominations: invalid pool"},
		{umage(1e6), fusd(0), "reserves must have two denominations: invalid pool"},
		{fusd(0), umage(1e6), "reserves must have two denominations: invalid pool"},
		{fusd(0), umage(1e6), "reserves must have two denominations: invalid pool"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("reservesA=%s reservesB=%s", tc.reservesA, tc.reservesB), func(t *testing.T) {
			pool, err := types.NewDenominatedPool(sdk.NewCoins(tc.reservesA, tc.reservesB))
			require.EqualError(t, err, tc.expectedErr)
			assert.Nil(t, pool)
		})
	}
}

func TestDenominatedPool_NewDenominatedPoolWithExistingShares_Validation(t *testing.T) {
	testCases := []struct {
		reservesA   sdk.Coin
		reservesB   sdk.Coin
		totalShares sdk.Int
		expectedErr string
	}{
		{umage(0), fusd(1e6), i(1), "reserves must have two denominations: invalid pool"},
		{fusd(0), umage(1e6), i(1), "reserves must have two denominations: invalid pool"},
		{umage(1e6), fusd(1e6), i(0), "total shares must be greater than zero: invalid pool"},
		{fusd(1e6), umage(1e6), i(-1), "total shares must be greater than zero: invalid pool"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("reservesA=%s reservesB=%s", tc.reservesA, tc.reservesB), func(t *testing.T) {
			pool, err := types.NewDenominatedPoolWithExistingShares(sdk.NewCoins(tc.reservesA, tc.reservesB), tc.totalShares)
			require.EqualError(t, err, tc.expectedErr)
			assert.Nil(t, pool)
		})
	}
}

func TestDenominatedPool_InitialState(t *testing.T) {
	reserves := sdk.NewCoins(umage(1e6), fusd(5e6))
	totalShares := i(2236067)

	pool, err := types.NewDenominatedPool(reserves)
	require.NoError(t, err)

	assert.Equal(t, pool.Reserves(), reserves)
	assert.Equal(t, pool.TotalShares(), totalShares)
}

func TestDenominatedPool_InitialState_ExistingShares(t *testing.T) {
	reserves := sdk.NewCoins(umage(1e6), fusd(5e6))
	totalShares := i(2e6)

	pool, err := types.NewDenominatedPoolWithExistingShares(reserves, totalShares)
	require.NoError(t, err)

	assert.Equal(t, pool.Reserves(), reserves)
	assert.Equal(t, pool.TotalShares(), totalShares)
}

func TestDenominatedPool_ShareValue(t *testing.T) {
	reserves := sdk.NewCoins(umage(10e6), fusd(50e6))

	pool, err := types.NewDenominatedPool(reserves)
	require.NoError(t, err)

	assert.Equal(t, reserves, pool.ShareValue(pool.TotalShares()))

	halfReserves := sdk.NewCoins(umage(4999999), fusd(24999998))
	assert.Equal(t, halfReserves, pool.ShareValue(pool.TotalShares().Quo(i(2))))
}

func TestDenominatedPool_AddLiquidity(t *testing.T) {
	reserves := sdk.NewCoins(umage(10e6), fusd(50e6))
	desired := sdk.NewCoins(umage(1e6), fusd(1e6))

	pool, err := types.NewDenominatedPool(reserves)
	require.NoError(t, err)
	initialShares := pool.TotalShares()

	deposit, shares := pool.AddLiquidity(desired)
	require.True(t, shares.IsPositive())
	require.True(t, deposit.IsAllPositive())

	assert.Equal(t, reserves.Add(deposit...), pool.Reserves())
	assert.Equal(t, initialShares.Add(shares), pool.TotalShares())
}

func TestDenominatedPool_RemoveLiquidity(t *testing.T) {
	reserves := sdk.NewCoins(umage(10e6), fusd(50e6))

	pool, err := types.NewDenominatedPool(reserves)
	require.NoError(t, err)

	withdraw := pool.RemoveLiquidity(pool.TotalShares())

	assert.True(t, pool.Reserves().IsZero())
	assert.True(t, pool.TotalShares().IsZero())
	assert.True(t, pool.IsEmpty())
	assert.Equal(t, reserves, withdraw)
}

func TestDenominatedPool_SwapWithExactInput(t *testing.T) {
	reserves := sdk.NewCoins(umage(10e6), fusd(50e6))

	pool, err := types.NewDenominatedPool(reserves)
	require.NoError(t, err)

	output, fee := pool.SwapWithExactInput(umage(1e6), d("0.003"))

	assert.Equal(t, fusd(4533054), output)
	assert.Equal(t, umage(3000), fee)
	assert.Equal(t, sdk.NewCoins(umage(11e6), fusd(45466946)), pool.Reserves())

	pool, err = types.NewDenominatedPool(reserves)
	require.NoError(t, err)

	output, fee = pool.SwapWithExactInput(fusd(5e6), d("0.003"))

	assert.Equal(t, umage(906610), output)
	assert.Equal(t, fusd(15000), fee)
	assert.Equal(t, sdk.NewCoins(umage(9093390), fusd(55e6)), pool.Reserves())

	assert.Panics(t, func() { pool.SwapWithExactInput(hard(1e6), d("0.003")) }, "SwapWithExactInput did not panic on invalid denomination")
}

func TestDenominatedPool_SwapWithExactOuput(t *testing.T) {
	reserves := sdk.NewCoins(umage(10e6), fusd(50e6))

	pool, err := types.NewDenominatedPool(reserves)
	require.NoError(t, err)

	input, fee := pool.SwapWithExactOutput(umage(1e6), d("0.003"))

	assert.Equal(t, fusd(5572273), input)
	assert.Equal(t, fusd(16717), fee)
	assert.Equal(t, sdk.NewCoins(umage(9e6), fusd(55572273)), pool.Reserves())

	pool, err = types.NewDenominatedPool(reserves)
	require.NoError(t, err)

	input, fee = pool.SwapWithExactOutput(fusd(5e6), d("0.003"))

	assert.Equal(t, umage(1114456), input)
	assert.Equal(t, umage(3344), fee)
	assert.Equal(t, sdk.NewCoins(umage(11114456), fusd(45e6)), pool.Reserves())

	assert.Panics(t, func() { pool.SwapWithExactOutput(hard(1e6), d("0.003")) }, "SwapWithExactOutput did not panic on invalid denomination")
}
