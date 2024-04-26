package client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/EspressoSystems/espresso-sequencer-go/types"
)

// Interface to the Espresso Sequencer query service.
type QueryService interface {
	// Get the latest block number
	FetchLatestBlockHeight(ctx context.Context) (uint64, error)
	// Get the header for block number `height`.
	FetchHeaderByHeight(ctx context.Context, height uint64) (types.Header, error)
	// Get the headers starting from the given :from up until the given :until
	FetchHeadersByRange(ctx context.Context, from uint64, until uint64) ([]types.Header, error)
	// Get the transactions belonging to the given namespace in the block with the given header,
	// along with a proof that these are all such transactions.
	FetchTransactionsInBlock(ctx context.Context, header *types.Header, namespace uint64) (TransactionsInBlock, error)
}

// Response to `FetchTransactionsInBlock`
type TransactionsInBlock struct {
	// The transactions.
	Transactions []types.Bytes `json:"transactions"`
	// A proof that these are all the transactions in the block with the requested namespace.
	Proof types.NamespaceProof `json:"proof"`
}

func (t *TransactionsInBlock) UnmarshalJSON(b []byte) error {
	// Parse using pointers so we can distinguish between missing and default fields.
	type Dec struct {
		Transactions *[]types.Bytes        `json:"transactions"`
		Proof        *types.NamespaceProof `json:"proof"`
	}

	var dec Dec
	if err := json.Unmarshal(b, &dec); err != nil {
		return err
	}

	if dec.Transactions == nil {
		return fmt.Errorf("Field transactions of type TransactionsInBlock is required")
	}
	t.Transactions = *dec.Transactions

	if dec.Proof == nil {
		return fmt.Errorf("Field proof of type TransactionsInBlock is required")
	}
	t.Proof = *dec.Proof

	return nil
}
