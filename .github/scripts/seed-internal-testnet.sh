#!/bin/bash
set -ex

# configure kava binary to talk to the desired chain endpoint
kava config node "${CHAIN_API_URL}"
kava config chain-id "${CHAIN_ID}"

# use the test keyring to allow scriptng key generation
kava config keyring-backend test

# wait for transactions to be committed per CLI command
kava config broadcast-mode block

# setup dev wallet
echo "${DEV_WALLET_MNEMONIC}" | kava keys add --recover dev-wallet

# setup kava etherum comptabile account for deploying
# erc20 contracts to the kava chain
echo "sweet ocean blush coil mobile ten floor sample nuclear power legend where place swamp young marble grit observe enforce lake blossom lesson upon plug" | kava keys add --recover --eth dev-erc20-deployer-wallet

# fund evm-contract-deployer account (using issuance)
kava tx issuance issue 200000000ukava kava1van3znl6597xgwwh46jgquutnqkwvwszjg04fz --from dev-wallet --gas-prices 0.5ukava -y

# deploy and fund USDC ERC20 contract
USD_CONTRACT_DEPLOY=$(npx hardhat --network "${ERC20_DEPLOYER_NETWORK_NAME}" deploy-erc20 "USD Coin" USDC 6)
USD_CONTRACT_ADDRESS=${USD_CONTRACT_DEPLOY: -42}
npx hardhat --network "${ERC20_DEPLOYER_NETWORK_NAME}" mint-erc20 "$USD_CONTRACT_ADDRESS" 0x6767114FFAA17C6439D7AEA480738B982CE63A02 1000000000000

# # deploy and fund wKava ERC20 contract
wKAVA_CONTRACT_DEPLOY=$(npx hardhat --network "${ERC20_DEPLOYER_NETWORK_NAME}" deploy-erc20 "Wrapped Kava" wKava 6)
wKAVA_CONTRACT_ADDRESS=${wKAVA_CONTRACT_DEPLOY: -42}
npx hardhat --network "${ERC20_DEPLOYER_NETWORK_NAME}" mint-erc20 "$wKAVA_CONTRACT_ADDRESS" 0x6767114FFAA17C6439D7AEA480738B982CE63A02 1000000000000

# deploy and fund BNB contract
BNB_CONTRACT_DEPLOY=$(npx hardhat --network "${ERC20_DEPLOYER_NETWORK_NAME}" deploy-erc20 "Binance" BNB 8)
BNB_CONTRACT_ADDRESS=${BNB_CONTRACT_DEPLOY: -42}
npx hardhat --network "${ERC20_DEPLOYER_NETWORK_NAME}" mint-erc20 "$BNB_CONTRACT_ADDRESS" 0x6767114FFAA17C6439D7AEA480738B982CE63A02 1000000000000

# deploy and fund USDT contract
USDT_CONTRACT_DEPLOY=$(npx hardhat --network "${ERC20_DEPLOYER_NETWORK_NAME}" deploy-erc20 "USDT" USDT 6)
USDT_CONTRACT_ADDRESS=${USDT_CONTRACT_DEPLOY: -42}
npx hardhat --network "${ERC20_DEPLOYER_NETWORK_NAME}" mint-erc20 "$USDT_CONTRACT_ADDRESS" 0x6767114FFAA17C6439D7AEA480738B982CE63A02 1000000000000

# deploy and fund DAI contract
DAI_CONTRACT_DEPLOY=$(npx hardhat --network "${ERC20_DEPLOYER_NETWORK_NAME}" deploy-erc20 "DAI" DAI 18)
DAI_CONTRACT_ADDRESS=${DAI_CONTRACT_DEPLOY: -42}
npx hardhat --network "${ERC20_DEPLOYER_NETWORK_NAME}" mint-erc20 "$DAI_CONTRACT_ADDRESS" 0x6767114FFAA17C6439D7AEA480738B982CE63A02 1000000000000

# give dev-wallet enough delegation power to pass proposals by itself
# enumerate all genesis validators
GENTX_VALIDATORS=("kavavaloper1xcgtffvv2yeqmgs3yz4gv29kgjrj8usxrnrlwp" "kavavaloper1w66m9hdzwgd6uc8g93zqkcumgwzrpcw958sh3s")

# issue 300KAVA to delegate to each validator
kava tx issuance issue 600000000ukava kava1vlpsrmdyuywvaqrv7rx6xga224sqfwz3fyfhwq \
  --from dev-wallet --gas-prices 0.5ukava -y

# delegate 300KAVA to each validator
for validator in "${GENTX_VALIDATORS[@]}"
do
  kava tx staking delegate "${validator}" 300000000ukava --from dev-wallet --gas-prices 0.5ukava -y
done
