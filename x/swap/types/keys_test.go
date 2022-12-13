package types_test

import (
	"testing"

	"github.com/mage-coven/mage/x/swap/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
)

func TestKeys(t *testing.T) {
	key := types.PoolKey(types.PoolID("umage", "fusd"))
	assert.Equal(t, types.PoolID("umage", "fusd"), string(key))

	key = types.DepositorPoolSharesKey(sdk.AccAddress("testaddress1"), types.PoolID("umage", "fusd"))
	assert.Equal(t, string(sdk.AccAddress("testaddress1"))+"|"+types.PoolID("umage", "fusd"), string(key))
}
