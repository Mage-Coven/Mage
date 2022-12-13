package client

import (
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"

	"github.com/mage-coven/mage/x/magedist/client/cli"
	"github.com/mage-coven/mage/x/magedist/client/rest"
)

// community-pool multi-spend proposal handler
var (
	ProposalHandler = govclient.NewProposalHandler(cli.GetCmdSubmitProposal, rest.ProposalRESTHandler)
)
