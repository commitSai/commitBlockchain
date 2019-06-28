package keeper

import (
	"fmt"
	
	sdk "github.com/comdex-blockchain/types"
	"github.com/comdex-blockchain/x/stake/types"
	"github.com/tendermint/tendermint/crypto"
)

// Slash a validator for an infraction committed at a known height
// Find the contributing stake at that height and burn the specified slashFactor
// of it, updating unbonding delegation & redelegations appropriately
//
// CONTRACT:
//    slashFactor is non-negative
// CONTRACT:
//    Infraction committed equal to or less than an unbonding period in the past,
//    so all unbonding delegations and redelegations from that height are stored
// CONTRACT:
//    Slash will not slash unbonded validators (for the above reason)
// CONTRACT:
//    Infraction committed at the current height or at a past height,
//    not at a height in the future
//
// nolint: gocyclo
func (k Keeper) Slash(ctx sdk.Context, pubkey crypto.PubKey, infractionHeight int64, power int64, slashFactor sdk.Dec) {
	logger := ctx.Logger().With("module", "x/stake")
	
	if slashFactor.LT(sdk.ZeroDec()) {
		panic(fmt.Errorf("attempted to slash with a negative slashFactor: %v", slashFactor))
	}
	
	// Amount of slashing = slash slashFactor * power at time of infraction
	slashAmount := sdk.NewDec(power).Mul(slashFactor)
	// ref https://github.com/comdex-blockchain/issues/1348
	// ref https://github.com/comdex-blockchain/issues/1471
	
	validator, found := k.GetValidatorByPubKey(ctx, pubkey)
	if !found {
		// If not found, the validator must have been overslashed and removed - so we don't need to do anything
		// NOTE:  Correctness dependent on invariant that unbonding delegations / redelegations must also have been completely
		//        slashed in this case - which we don't explicitly check, but should be true.
		// Log the slash attempt for future reference (maybe we should tag it too)
		logger.Error(fmt.Sprintf(
			"WARNING: Ignored attempt to slash a nonexistent validator with address %s, we recommend you investigate immediately",
			pubkey.Address()))
		return
	}
	
	// should not be slashing unbonded
	if validator.IsUnbonded(ctx) {
		panic(fmt.Sprintf("should not be slashing unbonded validator: %v", validator))
	}
	
	operatorAddress := validator.GetOperator()
	
	// Track remaining slash amount for the validator
	// This will decrease when we slash unbondings and
	// redelegations, as that stake has since unbonded
	remainingSlashAmount := slashAmount
	
	switch {
	case infractionHeight > ctx.BlockHeight():
		
		// Can't slash infractions in the future
		panic(fmt.Sprintf(
			"impossible attempt to slash future infraction at height %d but we are at height %d",
			infractionHeight, ctx.BlockHeight()))
	
	case infractionHeight == ctx.BlockHeight():
		
		// Special-case slash at current height for efficiency - we don't need to look through unbonding delegations or redelegations
		logger.Info(fmt.Sprintf(
			"Slashing at current height %d, not scanning unbonding delegations & redelegations",
			infractionHeight))
	
	case infractionHeight < ctx.BlockHeight():
		
		// Iterate through unbonding delegations from slashed validator
		unbondingDelegations := k.GetUnbondingDelegationsFromValidator(ctx, operatorAddress)
		for _, unbondingDelegation := range unbondingDelegations {
			amountSlashed := k.slashUnbondingDelegation(ctx, unbondingDelegation, infractionHeight, slashFactor)
			if amountSlashed.IsZero() {
				continue
			}
			remainingSlashAmount = remainingSlashAmount.Sub(amountSlashed)
		}
		
		// Iterate through redelegations from slashed validator
		redelegations := k.GetRedelegationsFromValidator(ctx, operatorAddress)
		for _, redelegation := range redelegations {
			amountSlashed := k.slashRedelegation(ctx, validator, redelegation, infractionHeight, slashFactor)
			if amountSlashed.IsZero() {
				continue
			}
			remainingSlashAmount = remainingSlashAmount.Sub(amountSlashed)
		}
	}
	
	// Cannot decrease balance below zero
	tokensToBurn := sdk.MinDec(remainingSlashAmount, validator.Tokens)
	
	// burn validator's tokens
	pool := k.GetPool(ctx)
	validator, pool = validator.RemoveTokens(pool, tokensToBurn)
	pool.LooseTokens = pool.LooseTokens.Sub(tokensToBurn)
	k.SetPool(ctx, pool)
	
	// update the validator, possibly kicking it out
	validator = k.UpdateValidator(ctx, validator)
	
	// remove validator if it has no more tokens
	if validator.Tokens.IsZero() {
		k.RemoveValidator(ctx, validator.Operator)
	}
	
	// Log that a slash occurred!
	logger.Info(fmt.Sprintf(
		"Validator %s slashed by slashFactor %s, burned %v tokens",
		pubkey.Address(), slashFactor.String(), tokensToBurn))
	
	// TODO Return event(s), blocked on https://github.com/tendermint/tendermint/pull/1803
	return
}

// jail a validator
func (k Keeper) Jail(ctx sdk.Context, pubkey crypto.PubKey) {
	k.setJailed(ctx, pubkey, true)
	logger := ctx.Logger().With("module", "x/stake")
	logger.Info(fmt.Sprintf("Validator %s jailed", pubkey.Address()))
	// TODO Return event(s), blocked on https://github.com/tendermint/tendermint/pull/1803
	return
}

// unjail a validator
func (k Keeper) Unjail(ctx sdk.Context, pubkey crypto.PubKey) {
	k.setJailed(ctx, pubkey, false)
	logger := ctx.Logger().With("module", "x/stake")
	logger.Info(fmt.Sprintf("Validator %s unjailed", pubkey.Address()))
	// TODO Return event(s), blocked on https://github.com/tendermint/tendermint/pull/1803
	return
}

// set the jailed flag on a validator
func (k Keeper) setJailed(ctx sdk.Context, pubkey crypto.PubKey, isJailed bool) {
	validator, found := k.GetValidatorByPubKey(ctx, pubkey)
	if !found {
		panic(fmt.Errorf("Validator with pubkey %s not found, cannot set jailed to %v", pubkey, isJailed))
	}
	validator.Jailed = isJailed
	k.UpdateValidator(ctx, validator) // update validator, possibly unbonding or bonding it
	return
}

// slash an unbonding delegation and update the pool
// return the amount that would have been slashed assuming
// the unbonding delegation had enough stake to slash
// (the amount actually slashed may be less if there's
// insufficient stake remaining)
func (k Keeper) slashUnbondingDelegation(ctx sdk.Context, unbondingDelegation types.UnbondingDelegation,
	infractionHeight int64, slashFactor sdk.Dec) (slashAmount sdk.Dec) {
	
	now := ctx.BlockHeader().Time
	
	// If unbonding started before this height, stake didn't contribute to infraction
	if unbondingDelegation.CreationHeight < infractionHeight {
		return sdk.ZeroDec()
	}
	
	if unbondingDelegation.MinTime.Before(now) {
		// Unbonding delegation no longer eligible for slashing, skip it
		// TODO Settle and delete it automatically?
		return sdk.ZeroDec()
	}
	
	// Calculate slash amount proportional to stake contributing to infraction
	slashAmount = sdk.NewDecFromInt(unbondingDelegation.InitialBalance.Amount).Mul(slashFactor)
	
	// Don't slash more tokens than held
	// Possible since the unbonding delegation may already
	// have been slashed, and slash amounts are calculated
	// according to stake held at time of infraction
	unbondingSlashAmount := sdk.MinInt(slashAmount.RoundInt(), unbondingDelegation.Balance.Amount)
	
	// Update unbonding delegation if necessary
	if !unbondingSlashAmount.IsZero() {
		unbondingDelegation.Balance.Amount = unbondingDelegation.Balance.Amount.Sub(unbondingSlashAmount)
		k.SetUnbondingDelegation(ctx, unbondingDelegation)
		pool := k.GetPool(ctx)
		
		// Burn loose tokens
		// Ref https://github.com/comdex-blockchain/pull/1278#discussion_r198657760
		pool.LooseTokens = pool.LooseTokens.Sub(slashAmount)
		k.SetPool(ctx, pool)
	}
	
	return
}

// slash a redelegation and update the pool
// return the amount that would have been slashed assuming
// the unbonding delegation had enough stake to slash
// (the amount actually slashed may be less if there's
// insufficient stake remaining)
func (k Keeper) slashRedelegation(ctx sdk.Context, validator types.Validator, redelegation types.Redelegation,
	infractionHeight int64, slashFactor sdk.Dec) (slashAmount sdk.Dec) {
	
	now := ctx.BlockHeader().Time
	
	// If redelegation started before this height, stake didn't contribute to infraction
	if redelegation.CreationHeight < infractionHeight {
		return sdk.ZeroDec()
	}
	
	if redelegation.MinTime.Before(now) {
		// Redelegation no longer eligible for slashing, skip it
		// TODO Delete it automatically?
		return sdk.ZeroDec()
	}
	
	// Calculate slash amount proportional to stake contributing to infraction
	slashAmount = sdk.NewDecFromInt(redelegation.InitialBalance.Amount).Mul(slashFactor)
	
	// Don't slash more tokens than held
	// Possible since the redelegation may already
	// have been slashed, and slash amounts are calculated
	// according to stake held at time of infraction
	redelegationSlashAmount := sdk.MinInt(slashAmount.RoundInt(), redelegation.Balance.Amount)
	
	// Update redelegation if necessary
	if !redelegationSlashAmount.IsZero() {
		redelegation.Balance.Amount = redelegation.Balance.Amount.Sub(redelegationSlashAmount)
		k.SetRedelegation(ctx, redelegation)
	}
	
	// Unbond from target validator
	sharesToUnbond := slashFactor.Mul(redelegation.SharesDst)
	if !sharesToUnbond.IsZero() {
		delegation, found := k.GetDelegation(ctx, redelegation.DelegatorAddr, redelegation.ValidatorDstAddr)
		if !found {
			// If deleted, delegation has zero shares, and we can't unbond any more
			return slashAmount
		}
		if sharesToUnbond.GT(delegation.Shares) {
			sharesToUnbond = delegation.Shares
		}
		tokensToBurn, err := k.unbond(ctx, redelegation.DelegatorAddr, redelegation.ValidatorDstAddr, sharesToUnbond)
		if err != nil {
			panic(fmt.Errorf("error unbonding delegator: %v", err))
		}
		
		// Burn loose tokens
		pool := k.GetPool(ctx)
		pool.LooseTokens = pool.LooseTokens.Sub(tokensToBurn)
		k.SetPool(ctx, pool)
	}
	
	return slashAmount
}