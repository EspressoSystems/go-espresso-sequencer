package client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/EspressoSystems/espresso-sequencer-go/types"
)

// Interface to the Espresso Sequencer query service.
type QueryService interface {
	FetchHeader(ctx context.Context, height uint64) (types.Header, error)
	// Get all the available headers whose timestamps fall in the window [start, end).
	FetchHeadersForWindow(ctx context.Context, start uint64, end uint64) (WindowStart, error)
	// Get all the available headers starting with the block numbered `from` whose timestamps are
	// less than `end`. This can be used to continue fetching headers in a time window if not all
	// headers in the window were available when `FetchHeadersForWindow` was called.
	FetchRemainingHeadersForWindow(ctx context.Context, from uint64, end uint64) (WindowMore, error)
	// Get the transactions belonging to the given namespace in the block with the given header,
	// along with a proof that these are all such transactions.
	FetchTransactionsInBlock(ctx context.Context, header *types.Header, namespace uint64) (TransactionsInBlock, error)
}

// Response to `FetchHeadersForWindow`.
type WindowStart struct {
	// The available block headers in the requested window.
	Window []types.Header `json:"window"`
	// The header of the last block before the start of the window. This proves that the query
	// service did not omit any blocks from the beginning of the window. This will be `nil` if
	// `From` is 0.
	Prev *types.Header `json:"prev"`
	// The first block after the end of the window. This proves that the query service did not omit
	// any blocks from the end of the window. This will be `nil` if the full window is not available
	// yet, in which case `FetchRemainingHeadersForWindow` should be called to retrieve the rest of
	// the window.
	Next *types.Header `json:"next"`
}

func (w *WindowStart) UnmarshalJSON(b []byte) error {
	// Parse using pointers so we can distinguish between missing and default fields.
	type Dec struct {
		Window *[]types.Header `json:"window"`
		Prev   *types.Header   `json:"prev"`
		Next   *types.Header   `json:"next"`
	}

	var dec Dec
	if err := json.Unmarshal(b, &dec); err != nil {
		return err
	}

	if dec.Window == nil {
		return fmt.Errorf("Field window of type WindowStart is required")
	}
	w.Window = *dec.Window

	w.Prev = dec.Prev
	w.Next = dec.Next
	return nil
}

// Response to `FetchRemainingHeadersForWindow`.
type WindowMore struct {
	// The additional blocks within the window which are available, if any.
	Window []types.Header `json:"window"`
	// The first block after the end of the window, if the full window is available.
	Next *types.Header `json:"next"`
}

func (w *WindowMore) UnmarshalJSON(b []byte) error {
	// Parse using pointers so we can distinguish between missing and default fields.
	type Dec struct {
		Window *[]types.Header `json:"window"`
		Next   *types.Header   `json:"next"`
	}

	var dec Dec
	if err := json.Unmarshal(b, &dec); err != nil {
		return err
	}

	if dec.Window == nil {
		return fmt.Errorf("Field window of type WindowMore is required")
	}
	w.Window = *dec.Window

	w.Next = dec.Next
	return nil
}

// Response to `FetchTransactionsInBlock`
type TransactionsInBlock struct {
	// The transactions.
	Transactions []types.Bytes `json:"transactions"`
	// A proof that these are all the transactions in the block with the requested namespace.
	Proof types.NmtProof `json:"proof"`
}

func (t *TransactionsInBlock) UnmarshalJSON(b []byte) error {
	// Parse using pointers so we can distinguish between missing and default fields.
	type Dec struct {
		Transactions *[]types.Bytes  `json:"transactions"`
		Proof        *types.NmtProof `json:"proof"`
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
