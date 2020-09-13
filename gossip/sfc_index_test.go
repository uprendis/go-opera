package gossip

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	eth "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	"github.com/Fantom-foundation/go-lachesis/gossip/sfc110"
	"github.com/Fantom-foundation/go-lachesis/gossip/sfc201"
	"github.com/Fantom-foundation/go-lachesis/gossip/sfcproxy"
	"github.com/Fantom-foundation/go-lachesis/inter/idx"
	"github.com/Fantom-foundation/go-lachesis/lachesis/genesis/sfc"
	"github.com/Fantom-foundation/go-lachesis/logger"
	"github.com/Fantom-foundation/go-lachesis/utils"
)

type commonSfc interface {
	CurrentSealedEpoch(opts *bind.CallOpts) (*big.Int, error)
	CalcValidatorRewards(opts *bind.CallOpts, stakerID *big.Int, fromEpoch *big.Int, maxEpochs *big.Int) (*big.Int, *big.Int, *big.Int, error)
}

func TestSFC(t *testing.T) {
	logger.SetTestMode(t)

	env := newTestEnv()
	defer env.Close()

	sfcProxy, err := sfcproxy.NewContract(sfc.ContractAddress, env)
	require.NoError(t, err)

	var (
		sfc1 *sfc110.Contract
		sfc2 *sfc201.Contract

		prev struct {
			epoch             *big.Int
			reward            *big.Int
			totalLockedAmount *big.Int
		}
	)

	_ = true &&

		t.Run("Genesis v1.0.0", func(t *testing.T) {
			// nothing to do
		}) &&

		t.Run("Some transfers I", func(t *testing.T) {
			cicleTransfers(t, env, 3)
		}) &&

		t.Run("Upgrade to v1.1.0-rc1", func(t *testing.T) {
			require := require.New(t)

			r := env.ApplyBlock(nextEpoch,
				env.Contract(1, utils.ToFtm(0), sfc110.ContractBin),
			)
			newImpl := r[0].ContractAddress

			admin := env.Payer(1)
			tx, err := sfcProxy.ContractTransactor.UpgradeTo(admin, newImpl)
			require.NoError(err)
			env.ApplyBlock(sameEpoch, tx)

			impl, err := sfcProxy.Implementation(env.ReadOnly())
			require.NoError(err)
			require.Equal(newImpl, impl, "SFC-proxy: implementation address")

			sfc1, err = sfc110.NewContract(sfc.ContractAddress, env)
			require.NoError(err)

			epoch, err := sfc1.ContractCaller.CurrentEpoch(env.ReadOnly())
			require.NoError(err)
			require.Equal(0, epoch.Cmp(big.NewInt(2)), "current epoch")
		}) &&

		t.Run("Upgrade stakers storage", func(t *testing.T) {
			require := require.New(t)

			stakers, err := sfc1.StakersLastID(env.ReadOnly())
			require.NoError(err)
			txs := make([]*eth.Transaction, 0, int(stakers.Int64()))
			for i := stakers.Int64(); i > 0; i-- {
				tx, err := sfc1.UpgradeStakerStorage(env.Payer(int(i)), big.NewInt(i))
				require.NoError(err)
				txs = append(txs, tx)
			}
			env.ApplyBlock(sameEpoch, txs...)

		}) &&

		t.Run("Some transfers II", func(t *testing.T) {
			cicleTransfers(t, env, 3)
		}) &&

		t.Run("Create staker 4", func(t *testing.T) {
			require := require.New(t)

			newStake := utils.ToFtm(genesisStake / 2)
			minStake, err := sfc1.MinStake(env.ReadOnly())
			require.NoError(err)
			require.Greater(newStake.Cmp(minStake), 0,
				fmt.Sprintf("newStake(%s) < minStake(%s)", newStake, minStake))

			env.ApplyBlock(sameEpoch,
				env.Transfer(1, 4, big.NewInt(0).Add(newStake, utils.ToFtm(10))),
			)
			tx, err := sfc1.CreateStake(env.Payer(4, newStake), nil)
			require.NoError(err)
			env.ApplyBlock(nextEpoch, tx)
			newId, err := sfc1.SfcAddressToStakerID(env.ReadOnly(), env.Address(4))
			require.NoError(err)
			env.AddValidator(idx.StakerID(newId.Uint64()))
		}) &&

		t.Run("Create delegator 5", func(t *testing.T) {
			require := require.New(t)

			newDelegation := utils.ToFtm(genesisStake / 2)
			env.ApplyBlock(sameEpoch,
				env.Transfer(1, 5, big.NewInt(0).Add(newDelegation, utils.ToFtm(10))),
			)

			staker, err := sfc1.SfcAddressToStakerID(env.ReadOnly(), env.Address(4))
			require.NoError(err)

			tx, err := sfc1.CreateDelegation(env.Payer(5, newDelegation), staker)
			require.NoError(err)
			env.ApplyBlock(sameEpoch, tx)
			env.AddDelegator(env.Address(5))
		}) &&

		t.Run("Upgrade to v2.0.1-rc2", func(t *testing.T) {
			require := require.New(t)

			r := env.ApplyBlock(sameEpoch,
				env.Contract(1, utils.ToFtm(0), sfc201.ContractBin),
			)
			newImpl := r[0].ContractAddress

			admin := env.Payer(1)
			tx, err := sfcProxy.ContractTransactor.UpgradeTo(admin, newImpl)
			require.NoError(err)
			env.ApplyBlock(sameEpoch, tx)

			impl, err := sfcProxy.Implementation(env.ReadOnly())
			require.NoError(err)
			require.Equal(newImpl, impl, "SFC-proxy: implementation address")

			sfc2, err = sfc201.NewContract(sfc.ContractAddress, env)
			require.NoError(err)

			epoch, err := sfc2.ContractCaller.CurrentEpoch(env.ReadOnly())
			require.NoError(err)
			require.Equal(0, epoch.Cmp(big.NewInt(3)), "current epoch: %d", epoch.Uint64())
			prev.epoch = epoch
		})

}

func requireRewards(
	t *testing.T, env *testEnv, sfc commonSfc, stakes []int64,
) (
	rewards []*big.Int,
) {
	require := require.New(t)

	epoch, err := sfc.CurrentSealedEpoch(env.ReadOnly())
	require.NoError(err)

	validators := env.Validators()
	rewards = make([]*big.Int, len(validators)+len(env.delegators))
	for i, id := range env.validators {
		staker := big.NewInt(int64(id))
		rewards[i], _, _, err = sfc.CalcValidatorRewards(env.ReadOnly(), staker, epoch, big.NewInt(1))
		require.NoError(err)
		t.Logf("validator reward %d: %s", i, rewards[i])
	}

	for i, addr := range env.delegators {
		i += len(env.validators)
		switch sfc := sfc.(type) {
		case *sfc110.Contract:
			rewards[i], _, _, err = sfc.CalcDelegationRewards(env.ReadOnly(), addr, epoch, big.NewInt(1))
			require.NoError(err)
		case *sfc201.Contract:
			sum := new(big.Int)
			for _, id := range env.validators {
				staker := big.NewInt(int64(id))
				r, _, _, err := sfc.CalcDelegationRewards(env.ReadOnly(), addr, staker, epoch, big.NewInt(1))
				require.NoError(err)
				sum = new(big.Int).Add(sum, r)
			}
			rewards[i] = sum
		default:
			panic("unknown contract type")
		}
		t.Logf("validator reward %d: %s", i, rewards[i])
	}

	for i := range env.validators {
		if i == 0 {
			continue
		}

		a := new(big.Int).Mul(rewards[0], big.NewInt(stakes[i]))
		b := new(big.Int).Mul(rewards[i], big.NewInt(stakes[0]))
		want := new(big.Int).Div(b, rewards[0])
		require.Equal(
			0, a.Cmp(b),
			"reward#0: %s, reward#%d: %s. Got %d:%d, want %d:%s proportion (validator)",
			rewards[0], i, rewards[i],
			stakes[0], stakes[i],
			stakes[0], want,
		)
	}

	for i := range env.delegators {
		i += len(env.validators)
		a := new(big.Int).Mul(rewards[0], big.NewInt(stakes[i]))
		b := new(big.Int).Mul(rewards[i], big.NewInt(stakes[0]))
		want := new(big.Int).Div(b, rewards[0])
		require.Equal(
			0, a.Cmp(b),
			"reward#0: %s, reward#%d: %s. Got %d:%d, want %d:%s proportion (delegator)",
			rewards[0], i, rewards[i],
			stakes[0], stakes[i],
			stakes[0], want,
		)
	}

	return
}

func totalLockedAmount(t *testing.T, env *testEnv, sfc2 *sfc201.Contract) *big.Int {
	epoch, err := sfc2.CurrentSealedEpoch(env.ReadOnly())
	require.NoError(t, err)
	es, err := sfc2.EpochSnapshots(env.ReadOnly(), epoch)
	require.NoError(t, err)

	return es.TotalLockedAmount
}

func printEpochStats(t *testing.T, env *testEnv, sfc2 *sfc201.Contract) {
	epoch, err := sfc2.CurrentSealedEpoch(env.ReadOnly())
	require.NoError(t, err)
	es, err := sfc2.EpochSnapshots(env.ReadOnly(), epoch)
	require.NoError(t, err)
	t.Logf("Epoch%sStat{dir: %s, BaseRewardPerSecond: %s, TotalLockedAmount: %s, TotalBaseRewardWeight: %s}",
		epoch, es.Duration, es.BaseRewardPerSecond, es.TotalLockedAmount, es.TotalBaseRewardWeight)
}

func printValidators(t *testing.T, env *testEnv, sfc2 *sfc201.Contract) {
	require := require.New(t)

	max, err := sfc2.StakersLastID(env.ReadOnly())
	require.NoError(err)

	for id := big.NewInt(1); id.Cmp(max) <= 0; id.Add(id, big.NewInt(1)) {
		s, err := sfc2.Stakers(env.ReadOnly(), id)
		require.NoError(err)
		t.Logf("%s: %#v", id, s)
	}
}

func cicleTransfers(t *testing.T, env *testEnv, count uint64) {
	require := require.New(t)

	balances := make([]*big.Int, 3)
	for i := range balances {
		balances[i] = env.State().GetBalance(env.Address(i + 1))
	}

	for i := uint64(0); i < count; i++ {
		env.ApplyBlock(sameEpoch,
			env.Transfer(1, 2, utils.ToFtm(100)),
		)
		env.ApplyBlock(sameEpoch,
			env.Transfer(2, 3, utils.ToFtm(100)),
		)
		env.ApplyBlock(sameEpoch,
			env.Transfer(3, 1, utils.ToFtm(100)),
		)
	}

	gas := big.NewInt(0).Mul(big.NewInt(int64(count*gasLimit)), env.GasPrice)
	for i := range balances {
		require.Equal(
			big.NewInt(0).Sub(balances[i], gas),
			env.State().GetBalance(env.Address(i+1)),
			fmt.Sprintf("account%d", i),
		)
	}
}
