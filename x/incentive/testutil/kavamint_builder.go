package testutil

import (
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/mage-coven/mage/app"

	mageminttypes "github.com/mage-coven/mage/x/magemint/types"
)

// MagemintGenesisBuilder is a tool for creating a mint genesis state.
// Helper methods add values onto a default genesis state.
// All methods are immutable and return updated copies of the builder.
type MagemintGenesisBuilder struct {
	mageminttypes.GenesisState
}

var _ GenesisBuilder = (*MagemintGenesisBuilder)(nil)

func NewMagemintGenesisBuilder() MagemintGenesisBuilder {
	gen := mageminttypes.DefaultGenesisState()
	gen.Params.CommunityPoolInflation = sdk.ZeroDec()
	gen.Params.StakingRewardsApy = sdk.ZeroDec()

	return MagemintGenesisBuilder{
		GenesisState: *gen,
	}
}

func (builder MagemintGenesisBuilder) Build() mageminttypes.GenesisState {
	return builder.GenesisState
}

func (builder MagemintGenesisBuilder) BuildMarshalled(cdc codec.JSONCodec) app.GenesisState {
	built := builder.Build()

	return app.GenesisState{
		mageminttypes.ModuleName: cdc.MustMarshalJSON(&built),
	}
}

func (builder MagemintGenesisBuilder) WithPreviousBlockTime(t time.Time) MagemintGenesisBuilder {
	builder.PreviousBlockTime = t
	return builder
}

func (builder MagemintGenesisBuilder) WithStakingRewardsApy(apy sdk.Dec) MagemintGenesisBuilder {
	builder.Params.StakingRewardsApy = apy
	return builder
}

func (builder MagemintGenesisBuilder) WithCommunityPoolInflation(
	inflation sdk.Dec,
) MagemintGenesisBuilder {
	builder.Params.CommunityPoolInflation = inflation
	return builder
}
